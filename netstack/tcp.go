package netstack

import (
	"log"
	"net"

	"github.com/FlowerWrong/netstack/tcpip"
	"github.com/FlowerWrong/netstack/tcpip/transport/tcp"
	"github.com/FlowerWrong/netstack/waiter"
	"github.com/FlowerWrong/tun2socks/tun2socks"
	"github.com/FlowerWrong/tun2socks/tunnel"
)

// NewTCPEndpointAndListenIt create a TCP endpoint, bind it, then start listening.
func NewTCPEndpointAndListenIt(proto tcpip.NetworkProtocolNumber, app *tun2socks.App) {
	var wq waiter.Queue
	ep, err := app.S.NewEndpoint(tcp.ProtocolNumber, proto, &wq)
	if err != nil {
		log.Fatal("NewEndpoint failed", err)
	}

	defer ep.Close()
	if err := ep.Bind(tcpip.FullAddress{NICId, "", app.HookPort}, nil); err != nil {
		log.Fatal("Bind failed", err)
	}
	if err := ep.Listen(Backlog); err != nil {
		log.Fatal("Listen failed", err)
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
			log.Println("[error] accept failed", err)
		}

		local, _ := endpoint.GetLocalAddress()
		// TODO ipv6
		ip := net.ParseIP(local.Addr.To4().String())

		contains, _ := IgnoreRanger.Contains(ip)
		if contains {
			endpoint.Close()
			continue
		}
		tcpTunnel, e := tunnel.NewTCP2Socks(wq, endpoint, ip, local.Port, app)
		if e != nil {
			endpoint.Close()
			continue
		}

		go tcpTunnel.Run()
	}
}
