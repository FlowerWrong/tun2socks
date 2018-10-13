package rawfile

import (
	"log"

	"github.com/FlowerWrong/netstack/tcpip"
	"github.com/FlowerWrong/water"
)

// Read from tun device, support OSX, linux and windows
func Read(ifce *water.Interface, b []byte) (int, *tcpip.Error) {
	for {
		n, err := ifce.Read(b)
		if err != nil {
			log.Println("Read from tun failed", err)
			return 0, &tcpip.Error{}
		}
		return n, nil
	}
}
