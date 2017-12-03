// fake dns server
package dns

import (
	"errors"
	"fmt"
	"github.com/FlowerWrong/tun2socks/configure"
	"github.com/miekg/dns"
	"github.com/miekg/dns/dnsutil"
	"log"
	"net"
	"sync"
	"time"
)

var resolveErr = errors.New("resolve error")

type Dns struct {
	server      *dns.Server
	client      *dns.Client
	nameservers []string
	rule        *Rule
	dnsTable    *DnsTable
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

		r, rtt, err := d.client.Exchange(r, ns)
		if err != nil {
			log.Println("[dns] resolve %s on %s failed: %v", qname, ns, err)
			return
		}

		if r.Rcode == dns.RcodeServerFailure {
			log.Println("[dns] resolve %s on %s failed: code %d", qname, ns, r.Rcode)
			return
		}

		log.Println("[dns] resolve %s on %s, code: %d, rtt: %d", qname, ns, r.Rcode, rtt)

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
		log.Println("[dns] query %s failed", qname)
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
	if d.dnsTable.IsNonProxyDomain(domain) {
		return d.resolve(r)
	}

	// if have already hijacked
	record := d.dnsTable.Get(domain)
	if record != nil {
		return record.Answer(r), nil
	}

	// match by domain
	matched, proxy := d.rule.Proxy(domain)

	// if domain use proxy
	if matched && proxy != "" {
		if record := d.dnsTable.Set(domain, proxy); record != nil {
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
				_, proxy = d.rule.Proxy(answer.A)
				break OuterLoop
			case *dns.CNAME:
				// test cname
				matched, proxy = d.rule.Proxy(answer.Target)
				if matched && proxy != "" {
					break OuterLoop
				}
			default:
				log.Println("[dns] unexpected response %s -> %v", domain, item)
			}
		}
		// if ip use proxy
		if proxy != "" {
			if record := d.dnsTable.Set(domain, proxy); record != nil {
				record.SetRealIP(msg)
				return record.Answer(r), nil
			}
		}
	}

	// set domain as a non-proxy-domain
	d.dnsTable.SetNonProxyDomain(domain, msg.Answer[0].Header().Ttl)

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
		Addr:         fmt.Sprintf("%s:%d", net.IPv4zero, 53),
		Handler:      dns.HandlerFunc(d.handler),
		UDPSize:      4096,
		ReadTimeout:  time.Duration(5) * time.Second,
		WriteTimeout: time.Duration(5) * time.Second,
	}

	client := &dns.Client{
		Net:          "udp",
		UDPSize:      4096,
		ReadTimeout:  time.Duration(5) * time.Second,
		WriteTimeout: time.Duration(5) * time.Second,
	}

	d.nameservers = append(d.nameservers, "114.114.114.114")
	d.nameservers = append(d.nameservers, "223.5.5.5")
	d.server = server
	d.client = client

	var network = "10.192.0.1/16"
	var ip, subnet, _ = net.ParseCIDR(network)
	// var dnsIPPool = NewDnsIPPool(ip, subnet)
	// new rule
	d.rule = NewRule(cfg.Rule, cfg.Pattern)

	// new dns cache
	d.dnsTable = NewDnsTable(ip, subnet)

	return d, nil
}
