package dns

import (
	"time"
	"sync"
	"github.com/miekg/dns"
	"log"
)

type dnsCacheEntry struct {
	msg *dns.Msg
	exp time.Time
}

type dnsCache struct {
	rwMutex sync.RWMutex
	storage map[string]*dnsCacheEntry
}

// Create a dnsCache struct pointer
func NewDnsCache() *dnsCache {
	return &dnsCache{
		rwMutex: sync.RWMutex{},
		storage: make(map[string]*dnsCacheEntry),
	}
}

// Global dnsCache struct pointer
var DNSCache = NewDnsCache()

// Pack uint 16
func packUint16(i uint16) []byte { return []byte{byte(i >> 8), byte(i)} }

// Cache key
func cacheKey(q dns.Question) string {
	return string(append([]byte(q.Name), packUint16(q.Qtype)...))
}

// Query a dns msg from cache with dns []byte
func (c *dnsCache) Query(payload []byte) *dns.Msg {
	request := new(dns.Msg)
	err := request.Unpack(payload)
	if err != nil {
		log.Println("Unpack request failed", err)
		return nil
	}
	if len(request.Question) == 0 {
		log.Println("Without question")
		return nil
	}

	c.rwMutex.Lock()
	defer c.rwMutex.Unlock()
	key := cacheKey(request.Question[0])
	entry := c.storage[key]
	if entry == nil {
		log.Println("Without entry")
		return nil
	}
	if time.Now().After(entry.exp) {
		delete(c.storage, key)
		return nil
	}
	entry.msg.Id = request.Id
	return entry.msg
}

// Store a dns msg to cache with dns []byte
func (c *dnsCache) Store(payload []byte) {
	dnsMsg := new(dns.Msg)
	err := dnsMsg.Unpack(payload)
	if err != nil {
		log.Println("Unpack chunk failed", err)
		return
	}
	if dnsMsg.Rcode != dns.RcodeSuccess {
		log.Println("RcodeSuccess failed")
		return
	}
	if len(dnsMsg.Question) == 0 || len(dnsMsg.Answer) == 0 {
		log.Println("Question or Answer failed")
		return
	}

	c.rwMutex.Lock()
	defer c.rwMutex.Unlock()
	key := cacheKey(dnsMsg.Question[0])
	log.Printf("cache DNS response for %s", key)
	c.storage[key] = &dnsCacheEntry{
		msg: dnsMsg,
		exp: time.Now().Add(time.Duration(dnsMsg.Answer[0].Header().Ttl) * time.Second),
	}
}
