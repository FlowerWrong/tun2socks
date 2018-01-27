package tun2socks

import (
	"log"
)

// ServeDNS ...
func (app *App) ServeDNS() error {
	if app.Cfg.DNS.AutoConfigSystemDNS {
		app.SetAndResetSystemDNSServers(true)
	}
	log.Printf("[dns] listen on %s", app.FakeDNS.Server.Addr)
	return app.FakeDNS.Server.ListenAndServe()
}

// StopDNS ...
func (app *App) StopDNS() error {
	<-QuitDNS
	log.Println("quit dns")
	err := app.FakeDNS.Server.Shutdown()
	if err != nil {
		log.Println(err)
	}
	return err
}
