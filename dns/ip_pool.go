// fake dns pool
// code is copy from https://github.com/xjdrew/kone, Thanks xjdrew
package dns

import (
	"github.com/FlowerWrong/tun2socks/configure"
	"github.com/FlowerWrong/tun2socks/util"
	"hash/adler32"
	"log"
	"net"
)

type DnsIPPool struct {
	base  uint32
	space uint32
	flags []bool
}

// Dns ip pool capacity
func (pool *DnsIPPool) Capacity() int {
	return int(pool.space)
}

// Check a ip is in or not in dns ip pool
func (pool *DnsIPPool) Contains(ip net.IP) bool {
	index := util.ConvertIPv4ToUint32(ip) - pool.base
	if index < pool.space {
		return true
	}
	return false
}

// Release a ip from dns ip pool
func (pool *DnsIPPool) Release(ip net.IP) {
	index := util.ConvertIPv4ToUint32(ip) - pool.base
	if index < pool.space {
		pool.flags[index] = false
	}
}

// use domain as a hint to find a stable index
func (pool *DnsIPPool) Alloc(domain string) net.IP {
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

func NewDnsIPPool(ip net.IP, subnet *net.IPNet) *DnsIPPool {
	base := util.ConvertIPv4ToUint32(subnet.IP) + 1
	max := base + ^util.ConvertIPv4ToUint32(net.IP(subnet.Mask))

	// space should not over 0x3ffff
	space := max - base
	if space > configure.DnsIPPoolMaxSpace {
		space = configure.DnsIPPoolMaxSpace
	}
	flags := make([]bool, space)

	// ip is used by tun
	index := util.ConvertIPv4ToUint32(ip) - base
	if index < space {
		flags[index] = true
	}

	return &DnsIPPool{
		base:  base,
		space: space,
		flags: flags,
	}
}
