package main

import (
	"flag"
	"github.com/FlowerWrong/netstack/tcpip"
	"github.com/FlowerWrong/tun2socks/configure"
	"github.com/FlowerWrong/tun2socks/dns"
	"github.com/FlowerWrong/tun2socks/netstack"
	"github.com/FlowerWrong/tun2socks/tun2socks"
	"github.com/FlowerWrong/tun2socks/util"
	"log"
	"math/rand"
	"runtime"
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

	app := new(tun2socks.App)

	// parse config
	app.Cfg = new(configure.AppConfig)
	err := app.Cfg.Parse(configFile)
	if err != nil {
		log.Fatal("Get default proxy failed", err)
	}

	var proto tcpip.NetworkProtocolNumber
	app.S, app.Ifce, proto, app.HookPort = netstack.NewNetstack(app.Cfg)

	if app.Cfg.Dns.DnsMode == "fake" {
		app.FakeDns, err = dns.NewFakeDnsServer(app.Cfg)
		if err != nil {
			log.Fatal("New fake dns server failed", err)
		}
	}

	app.Proxies, err = configure.NewProxies(app.Cfg.Proxy)
	if err != nil {
		log.Fatalln("New proxies failed", err)
	}

	app.AddRoutes()
	app.SignalHandler()

	app.WG.Add(1)
	go netstack.NewTCPEndpointAndListenIt(proto, app)
	if app.Cfg.Udp.Enabled {
		app.WG.Add(1)
		_, err := app.Cfg.UdpProxy()
		if err != nil {
			log.Fatal("Get udp socks 5 proxy failed", err)
		}
		go netstack.NewUDPEndpointAndListenIt(proto, app)
	}
	if app.Cfg.Dns.DnsMode == "fake" {
		app.WG.Add(1)
		go func(app *tun2socks.App) {
			util.UpdateDNSServers(true)
			app.FakeDns.Serve()
			app.WG.Done()
		}(app)

		app.WG.Add(1)
		go func(app *tun2socks.App) {
			// clearExpiredNonProxyDomain and clearExpiredDomain
			app.FakeDns.DnsTablePtr.Serve()
			app.WG.Done()
		}(app)
	}

	app.WG.Wait()
}
