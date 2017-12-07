package util

import (
	"fmt"
	"net"
)

// Add route
func AddRoute(tun string, subnet *net.IPNet) error {
	// route ADD destination_network MASK subnet_mask  gateway_ip metric_cost
	ip := subnet.IP
	maskIP := net.IP(subnet.Mask)
	sargs := fmt.Sprintf("add %s mask %s %s", ip, maskIP, ip)
	return ExecCommand("route", sargs)
	return nil
}
