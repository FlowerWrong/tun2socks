package tunnel

import (
	"context"
	"errors"
	"log"
	"net"
	"sync"

	"github.com/FlowerWrong/netstack/tcpip"
	"github.com/FlowerWrong/netstack/tcpip/stack"
	"github.com/FlowerWrong/netstack/tcpip/transport/udp"
	"github.com/FlowerWrong/water"
	"github.com/yinghuocho/gosocks"
	"time"
	"github.com/FlowerWrong/tun2socks/dns"
)

// Udp tunnel
type UdpTunnel struct {
	endpoint             stack.TransportEndpointID
	socks5TcpConn        *gosocks.SocksConn
	udpSocks5Listen      *net.UDPConn
	RemotePackets        chan []byte // write to local
	LocalPackets         chan []byte // write to remote, socks5
	ctx                  context.Context
	ctxCancel            context.CancelFunc
	localAddr            tcpip.FullAddress
	ifce                 *water.Interface
	cmdUDPAssociateReply *gosocks.SocksReply
	closeOne             sync.Once
	status               TunnelStatus // to avoid panic: send on closed channel
	rwMutex              sync.RWMutex
}

// Create a udp tunnel
func NewUdpTunnel(endpoint stack.TransportEndpointID, localAddr tcpip.FullAddress, ifce *water.Interface) (*UdpTunnel, error) {
	localTcpSocks5Dialer := &gosocks.SocksDialer{
		Auth:    &gosocks.AnonymousClientAuthenticator{},
		Timeout: DefaultConnectDuration,
	}
	socks5TcpConn, err := localTcpSocks5Dialer.Dial(Socks5Addr)
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

	// socks5TcpConn.SetWriteDeadline(DefaultReadWriteTimeout)
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
	// socks5TcpConn.SetReadDeadline(DefaultReadWriteTimeout)
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
	socks5TcpConn.SetDeadline(WithoutTimeout)

	return &UdpTunnel{
		endpoint:             endpoint,
		socks5TcpConn:        socks5TcpConn,
		udpSocks5Listen:      udpSocks5Listen,
		RemotePackets:        make(chan []byte, PktChannelSize),
		LocalPackets:         make(chan []byte, PktChannelSize),
		localAddr:            localAddr,
		ifce:                 ifce,
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
	udpTunnel.rwMutex.Lock()
	s := udpTunnel.status
	udpTunnel.rwMutex.Unlock()
	return s
}

func (udpTunnel *UdpTunnel) Run() {
	udpTunnel.ctx, udpTunnel.ctxCancel = context.WithCancel(context.Background())
	go udpTunnel.writeToLocal()
	go udpTunnel.readFromRemote()
	go udpTunnel.writeToRemote()
	udpTunnel.SetStatus(StatusProxying)
}

// Write udp packet to upstream
func (udpTunnel *UdpTunnel) writeToRemote() {
writeToRemote:
	for {
		select {
		case <-udpTunnel.ctx.Done():
			log.Printf("writeToRemote done because of '%s'", udpTunnel.ctx.Err())
			break writeToRemote
		case chunk := <-udpTunnel.LocalPackets:
			remoteHost := udpTunnel.endpoint.LocalAddress.To4().String()
			remotePort := udpTunnel.endpoint.LocalPort
			req := &gosocks.UDPRequest{
				Frag:     0,
				HostType: gosocks.SocksIPv4Host,
				DstHost:  remoteHost,
				DstPort:  remotePort,
				Data:     chunk,
			}
			// udpTunnel.udpSocks5Listen.SetWriteDeadline(DefaultReadWriteTimeout)
			_, err := udpTunnel.udpSocks5Listen.WriteTo(gosocks.PackUDPRequest(req), gosocks.SocksAddrToNetAddr("udp", udpTunnel.cmdUDPAssociateReply.BndHost, udpTunnel.cmdUDPAssociateReply.BndPort).(*net.UDPAddr))
			if err != nil {
				log.Println("WriteTo UDP tunnel failed", err)
				udpTunnel.Close(err)
				break writeToRemote
			}
		}
	}
}

// Read udp packet from upstream
func (udpTunnel *UdpTunnel) readFromRemote() {
readFromRemote:
	for {
		select {
		case <-udpTunnel.ctx.Done():
			log.Printf("readFromRemote done because of '%s'", udpTunnel.ctx.Err())
			break readFromRemote
		default:
			var udpSocks5Buf [4096]byte
			// udpTunnel.udpSocks5Listen.SetReadDeadline(WithoutTimeout)
			n, _, err := udpTunnel.udpSocks5Listen.ReadFromUDP(udpSocks5Buf[:])
			if n > 0 {
				udpBuf := make([]byte, n)
				copy(udpBuf, udpSocks5Buf[:n])
				if udpTunnel.status != StatusClosed {
					udpTunnel.RemotePackets <- udpBuf[10:]
				}
			}
			if err != nil {
				log.Println("ReadFromUDP tunnel failed", err)
				udpTunnel.Close(err)
				break readFromRemote
			}
		}
	}
}

// Write upstream udp packet to local
func (udpTunnel *UdpTunnel) writeToLocal() {
	start := time.Now()
writeToLocal:
	for {
		select {
		case <-udpTunnel.ctx.Done():
			log.Printf("WriteToRemote done because of '%s'", udpTunnel.ctx.Err())
			break writeToLocal
		case chunk := <-udpTunnel.RemotePackets:
			remoteHost := udpTunnel.endpoint.LocalAddress.To4().String()
			remotePort := udpTunnel.endpoint.LocalPort
			pkt := dns.CreateDNSResponse(net.ParseIP(remoteHost), remotePort, net.ParseIP(udpTunnel.localAddr.Addr.To4().String()), udpTunnel.localAddr.Port, chunk)
			if pkt == nil {
				udpTunnel.Close(errors.New("pack ip packet return nil"))
				break writeToLocal
			}
			_, err := udpTunnel.ifce.Write(pkt)
			if err != nil {
				log.Println("Write to tun failed", err)
			} else {
				// cache dns packet
				end := time.Now()
				ms := end.Sub(start).Nanoseconds() / 1000000
				log.Printf("DNS session response received: %d ms", ms)
				if dns.DNSCache != nil {
					dns.DNSCache.Store(chunk)
				}
			}
			if err != nil {
				log.Println(err)
				udpTunnel.Close(err)
				break writeToLocal
			}
			udpTunnel.Close(errors.New("OK"))
			break writeToLocal
		}
	}
}

// Close this udp tunnel
func (udpTunnel *UdpTunnel) Close(reason error) {
	udpTunnel.closeOne.Do(func() {
		log.Println("Close UDP tunnel because", reason.Error())
		udpTunnel.SetStatus(StatusClosed)
		udpTunnel.ctxCancel()
		err := udpTunnel.socks5TcpConn.Close()
		if err != nil {
			log.Println("Close socks5TcpConn falied", err)
		}
		err = udpTunnel.udpSocks5Listen.Close()
		if err != nil {
			log.Println("Close udpSocks5Listen falied", err)
		}
		udp.UDPNatList.DelUDPNat(udpTunnel.localAddr.Port)
		close(udpTunnel.LocalPackets)
		close(udpTunnel.RemotePackets)
	})
}
