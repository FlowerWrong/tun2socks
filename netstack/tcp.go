package netstack

import (
	"github.com/FlowerWrong/netstack/tcpip"
	"github.com/FlowerWrong/netstack/tcpip/transport/tcp"
	"github.com/FlowerWrong/netstack/waiter"
	"github.com/FlowerWrong/tun2socks/tun2socks"
	"github.com/FlowerWrong/tun2socks/tunnel"
	"log"
	"net"
)

// Create TCP endpoint, bind it, then start listening.
func NewTCPEndpointAndListenIt(proto tcpip.NetworkProtocolNumber, app *tun2socks.App) {
	var wq waiter.Queue
	ep, err := app.S.NewEndpoint(tcp.ProtocolNumber, proto, &wq)
	if err != nil {
		log.Fatal("New TCP Endpoint failed", err)
	}

	defer ep.Close()
	defer app.WG.Done()
	if err := ep.Bind(tcpip.FullAddress{0, "", app.HookPort}, nil); err != nil {
		log.Fatal("Bind failed", err)
	}
	if err := ep.Listen(1024); err != nil {
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
			log.Println("Accept failed", err)
		}

		local, _ := endpoint.GetLocalAddress()
		ip := net.ParseIP(local.Addr.To4().String())

		contains, _ := IgnoreRanger.Contains(ip)
		if contains {
			endpoint.Close()
			continue
		}
		tcpTunnel, e := tunnel.NewTCP2Socks(wq, endpoint, ip, local.Port, app)
		if e != nil {
			log.Println("NewTCP2Socks tunnel failed", e, tcpTunnel)
			endpoint.Close()
			continue
		}

		go tcpTunnel.Run()
	}
}
