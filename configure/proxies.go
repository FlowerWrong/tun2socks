package configure

import (
	"errors"
	"fmt"
	"net"
	"github.com/xjdrew/proxy"
	"log"
)

var errNoProxy = errors.New("no proxy")

type Proxies struct {
	proxies map[string]*proxy.Proxy
	Default string
}

func (p *Proxies) Dial(proxy string, addr string) (net.Conn, error) {
	if proxy == "" {
		return p.DefaultDial(addr)
	}

	dialer := p.proxies[proxy]
	if dialer != nil {
		return dialer.Dial("tcp", addr)
	}
	return nil, fmt.Errorf("Invalid proxy: %s", proxy)
}

func (p *Proxies) DefaultDial(addr string) (net.Conn, error) {
	dialer := p.proxies[p.Default]
	if dialer == nil {
		return nil, errNoProxy
	}
	return dialer.Dial("tcp", addr)
}

func NewProxies(config map[string]*ProxyConfig) (*Proxies, error) {
	p := &Proxies{}

	proxies := make(map[string]*proxy.Proxy)
	for name, item := range config {
		proxy, err := proxy.FromUrl(item.Url)
		if err != nil {
			return nil, err
		}

		if item.Default || p.Default == "" {
			p.Default = name
		}
		proxies[name] = proxy
	}
	p.proxies = proxies
	log.Printf("[proxies] default proxy: %q", p.Default)
	return p, nil
}
