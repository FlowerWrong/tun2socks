package proxy

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"strconv"
	"strings"
)

type socks5 struct {
	user, password string
	network, addr  string
	forward        Dialer
}

type SocksReply struct {
	HostType byte
	BndHost  string
	BndPort  uint16
}

const socks5Version = 5
const smallBufSize = 0x200

const (
	socks5AuthNone     = 0
	socks5AuthPassword = 2
)

const (
	socks5Connect = 1
	socks5UDP     = 3
)

const (
	Socks5AtypIP4    = 1
	Socks5AtypDomain = 3
	Socks5AtypIP6    = 4
)

var socks5Errors = []string{
	"",
	"general failure",
	"connection forbidden",
	"network unreachable",
	"host unreachable",
	"connection refused",
	"TTL expired",
	"command not supported",
	"address type not supported",
}

func Ntohs(data [2]byte) uint16 {
	return uint16(data[0])<<8 | uint16(data[1])<<0
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
	case Socks5AtypIP4:
		return readSocksIPv4Host(r)
	case Socks5AtypIP6:
		return readSocksIPv6Host(r)
	case Socks5AtypDomain:
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

// Dial connects to the address addr on the network net via the SOCKS5 proxy.
func (s *socks5) Dial(network, targetAddr string) (net.Conn, error) {
	switch network {
	case "tcp", "tcp6", "tcp4":
	case "udp", "udp6", "udp4":
	default:
		return nil, errors.New("proxy: no support for SOCKS5 proxy connections of type " + network)
	}

	host, portStr, err := net.SplitHostPort(targetAddr)
	if err != nil {
		return nil, err
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, errors.New("proxy: failed to parse port number: " + portStr)
	}
	if port < 1 || port > 0xffff {
		return nil, errors.New("proxy: port number out of range: " + portStr)
	}

	conn, err := s.forward.Dial(s.network, s.addr)
	if err != nil {
		return nil, err
	}

	if err := s.connectAndAuth(conn, host, port); err != nil {
		conn.Close()
		return nil, err
	}

	if strings.HasPrefix(network, "tcp") {
		if err := s.socks5ConnectRequest(conn, host, port); err != nil {
			conn.Close()
			return nil, err
		}
	}
	return conn, nil
}

// connectAndAuth takes an existing connection to a socks5 proxy server,
// and commands the server to extend that connection to target,
// which must be a canonical address with a host and port.
func (s *socks5) connectAndAuth(conn net.Conn, host string, port int) error {
	// the size here is just an estimate
	buf := make([]byte, 0, 6+len(host))

	buf = append(buf, socks5Version)
	if len(s.user) > 0 && len(s.user) < 256 && len(s.password) < 256 {
		buf = append(buf, 2 /* num auth methods */, socks5AuthNone, socks5AuthPassword)
	} else {
		buf = append(buf, 1 /* num auth methods */, socks5AuthNone)
	}

	if _, err := conn.Write(buf); err != nil {
		return errors.New("proxy: failed to write greeting to SOCKS5 proxy at " + s.addr + ": " + err.Error())
	}

	if _, err := io.ReadFull(conn, buf[:2]); err != nil {
		return errors.New("proxy: failed to read greeting from SOCKS5 proxy at " + s.addr + ": " + err.Error())
	}
	if buf[0] != 5 {
		return errors.New("proxy: SOCKS5 proxy at " + s.addr + " has unexpected version " + strconv.Itoa(int(buf[0])))
	}
	if buf[1] == 0xff {
		return errors.New("proxy: SOCKS5 proxy at " + s.addr + " requires authentication")
	}

	if buf[1] == socks5AuthPassword {
		buf = buf[:0]
		buf = append(buf, 1 /* password protocol version */)
		buf = append(buf, uint8(len(s.user)))
		buf = append(buf, s.user...)
		buf = append(buf, uint8(len(s.password)))
		buf = append(buf, s.password...)

		if _, err := conn.Write(buf); err != nil {
			return errors.New("proxy: failed to write authentication request to SOCKS5 proxy at " + s.addr + ": " + err.Error())
		}

		if _, err := io.ReadFull(conn, buf[:2]); err != nil {
			return errors.New("proxy: failed to read authentication reply from SOCKS5 proxy at " + s.addr + ": " + err.Error())
		}

		if buf[1] != 0 {
			return errors.New("proxy: SOCKS5 proxy at " + s.addr + " rejected username/password")
		}
	}

	return nil
}

func (s *socks5) socks5ConnectRequest(conn net.Conn, host string, port int) error {
	_, err := socks5Request(conn, host, port, socks5Connect)
	return err
}

func socks5Request(conn net.Conn, host string, port int, cmd byte) (reply *SocksReply, err error) {
	// the size here is just an estimate
	buf := make([]byte, 0, 6+len(host))

	buf = buf[:0]
	buf = append(buf, socks5Version, cmd, 0 /* reserved */)

	if ip := net.ParseIP(host); ip != nil {
		if ip4 := ip.To4(); ip4 != nil {
			buf = append(buf, Socks5AtypIP4)
			ip = ip4
		} else {
			buf = append(buf, Socks5AtypIP6)
		}
		buf = append(buf, ip...)
	} else {
		if len(host) > 255 {
			err = errors.New("proxy: destination hostname too long: " + host)
			return
		}
		buf = append(buf, Socks5AtypDomain)
		buf = append(buf, byte(len(host)))
		buf = append(buf, host...)
	}
	buf = append(buf, byte(port>>8), byte(port))

	if _, err = conn.Write(buf); err != nil {
		err = errors.New("proxy: failed to write connect request to SOCKS5 proxy: " + err.Error())
		return
	}

	if _, err = io.ReadFull(conn, buf[:4]); err != nil {
		err = errors.New("proxy: failed to read connect reply from SOCKS5 proxy: " + err.Error())
		return
	}

	failure := "unknown error"
	if int(buf[1]) < len(socks5Errors) {
		failure = socks5Errors[buf[1]]
	}

	if len(failure) > 0 {
		err = errors.New("proxy: SOCKS5 proxy failed to connect: " + failure)
		return
	}

	hostType := buf[3]
	bndAddr, err := readSocksHost(conn, hostType)
	if err != nil {
		err = fmt.Errorf("proxy: invalid request: fail to read dst host: %s", err)
		return
	}

	bndPort, err := readSocksPort(conn)
	if err != nil {
		err = fmt.Errorf("proxy: invalid request: fail to read dst port: %s", err)
		return
	}

	reply = &SocksReply{hostType, bndAddr, bndPort}
	return
}

func Socks5UDPRequest(conn net.Conn, host string, port int) (socks5UDPListen *net.UDPConn, reply *SocksReply, err error) {
	socks5UDPAddr := conn.LocalAddr().(*net.TCPAddr)
	socks5UDPListen, err = net.ListenUDP("udp", &net.UDPAddr{
		IP:   socks5UDPAddr.IP,
		Port: 0,
		Zone: socks5UDPAddr.Zone,
	})
	if err != nil {
		conn.Close()
		return
	}
	reply, err = socks5Request(conn, host, port, socks5UDP)
	if err != nil {
		socks5UDPListen.Close()
		conn.Close()
		return
	}

	return
}

func init() {
	registerDialerType("socks5", func(url *url.URL, forward Dialer) (Dialer, error) {
		s := &socks5{
			network: "tcp",
			addr:    url.Host,
			forward: forward,
		}

		if url.User != nil {
			s.user = url.User.Username()
			if p, ok := url.User.Password(); ok {
				s.password = p
			}
		}
		return s, nil
	})
}
