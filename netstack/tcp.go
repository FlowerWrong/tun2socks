package netstack

import (
	"sync"
	"fmt"
	"net"
	"github.com/FlowerWrong/tun2socks/tunnel"
	"github.com/FlowerWrong/netstack/tcpip/stack"
	"github.com/FlowerWrong/netstack/tcpip"
	"github.com/FlowerWrong/tun2socks/dns"
	"github.com/FlowerWrong/netstack/waiter"
	"github.com/FlowerWrong/netstack/tcpip/transport/tcp"
	"log"
)

// Create TCP endpoint, bind it, then start listening.
func NewTCPEndpointAndListenIt(s *stack.Stack, proto tcpip.NetworkProtocolNumber, localPort int, waitGroup sync.WaitGroup, fakeDns *dns.Dns) {
	var wq waiter.Queue
	ep, err := s.NewEndpoint(tcp.ProtocolNumber, proto, &wq)
	if err != nil {
		log.Fatal("New TCP Endpoint failed", err)
	}

	defer ep.Close()
	defer waitGroup.Done()
	if err := ep.Bind(tcpip.FullAddress{0, "", uint16(localPort)}, nil); err != nil {
		log.Fatal("Bind failed: ", err)
	}
	if err := ep.Listen(1024); err != nil {
		log.Fatal("Listen failed: ", err)
	}

	// Wait for connections to appear.
	waitEntry, notifyCh := waiter.NewChannelEntry(nil)
	wq.EventRegister(&waitEntry, waiter.EventIn)
	defer wq.EventUnregister(&waitEntry)

	for {
		endpoint, wq, err := ep.Accept()
		if err != nil {
			if err == tcpip.ErrWouldBlock {
				<-notifyCh
				continue
			}

			log.Println("Accept failed:", err)
		}

		local, _ := endpoint.GetLocalAddress()
		var targetAddr string

		ip := fmt.Sprintf("%v", local.Addr.To4())
		dd := fakeDns.DnsTablePtr.GetByIP(net.ParseIP(ip))
		if dd != nil {
			targetAddr = fmt.Sprintf("%v:%d", dd.Hostname, local.Port)
		} else {
			targetAddr = fmt.Sprintf("%v:%d", local.Addr.To4(), local.Port)
		}
		tcpTunnel, e := tunnel.NewTCP2Socks(wq, endpoint, "tcp", targetAddr)
		if e != nil {
			log.Println("NewTCP2Socks tunnel failed", e, tcpTunnel)
			endpoint.Close()
			continue
		}

		go tcpTunnel.Run()
	}
}
