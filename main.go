package main

import (
	"flag"
	"log"
	"net"
)

var tunName = flag.String("tun", "", "tun interface name")
var proxyAddr = flag.String("proxy", "", "socks5 proxy server address")

func main() {
	flag.Parse()

	fd, mtu, err := NewTun(*tunName)
	if err != nil {
		log.Panic(err)
	}

	if err := NewNetstack(fd, mtu, handleTcp); err != nil {
		log.Panic(err)
	}

	select {}
}

func handleTcp(dstAddr *net.TCPAddr, newConn func() (net.Conn, error), resetConn func()) {
	dstConn, err := net.Dial("tcp", *proxyAddr)
	if err != nil {
		resetConn()
		return
	}
	defer dstConn.Close()

	if err := SocksConnect(dstConn, dstAddr.IP, dstAddr.Port); err != nil {
		resetConn()
		return
	}

	srcConn, err := newConn()
	if err != nil {
		return
	}
	defer srcConn.Close()

	ConcatStream(srcConn, dstConn)
}
