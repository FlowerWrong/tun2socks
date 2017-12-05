package util

import (
	"github.com/FlowerWrong/netstack/tcpip"
	"io"
	"net"
)

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

func IsClosed(err *tcpip.Error) bool {
	if err == tcpip.ErrClosedForSend || err == tcpip.ErrClosedForReceive {
		return true
	}
	return false
}
