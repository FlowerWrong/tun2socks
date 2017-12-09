package util

import (
	"fmt"
	"net"
)

// Add subnet route
func AddRoute(tun string, subnet *net.IPNet) error {
	sargs := fmt.Sprintf("add -net %s dev %s", subnet, tun)
	return ExecCommand("route", sargs)
}

// Add host route
func AddHostRoute(tun string, host string) error {
	sargs := fmt.Sprintf("add -host %s dev %s", host, tun)
	return ExecCommand("route", sargs)
}
