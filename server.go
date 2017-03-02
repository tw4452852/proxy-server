package proxy_server

import (
	"context"
	"errors"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

type srv struct {
	dataAddr string
	reqs     chan *Request
	ctx      context.Context
	cancel   context.CancelFunc

	tunnelAddr   string
	tunnelConn   net.Conn
	tunnelErr    chan error
	tunnelCtx    context.Context
	tunnelCancel context.CancelFunc
	tunnelWaiter sync.WaitGroup
	lastRecvTime atomic.Value

	pluginAddr   string
	pluginConn   net.Conn
	pluginErr    chan error
	pluginCtx    context.Context
	pluginCancel context.CancelFunc
	pluginWaiter sync.WaitGroup
}

var (
	checkInterval = 1 * time.Second
	checkTimeout  = 30 * time.Second
	pollTimeout   = 1 * time.Second

	tunnelTimeoutErr = errors.New("tunnel ping timeout")
	setupPluginErr   = errors.New("setup plugin failed")
	setupTunnelErr   = errors.New("setup tunnel failed")
	pluginExitErr    = errors.New("to be killed")
)

func NewServer(pluginAddr, controlAddr, dataAddr string) (*srv, error) {
	Debug.Printf("[server]: addresses: plugin[%s], control[%s], data[%s]\n",
		pluginAddr, controlAddr, dataAddr)

	ctx, cancel := context.WithCancel(context.Background())

	s := &srv{
		ctx:        ctx,
		cancel:     cancel,
		dataAddr:   dataAddr,
		tunnelAddr: controlAddr,
		pluginAddr: pluginAddr,
		tunnelErr:  make(chan error, 1),
		pluginErr:  make(chan error, 1),
		reqs:       make(chan *Request, 16),
	}

	err := s.setupPlugin()
	if err != nil {
		log.Printf("[server]: setup plugin failed: %s\n", err)
		return nil, setupPluginErr
	}

	// just queue a fake error for tunnel setup then
	s.tunnelErr <- nil

	return s, nil
}

func (s *srv) setupPlugin() error {
	addr := s.pluginAddr
	if addr == "" {
		Debug.Println("[server]: plugin address is nil, exit")
		return nil
	}

	// terminate old one
	if s.pluginCancel != nil {
		s.pluginCancel()
		s.pluginWaiter.Wait()
	}

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(s.ctx)
	s.pluginConn = conn
	s.pluginCtx = ctx
	s.pluginCancel = cancel
	s.pluginWaiter.Add(1)

	go s.pollPlugin()

	return nil
}

func (s *srv) pollPlugin() {
	defer func() {
		s.pluginConn.Close()
		s.pluginWaiter.Done()
		Debug.Println("[server]: plugin poller exits")
	}()

	for {
		select {
		case <-s.pluginCtx.Done():
			return
		default:
			req, err := s.getPluginRequest()
			if err != nil {
				s.pluginErr <- err
				return
			}
			if req != nil {
				s.reqs <- req
			}
		}
	}
}

func (s *srv) setupTunnel() error {
	addr := s.tunnelAddr
	if addr == "" {
		Debug.Println("[server]: tunnel address is nil, exit")
		return nil
	}

	// terminate old one
	if s.tunnelCancel != nil {
		s.tunnelCancel()
		s.tunnelWaiter.Wait()
	}

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(s.ctx)
	s.tunnelConn = conn
	s.tunnelCtx = ctx
	s.tunnelCancel = cancel
	s.tunnelWaiter.Add(2)

	go s.pollTunnel()
	go s.checkTunnel()

	return nil
}

func (s *srv) pollTunnel() {
	defer func() {
		s.tunnelConn.Close()
		s.tunnelWaiter.Done()
		Debug.Println("[server]: tunnel poller exits")
	}()

	s.reqs <- &Request{Typ: TunnelConnectOk}

	for {
		select {
		case <-s.tunnelCtx.Done():
			return
		default:
			req, err := s.getCtrRequest()
			if err != nil {
				s.tunnelErr <- err
				return
			}
			if req != nil {
				// update receive timestamp
				s.lastRecvTime.Store(time.Now())

				s.reqs <- req
			}
		}
	}
}

func (s *srv) checkTunnel() {
	t := time.NewTicker(checkInterval)
	defer func() {
		t.Stop()
		s.tunnelWaiter.Done()
		Debug.Println("[server]: tunnel checker exit")
	}()

	// set current time at first
	s.lastRecvTime.Store(time.Now())

	for {
		select {
		case <-s.tunnelCtx.Done():
			return
		case cur := <-t.C:
			// check timeout first
			last := s.lastRecvTime.Load().(time.Time)
			if cur.After(last.Add(checkTimeout)) {
				s.tunnelErr <- tunnelTimeoutErr
				return
			}
			s.putCtrRequest(&Request{Typ: Ping})
		}
	}
}

func (s *srv) Loop() error {
	for {
		select {
		case <-s.ctx.Done():
			return nil
		case req := <-s.reqs:
			s.handleRequest(req)
		case err := <-s.tunnelErr:
			s.handleTunnelErr(err)
		case err := <-s.pluginErr:
			err = s.handlePluginErr(err)
			if err != nil {
				return err
			}
		}
	}
}

func (s *srv) handleTunnelErr(err error) error {
	log.Printf("[server]: A error happens on control link: %s\n", err)

	var reconnectErr error
	// try to reconnect
	for i := 0; i < 3; i++ {
		reconnectErr = s.setupTunnel()
		if reconnectErr == nil {
			return nil
		}
	}

	log.Printf("[server]: reconnect failure: %s\n", reconnectErr)
	go func() {
		s.reqs <- &Request{Typ: TunnelReconnectFailed}
	}()
	return reconnectErr
}

func (s *srv) handlePluginErr(err error) error {
	log.Printf("[server]: A error happens on plugin link: %s\n", err)
	return err
}

type RequestType int

const (
	CreateSSConnect RequestType = iota
	PushTaskRecv
	PushTask
	TaskResult
	TunnelReconnectFailed
	Ping
	TunnelConnectOk
	Exit

	TypeEnd
)

type Request struct {
	Typ       RequestType
	SocketKey string
	TaskData  []byte
}

func (s *srv) handleRequest(req *Request) error {
	Debug.Printf("[server]: handle request [%#v]\n", req)
	switch req.Typ {
	case CreateSSConnect:
		go HandleSSConnectRequest(s.dataAddr, req.SocketKey)
	case PushTaskRecv:
		go s.putCtrRequest(req)
	case PushTask:
		go s.putCtrRequest(req)
	case TaskResult:
		go s.putPluginRequest(req)
	case TunnelReconnectFailed:
		go s.putPluginRequest(req)
	case Ping:
		// ping ack, do nothing
		Debug.Println("[server]: recv ping ack")
	case TunnelConnectOk:
		go s.putPluginRequest(req)
	case Exit:
		go func() {
			s.pluginErr <- pluginExitErr
		}()
	default:
		log.Printf("[server]: unknown request type[%x]\n", req.Typ)
		return unknownTypeErr
	}
	return nil
}

// helpers
func (s *srv) putCtrRequest(req *Request) error {
	if s.tunnelConn == nil {
		Debug.Printf("[server]: tunnel connection is nil, skip this request[%#v]\n", req)
		return nil
	}
	return PutCtrRequest(s.tunnelConn, req)
}

func (s *srv) putPluginRequest(req *Request) error {
	if s.pluginConn == nil {
		Debug.Printf("[server]: plugin connection is nil, skip this request[%#v]\n", req)
		return nil
	}
	return PutPluginRequest(s.pluginConn, req)
}

func (s *srv) getPluginRequest() (*Request, error) {
	if s.pluginConn == nil {
		Debug.Printf("[server]: plugin connection is nil, skip request get")
		return nil, nil
	}
	err := s.pluginConn.SetReadDeadline(time.Now().Add(pollTimeout))
	if err != nil {
		log.Printf("[server]: set plugin read deadline error: %v\n", err)
	}
	r, err := GetPluginRequest(s.pluginConn)
	if err != nil {
		ne, ok := err.(net.Error)
		if ok && ne.Temporary() {
			return nil, nil
		}
	}
	return r, err
}

func (s *srv) getCtrRequest() (*Request, error) {
	if s.tunnelConn == nil {
		Debug.Printf("[server]: tunnel connection is nil, skip request get")
		return nil, nil
	}
	err := s.tunnelConn.SetReadDeadline(time.Now().Add(pollTimeout))
	if err != nil {
		log.Printf("[server]: set tunnel read deadline error: %v\n", err)
	}
	r, err := GetCtrRequest(s.tunnelConn)
	if err != nil {
		ne, ok := err.(net.Error)
		if ok && ne.Temporary() {
			return nil, nil
		}
	}
	return r, err
}
