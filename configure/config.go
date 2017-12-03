package configure

import "gopkg.in/gcfg.v1"

const (
	dnsDefaultPort         = 53
	dnsDefaultTtl          = 600
	dnsDefaultPacketSize   = 4096
	dnsDefaultReadTimeout  = 5
	dnsDefaultWriteTimeout = 5
)

type GeneralConfig struct {
	Network      string // tun network
	NetstackAddr string `gcfg:"netstack-addr"`
	NetstackPort uint16 `gcfg:"netstack-port"`
}

type DnsConfig struct {
	DnsPort         uint16 `gcfg:"dns-port"`
	DnsTtl          uint   `gcfg:"dns-ttl"`
	DnsPacketSize   uint16 `gcfg:"dns-packet-size"`
	DnsReadTimeout  uint   `gcfg:"dns-read-timeout"`
	DnsWriteTimeout uint   `gcfg:"dns-write-timeout"`
	Nameserver      []string // backend dns
}

type RouteConfig struct {
	V []string
}

type PatternConfig struct {
	Proxy  string
	Scheme string
	V      []string
}

type RuleConfig struct {
	Pattern []string
	Final   string
}

type ProxyConfig struct {
	Url     string
	Default bool
}

type AppConfig struct {
	General GeneralConfig
	Dns     DnsConfig
	Route   RouteConfig
	Proxy   map[string]*ProxyConfig
	Pattern map[string]*PatternConfig
	Rule    RuleConfig
}

func (cfg *AppConfig) check() error {
	// TODO
	return nil
}

func Parse(filename string) (*AppConfig, error) {
	cfg := new(AppConfig)

	// set default value
	cfg.General.Network = "10.192.0.1/16"
	cfg.General.NetstackAddr = "10.192.0.2"
	cfg.General.NetstackPort = 7777

	cfg.Dns.DnsPort = dnsDefaultPort
	cfg.Dns.DnsTtl = dnsDefaultTtl
	cfg.Dns.DnsPacketSize = dnsDefaultPacketSize
	cfg.Dns.DnsReadTimeout = dnsDefaultReadTimeout
	cfg.Dns.DnsWriteTimeout = dnsDefaultWriteTimeout

	// decode config value
	err := gcfg.ReadFileInto(cfg, filename)
	if err != nil {
		return nil, err
	}

	// set backend dns default value
	if len(cfg.Dns.Nameserver) == 0 {
		cfg.Dns.Nameserver = append(cfg.Dns.Nameserver, "114.114.114.114")
		cfg.Dns.Nameserver = append(cfg.Dns.Nameserver, "223.5.5.5")
	}

	err = cfg.check()
	if err != nil {
		return nil, err
	}

	return cfg, nil
}
