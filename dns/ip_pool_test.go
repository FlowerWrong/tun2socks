package dns

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
)

var network = "10.192.0.1/16"
var ip, subnet, _ = net.ParseCIDR(network)
var dnsIPPool = NewDnsIPPool(ip, subnet)

func init() {
}

func TestDnsIPPool_Alloc(t *testing.T) {
	dip := dnsIPPool.Alloc("lipuwater.com")
	assert.Equal(t, "10.192.43.74", dip.String(), "they should be equal")
}
