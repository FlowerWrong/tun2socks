package main

import (
	// "C"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"runtime"
	"time"

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
	StartTun2socks(configFile)
}

// FIXME segmentation fault
//export StopTun2socks
func StopTun2socks() {
	log.Println("stop tun2socks")
	tun2socks.Stop()
}

//export StartTun2socks
func StartTun2socks(configFile string) {
	var app = new(tun2socks.App)
	// app.Config(configFile).NewTun().AddRoutes()
	app.Config(configFile).NewTun().AddRoutes().SignalHandler()
	app.NetworkProtocolNumber = tun2socks.NewNetstack(app)

	wgw := new(util.WaitGroupWrapper)
	wgw.Wrap(func() {
		app.NewTCPEndpointAndListenIt()
	})
	if app.Cfg.UDP.Enabled {
		wgw.Wrap(func() {
			app.NewUDPEndpointAndListenIt()
		})
	}
	if app.Cfg.DNS.DNSMode == "fake" {
		wgw.Wrap(func() {
			app.ServeDNS()
		})
		go app.StopDNS()
	}

	if app.Cfg.Pprof.Enabled {
		wgw.Wrap(func() {
			app.ServePprof()
		})
		go app.StopPprof()
	}

	log.Println(fmt.Sprintf("[app] run tun2socks(%.2f) success", app.Version))
	wgw.WaitGroup.Wait()
}
