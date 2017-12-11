package netstack

import (
	"log"
	"net"

	"github.com/FlowerWrong/netstack/tcpip"
	"github.com/FlowerWrong/netstack/tcpip/transport/udp"
	"github.com/FlowerWrong/netstack/waiter"
	"github.com/FlowerWrong/tun2socks/dns"
	"github.com/FlowerWrong/tun2socks/tun2socks"
	"github.com/FlowerWrong/tun2socks/tunnel"
	"github.com/FlowerWrong/tun2socks/util"
)

// NewUDPEndpointAndListenIt create a UDP endpoint, bind it, then start read.
func NewUDPEndpointAndListenIt(proto tcpip.NetworkProtocolNumber, app *tun2socks.App) {
	var wq waiter.Queue
	ep, e := app.S.NewEndpoint(udp.ProtocolNumber, proto, &wq)
	if e != nil {
		log.Fatal("New UDP Endpoint failed", e)
	}
	defer ep.Close()
	defer app.WG.Done()
	if err := ep.Bind(tcpip.FullAddress{NICId, "", app.HookPort}, nil); err != nil {
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
		// TODO ipv6
		remoteHost := endpoint.LocalAddress.To4().String()
		contains, _ := IgnoreRanger.Contains(net.ParseIP(remoteHost))
		if contains {
			continue
		}

		if app.Cfg.Dns.DnsMode == "udp_relay_via_socks5" {
			answer := dns.DNSCache.Query(v)
			if answer != nil {
				data, err := answer.Pack()
				if err == nil {
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
