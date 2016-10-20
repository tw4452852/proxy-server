package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strconv"
	"testing"

	ss "github.com/shadowsocks/shadowsocks-go/shadowsocks"
)

func TestGetSSRequest(t *testing.T) {
	r, w := net.Pipe()
	sr, sw := ss.NewConn(r, cipher.Copy()), ss.NewConn(w, cipher.Copy())
	exit := make(chan struct{})
	defer close(exit)
	inputC := make(chan []byte)
	go func() {
		for {
			select {
			case b := <-inputC:
				sw.Write(b)
			case <-exit:
				return
			}
		}
	}()

	for name, c := range map[string]struct {
		input     func() ([]byte, error)
		shouldErr bool
		host      string
		ota       bool
	}{
		"dm": {
			input: func() ([]byte, error) {
				return ss.RawAddr("www.test.com:1311")
			},
			host: "www.test.com:1311",
		},
		"ipv4": {
			input: func() ([]byte, error) {
				return translateIpv4("1.1.1.1:1311")
			},
			host: "1.1.1.1:1311",
		},
		"ipv6": {
			input: func() ([]byte, error) {
				return translateIpv6("[fe80::6e0b:84ff:fe6a:5aa9]:1311")
			},
			host: "[fe80::6e0b:84ff:fe6a:5aa9]:1311",
		},
	} {
		c := c
		t.Run(name, func(t *testing.T) {
			b, err := c.input()
			if err != nil {
				t.Fatal(err)
			}
			inputC <- b
			host, ota, err := getSSRequest(sr, false)
			if !c.shouldErr && err != nil {
				t.Errorf("got unexpected error: %s", err)
			}
			if c.shouldErr && err == nil {
				t.Errorf("not get the expected error")
			}
			if host != c.host {
				t.Errorf("expect host[%s], but got[%s]", c.host, host)
			}
			if ota != c.ota {
				t.Errorf("expect ota[%t], but got[%t]", c.ota, ota)
			}
		})
	}
}

func TestHandleSSConnection(t *testing.T) {
	for name, f := range map[string]func(*testing.T){
		"clientClose": testHandleSSConnectionClientClose,
		"serverClose": testHandleSSConnectionServerClose,
	} {
		t.Run(name, f)
	}
}

func testHandleSSConnectionClientClose(t *testing.T) {
	Debug = false

	exit := make(chan struct{})
	defer close(exit)

	serverAddr := make(chan string)
	go func() {
		l, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Log(err)
			return
		}
		defer l.Close()

		// inform
		serverAddr <- l.Addr().String()

		// Wait for a connection.
		conn, err := l.Accept()
		if err != nil {
			t.Log(err)
			return
		}

		// echo
		io.Copy(conn, conn)

		<-exit
	}()

	c1, c2 := net.Pipe()
	sc1, sc2 := ss.NewConn(c1, cipher.Copy()), ss.NewConn(c2, cipher.Copy())

	done := make(chan struct{})
	go func() {
		handleSSConnection(sc1, false)
		close(done)
	}()

	req, err := ss.RawAddr(<-serverAddr)
	if err != nil {
		t.Fatal(err)
	}

	// send request first
	_, err = sc2.Write(req)
	if err != nil {
		t.Fatal(err)
	}

	// send content
	const content = "hello"
	_, err = sc2.Write([]byte(content))
	if err != nil {
		t.Fatal(err)
	}

	// receive what we write
	buf := make([]byte, len(content))
	_, err = sc2.Read(buf)
	if err != nil {
		t.Fatal(err)
	}

	if got := string(buf); got != content {
		t.Errorf("got[%s] not our expected[%s]", got, content)
	}

	err = sc2.Close()
	if err != nil {
		t.Fatal(err)
	}

	<-done
}

func testHandleSSConnectionServerClose(t *testing.T) {
	Debug = false
	const content = "hello"

	exit := make(chan struct{})
	defer close(exit)

	serverAddr := make(chan string)
	go func() {
		l, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Log(err)
			return
		}
		defer l.Close()

		// inform
		serverAddr <- l.Addr().String()

		// Wait for a connection.
		conn, err := l.Accept()
		if err != nil {
			t.Log(err)
			return
		}

		// echo
		buf := make([]byte, len(content))
		_, err = conn.Read(buf)
		if err != nil {
			t.Log(err)
		}
		_, err = conn.Write(buf)
		if err != nil {
			t.Log(err)
		}

		conn.Close()
	}()

	c1, c2 := net.Pipe()
	sc1, sc2 := ss.NewConn(c1, cipher.Copy()), ss.NewConn(c2, cipher.Copy())

	done := make(chan struct{})
	go func() {
		handleSSConnection(sc1, false)
		close(done)
	}()

	req, err := ss.RawAddr(<-serverAddr)
	if err != nil {
		t.Fatal(err)
	}

	// send request first
	_, err = sc2.Write(req)
	if err != nil {
		t.Fatal(err)
	}

	// send content
	_, err = sc2.Write([]byte(content))
	if err != nil {
		t.Fatal(err)
	}

	// receive what we write
	buf := make([]byte, len(content))
	_, err = sc2.Read(buf)
	if err != nil {
		t.Fatal(err)
	}

	if got := string(buf); got != content {
		t.Errorf("got[%s] not our expected[%s]", got, content)
	}

	<-done
}

func translateIpv4(addr string) ([]byte, error) {
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

func translateIpv6(addr string) ([]byte, error) {
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

func TestEstablishTunnel(t *testing.T) {
	c1, c2 := net.Pipe()
	todo := make(chan func() bool)
	result := make(chan bool)
	exit := make(chan struct{})
	defer close(exit)
	go func() {
		for {
			select {
			case <-exit:
				return
			case f := <-todo:
				result <- f()
			}
		}
	}()

	const (
		key     = "0xdeadbeef"
		jsonKey = `{"socketkey":"` + key + `"}`
	)
	var (
		buf    [64]byte
		expect = append([]byte{0, byte(len(jsonKey))}, []byte(jsonKey)...)
	)

	// normal case
	todo <- func() bool {
		return establishTunnel(c2, key)
	}
	n, err := c1.Read(buf[:])
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(buf[:n], expect) {
		t.Errorf("expect [%v], but got [%v]", expect, buf[:n])
	}
	_, err = c1.Write([]byte("200"))
	if err != nil {
		t.Fatal(err)
	}
	if !<-result {
		t.Errorf("should be all right, but not")
	}

	// wrong ack
	todo <- func() bool {
		return establishTunnel(c2, key)
	}
	n, err = c1.Read(buf[:])
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(buf[:n], expect) {
		t.Errorf("expect [%v], but got [%v]", expect, buf[:n])
	}
	_, err = c1.Write([]byte("201"))
	if err != nil {
		t.Fatal(err)
	}
	if <-result {
		t.Errorf("should not be all right, but not")
	}

	// close connection
	todo <- func() bool {
		return establishTunnel(c2, key)
	}
	c1.Close()
	if <-result {
		t.Errorf("should not be all right, but not")
	}
}
