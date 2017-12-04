package util

import (
	"net"
	"fmt"
)

// Add route
func AddRoute(tun string, subnet *net.IPNet) error {
	ip := subnet.IP
	maskIP := net.IP(subnet.Mask)
	sargs := fmt.Sprintf("-n add -net %s -netmask %s -interface %s", ip.String(), maskIP.String(), tun)
	return ExecCommand("route", sargs)
}
