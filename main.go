package main

import (
	"flag"
	"log"
	"net"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

var tunName = flag.String("tun", "", "tun interface name")
var proxyAddr = flag.String("proxy", "", "socks5 proxy server address")
var connectTimeout = flag.Duration("connect-timeout", 15*time.Second, "connect timeout")
var kaInterval = flag.Duration("keepalive-interval", 15*time.Second, "keepalive interval")
var kaCount = flag.Int("keepalive-Count", 2, "keepalive count")

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

func handleTcp(dstAddr *net.TCPAddr, newConn func(time.Duration, int) (net.Conn, error), resetConn func()) {

	deadline := time.Now().Add(*connectTimeout)

	dstConn, err := net.DialTimeout("tcp", *proxyAddr, *connectTimeout)
	if err != nil {
		resetConn()
		return
	}
	defer dstConn.Close()

	dstConn.SetDeadline(deadline)
	if err := SocksConnect(dstConn, dstAddr.IP, dstAddr.Port); err != nil {
		resetConn()
		return
	}
	dstConn.SetDeadline(time.Time{})

	dstTcp := dstConn.(*net.TCPConn)

	dstTcp.SetKeepAlive(true)
	dstTcp.SetKeepAlivePeriod(*kaInterval)

	dstRaw, err := dstTcp.SyscallConn()
	if err != nil {
		return
	}

	// Configures the connection to time out after peer has been idle for a
	// while, that is it has not sent or acknowledged any data or not replied to
	// keep-alive probes.
	userTimeout := UserTimeoutFromKeepalive(*kaInterval, *kaCount)

	if err := dstRaw.Control(func(s_ uintptr) {
		s := int(s_)
		syscall.SetsockoptInt(s, syscall.SOL_TCP, unix.TCP_KEEPCNT, *kaCount)
		userTimeoutMillis := int(userTimeout / time.Millisecond)
		syscall.SetsockoptInt(s, syscall.SOL_TCP, unix.TCP_USER_TIMEOUT, userTimeoutMillis)
	}); err != nil {
		return
	}

	srcConn, err := newConn(*kaInterval, *kaCount)
	if err != nil {
		return
	}
	defer srcConn.Close()

	ConcatStream(srcConn, dstConn)
}

//from https://github.com/cloudflare/slirpnetstack
func UserTimeoutFromKeepalive(kaInterval time.Duration, kaCount int) time.Duration {
	// The idle timeout period is determined from the keep-alive probe interval
	// and the total number of probes to sent, that is
	//
	//   TCP_USER_TIMEOUT = TCP_KEEPIDLE + TCP_KEEPINTVL * TCP_KEEPCNT
	//
	// in Go, TCPConn.SetKeepAlivePeriod(d) sets the value for both TCP_KEEPIDLE
	// and TCP_KEEPINTVL
	//
	// More info: https://blog.cloudflare.com/when-tcp-sockets-refuse-to-die/
	//
	return kaInterval + (kaInterval * time.Duration(kaCount))
}
