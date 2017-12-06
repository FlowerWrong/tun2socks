package tun2socks

import (
	"github.com/FlowerWrong/netstack/tcpip/stack"
	"github.com/FlowerWrong/tun2socks/configure"
	"github.com/FlowerWrong/tun2socks/dns"
	"github.com/FlowerWrong/tun2socks/util"
	"github.com/FlowerWrong/water"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

type App struct {
	FakeDns  *dns.Dns
	Cfg      *configure.AppConfig
	Proxies  *configure.Proxies
	S        *stack.Stack
	Ifce     *water.Interface
	Fd       int
	HookPort uint16
	WG       sync.WaitGroup
}

func (app *App) NewTun() *App {
	var err error
	app.Ifce, app.Fd, err = water.New(water.Config{
		DeviceType: water.TUN,
	})
	if err != nil {
		log.Fatal("Create tun interface failed", err)
	}
	log.Println("Interface Name:", app.Ifce.Name())
	util.Ifconfig(app.Ifce.Name(), app.Cfg.General.Network, app.Cfg.General.Mtu)
	return app
}

func (app *App) AddRoutes() *App {
	name := app.Ifce.Name()
	for _, val := range app.Cfg.Route.V {
		_, subnet, _ := net.ParseCIDR(val)
		if subnet != nil {
			util.AddRoute(name, subnet)
			log.Printf("add route %s by %s", val, name)
		}
	}
	return app
}

func (app *App) Config(configFile string) *App {
	// parse config
	app.Cfg = new(configure.AppConfig)
	err := app.Cfg.Parse(configFile)
	if err != nil {
		log.Fatal("Get default proxy failed", err)
	}

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

	return app
}

func (app *App) SignalHandler() *App {
	// signal handler
	c := make(chan os.Signal)
	signal.Notify(c, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGUSR1, syscall.SIGUSR2)
	go func(app *App) {
		for s := range c {
			switch s {
			case syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT:
				log.Println("Exit", s)
				util.Exit()
			case syscall.SIGUSR1:
				log.Println("Usr1", s)
			case syscall.SIGUSR2:
				log.Println("Usr2", s)
				// parse config
				file := app.Cfg.File
				app.Cfg = new(configure.AppConfig)
				err := app.Cfg.Parse(file)
				if err != nil {
					log.Fatal("Get default proxy failed", err)
				}
				if app.Cfg.Dns.DnsMode == "fake" {
					app.FakeDns.RulePtr.Reload(app.Cfg.Rule, app.Cfg.Pattern)

					var ip, subnet, _ = net.ParseCIDR(app.Cfg.General.Network)
					app.FakeDns.DnsTablePtr.Reload(ip, subnet)
				}
				app.Proxies.Reload(app.Cfg.Proxy)
				log.Println("Routes hot reloaded")
				app.AddRoutes()
				break
			default:
				log.Println("Other", s)
			}
		}
	}(app)

	return app
}
