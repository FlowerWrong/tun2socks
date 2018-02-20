package tun2socks

import "time"

// DefaultConnectDuration 1s
var DefaultConnectDuration = 1 * time.Second

// WithoutTimeout no timeout
var WithoutTimeout = time.Time{}

var (
	QuitTCPNetstack = make(chan struct{})
	QuitUDPNetstack = make(chan struct{})
	UseTCPNetstack  = false
	UseUDPNetstack  = false
	UseDNS          = false
	UsePprof        = false
)

// TunnelStatus struct
type TunnelStatus uint

const (
	StatusNew              TunnelStatus = iota // 0
	StatusConnecting                           // 1
	StatusConnectionFailed                     // 2
	StatusConnected                            // 3
	StatusProxying                             // 5
	StatusClosing                              // 6
	StatusClosed                               // 7
)

// BuffSize is default tcp and udp buffer size
const BuffSize = 1500

// PktChannelSize is default packet recv and send buffer size
const PktChannelSize = BuffSize * 4
