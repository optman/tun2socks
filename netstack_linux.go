package main

import (
	"fmt"
	"net"
	"strings"

	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"
	"gvisor.dev/gvisor/pkg/tcpip/link/fdbased"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv4"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv6"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
	"gvisor.dev/gvisor/pkg/tcpip/transport/tcp"
	"gvisor.dev/gvisor/pkg/waiter"
)

func NewNetstack(fd int, mtu uint32, tcpConnHandler func(*net.TCPAddr, func() (net.Conn, error), func())) error {

	linkEP, err := fdbased.New(&fdbased.Options{
		FDs: []int{fd},
		MTU: mtu,
	})
	if err != nil {
		return err
	}

	s := stack.New(stack.Options{
		NetworkProtocols:   []stack.NetworkProtocolFactory{ipv4.NewProtocol, ipv6.NewProtocol},
		TransportProtocols: []stack.TransportProtocolFactory{tcp.NewProtocol}})

	if err := s.CreateNIC(1, linkEP); err != nil {
		return fmt.Errorf("create nic fail %v", err)
	}

	s.SetNICForwarding(1, ipv4.ProtocolNumber, true)
	s.SetPromiscuousMode(1, true)
	s.SetSpoofing(1, true)

	subnet, _ := tcpip.NewSubnet(tcpip.Address(strings.Repeat("\x00", 4)),
		tcpip.AddressMask(strings.Repeat("\x00", 4)))
	subnet6, _ := tcpip.NewSubnet(tcpip.Address(strings.Repeat("\x00", 16)),
		tcpip.AddressMask(strings.Repeat("\x00", 16)))

	s.SetRouteTable([]tcpip.Route{
		{
			Destination: subnet,
			NIC:         1,
		},
		{
			Destination: subnet6,
			NIC:         1,
		},
	})

	tcpFwd := tcp.NewForwarder(s, 0, 256, func(r *tcp.ForwarderRequest) {
		id := r.ID()
		remoteAddr := &net.TCPAddr{IP: net.IP(id.LocalAddress), Port: int(id.LocalPort)}

		newConn := func() (net.Conn, error) {
			var wq waiter.Queue
			ep, err := r.CreateEndpoint(&wq)
			if err != nil {
				r.Complete(true)
				return nil, fmt.Errorf("netstack create endpoint fail %s", err)
			}
			r.Complete(false)
			return gonet.NewTCPConn(&wq, ep), nil
		}
		resetConn := func() { r.Complete(true) }

		tcpConnHandler(remoteAddr, newConn, resetConn)
	})

	s.SetTransportProtocolHandler(tcp.ProtocolNumber, tcpFwd.HandlePacket)

	return nil
}
