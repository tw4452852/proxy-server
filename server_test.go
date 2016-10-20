package main

import (
	"context"
	"net"
	"reflect"
	"testing"
	"time"
)

func TestNewServer(t *testing.T) {
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	sa := l.Addr().String()

	for name, c := range map[string]struct {
		pluginAddr, controlAddr, dataAddr string
		err                               error
	}{
		"pluginAddrNil": {
			controlAddr: sa,
			dataAddr:    sa,
		},
		"controlAddrNil": {
			pluginAddr: sa,
			dataAddr:   sa,
		},
		"bothNil": {
			dataAddr: sa,
		},
		"allNil": {},
		"pluginErr": {
			pluginAddr:  "127.0.0.1:1",
			controlAddr: sa,
			dataAddr:    sa,
			err:         setupPluginErr,
		},
		"tunnelErr": {
			pluginAddr:  sa,
			controlAddr: "127.0.0.1:1",
			dataAddr:    sa,
			err:         setupTunnelErr,
		},
	} {
		c := c
		t.Run(name, func(t *testing.T) {
			s, err := NewServer(c.pluginAddr, c.controlAddr, c.dataAddr)
			if c.err != err {
				t.Errorf("expect error %v, but got %v", c.err, err)
			}
			if s != nil {
				defer s.cancel()
			}
		})
	}
}

func TestPollPlugin(t *testing.T) {
	s, err := NewServer("", "", "")
	if err != nil {
		t.Fatal(err)
	}
	defer s.cancel()

	s.pluginCtx, s.pluginCancel = context.WithCancel(s.ctx)

	r, w := net.Pipe()
	s.pluginConn = r
	ret := make(chan struct{})
	defer close(ret)

	go func() {
		s.pollPlugin()
		ret <- struct{}{}
	}()

	// mock a PushTask request
	err = WriteTLV(w, TLV{T: pPushTask, L: 1, V: []byte{2}})
	if err != nil {
		t.Fatal(err)
	}
	expect := &Request{
		Typ:      PushTask,
		TaskData: []byte{2},
	}
	if got := <-s.reqs; !reflect.DeepEqual(got, expect) {
		t.Fatalf("expect request %#v, but got %#v", expect, got)
	}

	// mock a failure
	err = WriteTLV(w, TLV{T: 0xff, L: 1, V: []byte{2}})
	if err != nil {
		t.Fatal(err)
	}
	<-s.pluginErr
	<-ret

	// mock done
	go func() {
		s.pollPlugin()
		ret <- struct{}{}
	}()
	s.cancel()
	<-ret
}

func TestPollTunnel(t *testing.T) {
	s, err := NewServer("", "", "")
	if err != nil {
		t.Fatal(err)
	}
	defer s.cancel()

	s.tunnelCtx, s.tunnelCancel = context.WithCancel(s.ctx)

	r, w := net.Pipe()
	s.tunnelConn = r
	ret := make(chan struct{})
	defer close(ret)

	go func() {
		s.pollTunnel()
		ret <- struct{}{}
	}()

	// mock a TaskResult request
	err = WriteTLV(w, TLV{T: tTask, L: 1, V: []byte{2}})
	if err != nil {
		t.Fatal(err)
	}
	expect := &Request{
		Typ:      TaskResult,
		TaskData: []byte{2},
	}
	if got := <-s.reqs; !reflect.DeepEqual(got, expect) {
		t.Fatalf("expect request %#v, but got %#v", expect, got)
	}
	if gotT := s.lastRecvTime.Load(); gotT == nil {
		t.Fatal("timestamp doesn't update")
	}

	// mock a failure
	err = WriteTLV(w, TLV{T: 0xff, L: 1, V: []byte{2}})
	if err != nil {
		t.Fatal(err)
	}
	<-s.tunnelErr
	<-ret

	// mock done
	go func() {
		s.pollTunnel()
		ret <- struct{}{}
	}()
	s.cancel()
	<-ret
}

func TestCheckTunnel(t *testing.T) {
	s, err := NewServer("", "", "")
	if err != nil {
		t.Fatal(err)
	}
	defer s.cancel()

	r, w := net.Pipe()
	s.tunnelConn = r
	ret := make(chan struct{})
	defer close(ret)

	s.tunnelCtx, s.tunnelCancel = context.WithCancel(s.ctx)
	const expectCount = 5
	expectReq := &Request{Typ: Ping}
	checkInterval = 1 * time.Millisecond
	checkTimeout = (expectCount + 1) * checkInterval

	go func() {
		s.checkTunnel()
		ret <- struct{}{}
	}()

	for i := 0; i < expectCount; i++ {
		req, err := GetCtrRequest(w)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(req, expectReq) {
			t.Fatalf("%dth request not expected: got %#v, expect %#v",
				i, req, expectReq)
		}
	}

	if err = <-s.tunnelErr; err != tunnelTimeoutErr {
		t.Fatalf("expect timeout, but got %v", err)
	}
	<-ret

	// mock done
	go func() {
		s.checkTunnel()
		ret <- struct{}{}
	}()

	s.tunnelCancel()
	<-ret
}

func TestHandleTunnelErr(t *testing.T) {
	s, err := NewServer("", "", "")
	if err != nil {
		t.Fatal(err)
	}
	defer s.cancel()

	// mock a successful reconnection
	err = s.handleTunnelErr(tunnelTimeoutErr)
	if err != nil {
		t.Fatalf("got unexpected error: %v", err)
	}

	// mock a failed reconnection
	r, w := net.Pipe()
	s.pluginConn = w
	s.tunnelAddr = "127.0.0.1:1"
	err = s.handleTunnelErr(tunnelTimeoutErr)
	if err == nil {
		t.Fatal("not get expected error")
	}
	expect := TLV{T: pTunnelReconnectFailed, L: 0, V: []byte{}}
	got, err := ReadTLV(r)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, expect) {
		t.Fatalf("expect %#v, but got %#v", expect, got)
	}
}
