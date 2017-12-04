package tunnel

import "time"

var DefaultConnectDuration = 1 * time.Second
var DefaultReadWriteTimeout = time.Now().Add(time.Second * 60)
var DefaultReadWriteDuration = 10 * time.Second
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

var PktChannelSize = 1500 * 64
