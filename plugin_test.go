package proxy_server

import (
	"bytes"
	"reflect"
	"testing"
)

func TestGetPluginRequest(t *testing.T) {
	for name, c := range map[string]struct {
		data      []byte
		expectErr bool
		expect    *Request
	}{
		"readErr": {
			data:      []byte{0xff},
			expectErr: true,
		},
		"PushTaskRecv": {
			data: []byte{0x10, 0x01, 0, 0x4, 0, 0, 0, 1},
			expect: &Request{
				Typ:      PushTaskRecv,
				TaskData: []byte{0, 0, 0, 1},
			},
		},
		"PushTask": {
			data: []byte{0x10, 0x02, 0, 2, 3, 4},
			expect: &Request{
				Typ:      PushTask,
				TaskData: []byte{3, 4},
			},
		},
		"Exit": {
			data: []byte{0x10, 0x03, 0, 0},
			expect: &Request{
				Typ: Exit,
			},
		},
	} {
		c := c
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got, err := GetPluginRequest(bytes.NewReader(c.data))
			if (err != nil) != c.expectErr {
				t.Errorf("expect error %v, but got %v", c.expectErr, err)
			}
			if !reflect.DeepEqual(c.expect, got) {
				t.Errorf("expect %#v, but got %#v", c.expect, got)
			}
		})
	}
}

func TestPutPluginRequest(t *testing.T) {
	for name, c := range map[string]struct {
		data   *Request
		err    error
		expect []byte
	}{
		"unknownType": {
			data: &Request{Typ: 0xdead},
			err:  unknownTypeErr,
		},
		"task": {
			data: &Request{
				Typ:      TaskResult,
				TaskData: []byte{1, 2, 3},
			},
			expect: []byte{0, pTaskResult, 0, 3, 1, 2, 3},
		},
		"TunnelReconnectFailed": {
			data: &Request{
				Typ: TunnelReconnectFailed,
			},
			expect: []byte{0, pTunnelReconnectFailed, 0, 0},
		},
		"TunnelConnectOk": {
			data: &Request{
				Typ: TunnelConnectOk,
			},
			expect: []byte{0, pTunnelConnectOk, 0, 0},
		},
	} {
		c := c
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var b bytes.Buffer
			err := PutPluginRequest(&b, c.data)
			if err != c.err {
				t.Errorf("expect error %v, but got %v", c.err, err)
			}

			if got := b.Bytes(); !bytes.Equal(got, c.expect) {
				t.Errorf("expect %v, but got %v", c.expect, got)
			}
		})
	}
}
