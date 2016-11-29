package main

import (
	"bytes"
	"reflect"
	"testing"
)

func TestGetCtrRequest(t *testing.T) {
	for name, c := range map[string]struct {
		data      []byte
		expectErr bool
		expect    *Request
	}{
		"readErr": {
			data:      []byte{0},
			expectErr: true,
		},
		"unknownType": {
			data:      []byte{0x99, 0x99, 0, 1, 2},
			expectErr: true,
		},
		"CreateSSConnect": {
			data: []byte{0, 1, 0, 2, 0x74, 0x77},
			expect: &Request{
				Typ:       CreateSSConnect,
				SocketKey: "tw",
			},
		},
		"TaskResult": {
			data: []byte{0, 3, 0, 2, 0x74, 0x77},
			expect: &Request{
				Typ:      TaskResult,
				TaskData: []byte{0x74, 0x77},
			},
		},
		"Ping": {
			data: []byte{0, 4, 0, 0},
			expect: &Request{
				Typ: Ping,
			},
		},
	} {
		c := c
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got, err := GetCtrRequest(bytes.NewReader(c.data))
			if (err != nil) != c.expectErr {
				t.Errorf("expect error %v, but got %v", c.expectErr, err)
			}
			if !reflect.DeepEqual(c.expect, got) {
				t.Errorf("expect %#v, but got %#v", c.expect, got)
			}
		})
	}
}

func TestPutCtrRequest(t *testing.T) {
	for name, c := range map[string]struct {
		req    *Request
		err    error
		expect []byte
	}{
		"unknownType": {
			req: &Request{Typ: 0xdead},
			err: unknownTypeErr,
		},
		"PushTask": {
			req: &Request{
				Typ:      PushTask,
				TaskData: []byte{1, 2, 3},
			},
			expect: []byte{0, 3, 0, 3, 1, 2, 3},
		},
		"PushTaskRecv": {
			req: &Request{
				Typ:      PushTaskRecv,
				TaskData: []byte{1},
			},
			expect: []byte{0, 2, 0, 1, 1},
		},
		"Ping": {
			req: &Request{
				Typ: Ping,
			},
			expect: []byte{0, 4, 0, 0},
		},
	} {
		c := c
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var b bytes.Buffer
			err := PutCtrRequest(&b, c.req)
			if err != c.err {
				t.Errorf("expect error %v, but got %v", c.err, err)
			}
			if got := b.Bytes(); !bytes.Equal(c.expect, got) {
				t.Errorf("got %v, but expect %v", got, c.expect)
			}
		})
	}
}
