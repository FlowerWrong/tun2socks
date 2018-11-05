package util

import (
	"io"
	"net"
	"os"
	"runtime"
	"strings"
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

// IsConnectionReset ...
func IsConnectionReset(err error) bool {
	if opErr, ok := err.(*net.OpError); ok {
		if syscallErr, ok := opErr.Err.(*os.SyscallError); ok {
			if syscallErr.Err == syscall.ECONNRESET {
				return true
			}
		}
	}

	if strings.Contains(err.Error(), "connection reset by peer") {
		return true
	}
	return false
}

// IsTimeout check this is timeout or not
func IsTimeout(err error) bool {
	if e, ok := err.(net.Error); ok && e.Timeout() {
		return true
	}
	return false
}

// IsBrokenPipe check this is broken pipe or not
func IsBrokenPipe(err error) bool {
	if e, ok := err.(*net.OpError); ok && e.Err == syscall.EPIPE {
		return true
	}

	if runtime.GOOS == "windows" {
		if strings.Contains(err.Error(), "An established connection was aborted by the software in your host machine") {
			return true
		}
		if strings.Contains(err.Error(), "An existing connection was forcibly closed by the remote host") {
			return true
		}
	}

	// linux and darwin
	if strings.Contains(err.Error(), "broken pipe") {
		return true
	}
	return false
}
