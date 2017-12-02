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
	"sync"
)

type TcpTunnel struct {
	wq            *waiter.Queue
	ep            tcpip.Endpoint
	socks5Conn    net.Conn
	RemotePackets chan []byte // write to local
	localPackets  chan []byte // write to remote, socks5
	ctx           context.Context
	ctxCancel     context.CancelFunc
	closeOne      sync.Once    // to avoid multi close tunnel
	status        TunnelStatus // to avoid panic: send on closed channel
	statusMu      sync.Mutex
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
	} else {
		log.Println("No support network", network)
		return nil, errors.New("No support network" + network)
	}

	return &TcpTunnel{
		wq:            wq,
		ep:            ep,
		socks5Conn:    socks5Conn,
		RemotePackets: make(chan []byte, PktChannelSize),
		localPackets:  make(chan []byte, PktChannelSize),
	}, nil
}

func (tcpTunnel *TcpTunnel) SetStatus(s TunnelStatus) {
	tcpTunnel.statusMu.Lock()
	tcpTunnel.status = s
	tcpTunnel.statusMu.Unlock()
}

func (tcpTunnel *TcpTunnel) Status() TunnelStatus {
	tcpTunnel.statusMu.Lock()
	s := tcpTunnel.status
	tcpTunnel.statusMu.Unlock()
	return s
}

func (tcpTunnel *TcpTunnel) Run() {
	tcpTunnel.ctx, tcpTunnel.ctxCancel = context.WithCancel(context.Background())
	go tcpTunnel.writeToLocal()
	go tcpTunnel.readFromRemote()
	go tcpTunnel.writeToRemote()
	go tcpTunnel.readFromLocal()
	tcpTunnel.SetStatus(StatusProxying)
}

func (tcpTunnel *TcpTunnel) readFromLocal() {
	waitEntry, notifyCh := waiter.NewChannelEntry(nil)
	tcpTunnel.wq.EventRegister(&waitEntry, waiter.EventIn)
	defer tcpTunnel.wq.EventUnregister(&waitEntry)

readFromLocal:
	for {
		v, err := tcpTunnel.ep.Read(nil)
		if err != nil {
			if err == tcpip.ErrWouldBlock {
				select {
				case <-tcpTunnel.ctx.Done():
					log.Printf("readFromLocal done because of '%s'", tcpTunnel.ctx.Err())
					break readFromLocal
				case <-notifyCh:
					continue readFromLocal
				case <-time.After(DefaultReadWriteDuration):
					log.Println(err)
					tcpTunnel.Close(errors.New("Read from tun timeout"))
					break readFromLocal
				}
			}
			log.Println(err)
			tcpTunnel.Close(errors.New("ReadFromLocalWriteToRemote failed" + err.String()))
			break readFromLocal
		}
		if tcpTunnel.status != StatusClosed {
			tcpTunnel.localPackets <- v
		} else {
			break readFromLocal
		}
	}
}

func (tcpTunnel *TcpTunnel) writeToRemote() {
writeToRemote:
	for {
		select {
		case <-tcpTunnel.ctx.Done():
			log.Printf("writeToRemote done because of '%s'", tcpTunnel.ctx.Err())
			break writeToRemote
		case chunk := <-tcpTunnel.localPackets:
			// tcpTunnel.socks5Conn.SetWriteDeadline(DefaultReadWriteTimeout)
			_, err := tcpTunnel.socks5Conn.Write(chunk)
			if err != nil {
				log.Println(err)
				tcpTunnel.Close(err)
				break writeToRemote
			}
		}
	}
}

func (tcpTunnel *TcpTunnel) readFromRemote() {
readFromRemote:
	for {
		select {
		case <-tcpTunnel.ctx.Done():
			log.Printf("readFromRemote done because of '%s'", tcpTunnel.ctx.Err())
			break readFromRemote
		default:
			buf := make([]byte, 1500)
			// tcpTunnel.socks5Conn.SetReadDeadline(DefaultReadWriteTimeout)
			n, err := tcpTunnel.socks5Conn.Read(buf)
			if err != nil {
				log.Println(err)
				tcpTunnel.Close(err)
				break readFromRemote
			}

			if n > 0 && tcpTunnel.status != StatusClosed {
				tcpTunnel.RemotePackets <- buf[0:n]
			} else {
				break readFromRemote
			}
		}
	}
}

func (tcpTunnel *TcpTunnel) writeToLocal() {
writeToLocal:
	for {
		select {
		case <-tcpTunnel.ctx.Done():
			log.Printf("WriteToRemote done because of '%s'", tcpTunnel.ctx.Err())
			break writeToLocal
		case chunk := <-tcpTunnel.RemotePackets:
			_, err := tcpTunnel.ep.Write(chunk, nil)
			if err != nil {
				log.Println(err)
				tcpTunnel.Close(errors.New(err.String()))
				break writeToLocal
			}
		}
	}
}

func (tcpTunnel *TcpTunnel) Close(reason error) {
	tcpTunnel.closeOne.Do(func() {
		tcpTunnel.SetStatus(StatusClosed)
		log.Println("Close TCP tunnel because", reason.Error())
		tcpTunnel.ctxCancel()
		err := tcpTunnel.socks5Conn.Close()
		if err != nil {
			log.Println("Close socks5Conn falied", err)
		}
		tcpTunnel.ep.Close()
		close(tcpTunnel.localPackets)
		close(tcpTunnel.RemotePackets)
	})
}
