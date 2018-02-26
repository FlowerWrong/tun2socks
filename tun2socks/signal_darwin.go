package tun2socks

import (
	"log"
	"os"
	"os/signal"
	"syscall"
)

// SignalHandler of darwin
func (app *App) SignalHandler() *App {
	// signal handler
	c := make(chan os.Signal)

	signal.Notify(c, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGUSR1, syscall.SIGUSR2)
	go func(app *App) {
		for s := range c {
			switch s {
			case syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT:
				log.Println("[signal]", s)
				app.Stop()
			case syscall.SIGUSR1:
				log.Println("[signal]", s)
			case syscall.SIGUSR2:
				log.Println("[signal]", s)
				app.ReloadConfig()
				break
			default:
				log.Println("[signal]", s)
			}
		}
	}(app)

	return app
}
