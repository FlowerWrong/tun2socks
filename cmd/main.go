package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"runtime"
	"time"

	"github.com/FlowerWrong/tun2socks/tun2socks"
	"github.com/FlowerWrong/tun2socks/util"
	"github.com/fatih/color"
)

var app = new(tun2socks.App)

func main() {
	rand.Seed(time.Now().UnixNano())
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	runtime.GOMAXPROCS(runtime.NumCPU())

	app.Version = 0.5
	var version, help bool
	var configFile string
	flag.BoolVar(&version, "v", false, "show version and exit")
	flag.StringVar(&configFile, "c", "", "config file")
	flag.BoolVar(&help, "h", false, "help")
	flag.Parse()

	if version {
		fmt.Printf("tun2socks %s\n", color.New(color.FgRed).SprintFunc()(app.Version))
		os.Exit(0)
	}

	if help {
		fmt.Printf("Version: %s\n", color.New(color.Bold, color.FgGreen).SprintFunc()("sudo go run cmd/main.go -v"))
		fmt.Printf("Usage: %s\n", color.New(color.Bold, color.FgGreen).SprintFunc()("sudo go run cmd/main.go -c=config.example.ini"))
		os.Exit(0)
	}

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
	startTun2socks(configFile)
}

func startTun2socks(configFile string) {
	app.Config(configFile).NewTun().AddRoutes().SignalHandler()
	app.NetworkProtocolNumber = tun2socks.NewNetstack(app)

	wgw := new(util.WaitGroupWrapper)
	tun2socks.UseTCPNetstack = true
	wgw.Wrap(func() {
		app.NewTCPEndpointAndListenIt()
	})
	if app.Cfg.UDP.Enabled {
		tun2socks.UseUDPNetstack = true
		wgw.Wrap(func() {
			app.NewUDPEndpointAndListenIt()
		})
	}
	if app.Cfg.DNS.DNSMode == "fake" {
		go app.FakeDNS.DNSTablePtr.Serve()

		tun2socks.UseDNS = true
		wgw.Wrap(func() {
			app.ServeDNS()
		})
		go app.StopDNS()
	}

	if app.Cfg.Pprof.Enabled {
		tun2socks.UsePprof = true
		wgw.Wrap(func() {
			app.ServePprof()
		})
		go app.StopPprof()
	}

	log.Println(fmt.Sprintf("[app] run tun2socks(%.2f) success", app.Version))
	wgw.WaitGroup.Wait()
}
