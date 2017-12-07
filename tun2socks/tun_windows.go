package tun2socks

import (
	"fmt"
	"github.com/FlowerWrong/tun2socks/util"
	"github.com/FlowerWrong/water"
	"log"
	"net"
)

func Ifconfig(tunName, network string, _ uint32) {
	var ip, ipv4Net, _ = net.ParseCIDR(network)
	ipStr := ip.To4().String()
	sargs := fmt.Sprintf("interface ip set address \"%s\" static %s %s none", tunName, ipStr, util.Ipv4MaskString(ipv4Net.Mask))
	if err := util.ExecCommand("netsh", sargs); err != nil {
		log.Fatal("execCommand failed", err)
	}
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
