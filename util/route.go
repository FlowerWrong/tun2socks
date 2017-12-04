package util

import (
	"net"
	"github.com/FlowerWrong/water"
	"github.com/lunny/log"
)

func AddRoutes(vals []string, ifce *water.Interface) {
	name := ifce.Name()
	for _, val := range vals {
		_, subnet, _ := net.ParseCIDR(val)
		if subnet != nil {
			AddRoute(name, subnet)
			log.Printf("add route %s by %s", val, name)
		}
	}
}
