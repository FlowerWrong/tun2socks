package util

import (
	"fmt"
	"net"
)

// @see https://developer.apple.com/legacy/library/documentation/Darwin/Reference/ManPages/man8/route.8.html
// Add subnet route
func AddNetRoute(tun string, subnet *net.IPNet) error {
	ip := subnet.IP
	maskIP := net.IP(subnet.Mask)
	sargs := fmt.Sprintf("-n add -net %s -netmask %s -interface %s", ip.String(), maskIP.String(), tun)
	return ExecCommand("route", sargs)
}

// Add host route
func AddHostRoute(tun string, host string) error {
	sargs := fmt.Sprintf("-n add -host %s -interface %s", host, tun)
	return ExecCommand("route", sargs)
}
