package tunnel

import "time"

var Socks5Addr = "127.0.0.1:1080"

var DefaultConnectDuration = 1 * time.Second
var DefaultReadWriteTimeout = time.Now().Add(time.Minute * 1)
var DefaultReadWriteDuration = 1 * time.Minute
var WithoutTimeout = time.Time{}
