package util

import (
	"fmt"
	"net"
)

// AddNetRoute add subnet route
func AddNetRoute(_ string, subnet *net.IPNet) error {
	// route ADD destination_network MASK subnet_mask gateway_ip metric_cost
	ip := subnet.IP
	maskIP := net.IP(subnet.Mask)
	sargs := fmt.Sprintf("add %s mask %s %s", ip, maskIP, ip)
	return ExecCommand("route", sargs)
	return nil
}

// AddHostRoute add host route
func AddHostRoute(_ string, host string) error {
	sargs := fmt.Sprintf("add %s mask 255.255.255.255 %s", host, host)
	return ExecCommand("route", sargs)
}
