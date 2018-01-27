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

// ServeClearExpiredDNSTable ...
func (app *App) ServeClearExpiredDNSTable() error {
	tick := time.Tick(60 * time.Second)
	for now := range tick {
		app.FakeDNS.DNSTablePtr.ClearExpiredDomain(now)
		app.FakeDNS.DNSTablePtr.ClearExpiredNonProxyDomain(now)
	}
	return nil
}
