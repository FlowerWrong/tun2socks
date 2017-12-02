package tunnel

import "time"

var Socks5Addr = "127.0.0.1:1080"

var DefaultConnectDuration = 1 * time.Second
var DefaultReadWriteTimeout = time.Now().Add(time.Second * 60)
var DefaultReadWriteDuration = 10 * time.Second
var WithoutTimeout = time.Time{}
