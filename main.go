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
	"sync"
	"github.com/FlowerWrong/netstack/tcpip/transport/udp"
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
	if len(os.Args) != 3 {
		log.Fatal("Usage: ", os.Args[0], " <local-address> <local-port>")
	}

	addrName := os.Args[1]
	portName := os.Args[2]

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
	s := stack.New([]string{ipv4.ProtocolName, ipv6.ProtocolName}, []string{tcp.ProtocolName, udp.ProtocolName})

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

	var waitGroup sync.WaitGroup
	//waitGroup.Add(1)
	//go NewTCPEndpointAndListenIt(s, proto, localPort, waitGroup)
	waitGroup.Add(1)
	go NewUDPEndpointAndListenIt(s, proto, localPort, waitGroup)
	waitGroup.Wait()
}

// Create UDP endpoint, bind it, then start listening.
func NewUDPEndpointAndListenIt(s *stack.Stack, proto tcpip.NetworkProtocolNumber, localPort int, waitGroup sync.WaitGroup) {
	var wq waiter.Queue
	ep, e := s.NewEndpoint(udp.ProtocolNumber, proto, &wq)
	if e != nil {
		log.Fatal(e)
	}
	defer ep.Close()
	defer waitGroup.Done()
	if err := ep.Bind(tcpip.FullAddress{0, "", uint16(localPort + 1)}, nil); err != nil {
		log.Fatal("Bind failed: ", err)
	}

	// Wait for connections to appear.
	waitEntry, notifyCh := waiter.NewChannelEntry(nil)
	wq.EventRegister(&waitEntry, waiter.EventIn)
	defer wq.EventUnregister(&waitEntry)

	for {
		var addr tcpip.FullAddress
		v, err := ep.Read(&addr)
		if err != nil {
			if err == tcpip.ErrWouldBlock {
				<-notifyCh
				continue
			}

			log.Fatal("Read failed:", err)
		}
		log.Println(v)
	}
}

// Create TCP endpoint, bind it, then start listening.
func NewTCPEndpointAndListenIt(s *stack.Stack, proto tcpip.NetworkProtocolNumber, localPort int, waitGroup sync.WaitGroup) {
	var wq waiter.Queue
	ep, err := s.NewEndpoint(tcp.ProtocolNumber, proto, &wq)
	if err != nil {
		log.Fatal(err)
	}

	defer ep.Close()
	defer waitGroup.Done()
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
