package util

import (
	"fmt"
	"net"
)

// Add subnet route
func AddRoute(tun string, subnet *net.IPNet) error {
	ip := subnet.IP
	maskIP := net.IP(subnet.Mask)
	sargs := fmt.Sprintf("-n add -net %s -netmask %s -interface %s", ip.String(), maskIP.String(), tun)
	return ExecCommand("route", sargs)
}

// Add host route
func AddHostRoute(tun string, host string) error {
	sargs := fmt.Sprintf("add -host %s dev %s", host, tun)
	return ExecCommand("route", sargs)
}
