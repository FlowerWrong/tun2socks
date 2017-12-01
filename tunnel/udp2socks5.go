package tunnel

import (
	"context"
	"errors"
	"github.com/FlowerWrong/netstack/tcpip"
	"github.com/FlowerWrong/netstack/tcpip/stack"
	"github.com/FlowerWrong/netstack/tcpip/transport/udp"
	"github.com/FlowerWrong/water"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/yinghuocho/gosocks"
	"log"
	"net"
)

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
}

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

	socks5TcpConn.SetWriteDeadline(DefaultReadWriteTimeout)
	_, err = gosocks.WriteSocksRequest(socks5TcpConn, &gosocks.SocksRequest{
		Cmd:      gosocks.SocksCmdUDPAssociate,
		HostType: gosocks.SocksIPv4Host,
		DstHost:  "0.0.0.0",
		DstPort:  0,
	})
	if err != nil {
		log.Println("WriteSocksRequest failed", err)
		socks5TcpConn.Close()
		return nil, err
	}
	socks5TcpConn.SetReadDeadline(DefaultReadWriteTimeout)
	cmdUDPAssociateReply, err := gosocks.ReadSocksReply(socks5TcpConn)
	if err != nil {
		log.Println("ReadSocksReply failed", err)
		socks5TcpConn.Close()
		return nil, err
	}
	socks5TcpConn.SetDeadline(WithoutTimeout)

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
	return &UdpTunnel{
		endpoint:             endpoint,
		socks5TcpConn:        socks5TcpConn,
		udpSocks5Listen:      udpSocks5Listen,
		RemotePackets:        make(chan []byte, 1500),
		LocalPackets:         make(chan []byte, 1500),
		localAddr:            localAddr,
		ifce:                 ifce,
		cmdUDPAssociateReply: cmdUDPAssociateReply,
	}, nil
}

func (udpTunnel *UdpTunnel) Run() {
	udpTunnel.ctx, udpTunnel.ctxCancel = context.WithCancel(context.Background())
	go udpTunnel.writeToLocal()
	go udpTunnel.readFromRemote()
	go udpTunnel.writeToRemote()
}

func (udpTunnel *UdpTunnel) writeToRemote() {
WriteToRemote:
	for {
		select {
		case <-udpTunnel.ctx.Done():
			log.Printf("WriteToRemote done because of '%s'", udpTunnel.ctx.Err())
			break WriteToRemote
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
			_, err := udpTunnel.udpSocks5Listen.WriteTo(gosocks.PackUDPRequest(req), gosocks.SocksAddrToNetAddr("udp", udpTunnel.cmdUDPAssociateReply.BndHost, udpTunnel.cmdUDPAssociateReply.BndPort).(*net.UDPAddr))
			if err != nil {
				log.Println("WriteTo UDP tunnel failed", err)
				udpTunnel.Close(err)
				break WriteToRemote
			}
		}
	}
}

func (udpTunnel *UdpTunnel) readFromRemote() {
ReadFromRemote:
	for {
		select {
		case <-udpTunnel.ctx.Done():
			log.Printf("ReadFromRemote done because of '%s'", udpTunnel.ctx.Err())
			break ReadFromRemote
		default:
			var udpSocks5Buf [4096]byte
			udpTunnel.udpSocks5Listen.SetReadDeadline(DefaultReadWriteTimeout)
			n, _, err := udpTunnel.udpSocks5Listen.ReadFromUDP(udpSocks5Buf[:])
			if n > 0 {
				udpBuf := make([]byte, n)
				copy(udpBuf, udpSocks5Buf[:n])
				udpTunnel.RemotePackets <- udpBuf[10:]
			}
			if err != nil {
				log.Println("ReadFromUDP tunnel failed", err)
				udpTunnel.Close(err)
				break ReadFromRemote
			}
		}
	}
}

func (udpTunnel *UdpTunnel) writeToLocal() {
WriteToLocal:
	for {
		select {
		case <-udpTunnel.ctx.Done():
			log.Printf("WriteToRemote done because of '%s'", udpTunnel.ctx.Err())
			break WriteToLocal
		case chunk := <-udpTunnel.RemotePackets:
			remoteHost := udpTunnel.endpoint.LocalAddress.To4().String()
			remotePort := udpTunnel.endpoint.LocalPort
			dnsPkt := createDNSResponse(net.ParseIP(remoteHost), remotePort, net.ParseIP(udpTunnel.localAddr.Addr.To4().String()), udpTunnel.localAddr.Port, chunk)
			_, err := udpTunnel.ifce.Write(dnsPkt)
			if err != nil {
				log.Println("Write to tun failed", err)
			}
			if err != nil {
				log.Println(err)
				udpTunnel.Close(err)
				break WriteToLocal
			}
			udpTunnel.Close(errors.New("OK"))
		}
	}
}

func (udpTunnel *UdpTunnel) Close(reason error) {
	log.Println("Close UDP tunnel because", reason.Error())
	udpTunnel.ctxCancel()
	udp.UDPNatList.DelUDPNat(udpTunnel.localAddr.Port)
	udpTunnel.socks5TcpConn.Close()
	udpTunnel.udpSocks5Listen.Close()
	return
}

func createDNSResponse(SrcIP net.IP, SrcPort uint16, DstIP net.IP, DstPort uint16, pkt []byte) []byte {
	ip := &layers.IPv4{
		SrcIP:    SrcIP,
		DstIP:    DstIP,
		Protocol: layers.IPProtocolUDP,
		Version:  uint8(4),
		IHL:      uint8(5),
		TTL:      uint8(64),
	}
	udp := &layers.UDP{SrcPort: layers.UDPPort(SrcPort), DstPort: layers.UDPPort(DstPort)}
	if err := udp.SetNetworkLayerForChecksum(ip); err != nil {
		log.Println("SetNetworkLayerForChecksum failed", err)
		return nil
	}
	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{FixLengths: true, ComputeChecksums: true}
	gopacket.SerializeLayers(buf, opts,
		ip,
		udp,
		gopacket.Payload(pkt),
	)

	packetData := buf.Bytes()
	return packetData
}
