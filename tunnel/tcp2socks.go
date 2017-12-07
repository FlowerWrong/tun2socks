package tunnel

import (
	"context"
	"errors"
	"fmt"
	"github.com/FlowerWrong/netstack/tcpip"
	"github.com/FlowerWrong/netstack/waiter"
	"github.com/FlowerWrong/tun2socks/tun2socks"
	"github.com/FlowerWrong/tun2socks/util"
	"log"
	"net"
	"sync"
)

// Tcp tunnel
type TcpTunnel struct {
	wq                   *waiter.Queue
	localEndpoint        tcpip.Endpoint
	localEndpointStatus  TunnelStatus // to avoid panic: send on closed channel
	localEndpointRwMutex sync.RWMutex
	remoteConn           net.Conn
	remoteStatus         TunnelStatus // to avoid panic: send on closed channel
	remoteRwMutex        sync.RWMutex
	remotePacketBuf      chan []byte // write to local
	remotePacketSize     int
	localPacketBuf       chan []byte // write to remote, socks5
	localPacketSize      int
	ctx                  context.Context
	ctxCancel            context.CancelFunc
	closeOne             sync.Once // to avoid multi close tunnel
}

// Create a tcp tunnel
func NewTCP2Socks(wq *waiter.Queue, ep tcpip.Endpoint, ip net.IP, port uint16, app *tun2socks.App) (*TcpTunnel, error) {
	socks5Conn, err := NewSocks5Conneciton(ip, port, app)
	if err != nil {
		log.Println("New socks5 conn failed", err)
		return nil, err
	}

	return &TcpTunnel{
		wq:                   wq,
		localEndpoint:        ep,
		remoteConn:           *socks5Conn,
		remotePacketBuf:      make(chan []byte, PktChannelSize),
		remotePacketSize:     0,
		localPacketBuf:       make(chan []byte, PktChannelSize),
		localPacketSize:      0,
		localEndpointRwMutex: sync.RWMutex{},
		remoteRwMutex:        sync.RWMutex{},
	}, nil
}

// New socks5 connection
func NewSocks5Conneciton(ip net.IP, port uint16, app *tun2socks.App) (*net.Conn, error) {
	var remoteAddr string
	proxy := ""

	if app.FakeDns != nil {
		record := app.FakeDns.DnsTablePtr.GetByIP(ip)
		if record != nil {
			remoteAddr = fmt.Sprintf("%v:%d", record.Hostname, port)
			proxy = record.Proxy
		} else {
			remoteAddr = fmt.Sprintf("%v:%d", ip, port)
		}
	} else {
		remoteAddr = fmt.Sprintf("%v:%d", ip, port)
	}

	socks5Conn, err := app.Proxies.Dial(proxy, remoteAddr)
	if err != nil {
		log.Printf("[tcp] dial %s by proxy %q failed: %s", remoteAddr, proxy, err)
		return nil, err
	}
	socks5Conn.(*net.TCPConn).SetKeepAlive(true)
	socks5Conn.SetDeadline(WithoutTimeout)
	return &socks5Conn, nil
}

// Set tcp tunnel status with rwMutex
func (tcpTunnel *TcpTunnel) SetRemoteStatus(s TunnelStatus) {
	tcpTunnel.remoteRwMutex.Lock()
	tcpTunnel.remoteStatus = s
	tcpTunnel.remoteRwMutex.Unlock()
}

// Get tcp tunnel status with rwMutex
func (tcpTunnel *TcpTunnel) RemoteStatus() TunnelStatus {
	tcpTunnel.remoteRwMutex.Lock()
	s := tcpTunnel.remoteStatus
	tcpTunnel.remoteRwMutex.Unlock()
	return s
}

// Set tcp tunnel status with rwMutex
func (tcpTunnel *TcpTunnel) SetLocalEndpointStatus(s TunnelStatus) {
	tcpTunnel.localEndpointRwMutex.Lock()
	tcpTunnel.localEndpointStatus = s
	tcpTunnel.localEndpointRwMutex.Unlock()
}

// Get tcp tunnel status with rwMutex
func (tcpTunnel *TcpTunnel) LocalEndpointStatus() TunnelStatus {
	tcpTunnel.localEndpointRwMutex.Lock()
	s := tcpTunnel.localEndpointStatus
	tcpTunnel.localEndpointRwMutex.Unlock()
	return s
}

// Start tcp tunnel
func (tcpTunnel *TcpTunnel) Run() {
	tcpTunnel.ctx, tcpTunnel.ctxCancel = context.WithCancel(context.Background())
	go tcpTunnel.writeToLocal()
	go tcpTunnel.readFromRemote()
	go tcpTunnel.writeToRemote()
	go tcpTunnel.readFromLocal()
	tcpTunnel.SetRemoteStatus(StatusProxying)
	tcpTunnel.SetLocalEndpointStatus(StatusProxying)
}

// Read tcp packet form local netstack
func (tcpTunnel *TcpTunnel) readFromLocal() {
	waitEntry, notifyCh := waiter.NewChannelEntry(nil)
	tcpTunnel.wq.EventRegister(&waitEntry, waiter.EventIn)
	defer tcpTunnel.wq.EventUnregister(&waitEntry)

readFromLocal:
	for {
		v, err := tcpTunnel.localEndpoint.Read(nil)
		if err != nil {
			if err == tcpip.ErrWouldBlock {
				select {
				case <-tcpTunnel.ctx.Done():
					break readFromLocal
				case <-notifyCh:
					continue readFromLocal
				}
			}
			if !util.IsClosed(err) {
				log.Println("Read from local failed", err)
			}
			tcpTunnel.Close(errors.New("read from local failed" + err.String()))
			break readFromLocal
		}
		if tcpTunnel.localEndpointStatus != StatusClosed {
			tcpTunnel.localPacketBuf <- v
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
			break writeToRemote
		case chunk := <-tcpTunnel.localPacketBuf:
		WriteAllPacket:
			for {
				n, err := tcpTunnel.remoteConn.Write(chunk)
				if err != nil {
					if !util.IsEOF(err) {
						log.Println("Write packet to remote failed", err)
					}
					tcpTunnel.Close(err)
					break writeToRemote
				} else if n < len(chunk) {
					chunk = chunk[n:]
					continue WriteAllPacket
				} else {
					break WriteAllPacket
				}
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
			break readFromRemote
		default:
			buf := make([]byte, BuffSize)
			n, err := tcpTunnel.remoteConn.Read(buf)
			if err != nil {
				if !util.IsEOF(err) {
					log.Println("Read from remote failed", err)
				}
				tcpTunnel.Close(err)
				break readFromRemote
			}

			if n > 0 && tcpTunnel.remoteStatus != StatusClosed {
				tcpTunnel.remotePacketBuf <- buf[0:n]
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
			break writeToLocal
		case chunk := <-tcpTunnel.remotePacketBuf:
		WriteAllPacket:
			for {
				var m uintptr
				var err *tcpip.Error
				m, err = tcpTunnel.localEndpoint.Write(chunk, nil)
				n := int(m)
				if err != nil {
					if err == tcpip.ErrWouldBlock {
						if n < len(chunk) {
							chunk = chunk[n:]
							continue WriteAllPacket
						}
					}
					if !util.IsClosed(err) {
						log.Println("Write to local failed", err)
					}
					tcpTunnel.Close(errors.New(err.String()))
					break writeToLocal
				} else if n < len(chunk) {
					chunk = chunk[n:]
					continue WriteAllPacket
				} else {
					break WriteAllPacket
				}
			}
		}
	}
}

// Close this tcp tunnel
func (tcpTunnel *TcpTunnel) Close(reason error) {
	tcpTunnel.closeOne.Do(func() {
		tcpTunnel.ctxCancel()

		tcpTunnel.SetLocalEndpointStatus(StatusClosed)
		tcpTunnel.SetRemoteStatus(StatusClosed)

		tcpTunnel.localEndpoint.Close()
		tcpTunnel.remoteConn.Close()

		close(tcpTunnel.localPacketBuf)
		close(tcpTunnel.remotePacketBuf)
	})
}
