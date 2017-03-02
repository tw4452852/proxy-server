package proxy_server

import (
	"io"
	"log"
)

const (
	tCreateSSConnect = 1
	tTaskRecv        = 2
	tTask            = 3
	tPing            = 4
)

func GetCtrRequest(r io.Reader) (*Request, error) {
	tlv, err := ReadTLV(r)
	if err != nil {
		log.Printf("[tunnel]: read control request failed: %s\n", err)
		return nil, err
	}

	switch tlv.T {
	case tCreateSSConnect:
		return &Request{
			Typ:       CreateSSConnect,
			SocketKey: string(tlv.V),
		}, nil
	case tTask:
		return &Request{
			Typ:      TaskResult,
			TaskData: tlv.V,
		}, nil
	case tPing:
		return &Request{
			Typ: Ping,
		}, nil
	default:
		log.Printf("[tunnel]: unknown command type[%#x]\n", tlv.T)
		return nil, unknownTypeErr
	}
}

func PutCtrRequest(w io.Writer, req *Request) error {
	var tlv TLV
	switch req.Typ {
	case PushTaskRecv:
		tlv.T = tTaskRecv
		tlv.V = req.TaskData
	case PushTask:
		tlv.T = tTask
		tlv.V = req.TaskData
	case Ping:
		tlv.T = tPing
	default:
		log.Printf("[tunnel]: unknown type[%#x]\n", req.Typ)
		return unknownTypeErr
	}
	tlv.L = uint16(len(tlv.V))

	err := WriteTLV(w, tlv)
	if err != nil {
		log.Printf("[tunnel]: write control request[%#v] failed: %s\n",
			req, err)
	}
	return err
}
