// code is copy from https://github.com/xjdrew/kone/blob/master/k1/dns_ip_pool.go, Thanks xjdrew
package dns

import (
	"log"
	"net"
	"sync"
	"time"

	"github.com/FlowerWrong/tun2socks/configure"
	"github.com/miekg/dns"
)

// hijacked domain
type DomainRecord struct {
	Hostname string // hostname
	Proxy    string // proxy

	IP      net.IP // nat ip
	RealIP  net.IP // real ip
	Hits    int
	Expires time.Time

	answer *dns.A // cache dns answer
}

func (record *DomainRecord) SetRealIP(msg *dns.Msg) {
	if record.RealIP != nil {
		return
	}

	var ip net.IP
	for _, item := range msg.Answer {
		switch answer := item.(type) {
		case *dns.A:
			ip = answer.A
			break
		}
	}
	record.RealIP = ip
}

func (record *DomainRecord) Answer(request *dns.Msg) *dns.Msg {
	rsp := new(dns.Msg)
	rsp.SetReply(request)
	rsp.RecursionAvailable = true
	rsp.Answer = append(rsp.Answer, record.answer)
	return rsp
}

func (record *DomainRecord) Touch() {
	record.Hits++
	record.Expires = time.Now().Add(configure.DnsDefaultTtl * time.Second)
}

type DnsTable struct {
	// dns ip pool
	ipPool *DnsIPPool

	// hijacked domain records
	records     map[string]*DomainRecord // domain -> record
	ip2Domain   map[string]string        // ip -> domain: map hijacked ip address to domain
	recordsLock sync.Mutex               // protect records and ip2Domain

	nonProxyDomains map[string]time.Time // non proxy domain
	npdLock         sync.Mutex           // protect non proxy domain
}

func (c *DnsTable) get(domain string) *DomainRecord {
	record := c.records[domain]
	if record != nil {
		record.Touch()
	}
	return record
}

func (c *DnsTable) GetByIP(ip net.IP) *DomainRecord {
	c.recordsLock.Lock()
	defer c.recordsLock.Unlock()
	if domain, ok := c.ip2Domain[ip.String()]; ok {
		return c.get(domain)
	}
	return nil
}

func (c *DnsTable) Contains(ip net.IP) bool {
	return c.ipPool.Contains(ip)
}

func (c *DnsTable) Get(domain string) *DomainRecord {
	c.recordsLock.Lock()
	defer c.recordsLock.Unlock()
	return c.get(domain)
}

// forge a IPv4 dns reply
func forgeIPv4Answer(domain string, ip net.IP) *dns.A {
	rr := new(dns.A)
	rr.Hdr = dns.RR_Header{Name: dns.Fqdn(domain), Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: configure.DnsDefaultTtl}
	rr.A = ip.To4()
	return rr
}

func (c *DnsTable) Set(domain string, proxy string) *DomainRecord {
	c.recordsLock.Lock()
	defer c.recordsLock.Unlock()
	record := c.records[domain]
	if record != nil {
		return record
	}

	// alloc a ip
	ip := c.ipPool.Alloc(domain)
	if ip == nil {
		log.Printf("[dns] ip space is used up, domain:%s", domain)
		return nil
	}

	record = new(DomainRecord)
	record.IP = ip
	record.Hostname = domain
	record.Proxy = proxy
	record.answer = forgeIPv4Answer(domain, ip)

	record.Touch()

	c.records[domain] = record
	c.ip2Domain[ip.String()] = domain
	log.Printf("[dns] hijack %s -> %s", domain, ip.String())
	return record
}

func (c *DnsTable) IsNonProxyDomain(domain string) bool {
	c.npdLock.Lock()
	defer c.npdLock.Unlock()
	_, ok := c.nonProxyDomains[domain]
	return ok
}

func (c *DnsTable) SetNonProxyDomain(domain string, ttl uint32) {
	c.npdLock.Lock()
	defer c.npdLock.Unlock()
	c.nonProxyDomains[domain] = time.Now().Add(time.Duration(ttl) * time.Second)
}

func (c *DnsTable) clearExpiredNonProxyDomain(now time.Time) {
	c.npdLock.Lock()
	defer c.npdLock.Unlock()
	for domain, expired := range c.nonProxyDomains {
		if expired.Before(now) {
			delete(c.nonProxyDomains, domain)
			log.Printf("[dns] release non proxy domain: %s", domain)
		}
	}
}

func (c *DnsTable) clearExpiredDomain(now time.Time) {
	c.recordsLock.Lock()
	defer c.recordsLock.Unlock()

	threshold := 1000
	if threshold > c.ipPool.Capacity()/10 {
		threshold = c.ipPool.Capacity() / 10
	}

	if len(c.records) <= threshold {
		return
	}

	for domain, record := range c.records {
		if !record.Expires.Before(now) {
			continue
		}
		delete(c.records, domain)
		delete(c.ip2Domain, record.IP.String())
		c.ipPool.Release(record.IP)
	}
}

func (c *DnsTable) Serve() error {
	tick := time.Tick(60 * time.Second)
	for now := range tick {
		c.clearExpiredDomain(now)
		c.clearExpiredNonProxyDomain(now)
	}
	return nil
}

func (c *DnsTable) Reload(ip net.IP, subnet *net.IPNet) {
	log.Println("Dns table hot reloaded")
	c.npdLock.Lock()
	defer c.npdLock.Unlock()
	for domain, _ := range c.nonProxyDomains {
		delete(c.nonProxyDomains, domain)
	}

	c.recordsLock.Lock()
	defer c.recordsLock.Unlock()
	for domain, record := range c.records {
		delete(c.records, domain)
		delete(c.ip2Domain, record.IP.String())
		c.ipPool.Release(record.IP)
	}
}

func NewDnsTable(ip net.IP, subnet *net.IPNet) *DnsTable {
	c := new(DnsTable)
	c.ipPool = NewDnsIPPool(ip, subnet)
	c.records = make(map[string]*DomainRecord)
	c.ip2Domain = make(map[string]string)
	c.nonProxyDomains = make(map[string]time.Time)
	return c
}
