package tun2socks

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/FlowerWrong/tun2socks/util"
)

// SignalHandler of linux
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
				app.ReloadConfig()
				break
			default:
				log.Println("Other", s)
			}
		}
	}(app)

	return app
}
