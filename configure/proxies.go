package configure

import (
	"errors"
	"fmt"
	"log"
	"net"

	"github.com/FlowerWrong/proxy"
)

var errNoProxy = errors.New("no proxy")

// Proxies struct
type Proxies struct {
	proxies map[string]*proxy.Proxy
	Default string
}

// Dial a proxy
func (p *Proxies) Dial(proxy string, addr string) (net.Conn, error) {
	if proxy == "" {
		return p.DefaultDial(addr)
	}

	dialer := p.proxies[proxy]
	if dialer != nil {
		return dialer.Dial("tcp", addr)
	}
	return nil, fmt.Errorf("invalid proxy: %s", proxy)
}

// DefaultDial of proxyies
func (p *Proxies) DefaultDial(addr string) (net.Conn, error) {
	dialer := p.proxies[p.Default]
	if dialer == nil {
		return nil, errNoProxy
	}
	return dialer.Dial("tcp", addr)
}

// Reload config
func (p *Proxies) Reload(config map[string]*ProxyConfig) error {
	log.Println("Proxies hot reloaded")
	return p.setUp(config)
}

func (p *Proxies) setUp(config map[string]*ProxyConfig) error {
	proxies := make(map[string]*proxy.Proxy)
	for name, item := range config {
		proxy, err := proxy.FromUrl(item.URL)
		if err != nil {
			return err
		}

		if item.Default || p.Default == "" {
			p.Default = name
		}
		proxies[name] = proxy
	}
	p.proxies = proxies
	log.Printf("[proxies] default proxy: %q", p.Default)
	return nil
}

// NewProxies crate a new proxyies
func NewProxies(config map[string]*ProxyConfig) (*Proxies, error) {
	p := &Proxies{}
	err := p.setUp(config)
	if err != nil {
		return nil, err
	}
	return p, nil
}
