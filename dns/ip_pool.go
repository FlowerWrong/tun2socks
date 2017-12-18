// fake dns pool
// code is copy from https://github.com/xjdrew/kone, Thanks xjdrew
package dns

import (
	"hash/adler32"
	"log"
	"net"

	"github.com/FlowerWrong/tun2socks/configure"
	"github.com/FlowerWrong/tun2socks/util"
)

// DNSIPPool struct
type DNSIPPool struct {
	base  uint32
	space uint32
	flags []bool
}

// Capacity is dns ip pool capacity
func (pool *DNSIPPool) Capacity() int {
	return int(pool.space)
}

// Contains check a ip is in or not in dns ip pool
func (pool *DNSIPPool) Contains(ip net.IP) bool {
	index := util.ConvertIPv4ToUint32(ip) - pool.base
	if index < pool.space {
		return true
	}
	return false
}

// Release a ip from dns ip pool
func (pool *DNSIPPool) Release(ip net.IP) {
	index := util.ConvertIPv4ToUint32(ip) - pool.base
	if index < pool.space {
		pool.flags[index] = false
	}
}

// Alloc use domain as a hint to find a stable index
func (pool *DNSIPPool) Alloc(domain string) net.IP {
	index := adler32.Checksum([]byte(domain)) % pool.space
	if pool.flags[index] {
		log.Printf("[dns] %s is not in main index: %d", domain, index)
		for i, used := range pool.flags {
			if !used {
				index = uint32(i)
				break
			}
		}
	}

	if pool.flags[index] {
		return nil
	}
	pool.flags[index] = true
	return util.ConvertUint32ToIPv4(pool.base + index)
}

// NewDNSIPPool create a dns ip pool
func NewDNSIPPool(ip net.IP, subnet *net.IPNet) *DNSIPPool {
	base := util.ConvertIPv4ToUint32(subnet.IP) + 1
	max := base + ^util.ConvertIPv4ToUint32(net.IP(subnet.Mask))

	// space should not over 0x3ffff
	space := max - base
	if space > configure.DNSIPPoolMaxSpace {
		space = configure.DNSIPPoolMaxSpace
	}
	flags := make([]bool, space)

	// ip is used by tun
	index := util.ConvertIPv4ToUint32(ip) - base
	if index < space {
		flags[index] = true
	}

	return &DNSIPPool{
		base:  base,
		space: space,
		flags: flags,
	}
}
