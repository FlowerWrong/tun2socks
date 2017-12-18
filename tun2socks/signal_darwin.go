package tun2socks

import (
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/FlowerWrong/tun2socks/configure"
	"github.com/FlowerWrong/tun2socks/util"
)

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
				if app.Cfg.DNS.DNSMode == "fake" {
					app.FakeDNS.RulePtr.Reload(app.Cfg.Rule, app.Cfg.Pattern)

					var ip, subnet, _ = net.ParseCIDR(app.Cfg.General.Network)
					app.FakeDNS.DNSTablePtr.Reload(ip, subnet)
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
