package tun2socks

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/FlowerWrong/gosocks"
	"github.com/FlowerWrong/netstack/tcpip"
	"github.com/FlowerWrong/netstack/tcpip/buffer"
	"github.com/FlowerWrong/netstack/tcpip/stack"
	"github.com/FlowerWrong/netstack/tcpip/transport/udp"
	"github.com/FlowerWrong/tun2socks/util"
)

// UDPTunnelList id -> *UDPTunnel
var UDPTunnelList sync.Map

// UDPTunnel timeout read
type UDPTunnel struct {
	id                   string
	localEndpoint        stack.TransportEndpointID
	remoteHost           string // ip or domain
	remotePort           uint16
	remoteHostType       byte // ipv4 ipv6 or domain
	socks5TcpConn        *gosocks.SocksConn
	socks5UdpListen      *net.UDPConn
	ctx                  context.Context
	ctxCancel            context.CancelFunc
	localAddr            tcpip.FullAddress
	cmdUDPAssociateReply *gosocks.SocksReply
	closeOne             sync.Once
	app                  *App
	wg                   sync.WaitGroup
	localBufLen          int
	remoteBufLen         int
}

func id(remoteHost string, remotePort uint16, localAddr tcpip.FullAddress) string {
	return strings.Join([]string{
		fmt.Sprintf("%s:%d", localAddr.Addr.To4().String(), localAddr.Port),
		fmt.Sprintf("%s:%d", remoteHost, remotePort),
	}, "<->")
}

// NewUDPTunnel Create a udp tunnel
func NewUDPTunnel(endpoint stack.TransportEndpointID, localAddr tcpip.FullAddress, app *App) (*UDPTunnel, bool, error) {
	var localTCPSocks5Dialer *gosocks.SocksDialer
	localTCPSocks5Dialer = &gosocks.SocksDialer{
		Auth:    &gosocks.AnonymousClientAuthenticator{},
		Timeout: DefaultConnectDuration,
	}

	// TODO ipv6
	remoteHost := endpoint.LocalAddress.To4().String()
	var hostType byte = gosocks.SocksIPv4Host
	proxy := ""
	if app.FakeDNS != nil {
		ip := net.ParseIP(remoteHost)
		record := app.FakeDNS.DNSTablePtr.GetByIP(ip)
		if record != nil {
			if record.Proxy == "block" {
				return nil, false, errors.New(record.Hostname + " is blocked")
			}
			proxy = app.Cfg.GetProxySchema(record.Proxy)
			remoteHost = record.Hostname // domain
			hostType = gosocks.SocksDomainHost
		}
	}

	udpID := id(remoteHost, endpoint.LocalPort, localAddr)
	tunnel, ok := UDPTunnelList.Load(udpID)
	if ok && tunnel != nil {
		return tunnel.(*UDPTunnel), true, nil
	}

	if proxy == "" {
		proxy, _ = app.Cfg.UDPProxySchema()
	}

	socks5TcpConn, err := localTCPSocks5Dialer.Dial(proxy)
	if err != nil {
		log.Println("[error] Fail to connect SOCKS proxy ", err)
		return nil, false, err
	}

	udpSocks5Addr := socks5TcpConn.LocalAddr().(*net.TCPAddr)
	udpSocks5Listen, err := net.ListenUDP("udp", &net.UDPAddr{
		IP:   udpSocks5Addr.IP,
		Port: 0,
		Zone: udpSocks5Addr.Zone,
	})
	if err != nil {
		log.Println("[error] ListenUDP falied", err)
		socks5TcpConn.Close()
		return nil, false, err
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
		log.Println("[error] WriteSocksRequest failed", err)
		socks5TcpConn.Close()
		udpSocks5Listen.Close()
		return nil, false, err
	}

	cmdUDPAssociateReply, err := gosocks.ReadSocksReply(socks5TcpConn)
	if err != nil {
		log.Println("[error] ReadSocksReply failed", err)
		socks5TcpConn.Close()
		udpSocks5Listen.Close()
		return nil, false, err
	}
	if cmdUDPAssociateReply.Rep != gosocks.SocksSucceeded {
		log.Printf("[error] socks connect request fail, retcode: %d", cmdUDPAssociateReply.Rep)
		socks5TcpConn.Close()
		udpSocks5Listen.Close()
		return nil, false, err
	}
	// A zero value for t means I/O operations will not time out.
	socks5TcpConn.SetDeadline(WithoutTimeout)

	udpTunnel := UDPTunnel{
		id:                   id(remoteHost, endpoint.LocalPort, localAddr),
		localEndpoint:        endpoint,
		remoteHost:           remoteHost,
		remotePort:           endpoint.LocalPort,
		remoteHostType:       hostType,
		socks5TcpConn:        socks5TcpConn,
		socks5UdpListen:      udpSocks5Listen,
		localAddr:            localAddr,
		app:                  app,
		cmdUDPAssociateReply: cmdUDPAssociateReply,
		localBufLen:          0,
		remoteBufLen:         0,
	}
	udpTunnel.ctx, udpTunnel.ctxCancel = context.WithCancel(context.Background())
	UDPTunnelList.Store(udpTunnel.id, &udpTunnel)

	return &udpTunnel, false, nil
}

// Run udp tunnel
func (udpTunnel *UDPTunnel) Run(v buffer.View, existFlag bool) {
	req := &gosocks.UDPRequest{
		Frag:     0,
		HostType: udpTunnel.remoteHostType,
		DstHost:  udpTunnel.remoteHost,
		DstPort:  udpTunnel.remotePort,
		Data:     v,
	}

	n, err := udpTunnel.socks5UdpListen.WriteTo(gosocks.PackUDPRequest(req), gosocks.SocksAddrToNetAddr("udp", udpTunnel.cmdUDPAssociateReply.BndHost, udpTunnel.cmdUDPAssociateReply.BndPort).(*net.UDPAddr))
	if err != nil {
		log.Println(err)
		udpTunnel.Close(err)
		return
	}
	dataLen := len(v)
	udpTunnel.localBufLen += dataLen
	if n <= dataLen {
		log.Println("[error] only part pkt had been write to socks5", n, dataLen)
		udpTunnel.Close(errors.New("write part error"))
		return
	}

	if !existFlag {
		udpTunnel.wg.Add(1)
		go udpTunnel.readFromRemoteWriteToLocal()

		udpTunnel.wg.Wait()
		udpTunnel.Close(nil)
	}
}

func (udpTunnel *UDPTunnel) readFromRemoteWriteToLocal() {
	defer udpTunnel.wg.Done()
	var udpSocks5Buf [PktChannelSize]byte

readFromRemote:
	for {
		select {
		case <-udpTunnel.ctx.Done():
			break readFromRemote
		default:
			udpTunnel.socks5UdpListen.SetReadDeadline(time.Now().Add(time.Duration(udpTunnel.app.Cfg.UDP.Timeout) * time.Second))
			n, _, err := udpTunnel.socks5UdpListen.ReadFromUDP(udpSocks5Buf[0:])
			if n > 0 {
				udpReq, err := gosocks.ParseUDPRequest(udpSocks5Buf[0:n])
				if err != nil {
					udpTunnel.Close(err)
					break readFromRemote
				}
				udpTunnel.remoteBufLen += len(udpReq.Data)
				remoteHost := udpTunnel.localEndpoint.LocalAddress.To4().String()

				pkt := util.CreateUDPResponse(net.ParseIP(remoteHost), udpTunnel.remotePort, net.ParseIP(udpTunnel.localAddr.Addr.To4().String()), udpTunnel.localAddr.Port, udpReq.Data)
				if pkt == nil {
					udpTunnel.Close(errors.New("pack ip packet return nil"))
					break readFromRemote
				} else {
					n, err := udpTunnel.app.Ifce.Write(pkt)
					if err != nil {
						log.Println("[error] write udp package to tun failed", err)
						udpTunnel.Close(err)
						break readFromRemote
					}
					if n < len(pkt) {
						udpTunnel.Close(errors.New("write udp package to tun failed, just write success part of pkt"))
						break readFromRemote
					}
				}

				// DNS packet
				if udpTunnel.remotePort == 53 {
					udpTunnel.Close(nil)
					break readFromRemote
				}
			}
			if err != nil {
				if !util.IsEOF(err) && !util.IsTimeout(err) {
					udpTunnel.Close(err)
				} else {
					udpTunnel.Close(nil)
				}
				break readFromRemote
			}
		}
	}
}

// Close this udp tunnel
func (udpTunnel *UDPTunnel) Close(reason error) {
	udpTunnel.closeOne.Do(func() {
		if reason != nil {
			log.Println("udp tunnel closed reason:", reason.Error(), udpTunnel.id)
		}

		UDPTunnelList.Delete(udpTunnel.id)
		udpTunnel.ctxCancel()
		udpTunnel.socks5TcpConn.Close()
		udpTunnel.socks5UdpListen.Close()
		udp.UDPNatList.Delete(udpTunnel.localAddr.Port)
	})
}
