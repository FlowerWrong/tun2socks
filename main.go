// Copyright 2016 The Netstack Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This sample creates a stack with TCP and IPv4 protocols on top of a TUN
// device, and listens on a port. Data received by the server in the accepted
// connections is echoed back to the clients.
package main

import (
	"log"
	"math/rand"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"fmt"
	"github.com/FlowerWrong/netstack/tcpip"
	"github.com/FlowerWrong/netstack/tcpip/link/fdbased"
	"github.com/FlowerWrong/netstack/tcpip/network/ipv4"
	"github.com/FlowerWrong/netstack/tcpip/network/ipv6"
	"github.com/FlowerWrong/netstack/tcpip/stack"
	"github.com/FlowerWrong/netstack/tcpip/transport/tcp"
	"github.com/FlowerWrong/netstack/waiter"
	"github.com/FlowerWrong/water"
	"os/exec"
	"runtime"
	"github.com/FlowerWrong/tun2socks/socket"
)

func execCommand(name, sargs string) error {
	args := strings.Split(sargs, " ")
	cmd := exec.Command(name, args...)
	log.Println("exec command: %s %s", name, sargs)
	return cmd.Run()
}

func addOSXRoute(tun string, subnet *net.IPNet) error {
	ip := subnet.IP
	maskIP := net.IP(subnet.Mask)
	sargs := fmt.Sprintf("-n add -net %s -netmask %s -interface %s", ip.String(), maskIP.String(), tun)
	return execCommand("route", sargs)
}

func addLinuxRoute(tun string, subnet *net.IPNet) error {
	ip := subnet.IP
	sargs := fmt.Sprintf("add %s via %s", ip.String(), tun)
	return execCommand("ip route", sargs)
}

func main() {
	if len(os.Args) != 4 {
		log.Fatal("Usage: ", os.Args[0], " <tun-device> <local-address> <local-port>")
	}

	// tunName := os.Args[1]
	addrName := os.Args[2]
	portName := os.Args[3]

	rand.Seed(time.Now().UnixNano())

	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// Parse the IP address. Support both ipv4 and ipv6.
	parsedAddr := net.ParseIP(addrName)
	if parsedAddr == nil {
		log.Fatalf("Bad IP address: %v", addrName)
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
		log.Fatalf("Unknown IP type: %v", addrName)
	}

	localPort, err := strconv.Atoi(portName)
	if err != nil {
		log.Fatalf("Unable to convert port %v: %v", portName, err)
	}

	// Create the stack with ip and tcp protocols, then add a tun-based
	// NIC and address.
	s := stack.New([]string{ipv4.ProtocolName, ipv6.ProtocolName}, []string{tcp.ProtocolName})

	var mtu uint32 = 1500

	ifce, fd, err := water.New(water.Config{
		DeviceType: water.TUN,
	})
	if err != nil {
		log.Fatal(err)
		return
	}
	log.Printf("Interface Name: %s\n", ifce.Name())

	if runtime.GOOS == "darwin" {
		sargs := fmt.Sprintf("%s 10.0.0.1 10.0.0.2 mtu %d netmask 255.255.255.0 up", ifce.Name(), mtu)
		if err := execCommand("/sbin/ifconfig", sargs); err != nil {
			log.Println(err)
			return
		}
	} else if runtime.GOOS == "linux" {
		sargs := fmt.Sprintf("%s 10.0.0.1 netmask 255.255.255.0", ifce.Name())
		if err := execCommand("/sbin/ifconfig", sargs); err != nil {
			log.Println(err)
			return
		}

		//_, ipnet1, _ := net.ParseCIDR("119.23.10.5")
		//addLinuxRoute("10.0.0.1", ipnet1)
		//_, ipnet2, _ := net.ParseCIDR("47.90.32.252")
		//addLinuxRoute("192.168.1.1", ipnet2)
	} else {
		log.Println("not support os")
		return
	}

	linkID := fdbased.New(ifce, fd, mtu, nil)
	if err := s.CreateNIC(1, linkID, true, addr, uint16(localPort)); err != nil {
		log.Fatal(err)
	}

	if err := s.AddAddress(1, proto, addr); err != nil {
		log.Fatal(err)
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

	// Create TCP endpoint, bind it, then start listening.
	var wq waiter.Queue
	ep, e := s.NewEndpoint(tcp.ProtocolNumber, proto, &wq)
	if err != nil {
		log.Fatal(e)
	}

	defer ep.Close()

	if err := ep.Bind(tcpip.FullAddress{0, "", uint16(localPort)}, nil); err != nil {
		log.Fatal("Bind failed: ", err)
	}

	if err := ep.Listen(1024); err != nil {
		log.Fatal("Listen failed: ", err)
	}

	// Wait for connections to appear.
	waitEntry, notifyCh := waiter.NewChannelEntry(nil)
	wq.EventRegister(&waitEntry, waiter.EventIn)
	defer wq.EventUnregister(&waitEntry)

	for {
		n, wq, err := ep.Accept()
		if err != nil {
			if err == tcpip.ErrWouldBlock {
				<-notifyCh
				continue
			}

			log.Fatal("Accept() failed:", err)
		}

		go socket.NewTunnel(wq, n).ReadFromLocalWriteToRemote()
	}
}
