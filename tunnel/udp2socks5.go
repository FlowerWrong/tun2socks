package tunnel

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/FlowerWrong/tun2socks/util"

	"github.com/FlowerWrong/netstack/tcpip"
	"github.com/FlowerWrong/netstack/tcpip/stack"
	"github.com/FlowerWrong/netstack/tcpip/transport/udp"
	"github.com/FlowerWrong/tun2socks/tun2socks"
	"github.com/yinghuocho/gosocks"
)

// Udp tunnel
type UdpTunnel struct {
	localEndpoint        stack.TransportEndpointID
	remoteHost           string
	remotePort           uint16
	remoteHostType       byte
	socks5TcpConn        *gosocks.SocksConn
	socks5UdpListen      *net.UDPConn
	RemotePackets        chan []byte // write to local
	LocalPackets         chan []byte // write to remote, socks5
	ctx                  context.Context
	ctxCancel            context.CancelFunc
	localAddr            tcpip.FullAddress
	cmdUDPAssociateReply *gosocks.SocksReply
	closeOne             sync.Once
	status               TunnelStatus // to avoid panic: send on closed channel
	rwMutex              sync.RWMutex
	app                  *tun2socks.App
	wg                   sync.WaitGroup
}

// Create a udp tunnel
func NewUdpTunnel(endpoint stack.TransportEndpointID, localAddr tcpip.FullAddress, app *tun2socks.App) (*UdpTunnel, error) {
	localTcpSocks5Dialer := &gosocks.SocksDialer{
		Auth:    &gosocks.AnonymousClientAuthenticator{},
		Timeout: DefaultConnectDuration,
	}

	// TODO ipv6
	remoteHost := endpoint.LocalAddress.To4().String()
	var hostType byte = gosocks.SocksIPv4Host
	proxy := ""
	if app.FakeDns != nil {
		ip := net.ParseIP(remoteHost)
		record := app.FakeDns.DnsTablePtr.GetByIP(ip)
		if record != nil {
			if record.Proxy == "block" {
				return nil, errors.New(record.Hostname + " is blocked")
			}
			proxy = app.Cfg.GetProxy(record.Proxy)
			remoteHost = record.Hostname // domain
			hostType = gosocks.SocksDomainHost
		}
	}

	if proxy == "" {
		proxy, _ = app.Cfg.UdpProxy()
	}

	socks5TcpConn, err := localTcpSocks5Dialer.Dial(proxy)
	if err != nil {
		log.Println("Fail to connect SOCKS proxy ", err)
		return nil, err
	}

	udpSocks5Addr := socks5TcpConn.LocalAddr().(*net.TCPAddr)
	udpSocks5Listen, err := net.ListenUDP("udp", &net.UDPAddr{
		IP:   udpSocks5Addr.IP,
		Port: 0,
		Zone: udpSocks5Addr.Zone,
	})
	if err != nil {
		log.Println("ListenUDP falied", err)
		socks5TcpConn.Close()
		return nil, err
	}
	udpSocks5Listen.SetDeadline(WithoutTimeout)

	_, err = gosocks.WriteSocksRequest(socks5TcpConn, &gosocks.SocksRequest{
		Cmd:      gosocks.SocksCmdUDPAssociate,
		HostType: gosocks.SocksIPv4Host,
		DstHost:  "0.0.0.0",
		DstPort:  0,
	})
	if err != nil {
		// FIXME i/o timeout
		log.Println("WriteSocksRequest failed", err)
		socks5TcpConn.Close()
		udpSocks5Listen.Close()
		return nil, err
	}

	cmdUDPAssociateReply, err := gosocks.ReadSocksReply(socks5TcpConn)
	if err != nil {
		log.Println("ReadSocksReply failed", err)
		socks5TcpConn.Close()
		udpSocks5Listen.Close()
		return nil, err
	}
	if cmdUDPAssociateReply.Rep != gosocks.SocksSucceeded {
		log.Printf("socks connect request fail, retcode: %d", cmdUDPAssociateReply.Rep)
		socks5TcpConn.Close()
		udpSocks5Listen.Close()
		return nil, err
	}
	// A zero value for t means I/O operations will not time out.
	socks5TcpConn.SetDeadline(WithoutTimeout)

	return &UdpTunnel{
		localEndpoint:        endpoint,
		remoteHost:           remoteHost,
		remotePort:           endpoint.LocalPort,
		remoteHostType:       hostType,
		socks5TcpConn:        socks5TcpConn,
		socks5UdpListen:      udpSocks5Listen,
		RemotePackets:        make(chan []byte, PktChannelSize),
		LocalPackets:         make(chan []byte, PktChannelSize),
		localAddr:            localAddr,
		app:                  app,
		cmdUDPAssociateReply: cmdUDPAssociateReply,
	}, nil
}

// Set udp tunnel status with rwMutex
func (udpTunnel *UdpTunnel) SetStatus(s TunnelStatus) {
	udpTunnel.rwMutex.Lock()
	udpTunnel.status = s
	udpTunnel.rwMutex.Unlock()
}

// Get udp tunnel status with rwMutex
func (udpTunnel *UdpTunnel) Status() TunnelStatus {
	udpTunnel.rwMutex.RLock()
	s := udpTunnel.status
	udpTunnel.rwMutex.RUnlock()
	return s
}

func waitTimeout(wg *sync.WaitGroup, timeout time.Duration) bool {
	c := make(chan struct{})
	go func() {
		defer close(c)
		wg.Wait()
	}()
	select {
	case <-c:
		return false // completed normally
	case <-time.After(timeout):
		return true // timed out
	}
}

func (udpTunnel *UdpTunnel) Run() {
	udpTunnel.ctx, udpTunnel.ctxCancel = context.WithCancel(context.Background())
	udpTunnel.wg.Add(1)
	go udpTunnel.writeToLocal()
	udpTunnel.wg.Add(1)
	go udpTunnel.readFromRemote()
	udpTunnel.wg.Add(1)
	go udpTunnel.writeToRemote()
	udpTunnel.SetStatus(StatusProxying)

	if waitTimeout(&udpTunnel.wg, 10*time.Second) {
		fmt.Println("Timed out waiting for wait group", udpTunnel.remoteHost, udpTunnel.remotePort, udpTunnel.localAddr)
	}
	udpTunnel.Close(errors.New("OK"))
}

// Write udp packet to upstream
func (udpTunnel *UdpTunnel) writeToRemote() {
	defer udpTunnel.wg.Done()
writeToRemote:
	for {
		select {
		case <-udpTunnel.ctx.Done():
			break writeToRemote
		case chunk := <-udpTunnel.LocalPackets: // TODO write n < len(chunk)
			req := &gosocks.UDPRequest{
				Frag:     0,
				HostType: udpTunnel.remoteHostType,
				DstHost:  udpTunnel.remoteHost,
				DstPort:  udpTunnel.remotePort,
				Data:     chunk,
			}
			_, err := udpTunnel.socks5UdpListen.WriteTo(gosocks.PackUDPRequest(req), gosocks.SocksAddrToNetAddr("udp", udpTunnel.cmdUDPAssociateReply.BndHost, udpTunnel.cmdUDPAssociateReply.BndPort).(*net.UDPAddr))
			if err != nil {
				if !util.IsEOF(err) {
					log.Println("WriteTo UDP tunnel failed", err)
				}
				udpTunnel.Close(err)
				break writeToRemote
			}
			break writeToRemote
		}
	}
}

// Read udp packet from upstream
func (udpTunnel *UdpTunnel) readFromRemote() {
	defer udpTunnel.wg.Done()
readFromRemote:
	for {
		select {
		case <-udpTunnel.ctx.Done():
			break readFromRemote
		default:
			var udpSocks5Buf [PktChannelSize]byte
			n, _, err := udpTunnel.socks5UdpListen.ReadFromUDP(udpSocks5Buf[0:])
			if n > 0 {
				udpReq, err := gosocks.ParseUDPRequest(udpSocks5Buf[0:n])
				if err != nil {
					log.Println("Parse UDP reply data frailed", err)
					udpTunnel.Close(err)
					break readFromRemote
				}
				if udpTunnel.Status() != StatusClosed {
					udpTunnel.RemotePackets <- udpReq.Data
				}
				break readFromRemote
			}
			if err != nil {
				if !util.IsEOF(err) {
					log.Println("ReadFromUDP tunnel failed", err)
				}
				udpTunnel.Close(err)
				break readFromRemote
			}
		}
	}
}

// Write upstream udp packet to local
func (udpTunnel *UdpTunnel) writeToLocal() {
	defer udpTunnel.wg.Done()
writeToLocal:
	for {
		select {
		case <-udpTunnel.ctx.Done():
			break writeToLocal
		case chunk := <-udpTunnel.RemotePackets:
			// TODO ipv6
			remoteHost := udpTunnel.localEndpoint.LocalAddress.To4().String()
			pkt := util.CreateDNSResponse(net.ParseIP(remoteHost), udpTunnel.remotePort, net.ParseIP(udpTunnel.localAddr.Addr.To4().String()), udpTunnel.localAddr.Port, chunk)
			if pkt == nil {
				udpTunnel.Close(errors.New("pack ip packet return nil"))
				break writeToLocal
			}
			_, err := udpTunnel.app.Ifce.Write(pkt)
			if err != nil {
				log.Println("Write udp package to tun failed", err)
				udpTunnel.Close(err)
			}
			break writeToLocal
		}
	}
}

// Close this udp tunnel
func (udpTunnel *UdpTunnel) Close(reason error) {
	udpTunnel.closeOne.Do(func() {
		udpTunnel.SetStatus(StatusClosed)
		udpTunnel.ctxCancel()
		udpTunnel.socks5TcpConn.Close()
		udpTunnel.socks5UdpListen.Close()
		udp.UDPNatList.Delete(udpTunnel.localAddr.Port)
		close(udpTunnel.LocalPackets)
		close(udpTunnel.RemotePackets)
	})
}
