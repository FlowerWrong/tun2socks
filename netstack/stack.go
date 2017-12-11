package netstack

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
	"github.com/FlowerWrong/tun2socks/tun2socks"
)

const (
	// NICId is global nicid for stack
	NICId = 1
	// Backlog is tcp listen backlog
	Backlog = 1024
)

// NewNetstack create a tcp/ip stack
func NewNetstack(app *tun2socks.App) tcpip.NetworkProtocolNumber {
	tunIp, _, _ := net.ParseCIDR(app.Cfg.General.Network)

	// Parse the IP address. Support both ipv4 and ipv6.
	var addr tcpip.Address
	var proto tcpip.NetworkProtocolNumber
	if tunIp.To4() != nil {
		addr = tcpip.Address(tunIp.To4())
		proto = ipv4.ProtocolNumber
	} else if tunIp.To16() != nil {
		addr = tcpip.Address(tunIp.To16())
		proto = ipv6.ProtocolNumber
	} else {
		log.Fatalf("Unknown IP type: %v", app.Cfg.General.Network)
	}

	// Create the stack with ip and tcp protocols, then add a tun-based NIC and address.
	app.S = stack.New([]string{ipv4.ProtocolName, ipv6.ProtocolName}, []string{tcp.ProtocolName, udp.ProtocolName})

	app.HookPort = NewRandomPort(app.S)
	if app.HookPort == 0 {
		log.Fatal("New random port failed")
	}

	linkID := fdbased.New(app.Ifce, app.Cfg.General.Mtu, nil)
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
			Mask:        tcpip.Address(strings.Repeat("\x00", len(addr))),
			Gateway:     "",
			NIC:         NICId,
		},
	})
	return proto
}
