package main

import (
	"bytes"
	"reflect"
	"sync"
	"testing"
)

func TestWriteTLV(t *testing.T) {
	for name, c := range map[string]struct {
		data   TLV
		err    error
		expect []byte
	}{
		"lengthMismatch": {
			data:   TLV{T: 1, L: 2, V: make([]byte, 3)},
			err:    lengthMismatchErr,
			expect: []byte{},
		},
		"zeroLength": {
			data:   TLV{T: 1, L: 0, V: []byte{}},
			expect: []byte{0, 1, 0, 0},
		},
		"normal": {
			data:   TLV{T: 1, L: 2, V: []byte{3, 4}},
			expect: []byte{0, 1, 0, 2, 3, 4},
		},
	} {
		c := c
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			b := bytes.Buffer{}

			err := WriteTLV(&b, c.data)
			if err != c.err {
				t.Errorf("expect error %v, but got %v", c.err, err)
			}
			if got := b.Bytes(); !bytes.Equal(got, c.expect) {
				t.Errorf("expect result %v, but got %v", c.expect, got)
			}
		})
	}
}

func TestReadTLV(t *testing.T) {
	for name, c := range map[string]struct {
		data      []byte
		expectErr bool
		expect    TLV
	}{
		"readTypeexpectErr": {
			data:      []byte{1},
			expectErr: true,
		},
		"readLenexpectErr": {
			data:      []byte{1, 2, 3},
			expectErr: true,
		},
		"readValexpectErr": {
			data:      []byte{1, 2, 3, 4, 5},
			expectErr: true,
		},
		"zeroLength": {
			data:   []byte{0, 1, 0, 0},
			expect: TLV{T: 1, L: 0, V: []byte{}},
		},
		"normal": {
			data:   []byte{0, 1, 0, 2, 3, 4},
			expect: TLV{T: 1, L: 2, V: []byte{3, 4}},
		},
	} {
		c := c
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			b := bytes.NewBuffer(c.data)

			got, err := ReadTLV(b)
			if (err != nil) != c.expectErr {
				t.Errorf("expect error %v, but got %v", c.expectErr, err)
			}
			if !reflect.DeepEqual(got, c.expect) {
				t.Errorf("expect result %v, but got %v", c.expect, got)
			}
		})
	}
}

type concurrentBuffer struct {
	sync.Mutex
	bytes.Buffer
}

func (cb *concurrentBuffer) Write(p []byte) (int, error) {
	cb.Lock()
	defer cb.Unlock()

	return cb.Buffer.Write(p)
}

func TestConcurrentWriteTLV(t *testing.T) {
	writers := []struct {
		tlv    TLV
		result []byte
	}{
		{
			tlv:    TLV{T: 1, L: 2, V: []byte{3, 4}},
			result: []byte{0, 1, 0, 2, 3, 4},
		},
		{
			tlv:    TLV{T: 5, L: 6, V: []byte{7, 8, 9, 10, 11, 12}},
			result: []byte{0, 5, 0, 6, 7, 8, 9, 10, 11, 12},
		},
		{
			tlv:    TLV{T: 0xdead, L: 1, V: []byte{0xa}},
			result: []byte{0xde, 0xad, 0, 1, 0xa},
		},
	}

	var b concurrentBuffer
	waiter := sync.WaitGroup{}
	waiter.Add(len(writers))

	for _, w := range writers {
		w := w
		go func() {
			defer waiter.Done()

			for i := 0; i < 1000; i++ {
				err := WriteTLV(&b, w.tlv)
				if err != nil {
					t.Logf("write tlv error: %s", err)
					return
				}
			}
		}()
	}

	waiter.Wait()

	for _, w := range writers {
		if got := bytes.Count(b.Bytes(), w.result); got != 1000 {
			t.Errorf("expect tlv[%v] occur 1000 times, but got %d times", w.tlv, got)
		}
	}
}
