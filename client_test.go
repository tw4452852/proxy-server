package main

import (
	"encoding/binary"
	"fmt"
	"net"
	"reflect"
	"strconv"
	"testing"

	ss "github.com/shadowsocks/shadowsocks-go/shadowsocks"
)

func TestGetRequest(t *testing.T) {
	r, w := net.Pipe()
	exit := make(chan struct{})
	defer close(exit)
	inputC := make(chan []byte)
	go func() {
		for {
			select {
			case b := <-inputC:
				w.Write(b)
			case <-exit:
				return
			}
		}
	}()

	translateIpv4 := func(addr string) ([]byte, error) {
		host, portStr, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, fmt.Errorf("address error %s %v", addr, err)
		}
		port, err := strconv.Atoi(portStr)
		if err != nil {
			return nil, fmt.Errorf("invalid port %s", addr)
		}

		l := 1 + net.IPv4len + 2 // addrType + ipv4 + port
		buf := make([]byte, l)
		buf[0] = typeIPv4
		copy(buf[1:], net.ParseIP(host).To4())
		binary.BigEndian.PutUint16(buf[1+net.IPv4len:l], uint16(port))
		return buf, nil
	}

	translateIpv6 := func(addr string) ([]byte, error) {
		host, portStr, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, fmt.Errorf("address error %s %v", addr, err)
		}
		port, err := strconv.Atoi(portStr)
		if err != nil {
			return nil, fmt.Errorf("invalid port %s", addr)
		}

		l := 1 + net.IPv6len + 2 // addrType + ipv4 + port
		buf := make([]byte, l)
		buf[0] = typeIPv6
		copy(buf[1:], net.ParseIP(host).To16())
		binary.BigEndian.PutUint16(buf[1+net.IPv6len:l], uint16(port))
		return buf, nil
	}

	for name, c := range map[string]struct {
		input  func() ([]byte, error)
		expect *request
	}{
		"dm": {
			input: func() ([]byte, error) {
				return ss.RawAddr("www.test.com:1311")
			},
			expect: &request{"www.test.com:1311"},
		},
		"ipv4": {
			input: func() ([]byte, error) {
				return translateIpv4("1.1.1.1:1311")
			},
			expect: &request{"1.1.1.1:1311"},
		},
		"ipv6": {
			input: func() ([]byte, error) {
				return translateIpv6("[fe80::6e0b:84ff:fe6a:5aa9]:1311")
			},
			expect: &request{"[fe80::6e0b:84ff:fe6a:5aa9]:1311"},
		},
	} {
		c := c
		t.Run(name, func(t *testing.T) {
			b, err := c.input()
			if err != nil {
				t.Fatal(err)
			}
			inputC <- b
			got, err := getRequest(r)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, c.expect) {
				t.Errorf("expect %v, but got %v", c.expect, got)
			}
		})
	}
}
