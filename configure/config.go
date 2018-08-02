package configure

import (
	"errors"
	"log"
	"net/url"

	"gopkg.in/gcfg.v1"
)

const (
	DNSDefaultPort         = 53
	DNSDefaultTTL          = 600
	DNSDefaultPacketSize   = 4096
	DNSDefaultReadTimeout  = 5
	DNSDefaultWriteTimeout = 5
	DNSIPPoolMaxSpace      = 0x3ffff // 4*65535
)

// GeneralConfig ini
type GeneralConfig struct {
	Network   string // tun network
	Mtu       uint32
	Interface string
}

// PprofConfig ini
type PprofConfig struct {
	Enabled  bool
	ProfHost string `gcfg:"prof-host"`
	ProfPort uint16 `gcfg:"prof-port"`
}

// DNSConfig ini
type DNSConfig struct {
	DNSMode             string   `gcfg:"dns-mode"`
	DNSPort             uint16   `gcfg:"dns-port"`
	DNSTtl              uint     `gcfg:"dns-ttl"`
	DNSPacketSize       uint16   `gcfg:"dns-packet-size"`
	DNSReadTimeout      uint     `gcfg:"dns-read-timeout"`
	DNSWriteTimeout     uint     `gcfg:"dns-write-timeout"`
	AutoConfigSystemDNS bool     `gcfg:"auto-config-system-dns"`
	Nameserver          []string // backend dns
	OriginNameserver    string
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
	URL     string
	Default bool
}

type UDPConfig struct {
	Proxy   string
	Enabled bool
	Timeout int
}

type TCPConfig struct {
	Timeout int
}

type AppConfig struct {
	General GeneralConfig
	Pprof   PprofConfig
	DNS     DNSConfig
	UDP     UDPConfig
	TCP     TCPConfig
	Route   RouteConfig
	Proxy   map[string]*ProxyConfig
	Pattern map[string]*PatternConfig
	Rule    RuleConfig
	File    string
}

func (cfg *AppConfig) check() error {
	// TODO
	return nil
}

// Parse the config.ini file to AppConfig
func (cfg *AppConfig) Parse(filename string) error {
	// set default value
	cfg.General.Network = "198.18.0.0/15"
	cfg.General.Mtu = 1500
	cfg.General.Interface = ""

	cfg.Pprof.Enabled = false
	cfg.Pprof.ProfHost = "127.0.0.1"
	cfg.Pprof.ProfPort = 6060

	cfg.DNS.DNSMode = "fake"
	cfg.DNS.DNSPort = DNSDefaultPort
	cfg.DNS.DNSTtl = DNSDefaultTTL
	cfg.DNS.DNSPacketSize = DNSDefaultPacketSize
	cfg.DNS.DNSReadTimeout = DNSDefaultReadTimeout
	cfg.DNS.DNSWriteTimeout = DNSDefaultWriteTimeout
	cfg.DNS.OriginNameserver = "" // TODO
	cfg.DNS.AutoConfigSystemDNS = true

	cfg.TCP.Timeout = 60

	cfg.UDP.Enabled = true
	cfg.UDP.Timeout = 300

	// decode config value
	err := gcfg.ReadFileInto(cfg, filename)
	if err != nil {
		return err
	}

	// set backend dns default value
	if len(cfg.DNS.Nameserver) == 0 {
		cfg.DNS.Nameserver = append(cfg.DNS.Nameserver, "114.114.114.114:53")
		cfg.DNS.Nameserver = append(cfg.DNS.Nameserver, "223.5.5.5:53")
	}

	err = cfg.check()
	if err != nil {
		return err
	}

	cfg.File = filename
	return nil
}

// GetProxy addr from name
func (cfg *AppConfig) GetProxy(name string) string {
	proxyConfig := cfg.Proxy[name]
	url, err := url.Parse(proxyConfig.URL)
	if err != nil {
		log.Println("Parse url failed", err)
		return ""
	}
	return url.Host
}

// DefaultPorxy return default proxy addr, eg: socks5://127.0.0.1:1080, return 127.0.0.1:1080
func (cfg *AppConfig) DefaultPorxy() (string, error) {
	proxyConfig := cfg.DefaultPorxyConfig()
	u, err := url.Parse(proxyConfig.URL)
	if err != nil {
		log.Println("Parse url failed", err)
		return "", err
	}
	return u.Host, nil
}

// DefaultPorxyConfig return the default ProxyConfig pointer
func (cfg *AppConfig) DefaultPorxyConfig() *ProxyConfig {
	for _, proxyConfig := range cfg.Proxy {
		if proxyConfig.Default {
			return proxyConfig
		}
	}
	return nil
}

// UDPProxy return the configed udp proxy
func (cfg *AppConfig) UDPProxy() (string, error) {
	proxyConfig := cfg.Proxy[cfg.UDP.Proxy]
	if proxyConfig == nil {
		proxyConfig = cfg.DefaultPorxyConfig()
	}
	if proxyConfig != nil {
		u, err := url.Parse(proxyConfig.URL)
		if err != nil {
			log.Println("Parse url failed", err)
			return "", err
		}
		return u.Host, nil
	}

	return "", errors.New("404")
}
