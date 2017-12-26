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
	"strings"
	"time"

	"github.com/FlowerWrong/netstack/tcpip"
	"github.com/FlowerWrong/tun2socks/netstack"
	"github.com/FlowerWrong/tun2socks/tun2socks"
	"github.com/FlowerWrong/tun2socks/util"
	"github.com/fsnotify/fsnotify"
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
		if configFile == "" {
			if runtime.GOOS == "linux" {
				configFile = "/home/" + os.Getenv("SUDO_USER") + "/.tun2socks/config.ini"
			} else if runtime.GOOS == "darwin" {
				configFile = "/Users/" + os.Getenv("SUDO_USER") + "/.tun2socks/config.ini"
			}
		}
	}
	log.Println("config file is", configFile)

	app := new(tun2socks.App)
	app.Config(configFile).NewTun().AddRoutes().SignalHandler()

	var proto tcpip.NetworkProtocolNumber
	proto = netstack.NewNetstack(app)

	app.WG.Add(1)
	go netstack.NewTCPEndpointAndListenIt(proto, app)
	if app.Cfg.UDP.Enabled {
		app.WG.Add(1)
		_, err := app.Cfg.UDPProxy()
		if err != nil {
			log.Fatal("Get udp socks 5 proxy failed", err)
		}
		go netstack.NewUDPEndpointAndListenIt(proto, app)
	}
	if app.Cfg.DNS.DNSMode == "fake" {
		app.WG.Add(1)
		go func(app *tun2socks.App) {
			util.UpdateDNSServers(true)
			app.FakeDNS.Serve()
			app.WG.Done()
		}(app)

		app.WG.Add(1)
		go func(app *tun2socks.App) {
			// clearExpiredNonProxyDomain and clearExpiredDomain
			app.FakeDNS.DNSTablePtr.Serve()
			app.WG.Done()
		}(app)
	}

	if app.Cfg.Pprof.Enabled {
		app.WG.Add(1)
		go func(app *tun2socks.App) {
			pprofAddr := fmt.Sprintf("%s:%d", app.Cfg.Pprof.ProfHost, app.Cfg.Pprof.ProfPort)
			log.Println("Http pprof listen on", pprofAddr, " see", fmt.Sprintf("http://%s/debug/pprof/", pprofAddr))
			http.ListenAndServe(pprofAddr, nil)
			app.WG.Done()
		}(app)
	}

	// fsnotify to watch config file
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()
	app.WG.Add(1)
	go func(app *tun2socks.App, configFile string) {
		defer app.WG.Done()
		var t *time.Timer
		for {
			select {
			case event := <-watcher.Events:
				if event.Op&fsnotify.Write == fsnotify.Write {
					if strings.HasSuffix(configFile, event.Name) {
						// to avoid double trigger write events
						if t == nil {
							log.Println("config file:", event.Name, "modified, now reload")
							t = time.NewTimer(time.Second * 2)
							app.ReloadConfig()
							go func() {
								<-t.C
								t = nil
							}()
						}
					}
				}
			case watcherErr := <-watcher.Errors:
				log.Println("error:", watcherErr)
			}
		}
	}(app, configFile)

	err = watcher.Add(configFile)
	if err != nil {
		log.Fatal(err)
	}

	app.WG.Wait()
}
