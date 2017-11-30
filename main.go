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
	"github.com/FlowerWrong/netstack/tcpip/transport/udp"
	"github.com/FlowerWrong/netstack/waiter"
	"github.com/FlowerWrong/tun2socks/socket"
	"github.com/FlowerWrong/water"
	"github.com/yinghuocho/gosocks"
	"io"
	"os/exec"
	"runtime"
	"sync"
)

func execCommand(name, sargs string) error {
	args := strings.Split(sargs, " ")
	cmd := exec.Command(name, args...)
	log.Println("exec command: %s %s", name, sargs)
	return cmd.Run()
}

var s *stack.Stack

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
	s = stack.New([]string{ipv4.ProtocolName, ipv6.ProtocolName}, []string{tcp.ProtocolName, udp.ProtocolName})

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
	waitGroup.Add(1)
	//go NewTCPEndpointAndListenIt(s, proto, localPort, waitGroup)
	//waitGroup.Add(1)
	go NewUDPEndpointAndListenIt(s, proto, localPort, waitGroup)
	waitGroup.Wait()
}

const socks5Version = 5
const (
	socks5AuthNone = 0
)

type UDPPacket struct {
	Addr *net.UDPAddr
	Data []byte
}

// Information maintained for each client/server connection
type UDPConnection struct {
	ClientAddr *net.UDPAddr // Address of the client
	ServerConn *net.UDPConn // UDP connection to server
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
	if err := ep.Bind(tcpip.FullAddress{0, "", uint16(localPort)}, nil); err != nil {
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

		socks5Addr := "127.0.0.1:1090"
		targetAddr := fmt.Sprintf("%v:%d", "8.8.8.8", 53)
		log.Println("targetAddr2", targetAddr)

		host, portStr, e := net.SplitHostPort(targetAddr)
		if e != nil {
			log.Println(e)
			return
		}

		port, e := strconv.Atoi(portStr)
		if e != nil {
			log.Println(e)
			return
		}
		if port < 1 || port > 0xffff {
			return
		}

		buf := make([]byte, 0, 6+len(host))
		buf = append(buf, socks5Version)
		buf = append(buf, 1 /* num auth methods */ , socks5AuthNone)

		c, e := net.Dial("tcp", socks5Addr)
		c.Write(buf)

		if _, err := io.ReadFull(c, buf[:2]); err != nil {
			log.Println(err)
			return
		}
		if buf[0] != 5 {
			return
		}
		if buf[1] == 0xff {
			return
		}

		_, e = gosocks.WriteSocksRequest(c, &gosocks.SocksRequest{
			Cmd:      gosocks.SocksCmdUDPAssociate,
			HostType: gosocks.SocksIPv4Host,
			DstHost:  "8.8.8.8",
			DstPort:  53,
		})
		if e != nil {
			log.Println(e)
			return
		}

		reply, e := gosocks.ReadSocksReply(c)
		if e != nil {
			log.Println(e)
			return
		}
		relayAddr := gosocks.SocksAddrToNetAddr("udp", reply.BndHost, reply.BndPort).(*net.UDPAddr)

		log.Println("relayAddr", relayAddr)

		// 127.0.0.1:49558 127.0.0.1:1090
		log.Println(c.LocalAddr(), c.RemoteAddr())

		socksAddr := c.LocalAddr().(*net.TCPAddr)
		udpBind, e := net.ListenUDP("udp", &net.UDPAddr{
			IP:   socksAddr.IP,
			Port: 0,
			Zone: socksAddr.Zone,
		})
		if e != nil {
			log.Println(e)
			return
		}

		req := &gosocks.UDPRequest{
			Frag:     0,
			HostType: gosocks.SocksIPv4Host,
			DstHost:  "8.8.8.8",
			DstPort:  53,
			Data:     v,
		}
		datagram := gosocks.PackUDPRequest(req)

		// relayAddr 127.0.0.1:1090
		udpBind.WriteTo(datagram, gosocks.SocksAddrToNetAddr("udp", reply.BndHost, reply.BndPort).(*net.UDPAddr))

		var b [4096]byte
		n, remote_addr, e := udpBind.ReadFromUDP(b[:])
		fmt.Println("from", remote_addr, "got message:", n)
		switch {
		case n != 0:
			c := make([]byte, n)
			copy(c, b[:n])
			ep.Write(c[10:], &addr)
		case err != nil:
			log.Fatal(err)
		}
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

		go socket.NewTunnel(wq, n, "tcp").ReadFromLocalWriteToRemote()
	}
}
