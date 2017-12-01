package tunnel

import (
	"net"
	"github.com/FlowerWrong/netstack/waiter"
	"github.com/FlowerWrong/netstack/tcpip"
	"log"
	"io"
	"golang.org/x/net/proxy"
	"fmt"
)

type TcpTunnel struct {
	wq         *waiter.Queue
	ep         tcpip.Endpoint
	socks5Conn net.Conn
}

func NewTCP2Socks(wq *waiter.Queue, ep tcpip.Endpoint, network string) *TcpTunnel {
	// connect to socks5
	var socks5Conn net.Conn
	var err error
	local, _ := ep.GetLocalAddress()
	targetAddr := fmt.Sprintf("%v:%d", local.Addr.To4(), local.Port)

	if network == "tcp" {
		dialer, socks5Err := proxy.SOCKS5(network, Socks5Addr, nil, proxy.Direct)
		if socks5Err != nil {
			log.Println("Create SOCKS5 failed", socks5Err)
			return nil
		}
		socks5Conn, err = dialer.Dial(network, targetAddr)
		if err != nil {
			log.Println("Connect to remote via socks5 failed", err)
			return nil
		}
		socks5Conn.SetDeadline(DefaultReadWriteTimeout)
	} else {
		log.Println("No support network", network)
		return nil
	}

	return &TcpTunnel{
		wq,
		ep,
		socks5Conn,
	}
}

func (tcpTunnel *TcpTunnel) ReadFromLocalWriteToRemote() {
	defer tcpTunnel.ep.Close()
	defer tcpTunnel.socks5Conn.Close()

	// Create wait queue entry that notifies a channel.
	waitEntry, notifyCh := waiter.NewChannelEntry(nil)

	tcpTunnel.wq.EventRegister(&waitEntry, waiter.EventIn)
	defer tcpTunnel.wq.EventUnregister(&waitEntry)

	for {
		v, err := tcpTunnel.ep.Read(nil)
		if err != nil {
			if err == tcpip.ErrWouldBlock {
				<-notifyCh
				continue
			}
			log.Println("ReadFromLocalWriteToRemote failed", err)
			return
		}

		go tcpTunnel.ReadFromRemoteWriteToLocal()
		tcpTunnel.socks5Conn.Write(v)
	}
}

func (tcpTunnel *TcpTunnel) ReadFromRemoteWriteToLocal() {
	defer tcpTunnel.ep.Close()
	defer tcpTunnel.socks5Conn.Close()

	buf := make([]byte, 1500)

	for {
		_, err := tcpTunnel.socks5Conn.Read(buf)
		if err != nil {
			if err == io.EOF {
				log.Println("read eof from remote")
				return
			}

			log.Println("ReadFromRemoteWriteToLocal failed", err) // FIXME use of closed network connection
			return
		}

		tcpTunnel.ep.Write(buf, nil)
	}
}
