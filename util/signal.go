package util

import (
	"os"
)

// Exit tun2socks
func Exit() {
	UpdateDNSServers(false)
	os.Exit(0)
}
