package tunnel

import "time"

var Socks5Addr = "127.0.0.1:1080"

var DefaultConnectTimeout = 1 * time.Second
var DefaultReadWriteTimeout = time.Now().Add(time.Minute * 1)
var WithoutTimeout = time.Time{}