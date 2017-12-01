package tunnel

import (
	"github.com/FlowerWrong/netstack/tcpip"
	"github.com/FlowerWrong/netstack/tcpip/buffer"
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
	endpoint        stack.TransportEndpointID
	socks5TcpConn   *gosocks.SocksConn
	udpSocks5Listen *net.UDPConn
}

func NewUDP2Socks5(endpoint stack.TransportEndpointID, localAddr tcpip.FullAddress, view buffer.View, ifce *water.Interface) {
	remoteHost := endpoint.LocalAddress.To4().String()
	remotePort := endpoint.LocalPort

	localTcpSocks5Dialer := &gosocks.SocksDialer{
		Auth:    &gosocks.AnonymousClientAuthenticator{},
		Timeout: DefaultConnectTimeout,
	}
	socks5TcpConn, err := localTcpSocks5Dialer.Dial(Socks5Addr)
	if err != nil {
		log.Println("Fail to connect SOCKS proxy ", err)
		return
	}
	defer socks5TcpConn.Close()
	socks5TcpConn.SetDeadline(DefaultReadWriteTimeout)

	_, err = gosocks.WriteSocksRequest(socks5TcpConn, &gosocks.SocksRequest{
		Cmd:      gosocks.SocksCmdUDPAssociate,
		HostType: gosocks.SocksIPv4Host,
		DstHost:  "0.0.0.0",
		DstPort:  0,
	})
	if err != nil {
		log.Println("WriteSocksRequest failed", err)
		return
	}
	cmdUDPAssociateReply, e := gosocks.ReadSocksReply(socks5TcpConn)
	log.Println("cmdUDPAssociateReply", cmdUDPAssociateReply)
	if e != nil {
		log.Println("ReadSocksReply failed", e)
		return
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
		return
	}
	defer udpSocks5Listen.Close()
	udpSocks5Listen.SetDeadline(DefaultReadWriteTimeout)

	req := &gosocks.UDPRequest{
		Frag:     0,
		HostType: gosocks.SocksIPv4Host,
		DstHost:  remoteHost,
		DstPort:  remotePort,
		Data:     view,
	}
	n, err := udpSocks5Listen.WriteTo(gosocks.PackUDPRequest(req), gosocks.SocksAddrToNetAddr("udp", cmdUDPAssociateReply.BndHost, cmdUDPAssociateReply.BndPort).(*net.UDPAddr))
	if err != nil {
		log.Println("WriteTo UDP tunnel failed", err)
		return
	}

	var udpSocks5Buf [4096]byte
	n, _, err = udpSocks5Listen.ReadFromUDP(udpSocks5Buf[:])

	if n > 0 {
		udpBuf := make([]byte, n)
		copy(udpBuf, udpSocks5Buf[:n])
		dnsPkt := createDNSResponse(net.ParseIP(remoteHost), remotePort, net.ParseIP(localAddr.Addr.To4().String()), localAddr.Port, udpBuf[10:])
		_, err := ifce.Write(dnsPkt)
		if err != nil {
			log.Println("Write to tun failed", err)
		}
		udp.UDPNatList.DelUDPNat(localAddr.Port)
	}
	if err != nil {
		log.Println("ReadFromUDP tunnel failed", err)
	}
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
