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
	"time"
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
		Timeout: 1 * time.Second,
	}
	socks5TcpConn, err := localTcpSocks5Dialer.Dial(Socks5Addr)
	if err != nil {
		log.Println("fail to connect SOCKS proxy ", err)
		return
	} else {
		// need to finish handshake in 1 mins
		socks5TcpConn.SetDeadline(time.Now().Add(time.Minute * 1))
	}

	_, err = gosocks.WriteSocksRequest(socks5TcpConn, &gosocks.SocksRequest{
		Cmd:      gosocks.SocksCmdUDPAssociate,
		HostType: gosocks.SocksIPv4Host,
		DstHost:  remoteHost,
		DstPort:  remotePort,
	})
	if err != nil {
		log.Println("WriteSocksRequest failed", err)
		socks5TcpConn.Close()
		return
	}
	cmdUDPAssociateReply, e := gosocks.ReadSocksReply(socks5TcpConn)
	if e != nil {
		log.Println(e)
		socks5TcpConn.Close()
		return
	}
	socks5TcpConn.SetDeadline(time.Time{})

	udpSocks5Addr := socks5TcpConn.LocalAddr().(*net.TCPAddr)
	udpSocks5Listen, err := net.ListenUDP("udp", &net.UDPAddr{
		IP:   udpSocks5Addr.IP,
		Port: 0,
		Zone: udpSocks5Addr.Zone,
	})
	if err != nil {
		log.Println("ListenUDP falied", err)
		socks5TcpConn.Close()
		return
	}
	udpSocks5Listen.SetDeadline(time.Now().Add(time.Minute * 1))

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
		socks5TcpConn.Close()
		udpSocks5Listen.Close()
		return
	}

	var udpSocks5Buf [4096]byte
	n, remoteAddr, err := udpSocks5Listen.ReadFromUDP(udpSocks5Buf[:])
	log.Println("from", remoteAddr, "got", n, "bytes message")
	switch {
	case n != 0:
		udpBuf := make([]byte, n)
		copy(udpBuf, udpSocks5Buf[:n])
		dnsPkt := createDNSResponse(net.ParseIP(remoteHost), remotePort, net.ParseIP(localAddr.Addr.To4().String()), localAddr.Port, udpBuf[10:])
		_, err := ifce.Write(dnsPkt)
		if err != nil {
			log.Println("Write to tun failed", err)
		}
		delete(udp.UDPNatList, localAddr.Port)
		socks5TcpConn.Close()
		udpSocks5Listen.Close()
	case err != nil:
		log.Println("ReadFromUDP tunnel failed", err)
		socks5TcpConn.Close()
		udpSocks5Listen.Close()
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
		log.Println(err)
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
