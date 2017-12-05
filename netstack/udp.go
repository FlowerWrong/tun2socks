package netstack

import (
	"github.com/FlowerWrong/netstack/tcpip"
	"github.com/FlowerWrong/netstack/tcpip/stack"
	"github.com/FlowerWrong/netstack/tcpip/transport/udp"
	"github.com/FlowerWrong/netstack/waiter"
	"github.com/FlowerWrong/tun2socks/configure"
	"github.com/FlowerWrong/tun2socks/dns"
	"github.com/FlowerWrong/tun2socks/tunnel"
	"github.com/FlowerWrong/tun2socks/util"
	"github.com/FlowerWrong/water"
	"log"
	"net"
	"sync"
)

// Create UDP endpoint, bind it, then start listening.
func NewUDPEndpointAndListenIt(s *stack.Stack, proto tcpip.NetworkProtocolNumber, localPort int, waitGroup sync.WaitGroup, ifce *water.Interface, dnsProxy string, fakeDns *dns.Dns, cfg *configure.AppConfig) {
	var wq waiter.Queue
	ep, e := s.NewEndpoint(udp.ProtocolNumber, proto, &wq)
	if e != nil {
		log.Fatal("New UDP Endpoint failed", e)
	}
	defer ep.Close()
	defer waitGroup.Done()
	if err := ep.Bind(tcpip.FullAddress{0, "", uint16(localPort)}, nil); err != nil {
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
			log.Println("Read from netstack failed", err)
			udp.UDPNatList.DelUDPNat(localAddr.Port)
			continue
		}

		log.Println("There are", len(udp.UDPNatList.Data), "UDP connections")
		endpoint := udp.UDPNatList.GetUDPNat(localAddr.Port)

		if cfg.Dns.DnsMode == "udp_relay_via_socks5" {
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
					_, err := ifce.Write(pkt)
					if err != nil {
						log.Println("Write to tun failed", err)
					} else {
						udp.UDPNatList.DelUDPNat(localAddr.Port)
						continue
					}
				}
			}
		}

		udpTunnel, e := tunnel.NewUdpTunnel(endpoint, localAddr, ifce, dnsProxy, fakeDns, cfg)
		if e != nil {
			log.Println("NewUdpTunnel failed", e)
			udp.UDPNatList.DelUDPNat(localAddr.Port)
			continue
		}
		go udpTunnel.Run()
		udpTunnel.LocalPackets <- v
	}
}
