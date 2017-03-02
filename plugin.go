package proxy_server

import (
	"errors"
	"io"
	"log"
)

const (
	pPushTaskRecv          = 0x1001
	pPushTask              = 0x1002
	pExit                  = 0x1003
	pTaskResult            = 1
	pTunnelReconnectFailed = 2
	pTunnelConnectOk       = 3
)

var unknownTypeErr = errors.New("unknow type")

func GetPluginRequest(r io.Reader) (*Request, error) {
	tlv, err := ReadTLV(r)
	if err != nil {
		log.Printf("[plugin]: read request failed: %s\n", err)
		return nil, err
	}

	switch tlv.T {
	case pPushTaskRecv:
		return &Request{
			Typ:      PushTaskRecv,
			TaskData: tlv.V,
		}, nil
	case pPushTask:
		return &Request{
			Typ:      PushTask,
			TaskData: tlv.V,
		}, nil
	case pExit:
		return &Request{Typ: Exit}, nil
	default:
		log.Printf("[plugin]: unknow type[%#x]\n", tlv.T)
		return nil, unknownTypeErr
	}
}

func PutPluginRequest(w io.Writer, req *Request) error {
	var tlv TLV
	switch req.Typ {
	case TaskResult:
		tlv.T = pTaskResult
		tlv.V = req.TaskData
	case TunnelReconnectFailed:
		tlv.T = pTunnelReconnectFailed
		tlv.V = []byte{}
	case TunnelConnectOk:
		tlv.T = pTunnelConnectOk
		tlv.V = []byte{}
	default:
		log.Printf("[plugin]: unknown type[%#x]\n", req.Typ)
		return unknownTypeErr
	}
	tlv.L = uint16(len(tlv.V))

	err := WriteTLV(w, tlv)
	if err != nil {
		log.Printf("[plugin]: write plugin request[%#v] failed: %s\n",
			req, err)
	}
	return err
}
