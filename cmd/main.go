package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	_ "net/http/pprof"
	"os"
	"runtime"
	"time"

	"github.com/FlowerWrong/netstack/tcpip"
	"github.com/FlowerWrong/tun2socks/netstack"
	"github.com/FlowerWrong/tun2socks/tun2socks"
	"github.com/FlowerWrong/tun2socks/util"
)

func main() {
	rand.Seed(time.Now().UnixNano())
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	runtime.GOMAXPROCS(runtime.NumCPU())

	config := flag.String("config", "", "config file")
	flag.Parse()
	configFile := *config
	if configFile == "" {
		configFile = flag.Arg(0)
		if configFile == "" {
			if runtime.GOOS == "linux" {
				configFile = "/home/" + os.Getenv("SUDO_USER") + "/.tun2socks/config.ini"
			} else if runtime.GOOS == "darwin" {
				configFile = "/Users/" + os.Getenv("SUDO_USER") + "/.tun2socks/config.ini"
			}
		}
	}
	log.Println("[app] config file path is", configFile)
	RunTun2socks(configFile)
}

//export RunTun2socks
func RunTun2socks(configFile string) {
	app := new(tun2socks.App)
	app.Config(configFile).NewTun().AddRoutes().SignalHandler()

	var proto tcpip.NetworkProtocolNumber
	proto = netstack.NewNetstack(app)

	wgw := new(util.WaitGroupWrapper)
	wgw.Wrap(func() {
		netstack.NewTCPEndpointAndListenIt(proto, app)
	})
	if app.Cfg.UDP.Enabled {
		_, err := app.Cfg.UDPProxy()
		if err != nil {
			log.Fatal("Get udp socks 5 proxy failed", err)
		}
		wgw.Wrap(func() {
			netstack.NewUDPEndpointAndListenIt(proto, app)
		})
	}
	if app.Cfg.DNS.DNSMode == "fake" {
		wgw.Wrap(func() {
			if app.Cfg.DNS.AutoConfigSystemDNS {
				app.SetAndResetSystemDNSServers(true)
			}
			app.FakeDNS.Serve()
		})
		wgw.Wrap(func() {
			// clearExpiredNonProxyDomain and clearExpiredDomain
			app.FakeDNS.DNSTablePtr.Serve()
		})
	}

	if app.Cfg.Pprof.Enabled {
		wgw.Wrap(func() {
			pprofAddr := fmt.Sprintf("%s:%d", app.Cfg.Pprof.ProfHost, app.Cfg.Pprof.ProfPort)
			log.Println("[pprof] Http pprof listen on", pprofAddr, " see", fmt.Sprintf("http://%s/debug/pprof/", pprofAddr))
			http.ListenAndServe(pprofAddr, nil)
		})
	}

	log.Println(fmt.Sprintf("[app] run tun2socks(%.2f) success", app.Version))
	wgw.WaitGroup.Wait()
}
