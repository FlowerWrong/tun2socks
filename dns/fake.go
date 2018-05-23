// fake dns server
package dns

import (
	"errors"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/FlowerWrong/go-hostsfile"
	"github.com/FlowerWrong/tun2socks/configure"
	"github.com/miekg/dns"
	"github.com/miekg/dns/dnsutil"
	"github.com/xjdrew/proxy"
)

var errResolve = errors.New("resolve error")

// DNS struct
type DNS struct {
	Server      *dns.Server
	client      *dns.Client
	nameservers []string
	RulePtr     *Rule
	DNSTablePtr *DNSTable
}

func isIPv4Query(q dns.Question) bool {
	if q.Qclass == dns.ClassINET && q.Qtype == dns.TypeA {
		return true
	}
	return false
}

func (d *DNS) resolve(r *dns.Msg) (*dns.Msg, error) {
	var wg sync.WaitGroup
	msgCh := make(chan *dns.Msg, 1)

	qname := r.Question[0].Name

	Q := func(ns string) {
		defer wg.Done()

		r, _, err := d.client.Exchange(r, ns)
		if err != nil {
			// eg: write: network is down
			// eg: i/o timeout
			log.Printf("[dns] resolve %s on %s failed: %v", qname, ns, err)
			return
		}

		if r.Rcode == dns.RcodeServerFailure {
			// eg: code 2
			log.Printf("[dns] resolve %s on %s failed: code %d", qname, ns, r.Rcode)
			return
		}

		select {
		case msgCh <- r:
		default:
		}
	}

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for _, ns := range d.nameservers {
		wg.Add(1)
		go Q(ns)

		select {
		case r := <-msgCh:
			return r, nil
		case <-ticker.C:
			continue
		}
	}

	wg.Wait()

	select {
	case r := <-msgCh:
		return r, nil
	default:
		log.Printf("[dns] query %s failed", qname)
		return nil, errResolve
	}
}

func (d *DNS) fillRealIP(record *DomainRecord, r *dns.Msg) {
	// resolve
	msg, err := d.resolve(r)
	if err != nil || len(msg.Answer) == 0 {
		return
	}
	record.SetRealIP(msg)
}

func (d *DNS) doIPv4Query(r *dns.Msg) (*dns.Msg, error) {
	domain := dnsutil.TrimDomainName(r.Question[0].Name, ".")
	// if is a non-proxy-domain
	if d.DNSTablePtr.IsNonProxyDomain(domain) {
		return d.resolve(r)
	}

	// if have already hijacked
	record := d.DNSTablePtr.Get(domain)
	if record != nil {
		return record.Answer(r), nil
	}

	// match by domain
	matched, p := d.RulePtr.Proxy(domain)

	// if domain use proxy
	if matched && p != "" {
		if record := d.DNSTablePtr.Set(domain, p); record != nil {
			// go d.fillRealIP(record, r)
			return record.Answer(r), nil
		}
	}

	// resolve
	msg, err := d.resolve(r)
	if err != nil || len(msg.Answer) == 0 {
		// dns failed
		return msg, err
	}

	if !matched {
		// try match by cname and ip
	OuterLoop:
		for _, item := range msg.Answer {
			switch answer := item.(type) {
			case *dns.A:
				// test ip
				_, p = d.RulePtr.Proxy(answer.A)
				break OuterLoop
			case *dns.CNAME:
				// test cname
				matched, p = d.RulePtr.Proxy(answer.Target)
				if matched && p != "" {
					break OuterLoop
				}
			default:
				log.Printf("[dns] unexpected response %s -> %v", domain, item)
			}
		}
		// if ip use proxy
		if p != "" {
			if record := d.DNSTablePtr.Set(domain, p); record != nil {
				// record.SetRealIP(msg)
				log.Println("[dns] --------------------------", domain, "via proxy", p, "is a proxy domain config it????")
				return record.Answer(r), nil
			}
		} else {
			log.Println("[dns] --------------------------", domain, "is a non-proxy-domain config it????")
		}
	}

	// set domain as a non-proxy-domain
	d.DNSTablePtr.SetNonProxyDomain(domain, msg.Answer[0].Header().Ttl)

	return msg, err
}

func (d *DNS) handler(w dns.ResponseWriter, r *dns.Msg) {
	// /etc/hosts
	domain := dnsutil.TrimDomainName(r.Question[0].Name, ".")
	ip, err := hostsfile.Lookup(domain)
	if err == nil && ip != "" {
		rsp := new(dns.Msg)
		rsp.SetReply(r)
		rsp.RecursionAvailable = true
		rsp.Answer = append(rsp.Answer, ForgeIPv4Answer(domain, net.ParseIP(ip)))
		w.WriteMsg(rsp)
		return
	}

	isIPv4 := isIPv4Query(r.Question[0])

	var msg *dns.Msg

	if isIPv4 {
		msg, err = d.doIPv4Query(r)
	} else {
		msg, err = d.resolve(r)
	}

	if err != nil {
		dns.HandleFailed(w, r)
	} else {
		w.WriteMsg(msg)
	}
}

// Serve run a dns server
func (d *DNS) Serve() error {
	log.Printf("[dns] listen on %s", d.Server.Addr)
	return d.Server.ListenAndServe()
}

// NewFakeDNSServer create a fake dns srever with config
func NewFakeDNSServer(cfg *configure.AppConfig) (*DNS, error) {
	d := new(DNS)

	server := &dns.Server{
		Net:          "udp",
		Addr:         fmt.Sprintf("%s:%d", net.IPv4zero, cfg.DNS.DNSPort),
		Handler:      dns.HandlerFunc(d.handler),
		UDPSize:      int(cfg.DNS.DNSPacketSize),
		ReadTimeout:  time.Duration(cfg.DNS.DNSReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.DNS.DNSWriteTimeout) * time.Second,
	}

	client := &dns.Client{
		Net:          "udp",
		UDPSize:      cfg.DNS.DNSPacketSize,
		ReadTimeout:  time.Duration(cfg.DNS.DNSReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.DNS.DNSWriteTimeout) * time.Second,
	}

	d.nameservers = cfg.DNS.Nameserver
	d.Server = server
	d.client = client

	var ip, subnet, _ = net.ParseCIDR(cfg.General.Network)
	// new RulePtr
	d.RulePtr = NewRule(cfg.Rule, cfg.Pattern)

	// new dns cache
	d.DNSTablePtr = NewDnsTable(ip, subnet)

	// don't hijack proxy domain
	for _, item := range cfg.Proxy {
		p, err := proxy.FromUrl(item.URL)
		if err != nil {
			return nil, err
		}
		host := p.Url.Host
		index := strings.IndexByte(p.Url.Host, ':')
		if index > 0 {
			host = p.Url.Host[:index]
		}
		d.RulePtr.DirectDomain(host)
	}

	return d, nil
}
