package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"log"
)

type TLV struct {
	T uint16
	L uint16
	V []byte
}

var lengthMismatchErr = errors.New("length is mismatch")

func WriteTLV(w io.Writer, tlv TLV) error {
	Debug.Printf("[tlv]: write %#v\n", tlv)
	// TODO: use buffer pool
	var b bytes.Buffer

	if int(tlv.L) != binary.Size(tlv.V) {
		log.Printf("[tlv]: length mismatch expect[%d], but got[%d]\n",
			binary.Size(tlv.V), int(tlv.L))
		return lengthMismatchErr
	}

	err := binary.Write(&b, binary.BigEndian, tlv.T)
	if err != nil {
		log.Printf("[tlv]: write type[%#x] error: %s\n", tlv.T, err)
		return err
	}

	err = binary.Write(&b, binary.BigEndian, tlv.L)
	if err != nil {
		log.Printf("[tlv]: write length[%#x] error: %s\n", tlv.L, err)
		return err
	}

	err = binary.Write(&b, binary.BigEndian, tlv.V)
	if err != nil {
		log.Printf("[tlv]: write value[%v] error: %s\n", tlv.V, err)
		return err
	}

	err = binary.Write(w, binary.BigEndian, b.Bytes())
	if err != nil {
		log.Printf("[tlv]: write tlv[%v] error: %s\n", b.Bytes(), err)
		return err
	}

	return nil
}

var (
	readTypeErr = errors.New("read type error")
	readLenErr  = errors.New("read length error")
	readValErr  = errors.New("read value error")
)

func ReadTLV(r io.Reader) (tlv TLV, err error) {
	var (
		t uint16
		l uint16
		v []byte
	)

	err = binary.Read(r, binary.BigEndian, &t)
	if err != nil {
		log.Printf("[tlv]: read type error: %s\n", err)
		err = readTypeErr
		return
	}

	err = binary.Read(r, binary.BigEndian, &l)
	if err != nil {
		log.Printf("[tlv]: read length error: %s\n", err)
		err = readLenErr
		return
	}

	v = make([]byte, l)
	err = binary.Read(r, binary.BigEndian, &v)
	if err != nil {
		log.Printf("[tlv]: read value error: %s\n", err)
		err = readValErr
		return
	}

	tlv = TLV{
		T: t,
		L: l,
		V: v,
	}
	Debug.Printf("[tlv]: read %#v\n", tlv)

	return tlv, nil
}
