package main

import (
	"flag"
	"github.com/FlowerWrong/tun2socks/configure"
	"log"
	"math/rand"
	"runtime"
	"sync"
	"github.com/FlowerWrong/tun2socks/dns"
	"github.com/FlowerWrong/tun2socks/netstack"
	"github.com/FlowerWrong/tun2socks/util"
	"time"
)

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
	if err != nil {
		log.Fatalln("Get default proxy failed", err)
	}

	// signal handler
	util.NewSignalHandler()

	s, ifce, proto := netstack.NewNetstack(cfg)

	var fakeDns *dns.Dns
	if cfg.Dns.DnsMode == "fake" {
		fakeDns, err = dns.NewFakeDnsServer(cfg)
		if err != nil {
			log.Fatal("new fake dns server failed", err)
		}
	}

	proxies, err := configure.NewProxies(cfg.Proxy)
	if err != nil {
		log.Fatalln("New proxies failed", err)
	}

	util.AddRoutes(cfg.Route.V, ifce)

	var waitGroup sync.WaitGroup
	waitGroup.Add(1)
	go netstack.NewTCPEndpointAndListenIt(s, proto, int(cfg.General.NetstackPort), waitGroup, fakeDns, proxies)
	waitGroup.Add(1)
	dnsProxy, err := cfg.UdpProxy()
	if err != nil {
		log.Fatal("Get udp socks 5 proxy failed", err)
	}
	go netstack.NewUDPEndpointAndListenIt(s, proto, int(cfg.General.NetstackPort), waitGroup, ifce, dnsProxy)
	if cfg.Dns.DnsMode == "fake" {
		waitGroup.Add(1)
		go func(waitGroup sync.WaitGroup, fakeDns *dns.Dns) {
			fakeDns.Serve()
			waitGroup.Done()
		}(waitGroup, fakeDns)
	}
	waitGroup.Wait()
}
