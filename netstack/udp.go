package netstack

import (
	"github.com/FlowerWrong/netstack/tcpip"
	"github.com/FlowerWrong/netstack/tcpip/transport/udp"
	"github.com/FlowerWrong/netstack/waiter"
	"github.com/FlowerWrong/tun2socks/dns"
	"github.com/FlowerWrong/tun2socks/tun2socks"
	"github.com/FlowerWrong/tun2socks/tunnel"
	"github.com/FlowerWrong/tun2socks/util"
	"log"
	"net"
)

// Create UDP endpoint, bind it, then start listening.
func NewUDPEndpointAndListenIt(proto tcpip.NetworkProtocolNumber, app *tun2socks.App) {
	var wq waiter.Queue
	ep, e := app.S.NewEndpoint(udp.ProtocolNumber, proto, &wq)
	if e != nil {
		log.Fatal("New UDP Endpoint failed", e)
	}
	defer ep.Close()
	defer app.WG.Done()
	if err := ep.Bind(tcpip.FullAddress{0, "", app.HookPort}, nil); err != nil {
		log.Fatal("Bind failed", err)
	}

	// Wait for connections to appear.
	waitEntry, notifyCh := waiter.NewChannelEntry(nil)
	wq.EventRegister(&waitEntry, waiter.EventIn)
	defer wq.EventUnregister(&waitEntry)

	for {
		var localAddr tcpip.FullAddress
		v, err := ep.Read(&localAddr)
		if err != nil {
			if err == tcpip.ErrWouldBlock {
				<-notifyCh
				continue
			}
			if !util.IsClosed(err) {
				log.Println("Read from netstack failed", err)
			}
			udp.UDPNatList.DelUDPNat(localAddr.Port)
			continue
		}

		endpoint := udp.UDPNatList.GetUDPNat(localAddr.Port)

		if app.Cfg.Dns.DnsMode == "udp_relay_via_socks5" {
			answer := dns.DNSCache.Query(v)
			if answer != nil {
				data, err := answer.Pack()
				if err == nil {
					remoteHost := endpoint.LocalAddress.To4().String()
					remotePort := endpoint.LocalPort
					pkt := util.CreateDNSResponse(net.ParseIP(remoteHost), remotePort, net.ParseIP(localAddr.Addr.To4().String()), localAddr.Port, data)
					if pkt == nil {
						continue
					}
					_, err := app.Ifce.Write(pkt)
					if err != nil {
						log.Println("Write to tun failed", err)
					} else {
						udp.UDPNatList.DelUDPNat(localAddr.Port)
						continue
					}
				}
			}
		}

		udpTunnel, e := tunnel.NewUdpTunnel(endpoint, localAddr, app)
		if e != nil {
			log.Println("NewUdpTunnel failed", e)
			udp.UDPNatList.DelUDPNat(localAddr.Port)
			continue
		}
		go udpTunnel.Run()
		udpTunnel.LocalPackets <- v
	}
}
