package util

import (
	"io"
	"net"
	"syscall"

	"github.com/FlowerWrong/netstack/tcpip"
)

// IsEOF do not log EOF and `use of closed network connection` error.
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

// IsClosed do not log `endpoint is closed for send`, `endpoint is closed for receive` and `connection reset by peer` error.
func IsClosed(err *tcpip.Error) bool {
	if err == tcpip.ErrClosedForSend || err == tcpip.ErrClosedForReceive || err == tcpip.ErrConnectionReset {
		return true
	}
	return false
}

// IsTimeout check this is timeout or not
func IsTimeout(err error) bool {
	if err, ok := err.(net.Error); ok && err.Timeout() {
		return true
	}
	return false
}

// IsBrokenPipe check this is broken pipe or not
func IsBrokenPipe(err error) bool {
	if err == syscall.EPIPE {
		return true
	}
	return false
}
