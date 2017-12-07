package tun2socks

import (
	"github.com/FlowerWrong/water"
	"log"
)

func Ifconfig(_, _ string, _ uint32) {
}

func NewTun(app *App) {
	var err error
	app.Ifce, app.Fd, err = water.New(water.Config{
		DeviceType: water.TUN,
		PlatformSpecificParams: water.PlatformSpecificParams{
			ComponentID: "tap0901",
			Network:     app.Cfg.General.Network,
		},
	})
	if err != nil {
		log.Fatal("Create tun interface failed", err)
	}
	log.Println("Interface Name:", app.Ifce.Name())
	Ifconfig(app.Ifce.Name(), app.Cfg.General.Network, app.Cfg.General.Mtu)
}
