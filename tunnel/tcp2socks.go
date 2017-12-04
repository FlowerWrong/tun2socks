package tunnel

import (
	"context"
	"errors"
	"net"
	"github.com/FlowerWrong/netstack/waiter"
	"github.com/FlowerWrong/netstack/tcpip"
	"log"
	"time"
	"sync"
	"fmt"
	"github.com/FlowerWrong/tun2socks/dns"
	"github.com/FlowerWrong/tun2socks/configure"
)

// Tcp tunnel
type TcpTunnel struct {
	wq            *waiter.Queue
	ep            tcpip.Endpoint
	socks5Conn    net.Conn
	remotePackets chan []byte // write to local
	localPackets  chan []byte // write to remote, socks5
	ctx           context.Context
	ctxCancel     context.CancelFunc
	closeOne      sync.Once    // to avoid multi close tunnel
	status        TunnelStatus // to avoid panic: send on closed channel
	rwMutex       sync.RWMutex
}

// Create a tcp tunnel
func NewTCP2Socks(wq *waiter.Queue, ep tcpip.Endpoint, ip net.IP, port uint16, fakeDns *dns.Dns, proxies *configure.Proxies) (*TcpTunnel, error) {
	socks5Conn, err := NewSocks5Conneciton(ip, port, fakeDns, proxies)
	if err != nil {
		log.Println("New socks5 conn failed", err)
		return nil, err
	}

	return &TcpTunnel{
		wq:            wq,
		ep:            ep,
		socks5Conn:    *socks5Conn,
		remotePackets: make(chan []byte, PktChannelSize),
		localPackets:  make(chan []byte, PktChannelSize),
		rwMutex:       sync.RWMutex{},
	}, nil
}

// New socks5 connection
func NewSocks5Conneciton(ip net.IP, port uint16, fakeDns *dns.Dns, proxies *configure.Proxies) (*net.Conn, error) {
	var remoteAddr string
	proxy := ""

	record := fakeDns.DnsTablePtr.GetByIP(ip)
	if record != nil {
		remoteAddr = fmt.Sprintf("%v:%d", record.Hostname, port)
		proxy = record.Proxy
	} else {
		remoteAddr = fmt.Sprintf("%v:%d", ip, port)
	}

	socks5Conn, err := proxies.Dial(proxy, remoteAddr)
	if err != nil {
		socks5Conn.Close()
		log.Printf("[tcp] dial %s by proxy %q failed: %s", remoteAddr, proxy, err)
		return nil, err
	}
	return &socks5Conn, nil
}

// Set tcp tunnel status with rwMutex
func (tcpTunnel *TcpTunnel) SetStatus(s TunnelStatus) {
	tcpTunnel.rwMutex.Lock()
	tcpTunnel.status = s
	tcpTunnel.rwMutex.Unlock()
}

// Get tcp tunnel status with rwMutex
func (tcpTunnel *TcpTunnel) Status() TunnelStatus {
	tcpTunnel.rwMutex.Lock()
	s := tcpTunnel.status
	tcpTunnel.rwMutex.Unlock()
	return s
}

// Start tcp tunnel
func (tcpTunnel *TcpTunnel) Run() {
	tcpTunnel.ctx, tcpTunnel.ctxCancel = context.WithCancel(context.Background())
	go tcpTunnel.writeToLocal()
	go tcpTunnel.readFromRemote()
	go tcpTunnel.writeToRemote()
	go tcpTunnel.readFromLocal()
	tcpTunnel.SetStatus(StatusProxying)
}

// Read tcp packet form local netstack
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

// Write tcp packet to upstream
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

// Read tcp packet from upstream
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
				tcpTunnel.remotePackets <- buf[0:n]
			} else {
				break readFromRemote
			}
		}
	}
}

// Write tcp packet to local netstack
func (tcpTunnel *TcpTunnel) writeToLocal() {
writeToLocal:
	for {
		select {
		case <-tcpTunnel.ctx.Done():
			log.Printf("WriteToRemote done because of '%s'", tcpTunnel.ctx.Err())
			break writeToLocal
		case chunk := <-tcpTunnel.remotePackets:
			_, err := tcpTunnel.ep.Write(chunk, nil)
			if err != nil {
				log.Println(err)
				tcpTunnel.Close(errors.New(err.String()))
				break writeToLocal
			}
		}
	}
}

// Close this tcp tunnel
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
		close(tcpTunnel.remotePackets)
	})
}
