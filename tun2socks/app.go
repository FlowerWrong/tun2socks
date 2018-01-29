package tun2socks

import (
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"runtime"

	"github.com/FlowerWrong/netstack/tcpip"
	"github.com/FlowerWrong/netstack/tcpip/stack"
	"github.com/FlowerWrong/tun2socks/configure"
	"github.com/FlowerWrong/tun2socks/dns"
	"github.com/FlowerWrong/tun2socks/util"
	"github.com/FlowerWrong/water"
)

// App struct
type App struct {
	FakeDNS               *dns.DNS
	Pprof                 *http.Server
	Cfg                   *configure.AppConfig
	Proxies               *configure.Proxies
	S                     *stack.Stack
	Ifce                  *water.Interface
	HookPort              uint16
	Version               float64
	NetworkProtocolNumber tcpip.NetworkProtocolNumber
}

// Stop ...
func Stop() {
	defer func() {
		close(QuitTCPNetstack)
		close(QuitUDPNetstack)
		close(QuitDNS)
		close(QuitPprof)
	}()
	if UseTCPNetstack {
		QuitTCPNetstack <- true
	}
	if UseUDPNetstack {
		QuitUDPNetstack <- true
	}
	if UseDNS {
		QuitDNS <- true
	}
	if UsePprof {
		QuitPprof <- true
	}
}

// NewTun create a tun interface
func (app *App) NewTun() *App {
	NewTun(app)
	return app
}

// AddRoutes add route table
func (app *App) AddRoutes() *App {
	name := app.Ifce.Name()
	for _, val := range app.Cfg.Route.V {
		_, subnet, _ := net.ParseCIDR(val)
		if subnet != nil {
			util.AddNetRoute(name, subnet)
		} else {
			util.AddHostRoute(name, val)
		}
	}
	return app
}

// Config parse config from file
func (app *App) Config(configFile string) *App {
	app.Version = 0.5
	// parse config
	app.Cfg = new(configure.AppConfig)
	err := app.Cfg.Parse(configFile)
	if err != nil {
		log.Fatal("Get default proxy failed", err)
	}

	if app.Cfg.DNS.DNSMode == "fake" {
		app.FakeDNS, err = dns.NewFakeDNSServer(app.Cfg)
		if err != nil {
			log.Fatal("New fake dns server failed", err)
		}
	}

	app.Proxies, err = configure.NewProxies(app.Cfg.Proxy)
	if err != nil {
		log.Fatalln("New proxies failed", err)
	}

	return app
}

// ReloadConfig reload config file
func (app *App) ReloadConfig() {
	// parse config
	file := app.Cfg.File
	app.Cfg = new(configure.AppConfig)
	err := app.Cfg.Parse(file)
	if err != nil {
		log.Fatal("Get default proxy failed", err)
	}
	if app.Cfg.DNS.DNSMode == "fake" {
		app.FakeDNS.RulePtr.Reload(app.Cfg.Rule, app.Cfg.Pattern)

		var ip, subnet, _ = net.ParseCIDR(app.Cfg.General.Network)
		app.FakeDNS.DNSTablePtr.Reload(ip, subnet)
	}
	app.Proxies.Reload(app.Cfg.Proxy)
	log.Println("Routes hot reloaded")
	app.AddRoutes()
}

// Exit tun2socks
func (app *App) Exit() {
	if app.Cfg.DNS.AutoConfigSystemDNS {
		app.SetAndResetSystemDNSServers(false)
	}
	Stop()
}

// SetAndResetSystemDNSServers ...
func (app *App) SetAndResetSystemDNSServers(setFlag bool) {
	var shell string
	if runtime.GOOS == "darwin" {
		shell = `
function scutil_query {
  key=$1
  scutil<<EOT
  open
  get $key
  d.show
  close
EOT
}
`
		shell += `
function updateDNS {
  SERVICE_GUID=$(scutil_query State:/Network/Global/IPv4 | grep "PrimaryService" | awk '{print $3}')
  currentservice=$(scutil_query Setup:/Network/Service/$SERVICE_GUID | grep "UserDefinedName" | awk -F': ' '{print $2}')
  echo "Current active networkservice is $currentservice, $SERVICE_GUID"

  olddns=$(networksetup -getdnsservers "$currentservice")

  case "$1" in
    d|default)
      echo "old dns is $olddns, set dns to default"
      networksetup -setdnsservers "$currentservice" empty
      ;;
    g|google)
      echo "old dns is $olddns, set dns to google dns"
      networksetup -setdnsservers "$currentservice" 8.8.8.8 4.4.4.4
      ;;
    a|ali)
      echo "old dns is $olddns, set dns to alidns"
      networksetup -setdnsservers "$currentservice" "223.5.5.5"
      ;;
    l|local)
      echo "old dns is $olddns, set dns to 127.0.0.1"
      networksetup -setdnsservers "$currentservice" "127.0.0.1"
      ;;
    *)
      echo "You have failed to specify what to do correctly."
      exit 1
      ;;
  esac
}

function flushCache {
  sudo dscacheutil -flushcache
  sudo killall -HUP mDNSResponder
}
`
	} else if runtime.GOOS == "linux" {
		shell = `
function updateDNS {
  case "$1" in
    g|google)
      echo "nameserver 8.8.8.8" | sudo tee /etc/resolv.conf
      ;;
    a|ali)
      echo "nameserver 223.5.5.5" | sudo tee /etc/resolv.conf
      ;;
    l|local)
      echo "nameserver 127.0.0.1" | sudo tee /etc/resolv.conf
      ;;
    *)
      echo "You have failed to specify what to do correctly."
      exit 1
      ;;
  esac
}

function flushCache {
	echo "You need to flush cache yourself!!!"
}
`
	} else if runtime.GOOS == "windows" {
		var name string
		var err error
		if app.Cfg.General.Interface == "" {
			name, err = app.ActiveInterfaceName()
			if err != nil {
				log.Println("execCommand failed", err)
				log.Println("NOTE: please setup your network interface (eg Ethernet) dns server to 127.0.0.1 by hand.")
			}
		} else {
			name = app.Cfg.General.Interface
		}

		var sargs string
		if setFlag {
			sargs = fmt.Sprintf("interface ip set dns name=\"%s\" source=static addr=127.0.0.1 register=primary", name)
		} else {
			sargs = fmt.Sprintf("interface ip set dns name=\"%s\" source=dhcp", name)
		}
		err = util.ExecCommand("netsh", sargs)
		if err != nil {
			log.Println("execCommand failed", err)
			log.Println("NOTE: please setup your network interface (eg Ethernet) dns server to 127.0.0.1 by hand.")
		}
		return
	} else {
		log.Println("Without support for", runtime.GOOS)
		return
	}
	if setFlag {
		shell += `
updateDNS l
`
	} else {
		shell += `
updateDNS a
flushCache
`
	}
	util.ExecShell(shell)
}

// ActiveInterfaceName get windows current active interface name
func (app *App) ActiveInterfaceName() (string, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}
	for _, intf := range interfaces {
		log.Println(intf)
	}
	for _, intf := range interfaces {
		if intf.Flags&(1<<0) != 0 && intf.Flags&(1<<2) == 0 && intf.Flags&(1<<3) == 0 && app.Ifce.Name() != intf.Name {
			return intf.Name, nil
		}
	}
	return "", errors.New("not found")
}
