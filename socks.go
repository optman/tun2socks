package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net"
)

//sock5 protocol: https://datatracker.ietf.org/doc/html/rfc1928

func SocksConnect(conn io.ReadWriter, ip net.IP, port int) error {

	conn.Write([]byte{0x05, 0x01, 0x00})
	resp := make([]byte, 2)
	if _, err := io.ReadFull(conn, resp); err != nil {
		return fmt.Errorf("socks auth fail, %v", err)
	}

	var addrType byte
	var respLen int
	if len(ip) == net.IPv4len {
		addrType = 1
		respLen = 10
	} else {
		addrType = 4
		respLen = 22
	}

	buf := bytes.NewBuffer([]byte{0x05, 0x01, 0x00, addrType})
	binary.Write(buf, binary.BigEndian, ip)
	binary.Write(buf, binary.BigEndian, uint16(port))
	conn.Write(buf.Bytes())
	resp = make([]byte, respLen)
	if _, err := io.ReadFull(conn, resp); err != nil || resp[1] != 0 {
		return fmt.Errorf("socks connect fail, %v, code %d", err, resp[1])
	}

	return nil
}
