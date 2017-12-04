package main

import (
	"flag"
	"github.com/FlowerWrong/tun2socks/configure"
	"log"
	"math/rand"
	"github.com/FlowerWrong/tun2socks/tunnel"
	"runtime"
	"sync"
	"github.com/FlowerWrong/tun2socks/dns"
	"github.com/FlowerWrong/tun2socks/netstack"
	"github.com/FlowerWrong/tun2socks/util"
	"time"
)

var cfg *configure.AppConfig
var fakeDns *dns.Dns

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
	var err error
	cfg, err = configure.Parse(configFile)
	tunnel.Socks5Addr, err = cfg.DefaultPorxy()
	if err != nil {
		log.Fatalln("Get default proxy failed", err)
	}

	// signal handler
	util.NewSignalHandler()

	s, _, proto := netstack.NewNetstack(cfg)
	fakeDns, err = dns.NewFakeDnsServer(cfg)
	if err != nil {
		log.Fatal("new fake dns server failed", err)
	}

	var waitGroup sync.WaitGroup
	waitGroup.Add(1)
	go netstack.NewTCPEndpointAndListenIt(s, proto, int(cfg.General.NetstackPort), waitGroup, fakeDns)
	//waitGroup.Add(1)
	//go netstack.NewUDPEndpointAndListenIt(s, proto, int(cfg.General.NetstackPort), waitGroup, ifce)
	waitGroup.Add(1)
	go func(waitGroup sync.WaitGroup, fakeDns *dns.Dns) {
		fakeDns.Serve()
		waitGroup.Done()
	}(waitGroup, fakeDns)
	waitGroup.Wait()
}
