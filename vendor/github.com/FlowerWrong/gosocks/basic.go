package gosocks

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"
)

const (
	SocksVersion = 0x05

	SocksReserved = 0x00

	SocksNoAuthentication    = 0x00
	SocksAuthUserPassword    = 0x02
	SocksNoAcceptableMethods = 0xFF

	SocksIPv4Host   = 0x01
	SocksIPv6Host   = 0x04
	SocksDomainHost = 0x03

	SocksCmdConnect      = 0x01
	SocksCmdBind         = 0x02
	SocksCmdUDPAssociate = 0x03

	SocksSucceeded      = 0x00
	SocksGeneralFailure = 0x01

	SocksNoFragment = 0x00

	smallBufSize = 0x200
	largeBufSize = 0x10000
)

type SocksRequest struct {
	Cmd      byte
	HostType byte
	DstHost  string
	DstPort  uint16
}

type SocksReply struct {
	Rep      byte
	HostType byte
	BndHost  string
	BndPort  uint16
}

// This is for SocksReply and SocksRequest to share pack/unpack methods.
type socksCommon struct {
	Flag     byte
	HostType byte
	Host     string
	Port     uint16
}

type UDPRequest struct {
	Frag     byte
	HostType byte
	DstHost  string
	DstPort  uint16
	Data     []byte
}

type SocksConn struct {
	net.Conn
	Timeout time.Duration
}

func Ntohs(data [2]byte) uint16 {
	return uint16(data[0])<<8 | uint16(data[1])<<0
}

func Htons(data uint16) (ret [2]byte) {
	ret[0] = byte((data >> 8) & 0xff)
	ret[1] = byte((data >> 0) & 0xff)
	return
}

func SockAddrString(host string, port uint16) string {
	return net.JoinHostPort(host, strconv.Itoa(int(port)))
}

func SocksAddrToNetAddr(nw, host string, port uint16) net.Addr {
	s := net.JoinHostPort(host, strconv.Itoa(int(port)))
	var addr net.Addr
	switch nw {
	case "tcp":
		addr, _ = net.ResolveTCPAddr(nw, s)
	case "udp":
		addr, _ = net.ResolveUDPAddr(nw, s)
	}

	return addr
}

func ParseHost(host string) (byte, string) {
	i := strings.LastIndex(host, "%")
	s := host
	if i > 0 {
		s = host[:i]
	}

	ip := net.ParseIP(s)
	var t byte
	if ip != nil {
		if ip.To16() != nil {
			t = SocksIPv6Host
		} else {
			t = SocksIPv4Host
		}
	} else {
		t = SocksDomainHost
	}
	return t, s
}

func NetAddrToSocksAddr(addr interface{}) (hostType byte, host string, port uint16) {
	var ip net.IP
	switch addr.(type) {
	case *net.UDPAddr:
		a := addr.(*net.UDPAddr)
		ip = a.IP
		port = uint16(a.Port)
	case *net.TCPAddr:
		a := addr.(*net.TCPAddr)
		ip = a.IP
		port = uint16(a.Port)
	}

	hostType = SocksIPv4Host
	if len(ip) == 16 {
		hostType = SocksIPv6Host
	}
	host = ip.String()
	return
}

func readSocksIPv4Host(r io.Reader) (host string, err error) {
	var buf [4]byte
	_, err = io.ReadFull(r, buf[:])
	if err != nil {
		return
	}

	var ip net.IP = buf[:]
	host = ip.String()
	return
}

func readSocksIPv6Host(r io.Reader) (host string, err error) {
	var buf [16]byte
	_, err = io.ReadFull(r, buf[:])
	if err != nil {
		return
	}

	var ip net.IP = buf[:]
	host = ip.String()
	return
}

func readSocksDomainHost(r io.Reader) (host string, err error) {
	var buf [smallBufSize]byte
	_, err = r.Read(buf[0:1])
	if err != nil {
		return
	}
	length := buf[0]
	_, err = io.ReadFull(r, buf[1:1+length])
	if err != nil {
		return
	}
	host = string(buf[1 : 1+length])
	return
}

func readSocksHost(r io.Reader, hostType byte) (string, error) {
	switch hostType {
	case SocksIPv4Host:
		return readSocksIPv4Host(r)
	case SocksIPv6Host:
		return readSocksIPv6Host(r)
	case SocksDomainHost:
		return readSocksDomainHost(r)
	default:
		return string(""), fmt.Errorf("Unknown address type 0x%02x", hostType)
	}
}

func readSocksPort(r io.Reader) (port uint16, err error) {
	var buf [2]byte
	_, err = io.ReadFull(r, buf[:])
	if err != nil {
		return
	}

	port = Ntohs(buf)
	return
}

func packSocksHost(hostType byte, host string) (data []byte, err error) {
	switch hostType {
	case SocksIPv4Host:
		ip := net.ParseIP(host)
		if ip == nil {
			err = fmt.Errorf("Invalid host %s", host)
			return
		}
		data = ip.To4()
		return
	case SocksIPv6Host:
		ip := net.ParseIP(host)
		if ip == nil {
			err = fmt.Errorf("Invalid host %s", host)
			return
		}
		data = ip.To16()
		return
	case SocksDomainHost:
		data = append(data, byte(len(host)))
		data = append(data, []byte(host)...)
		return
	default:
		err = fmt.Errorf("Unknown address type 0x%02x", hostType)
		return
	}
}

func readSocksComm(r io.Reader) (data socksCommon, err error) {
	var h [4]byte
	r = bufio.NewReader(r)
	_, err = io.ReadFull(r, h[:])
	if err != nil {
		return
	}

	if h[0] != SocksVersion {
		err = fmt.Errorf("Unsupported version 0x%02x", h[0])
		return
	}

	host, err := readSocksHost(r, h[3])
	if err != nil {
		return
	}

	port, err := readSocksPort(r)
	if err != nil {
		return
	}

	data.Flag = h[1]
	data.HostType = h[3]
	data.Host = host
	data.Port = port
	return
}

func writeSocksComm(w io.Writer, data *socksCommon) (n int, err error) {
	// buf := make([]byte, 4, smallBufSize)
	var buf [smallBufSize]byte

	buf[0] = SocksVersion
	buf[1] = data.Flag
	buf[2] = SocksReserved
	buf[3] = data.HostType

	h, err := packSocksHost(data.HostType, data.Host)
	if err != nil {
		return
	}
	copy(buf[4:], h)
	// buf = append(buf, h...)
	p := Htons(data.Port)
	// buf = append(buf, p[:]...)
	copy(buf[(4+len(h)):], p[:])

	n, err = w.Write(buf[0 : 4+len(h)+2])
	return
}

func ReadSocksRequest(r io.Reader) (req *SocksRequest, err error) {
	data, err := readSocksComm(r)
	if err != nil {
		return
	}
	req = &SocksRequest{data.Flag, data.HostType, data.Host, data.Port}
	return
}

func WriteSocksRequest(w io.Writer, req *SocksRequest) (n int, err error) {
	data := &socksCommon{req.Cmd, req.HostType, req.DstHost, req.DstPort}
	return writeSocksComm(w, data)
}

func ReadSocksReply(r io.Reader) (reply *SocksReply, err error) {
	data, err := readSocksComm(r)
	if err != nil {
		return
	}
	reply = &SocksReply{data.Flag, data.HostType, data.Host, data.Port}
	return
}

func ReplyGeneralFailure(w io.Writer, req *SocksRequest) (n int, err error) {
	host := "0.0.0.0"
	if req.HostType == SocksIPv6Host {
		host = "::"
	}
	return WriteSocksReply(w, &SocksReply{SocksGeneralFailure, SocksIPv4Host, host, 0})
}

func WriteSocksReply(w io.Writer, reply *SocksReply) (n int, err error) {
	data := &socksCommon{reply.Rep, reply.HostType, reply.BndHost, reply.BndPort}
	return writeSocksComm(w, data)
}

func ParseUDPRequest(data []byte) (udpReq *UDPRequest, err error) {
	udpReq = &UDPRequest{}
	total := len(data)
	if total <= 8 {
		err = fmt.Errorf("Invalid UDP Request: only %d bytes data", total)
		return
	}
	udpReq.Frag = data[2]
	udpReq.HostType = data[3]
	pos := 4
	r := bytes.NewReader(data[pos:])
	host, e := readSocksHost(r, udpReq.HostType)
	if err != nil {
		err = fmt.Errorf("Invalid UDP Request: fail to read dst host: %s", e)
		return
	}
	udpReq.DstHost = host
	port, e := readSocksPort(r)
	if err != nil {
		err = fmt.Errorf("Invalid UDP Request: fail to read dst port: %s", e)
		return
	}
	udpReq.DstPort = port
	udpReq.Data = data[total-r.Len():]
	return
}

func PackUDPRequest(udpReq *UDPRequest) []byte {
	buf := make([]byte, 4, largeBufSize)
	buf[0] = SocksReserved
	buf[1] = SocksReserved
	buf[2] = udpReq.Frag
	buf[3] = udpReq.HostType
	h, _ := packSocksHost(udpReq.HostType, udpReq.DstHost)
	buf = append(buf, h...)
	p := Htons(udpReq.DstPort)
	buf = append(buf, p[:]...)
	buf = append(buf, udpReq.Data...)
	return buf
}

func LegalClientAddr(clientAssociate *net.UDPAddr, addr *net.UDPAddr) bool {
	ip := clientAssociate.IP.String()
	if ip == "0.0.0.0" || ip == "::" {
		return true
	}
	if addr.String() == clientAssociate.String() {
		return true
	}
	return false
}
