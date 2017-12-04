package main

import (
	"flag"
	"github.com/FlowerWrong/tun2socks/configure"
	"github.com/FlowerWrong/tun2socks/util"
	"log"
	"math/rand"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
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
	"github.com/FlowerWrong/tun2socks/tunnel"
	"github.com/FlowerWrong/water"
	"os/exec"
	"runtime"
	"sync"
	"github.com/FlowerWrong/tun2socks/dns"
)

func execCommand(name, sargs string) error {
	args := strings.Split(sargs, " ")
	cmd := exec.Command(name, args...)
	log.Println("exec command: %s %s", name, sargs)
	return cmd.Run()
}

func main() {
	rand.Seed(time.Now().UnixNano())

	// log with file and line number
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	log.Println("Use CPU number", runtime.NumCPU())
	runtime.GOMAXPROCS(runtime.NumCPU())

	config := flag.String("config", "", "config file")
	flag.Parse()

	configFile := *config
	if configFile == "" {
		configFile = flag.Arg(0)
	}
	// parse config
	cfg, err := configure.Parse(configFile)
	tunnel.Socks5Addr, err = cfg.DefaultPorxy()
	if err != nil {
		log.Fatalln("Get default proxy failed", err)
	}

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

	// signal handler
	c := make(chan os.Signal)
	signal.Notify(c, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGUSR1, syscall.SIGUSR2)
	go func() {
		for s := range c {
			switch s {
			case syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT:
				fmt.Println("退出", s)
				Exit()
			case syscall.SIGUSR1:
				fmt.Println("usr1", s)
			case syscall.SIGUSR2:
				fmt.Println("usr2", s)
			default:
				fmt.Println("other", s)
			}
		}
	}()

	// Create the stack with ip and tcp protocols, then add a tun-based NIC and address.
	s := stack.New([]string{ipv4.ProtocolName, ipv6.ProtocolName}, []string{tcp.ProtocolName, udp.ProtocolName})

	ifce, fd, err := water.New(water.Config{
		DeviceType: water.TUN,
	})
	if err != nil {
		log.Fatal("Create tun interface failed", err)
	}
	log.Printf("Interface Name: %s\n", ifce.Name())

	if runtime.GOOS == "darwin" {
		sargs := fmt.Sprintf("%s %s %s mtu %d netmask 255.255.255.0 up", ifce.Name(), cfg.General.NetstackAddr, cfg.General.NetstackAddr, cfg.General.Mtu)
		if err := execCommand("/sbin/ifconfig", sargs); err != nil {
			log.Fatal("execCommand failed", err)
		}
	} else if runtime.GOOS == "linux" {
		sargs := fmt.Sprintf("%s %s netmask 255.255.255.0", ifce.Name(), cfg.General.NetstackAddr)
		if err := execCommand("/sbin/ifconfig", sargs); err != nil {
			log.Fatal("execCommand failed", err)
		}
	} else {
		log.Fatal("Not support os")
	}

	linkID := fdbased.New(ifce, fd, cfg.General.Mtu, nil)
	if err := s.CreateNIC(1, linkID, true, addr, cfg.General.NetstackPort); err != nil {
		log.Fatal("Create NIC failed", err)
	}

	if err := s.AddAddress(1, proto, addr); err != nil {
		log.Fatal("Add address failed", err)
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
	//waitGroup.Add(1)
	//go NewUDPEndpointAndListenIt(s, proto, localPort, waitGroup, ifce)
	waitGroup.Add(1)
	go func(waitGroup sync.WaitGroup, cfg *configure.AppConfig) {
		dns, err := dns.NewFakeDnsServer(cfg)
		if err != nil {
			log.Fatal("new fake dns server failed", err)
		}
		dns.Serve()
		waitGroup.Done()
	}(waitGroup, cfg)
	waitGroup.Wait()
}

// Exit tun2socks
func Exit() {
	// TODO cleanup
	os.Exit(0)
}

// Create UDP endpoint, bind it, then start listening.
func NewUDPEndpointAndListenIt(s *stack.Stack, proto tcpip.NetworkProtocolNumber, localPort int, waitGroup sync.WaitGroup, ifce *water.Interface) {
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

		udpTunnel, e := tunnel.NewUdpTunnel(endpoint, localAddr, ifce)
		if e != nil {
			log.Println("NewUdpTunnel failed", e)
			udp.UDPNatList.DelUDPNat(localAddr.Port)
			continue
		}
		go udpTunnel.Run()
		udpTunnel.LocalPackets <- v
	}
}

// Create TCP endpoint, bind it, then start listening.
func NewTCPEndpointAndListenIt(s *stack.Stack, proto tcpip.NetworkProtocolNumber, localPort int, waitGroup sync.WaitGroup) {
	var wq waiter.Queue
	ep, err := s.NewEndpoint(tcp.ProtocolNumber, proto, &wq)
	if err != nil {
		log.Fatal("New TCP Endpoint failed", err)
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
		endpoint, wq, err := ep.Accept()
		if err != nil {
			if err == tcpip.ErrWouldBlock {
				<-notifyCh
				continue
			}

			log.Println("Accept failed:", err)
		}

		tcpTunnel, e := tunnel.NewTCP2Socks(wq, endpoint, "tcp")
		if e != nil {
			log.Println("NewTCP2Socks tunnel failed", e, tcpTunnel)
			endpoint.Close()
			continue
		}

		go tcpTunnel.Run()
	}
}
