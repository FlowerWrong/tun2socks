package util

import (
	"net"
	"fmt"
)

// Add route
func AddRoute(tun string, subnet *net.IPNet) error {
	sargs := fmt.Sprintf("route add %s dev %s", subnet, tun)
	return ExecCommand("ip", sargs)
}
