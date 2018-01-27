package tun2socks

import (
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"
)

// ServePprof ...
func (app *App) ServePprof() error {
	pprofAddr := fmt.Sprintf("%s:%d", app.Cfg.Pprof.ProfHost, app.Cfg.Pprof.ProfPort)
	app.Pprof = &http.Server{Addr: pprofAddr}
	log.Println("[pprof] Http pprof listen on", pprofAddr, " see", fmt.Sprintf("http://%s/debug/pprof/", pprofAddr))
	return app.Pprof.ListenAndServe()
}

// StopPprof ...
func (app *App) StopPprof() error {
	<-QuitPprof
	log.Println("quit http pprof")
	err := app.Pprof.Shutdown(nil)
	if err != nil {
		log.Println(err)
	}
	return err
}
