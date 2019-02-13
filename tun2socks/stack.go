package tun2socks

import (
	"log"
	"net"
	"strings"

	"github.com/FlowerWrong/netstack/tcpip"
	"github.com/FlowerWrong/netstack/tcpip/link/fdbased"
	"github.com/FlowerWrong/netstack/tcpip/network/ipv4"
	"github.com/FlowerWrong/netstack/tcpip/network/ipv6"
	"github.com/FlowerWrong/netstack/tcpip/stack"
	"github.com/FlowerWrong/netstack/tcpip/transport/tcp"
	"github.com/FlowerWrong/netstack/tcpip/transport/udp"
	"github.com/FlowerWrong/tun2socks/util"
)

const (
	// NICId is global nicid for stack
	NICId = 1
	// Backlog is tcp listen backlog
	Backlog = 1024
)

// NewNetstack create a tcp/ip stack
func NewNetstack(app *App) tcpip.NetworkProtocolNumber {
	tunIP, _, _ := net.ParseCIDR(app.Cfg.General.Network)

	// Parse the IP address. Support both ipv4 and ipv6.
	var addr tcpip.Address
	var proto tcpip.NetworkProtocolNumber
	if tunIP.To4() != nil {
		addr = tcpip.Address(tunIP.To4())
		proto = ipv4.ProtocolNumber
	} else if tunIP.To16() != nil {
		addr = tcpip.Address(tunIP.To16())
		proto = ipv6.ProtocolNumber
	} else {
		log.Fatalf("Unknown IP type: %v", app.Cfg.General.Network)
	}

	// Create the stack with ip and tcp protocols, then add a tun-based NIC and address.
	app.S = stack.New([]string{ipv4.ProtocolName, ipv6.ProtocolName}, []string{tcp.ProtocolName, udp.ProtocolName}, stack.Options{})

	app.HookPort = util.NewRandomPort(app.S)
	if app.HookPort == 0 {
		log.Fatal("New random port failed")
	}

	// Parse the mac address.
	maddr, err := net.ParseMAC("aa:00:01:01:01:01")
	if err != nil {
		log.Fatalf("Bad MAC address: aa:00:01:01:01:01")
	}

	linkID := fdbased.New(&fdbased.Options{
		FD:             app.Ifce.Fd(),
		MTU:            app.Cfg.General.Mtu,
		EthernetHeader: false,
		Address:        tcpip.LinkAddress(maddr),
		UseRecvMMsg:    false,
	})
	if err := app.S.CreateNIC(NICId, linkID, true, addr, app.HookPort); err != nil {
		log.Fatal("Create NIC failed", err)
	}

	if err := app.S.AddAddress(NICId, proto, addr); err != nil {
		log.Fatal("Add address to stack failed", err)
	}

	// Add default route.
	app.S.SetRouteTable([]tcpip.Route{
		{
			Destination: tcpip.Address(strings.Repeat("\x00", len(addr))),
			Mask:        tcpip.AddressMask(strings.Repeat("\x00", len(addr))),
			Gateway:     "",
			NIC:         NICId,
		},
	})
	return proto
}
