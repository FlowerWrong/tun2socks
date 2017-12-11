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
	sargs := fmt.Sprintf("%s %s netmask %s", tunName, ipStr, util.Ipv4MaskString(ipv4Net.Mask))
	if err := util.ExecCommand("ifconfig", sargs); err != nil {
		log.Fatal("execCommand failed", err)
	}
}

func NewTun(app *App) {
	var err error
	app.Ifce, err = water.New(water.Config{
		DeviceType: water.TUN,
	})
	if err != nil {
		log.Fatal("Create tun interface failed", err)
	}
	log.Println("Interface Name:", app.Ifce.Name())
	Ifconfig(app.Ifce.Name(), app.Cfg.General.Network, app.Cfg.General.Mtu)
}
