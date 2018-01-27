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
	log.Println("[pprof] Http pprof listen on", pprofAddr, " see", fmt.Sprintf("http://%s/debug/pprof/", pprofAddr))
	return http.ListenAndServe(pprofAddr, nil)
}
