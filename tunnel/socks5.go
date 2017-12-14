package tunnel

import "time"

var DefaultConnectDuration = 1 * time.Second
var WithoutTimeout = time.Time{}

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

const BuffSize = 1500
const PktChannelSize = BuffSize * 4
