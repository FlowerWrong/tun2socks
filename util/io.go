package util

import (
	"io"
	"net"

	"github.com/FlowerWrong/netstack/tcpip"
)

// Do not log EOF and `use of closed network connection` error.
func IsEOF(err error) bool {
	if err == nil {
		return false
	} else if err == io.EOF {
		return true
	} else if oerr, ok := err.(*net.OpError); ok {
		if oerr.Err.Error() == "use of closed network connection" {
			return true
		}
	} else {
		if err.Error() == "use of closed network connection" {
			return true
		}
	}
	return false
}

// Do not log `endpoint is closed for send`, `endpoint is closed for receive` and `connection reset by peer` error.
func IsClosed(err *tcpip.Error) bool {
	if err == tcpip.ErrClosedForSend || err == tcpip.ErrClosedForReceive || err == tcpip.ErrConnectionReset {
		return true
	}
	return false
}
