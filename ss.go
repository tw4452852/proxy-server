package proxy_server

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"strconv"

	ss "github.com/shadowsocks/shadowsocks-go/shadowsocks"
)

var (
	cipher *ss.Cipher
)

func init() {
	var err error
	cipher, err = ss.NewCipher("aes-128-cfb", "123")
	if err != nil {
		panic(err)
	}
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

func getSSRequest(conn *ss.Conn, auth bool) (host string, ota bool, err error) {
	// buf size should at least have the same size with the largest possible
	// request size (when addrType is 3, domain name has at most 256 bytes)
	// 1(addrType) + 1(lenByte) + 256(max length address) + 2(port) + 10(hmac-sha1)
	buf := make([]byte, 270)
	// read till we get possible domain length field
	if _, err = io.ReadFull(conn, buf[:idType+1]); err != nil {
		return
	}

	var reqStart, reqEnd int
	addrType := buf[idType]
	switch addrType & ss.AddrMask {
	case typeIPv4:
		reqStart, reqEnd = idIP0, idIP0+lenIPv4
	case typeIPv6:
		reqStart, reqEnd = idIP0, idIP0+lenIPv6
	case typeDm:
		if _, err = io.ReadFull(conn, buf[idType+1:idDmLen+1]); err != nil {
			return
		}
		reqStart, reqEnd = idDm0, int(idDm0+buf[idDmLen]+lenDmBase)
	default:
		err = fmt.Errorf("addr type %d not supported", addrType&ss.AddrMask)
		return
	}

	if _, err = io.ReadFull(conn, buf[reqStart:reqEnd]); err != nil {
		return
	}

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
	// if specified one time auth enabled, we should verify this
	if auth || addrType&ss.OneTimeAuthMask > 0 {
		ota = true
		if _, err = io.ReadFull(conn, buf[reqEnd:reqEnd+lenHmacSha1]); err != nil {
			return
		}
		iv := conn.GetIv()
		key := conn.GetKey()
		actualHmacSha1Buf := ss.HmacSha1(append(iv, key...), buf[:reqEnd])
		if !bytes.Equal(buf[reqEnd:reqEnd+lenHmacSha1], actualHmacSha1Buf) {
			err = fmt.Errorf("verify one time auth failed, iv=%v key=%v data=%v", iv, key, buf[:reqEnd])
			return
		}
	}
	return
}

func handleSSConnection(conn *ss.Conn, auth bool) {
	Debug.Printf("[ss]: new client %s->%s\n", conn.LocalAddr(), conn.RemoteAddr().String())
	closed := false
	closeConn := func(conn net.Conn) {
		if !closed {
			conn.Close()
		}
	}
	defer closeConn(conn)

	host, ota, err := getSSRequest(conn, auth)
	if err != nil {
		log.Printf("[ss]: error getting request %s->%s: %s\n", conn.LocalAddr(), conn.RemoteAddr(), err)
		return
	}

	Debug.Printf("[ss]: connecting %s\n", host)

	// TODO: support udp
	remote, err := net.Dial("tcp", host)
	if err != nil {
		log.Printf("[ss]: connect to %s error: %s\n", host, err)
		return
	}
	defer closeConn(remote)

	Debug.Printf("[ss]: piping local[%s]<->remote[%s] ota=%v connOta=%v\n", conn.LocalAddr(), host, ota, conn.IsOta())

	if ota {
		log.Println("[ss] ota not supported")
		return
	}

	go func() {
		PipeThenClose(conn, remote)
		Debug.Printf("[ss]: piping local[%s]->remote[%s] return\n",
			conn.LocalAddr(), host)
	}()
	PipeThenClose(remote, conn)
	Debug.Printf("[ss]: piping remote[%s]->local[%s] return\n",
		host, conn.LocalAddr())
}

func HandleSSConnectRequest(clientAddr, key string) {
	Debug.Printf("[ss]: handle ss connection request, clientAddr[%s], key[%s]\n",
		clientAddr, key)
	conn, err := makeSSTunnel(clientAddr, key)
	if err != nil {
		log.Printf("[ss]: make tunnel failed: %s\n", err)
		return
	}
	handleSSConnection(ss.NewConn(conn, cipher.Copy()), false)
}

var establishError = errors.New("establish tunnel failed")

func makeSSTunnel(clientAddr, key string) (net.Conn, error) {
	conn, err := net.Dial("tcp", clientAddr)
	if err != nil {
		return nil, err
	}

	if !establishTunnel(conn, key) {
		conn.Close()
		return nil, establishError
	}

	return conn, nil
}

func establishTunnel(conn net.Conn, reqAddr string) bool {
	d, err := json.Marshal(&struct {
		Addr string `json:"socketkey"`
	}{reqAddr})
	if err != nil {
		log.Printf("[ss]: marshal address[%s] failed: %s\n", reqAddr, err)
		return false
	}

	var b bytes.Buffer

	err = binary.Write(&b, binary.BigEndian, uint16(binary.Size(d)))
	if err != nil {
		log.Printf("[ss]: write marshaled key length failed: %s\n", err)
		return false
	}
	_, err = b.Write(d)
	if err != nil {
		log.Printf("[ss]: write marshaled key[%s] failed: %s\n", string(d), err)
		return false
	}

	_, err = conn.Write(b.Bytes())
	if err != nil {
		log.Printf("[ss]: write connection failed: %s\n", err)
		return false
	}

	var buf [3]byte
	n, err := conn.Read(buf[:])
	if err != nil {
		log.Printf("[ss]: read response failed: %s\n", err)
		return false
	}

	return string(buf[:n]) == "200"
}
