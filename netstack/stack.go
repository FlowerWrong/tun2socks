package netstack

import (
	"github.com/FlowerWrong/netstack/tcpip"
	"github.com/FlowerWrong/netstack/tcpip/link/fdbased"
	"github.com/FlowerWrong/netstack/tcpip/network/ipv4"
	"github.com/FlowerWrong/netstack/tcpip/network/ipv6"
	"github.com/FlowerWrong/netstack/tcpip/stack"
	"github.com/FlowerWrong/netstack/tcpip/transport/tcp"
	"github.com/FlowerWrong/netstack/tcpip/transport/udp"
	"github.com/FlowerWrong/tun2socks/tun2socks"
	"log"
	"net"
	"strings"
)

func NewNetstack(app *tun2socks.App) tcpip.NetworkProtocolNumber {
	var ip, _, _ = net.ParseCIDR(app.Cfg.General.Network)

	// Parse the IP address. Support both ipv4 and ipv6.
	parsedAddr := net.ParseIP(ip.To4().String())
	if parsedAddr == nil {
		log.Fatalf("Bad IP address: %v", app.Cfg.General.Network)
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
		log.Fatalf("Unknown IP type: %v", app.Cfg.General.Network)
	}

	// Create the stack with ip and tcp protocols, then add a tun-based NIC and address.
	app.S = stack.New([]string{ipv4.ProtocolName, ipv6.ProtocolName}, []string{tcp.ProtocolName, udp.ProtocolName})

	app.HookPort = NewRandomPort(app.S)
	if app.HookPort == 0 {
		log.Fatal("New random port failed")
	}

	linkID := fdbased.New(app.Ifce, app.Fd, app.Cfg.General.Mtu, nil)
	if err := app.S.CreateNIC(1, linkID, true, addr, app.HookPort); err != nil {
		log.Fatal("Create NIC failed", err)
	}

	if err := app.S.AddAddress(1, proto, addr); err != nil {
		log.Fatal("Add address to stack failed", err)
	}

	// Add default route.
	app.S.SetRouteTable([]tcpip.Route{
		{
			Destination: tcpip.Address(strings.Repeat("\x00", len(addr))),
			Mask:        tcpip.Address(strings.Repeat("\x00", len(addr))),
			Gateway:     "",
			NIC:         1,
		},
	})
	return proto
}
