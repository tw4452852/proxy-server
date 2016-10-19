package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"strconv"

	ss "github.com/shadowsocks/shadowsocks-go/shadowsocks"
)

func clientLoop(addr string) error {
	if debug {
		log.Printf("[client]: address %q\n", addr)
	}

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return err
	}
	defer conn.Close()

	for {
		req, err := getRequest(conn)
		if err != nil {
			log.Printf("[client]: get request failed: %s\n", err)
			continue
		}
		go handleRequest(req)
	}
}

type request struct {
	addr string
}

func (r *request) String() string {
	return fmt.Sprintf("%s", r.addr)
}

const (
	idType  = 0 // address type index
	idIP0   = 1 // ip addres start index
	idDmLen = 1 // domain address length index
	idDm0   = 2 // domain address start index

	typeIPv4 = 1 // type is ipv4 address
	typeDm   = 3 // type is domain address
	typeIPv6 = 4 // type is ipv6 address

	lenIPv4     = net.IPv4len + 2 // ipv4 + 2port
	lenIPv6     = net.IPv6len + 2 // ipv6 + 2port
	lenDmBase   = 2               // 1addrLen + 2port, plus addrLen
	lenHmacSha1 = 10
)

func getRequest(conn net.Conn) (*request, error) {
	// buf size should at least have the same size with the largest possible
	// request size (when addrType is 3, domain name has at most 256 bytes)
	// 1(addrType) + 1(lenByte) + 256(max length address) + 2(port) + 10(hmac-sha1)
	// TODO: buffer pool
	buf := make([]byte, 270)
	// read till we get possible domain length field
	if _, err := io.ReadFull(conn, buf[:idType+1]); err != nil {
		return nil, fmt.Errorf("read idType error %s", err)
	}

	var reqStart, reqEnd int
	addrType := buf[idType]
	switch addrType & ss.AddrMask {
	case typeIPv4:
		reqStart, reqEnd = idIP0, idIP0+lenIPv4
	case typeIPv6:
		reqStart, reqEnd = idIP0, idIP0+lenIPv6
	case typeDm:
		if _, err := io.ReadFull(conn, buf[idType+1:idDmLen+1]); err != nil {
			return nil, fmt.Errorf("read idDm error %s", err)
		}
		reqStart, reqEnd = idDm0, int(idDm0+buf[idDmLen]+lenDmBase)
	default:
		return nil, fmt.Errorf("addr type %d not supported", addrType&ss.AddrMask)
	}

	if _, err := io.ReadFull(conn, buf[reqStart:reqEnd]); err != nil {
		return nil, fmt.Errorf("read address error %s", err)
	}

	var host string
	// Return string for typeIP is not most efficient, but browsers (Chrome,
	// Safari, Firefox) all seems using typeDm exclusively. So this is not a
	// big problem.
	switch addrType & ss.AddrMask {
	case typeIPv4:
		host = net.IP(buf[idIP0 : idIP0+net.IPv4len]).String()
	case typeIPv6:
		host = net.IP(buf[idIP0 : idIP0+net.IPv6len]).String()
	case typeDm:
		host = string(buf[idDm0 : idDm0+buf[idDmLen]])
	}
	// parse port
	port := binary.BigEndian.Uint16(buf[reqEnd-2 : reqEnd])
	host = net.JoinHostPort(host, strconv.Itoa(int(port)))

	return &request{host}, nil
}

func handleRequest(req *request) {

}
