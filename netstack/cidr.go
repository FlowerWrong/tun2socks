package netstack

import (
	"net"

	"github.com/yl2chen/cidranger"
)

var IgnoreRanger = cidranger.NewPCTrieRanger()

func init() {
	// @see https://tools.ietf.org/html/rfc1112
	// multicast 224.0.0.0/4 case too many unclosed tcp and udp connections, see `netstat -an | grep '127.0.0.1'`
	_, rfc1112Network, _ := net.ParseCIDR("224.0.0.0/4")
	IgnoreRanger.Insert(cidranger.NewBasicRangerEntry(*rfc1112Network))
}
