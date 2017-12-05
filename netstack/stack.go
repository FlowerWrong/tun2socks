package netstack

import (
	"github.com/FlowerWrong/netstack/tcpip"
	"github.com/FlowerWrong/netstack/tcpip/link/fdbased"
	"github.com/FlowerWrong/netstack/tcpip/network/ipv4"
	"github.com/FlowerWrong/netstack/tcpip/network/ipv6"
	"github.com/FlowerWrong/netstack/tcpip/stack"
	"github.com/FlowerWrong/netstack/tcpip/transport/tcp"
	"github.com/FlowerWrong/netstack/tcpip/transport/udp"
	"github.com/FlowerWrong/tun2socks/configure"
	"github.com/FlowerWrong/tun2socks/util"
	"github.com/FlowerWrong/water"
	"log"
	"net"
	"strings"
)

func NewNetstack(cfg *configure.AppConfig) (*stack.Stack, *water.Interface, tcpip.NetworkProtocolNumber) {
	// Parse the IP address. Support both ipv4 and ipv6.
	parsedAddr := net.ParseIP(cfg.General.NetstackAddr)
	if parsedAddr == nil {
		log.Fatalf("Bad IP address: %v", cfg.General.NetstackAddr)
	}

	var addr tcpip.Address
	var proto tcpip.NetworkProtocolNumber
	if parsedAddr.To4() != nil {
		addr = tcpip.Address(parsedAddr.To4())
		proto = ipv4.ProtocolNumber
	} else if parsedAddr.To16() != nil {
		addr = tcpip.Address(parsedAddr.To16())
		proto = ipv6.ProtocolNumber
	} else {
		log.Fatalf("Unknown IP type: %v", cfg.General.NetstackAddr)
	}

	// Create the stack with ip and tcp protocols, then add a tun-based NIC and address.
	s := stack.New([]string{ipv4.ProtocolName, ipv6.ProtocolName}, []string{tcp.ProtocolName, udp.ProtocolName})

	ifce, fd, err := water.New(water.Config{
		DeviceType: water.TUN,
	})
	if err != nil {
		log.Fatal("Create tun interface failed", err)
	}
	log.Println("Interface Name:", ifce.Name())

	util.Ifconfig(ifce.Name(), cfg.General.Network, cfg.General.Mtu)

	linkID := fdbased.New(ifce, fd, cfg.General.Mtu, nil)
	if err := s.CreateNIC(1, linkID, true, addr, cfg.General.NetstackPort); err != nil {
		log.Fatal("Create NIC failed", err)
	}

	if err := s.AddAddress(1, proto, addr); err != nil {
		log.Fatal("Add address to stack failed", err)
	}

	// Add default route.
	s.SetRouteTable([]tcpip.Route{
		{
			Destination: tcpip.Address(strings.Repeat("\x00", len(addr))),
			Mask:        tcpip.Address(strings.Repeat("\x00", len(addr))),
			Gateway:     "",
			NIC:         1,
		},
	})
	return s, ifce, proto
}
