package util

import (
	"fmt"
	"log"
	"net"
	"runtime"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

// CreateDNSResponse with dns data, pack with udp, and ip header.
func CreateDNSResponse(SrcIP net.IP, SrcPort uint16, DstIP net.IP, DstPort uint16, pkt []byte) []byte {
	ip := &layers.IPv4{
		SrcIP:    SrcIP,
		DstIP:    DstIP,
		Protocol: layers.IPProtocolUDP,
		Version:  uint8(4),
		IHL:      uint8(5),
		TTL:      uint8(64),
	}
	udp := &layers.UDP{SrcPort: layers.UDPPort(SrcPort), DstPort: layers.UDPPort(DstPort)}
	if err := udp.SetNetworkLayerForChecksum(ip); err != nil {
		log.Println("SetNetworkLayerForChecksum failed", err)
		return nil
	}
	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{FixLengths: true, ComputeChecksums: true}
	gopacket.SerializeLayers(buf, opts,
		ip,
		udp,
		gopacket.Payload(pkt),
	)

	packetData := buf.Bytes()
	return packetData
}

// UpdateDNSServers ...
func UpdateDNSServers(setFlag bool) {
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
	} else if runtime.GOOS == "linux" || runtime.GOOS == "freebsd" {
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
  nscd -K
  nscd
}
`
	} else if runtime.GOOS == "windows" {
		// FIXME How to get current active network interface in windows?
		var sargs string
		if setFlag {
			sargs = fmt.Sprintf("interface ipv4 add dnsserver \"以太网\" 127.0.0.1 index=1")
		} else {
			sargs = fmt.Sprintf("interface ipv4 add dnsserver \"以太网\" 223.5.5.5 index=1")
		}
		if err := ExecCommand("netsh", sargs); err != nil {
			log.Println("execCommand failed", err)
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
	ExecShell(shell)
}
