package tunnel

import (
	"context"
	"errors"
	"net"
	"github.com/FlowerWrong/netstack/waiter"
	"github.com/FlowerWrong/netstack/tcpip"
	"log"
	"golang.org/x/net/proxy"
	"fmt"
	"time"
)

type TcpTunnel struct {
	wq            *waiter.Queue
	ep            tcpip.Endpoint
	socks5Conn    net.Conn
	RemotePackets chan []byte // write to local
	localPackets  chan []byte // write to remote, socks5
	ctx           context.Context
	ctxCancel     context.CancelFunc
}

func NewTCP2Socks(wq *waiter.Queue, ep tcpip.Endpoint, network string) (*TcpTunnel, error) {
	// connect to socks5
	var socks5Conn net.Conn
	local, _ := ep.GetLocalAddress()
	targetAddr := fmt.Sprintf("%v:%d", local.Addr.To4(), local.Port)

	if network == "tcp" {
		dialer, err := proxy.SOCKS5(network, Socks5Addr, nil, proxy.Direct)
		if err != nil {
			log.Println("Create SOCKS5 failed", err)
			return nil, err
		}
		socks5Conn, err = dialer.Dial(network, targetAddr)
		if err != nil {
			log.Println("Connect to remote via socks5 failed", err)
			return nil, err
		}
		socks5Conn.SetDeadline(DefaultReadWriteTimeout)
	} else {
		log.Println("No support network", network)
		return nil, errors.New("No support network" + network)
	}

	return &TcpTunnel{
		wq:            wq,
		ep:            ep,
		socks5Conn:    socks5Conn,
		RemotePackets: make(chan []byte, 1500),
		localPackets:  make(chan []byte, 1500),
	}, nil
}

func (tcpTunnel *TcpTunnel) Run() {
	tcpTunnel.ctx, tcpTunnel.ctxCancel = context.WithCancel(context.Background())
	go tcpTunnel.writeToLocal()
	go tcpTunnel.readFromRemote()
	go tcpTunnel.writeToRemote()
	go tcpTunnel.readFromLocal()
}

func (tcpTunnel *TcpTunnel) readFromLocal() {
	waitEntry, notifyCh := waiter.NewChannelEntry(nil)
	tcpTunnel.wq.EventRegister(&waitEntry, waiter.EventIn)
	defer tcpTunnel.wq.EventUnregister(&waitEntry)

ReadFromLocal:
	for {
		v, err := tcpTunnel.ep.Read(nil)
		if err != nil {
			if err == tcpip.ErrWouldBlock {
				select {
				case <-tcpTunnel.ctx.Done():
					log.Printf("ReadFromLocal done because of '%s'", tcpTunnel.ctx.Err())
					break ReadFromLocal
				case <-notifyCh:
					continue ReadFromLocal
				case <-time.After(DefaultReadWriteDuration):
					log.Println(err)
					tcpTunnel.Close(errors.New("Read from tun timeout"))
					break ReadFromLocal
				}
			}
			log.Println(err)
			tcpTunnel.Close(errors.New("ReadFromLocalWriteToRemote failed" + err.String()))
			break ReadFromLocal
		}
		tcpTunnel.localPackets <- v
	}
}

func (tcpTunnel *TcpTunnel) writeToRemote() {
WriteToRemote:
	for {
		select {
		case <-tcpTunnel.ctx.Done():
			log.Printf("WriteToRemote done because of '%s'", tcpTunnel.ctx.Err())
			break WriteToRemote
		case chunk := <-tcpTunnel.localPackets:
			tcpTunnel.socks5Conn.SetWriteDeadline(DefaultReadWriteTimeout)
			_, err := tcpTunnel.socks5Conn.Write(chunk)
			if err != nil {
				log.Println(err)
				tcpTunnel.Close(err)
				break WriteToRemote
			}
		}
	}
}

func (tcpTunnel *TcpTunnel) readFromRemote() {
ReadFromRemote:
	for {
		select {
		case <-tcpTunnel.ctx.Done():
			log.Printf("ReadFromRemote done because of '%s'", tcpTunnel.ctx.Err())
			break ReadFromRemote
		default:
			buf := make([]byte, 1500)
			tcpTunnel.socks5Conn.SetReadDeadline(DefaultReadWriteTimeout)
			n, err := tcpTunnel.socks5Conn.Read(buf)
			if err != nil {
				log.Println(err)
				tcpTunnel.Close(err)
				break ReadFromRemote
			}

			if n > 0 {
				tcpTunnel.RemotePackets <- buf[0:n]
			}
		}
	}
}

func (tcpTunnel *TcpTunnel) writeToLocal() {
WriteToLocal:
	for {
		select {
		case <-tcpTunnel.ctx.Done():
			log.Printf("WriteToRemote done because of '%s'", tcpTunnel.ctx.Err())
			break WriteToLocal
		case chunk := <-tcpTunnel.RemotePackets:
			_, err := tcpTunnel.ep.Write(chunk, nil)
			if err != nil {
				log.Println(err)
				tcpTunnel.Close(errors.New(err.String()))
				break WriteToLocal
			}
		}
	}
}

func (tcpTunnel *TcpTunnel) Close(reason error) {
	log.Println("Close TCP tunnel because", reason.Error())
	tcpTunnel.ctxCancel()
	tcpTunnel.ep.Close()
	tcpTunnel.socks5Conn.Close()
	return
}
