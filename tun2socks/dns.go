package tun2socks

import (
	"log"
	"time"
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
	<-app.QuitDNS
	log.Println("quit dns")
	err := app.FakeDNS.Server.Shutdown()
	if err != nil {
		log.Println(err)
	}
	return err
}

// ServeClearExpiredDNSTable ...
func (app *App) ServeClearExpiredDNSTable() error {
	tick := time.Tick(60 * time.Second)
	for now := range tick {
		select {
		case <-app.QuitDNSClear:
			log.Println("quit dns clear task")
			return nil
		default:
			app.FakeDNS.DNSTablePtr.ClearExpiredDomain(now)
			app.FakeDNS.DNSTablePtr.ClearExpiredNonProxyDomain(now)
		}
	}
	return nil
}
