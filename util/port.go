package util

import (
	"github.com/FlowerWrong/netstack/tcpip"
	"github.com/FlowerWrong/netstack/tcpip/stack"
)

// NewRandomPort from stack
func NewRandomPort(stack *stack.Stack) (port uint16) {
	stack.PickEphemeralPort(func(p uint16) (bool, *tcpip.Error) {
		port = p
		return true, nil
	})
	return port
}
