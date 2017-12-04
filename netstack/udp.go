package netstack

import (
	"sync"
	"net"
	"github.com/FlowerWrong/tun2socks/tunnel"
	"github.com/FlowerWrong/netstack/tcpip/stack"
	"github.com/FlowerWrong/netstack/tcpip"
	"github.com/FlowerWrong/water"
	"github.com/FlowerWrong/netstack/waiter"
	"github.com/FlowerWrong/netstack/tcpip/transport/udp"
	"log"
	"github.com/FlowerWrong/tun2socks/dns"
	"github.com/FlowerWrong/tun2socks/util"
)

// Create UDP endpoint, bind it, then start listening.
func NewUDPEndpointAndListenIt(s *stack.Stack, proto tcpip.NetworkProtocolNumber, localPort int, waitGroup sync.WaitGroup, ifce *water.Interface, dnsProxy string) {
	var wq waiter.Queue
	ep, e := s.NewEndpoint(udp.ProtocolNumber, proto, &wq)
	if e != nil {
		log.Fatal("New UDP Endpoint failed", e)
	}
	defer ep.Close()
	defer waitGroup.Done()
	if err := ep.Bind(tcpip.FullAddress{0, "", uint16(localPort)}, nil); err != nil {
		log.Fatal("Bind failed: ", err)
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
			log.Println("Read failed:", err)
			udp.UDPNatList.DelUDPNat(localAddr.Port)
			continue
		}

		log.Println("There are", len(udp.UDPNatList.Data), "UDP connections")
		endpoint := udp.UDPNatList.GetUDPNat(localAddr.Port)
		remoteHost := endpoint.LocalAddress.To4().String()
		remotePort := endpoint.LocalPort

		answer := dns.DNSCache.Query(v)
		if answer != nil {
			data, err := answer.Pack()
			if err == nil {
				pkt := util.CreateDNSResponse(net.ParseIP(remoteHost), remotePort, net.ParseIP(localAddr.Addr.To4().String()), localAddr.Port, data)
				if pkt == nil {
					continue
				}
				_, err := ifce.Write(pkt)
				if err != nil {
					log.Println("Write to tun failed", err)
				} else {
					log.Println("Use dns cache")
					udp.UDPNatList.DelUDPNat(localAddr.Port)
					continue
				}
			}
		}

		udpTunnel, e := tunnel.NewUdpTunnel(endpoint, localAddr, ifce, dnsProxy)
		if e != nil {
			log.Println("NewUdpTunnel failed", e)
			udp.UDPNatList.DelUDPNat(localAddr.Port)
			continue
		}
		go udpTunnel.Run()
		udpTunnel.LocalPackets <- v
	}
}
