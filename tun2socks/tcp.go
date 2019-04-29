package tun2socks

import (
	"errors"
	"log"
	"net"

	"github.com/FlowerWrong/netstack/tcpip"
	"github.com/FlowerWrong/netstack/tcpip/transport/tcp"
	"github.com/FlowerWrong/netstack/waiter"
)

// NewTCPEndpointAndListenIt create a TCP endpoint, bind it, then start listening.
func (app *App) NewTCPEndpointAndListenIt() error {
	var wq waiter.Queue
	ep, err := app.S.NewEndpoint(tcp.ProtocolNumber, app.NetworkProtocolNumber, &wq)
	if err != nil {
		return errors.New(err.String())
	}

	defer ep.Close()
	if err := ep.Bind(tcpip.FullAddress{NICId, "", app.HookPort}); err != nil {
		return errors.New(err.String())
	}
	if err := ep.Listen(Backlog); err != nil {
		return errors.New(err.String())
	}

	// Wait for connections to appear.
	waitEntry, notifyCh := waiter.NewChannelEntry(nil)
	wq.EventRegister(&waitEntry, waiter.EventIn)
	defer wq.EventUnregister(&waitEntry)

	for {
		select {
		case <-QuitTCPNetstack:
			log.Println("quit tcp netstack")
			return nil
		default:
		}

		endpoint, wq, err := ep.Accept()
		if err != nil {
			if err == tcpip.ErrWouldBlock {
				select {
				case <-QuitTCPNetstack:
					log.Println("quit tcp netstack")
					return nil
				case <-notifyCh:
					continue
				}
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
		tcpTunnel, e := NewTCP2Socks(wq, endpoint, ip, local.Port, app)
		if e != nil {
			endpoint.Close()
			continue
		}
		go tcpTunnel.Run()
	}
}
