// fake dns server
package dns

import (
	"errors"
	"fmt"
	"github.com/FlowerWrong/tun2socks/configure"
	"github.com/miekg/dns"
	"github.com/miekg/dns/dnsutil"
	"github.com/xjdrew/proxy"
	"log"
	"net"
	"strings"
	"sync"
	"time"
)

var resolveErr = errors.New("resolve error")

type Dns struct {
	server      *dns.Server
	client      *dns.Client
	nameservers []string
	RulePtr     *Rule
	DnsTablePtr *DnsTable
}

func isIPv4Query(q dns.Question) bool {
	if q.Qclass == dns.ClassINET && q.Qtype == dns.TypeA {
		return true
	}
	return false
}

func (d *Dns) resolve(r *dns.Msg) (*dns.Msg, error) {
	var wg sync.WaitGroup
	msgCh := make(chan *dns.Msg, 1)

	qname := r.Question[0].Name

	Q := func(ns string) {
		defer wg.Done()

		r, _, err := d.client.Exchange(r, ns)
		if err != nil {
			log.Printf("[dns] resolve %s on %s failed: %v", qname, ns, err)
			return
		}

		if r.Rcode == dns.RcodeServerFailure {
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
		return nil, resolveErr
	}
}

func (d *Dns) fillRealIP(record *DomainRecord, r *dns.Msg) {
	// resolve
	msg, err := d.resolve(r)
	if err != nil || len(msg.Answer) == 0 {
		return
	}
	record.SetRealIP(msg)
}

func (d *Dns) doIPv4Query(r *dns.Msg) (*dns.Msg, error) {
	domain := dnsutil.TrimDomainName(r.Question[0].Name, ".")
	// if is a non-proxy-domain
	if d.DnsTablePtr.IsNonProxyDomain(domain) {
		return d.resolve(r)
	}

	// if have already hijacked
	record := d.DnsTablePtr.Get(domain)
	if record != nil {
		return record.Answer(r), nil
	}

	// match by domain
	matched, proxy := d.RulePtr.Proxy(domain)

	// if domain use proxy
	if matched && proxy != "" {
		if record := d.DnsTablePtr.Set(domain, proxy); record != nil {
			go d.fillRealIP(record, r)
			return record.Answer(r), nil
		}
	}

	// resolve
	msg, err := d.resolve(r)
	if err != nil || len(msg.Answer) == 0 {
		return msg, err
	}

	if !matched {
		// try match by cname and ip
	OuterLoop:
		for _, item := range msg.Answer {
			switch answer := item.(type) {
			case *dns.A:
				// test ip
				_, proxy = d.RulePtr.Proxy(answer.A)
				break OuterLoop
			case *dns.CNAME:
				// test cname
				matched, proxy = d.RulePtr.Proxy(answer.Target)
				if matched && proxy != "" {
					break OuterLoop
				}
			default:
				log.Printf("[dns] unexpected response %s -> %v", domain, item)
			}
		}
		// if ip use proxy
		if proxy != "" {
			if record := d.DnsTablePtr.Set(domain, proxy); record != nil {
				record.SetRealIP(msg)
				return record.Answer(r), nil
			}
		}
	}

	// set domain as a non-proxy-domain
	d.DnsTablePtr.SetNonProxyDomain(domain, msg.Answer[0].Header().Ttl)

	// final
	return msg, err
}

func (d *Dns) handler(w dns.ResponseWriter, r *dns.Msg) {
	isIPv4 := isIPv4Query(r.Question[0])

	var msg *dns.Msg
	var err error

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

func (d *Dns) Serve() error {
	log.Printf("[dns] listen on %s", d.server.Addr)
	return d.server.ListenAndServe()
}

func NewFakeDnsServer(cfg *configure.AppConfig) (*Dns, error) {
	d := new(Dns)

	server := &dns.Server{
		Net:          "udp",
		Addr:         fmt.Sprintf("%s:%d", net.IPv4zero, cfg.Dns.DnsPort),
		Handler:      dns.HandlerFunc(d.handler),
		UDPSize:      int(cfg.Dns.DnsPacketSize),
		ReadTimeout:  time.Duration(cfg.Dns.DnsReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.Dns.DnsWriteTimeout) * time.Second,
	}

	client := &dns.Client{
		Net:          "udp",
		UDPSize:      cfg.Dns.DnsPacketSize,
		ReadTimeout:  time.Duration(cfg.Dns.DnsReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.Dns.DnsWriteTimeout) * time.Second,
	}

	d.nameservers = cfg.Dns.Nameserver
	d.server = server
	d.client = client

	var ip, subnet, _ = net.ParseCIDR(cfg.General.Network)
	// new RulePtr
	d.RulePtr = NewRule(cfg.Rule, cfg.Pattern)

	// new dns cache
	d.DnsTablePtr = NewDnsTable(ip, subnet)

	// don't hijack proxy domain
	for _, item := range cfg.Proxy {
		proxy, err := proxy.FromUrl(item.Url)
		if err != nil {
			return nil, err
		}
		host := proxy.Url.Host
		index := strings.IndexByte(proxy.Url.Host, ':')
		if index > 0 {
			host = proxy.Url.Host[:index]
		}
		d.RulePtr.DirectDomain(host)
	}

	return d, nil
}
