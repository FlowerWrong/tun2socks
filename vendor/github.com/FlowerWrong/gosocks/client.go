package gosocks

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"strconv"
	"time"
)

type ClientAuthenticator interface {
	ClientAuthenticate(conn *SocksConn) error
}

type SocksDialer struct {
	Timeout time.Duration
	Auth    ClientAuthenticator
}

type AnonymousClientAuthenticator struct{}

type UserPasswordClientAuthenticator struct {
	user, password string
}

func (a *UserPasswordClientAuthenticator) ClientAuthenticate(conn *SocksConn) (err error) {
	conn.SetWriteDeadline(time.Now().Add(conn.Timeout))
	var req [4]byte
	req[0] = SocksVersion
	req[1] = 2
	req[2] = SocksNoAuthentication
	req[3] = SocksAuthUserPassword
	_, err = conn.Write(req[:])
	if err != nil {
		return
	}

	conn.SetReadDeadline(time.Now().Add(conn.Timeout))
	var resp [2]byte
	r := bufio.NewReader(conn)
	_, err = io.ReadFull(r, resp[:2])
	if err != nil {
		return
	}
	if resp[0] != SocksVersion {
		err = errors.New("proxy: SOCKS5 proxy has unexpected version " + strconv.Itoa(int(resp[0])))
		return
	}
	if resp[1] == SocksAuthUserPassword {
		buf := make([]byte, 0, 3+uint8(len(a.user))+uint8(len(a.password)))
		buf = append(buf, 1 /* password protocol version */)
		buf = append(buf, uint8(len(a.user)))
		buf = append(buf, a.user...)
		buf = append(buf, uint8(len(a.password)))
		buf = append(buf, a.password...)

		if _, err := conn.Write(buf); err != nil {
			return errors.New("proxy: failed to write authentication request to SOCKS5 proxy : " + err.Error())
		}

		if _, err := io.ReadFull(conn, buf[:2]); err != nil {
			return errors.New("proxy: failed to read authentication reply from SOCKS5 proxy : " + err.Error())
		}

		if buf[1] != 0 {
			return errors.New("proxy: SOCKS5 proxy rejected username/password")
		}
	}
	return
}

func (a *AnonymousClientAuthenticator) ClientAuthenticate(conn *SocksConn) (err error) {
	conn.SetWriteDeadline(time.Now().Add(conn.Timeout))
	var req [3]byte
	req[0] = SocksVersion
	req[1] = 1
	req[2] = SocksNoAuthentication
	_, err = conn.Write(req[:])
	if err != nil {
		return
	}

	conn.SetReadDeadline(time.Now().Add(conn.Timeout))
	var resp [2]byte
	r := bufio.NewReader(conn)
	_, err = io.ReadFull(r, resp[:2])
	if err != nil {
		return
	}
	if resp[0] != SocksVersion || resp[1] != SocksNoAuthentication {
		err = fmt.Errorf("Fail to pass anonymous authentication: (0x%02x, 0x%02x)", resp[0], resp[1])
		return
	}
	return
}

func (d *SocksDialer) Dial(rawURL string) (conn *SocksConn, err error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	c, err := net.DialTimeout("tcp", u.Host, d.Timeout)
	if err != nil {
		return
	}
	conn = &SocksConn{c.(*net.TCPConn), d.Timeout}

	password, _ := u.User.Password()
	username := u.User.Username()
	if len(username) > 0 && len(username) < 256 && len(password) < 256 {
		d.Auth = &UserPasswordClientAuthenticator{username, password}
	}

	err = d.Auth.ClientAuthenticate(conn)
	if err != nil {
		conn.Close()
		return
	}
	return
}

func ClientAuthAnonymous(conn *SocksConn) (err error) {
	conn.SetWriteDeadline(time.Now().Add(conn.Timeout))
	var req [3]byte
	req[0] = SocksVersion
	req[1] = 1
	req[2] = SocksNoAuthentication
	_, err = conn.Write(req[:])
	if err != nil {
		return
	}

	conn.SetReadDeadline(time.Now().Add(conn.Timeout))
	var resp [2]byte
	r := bufio.NewReader(conn)
	_, err = io.ReadFull(r, resp[:2])
	if err != nil {
		return
	}
	if resp[0] != SocksVersion || resp[1] != SocksNoAuthentication {
		err = fmt.Errorf("Fail to pass anonymous authentication: (0x%02x, 0x%02x)", resp[0], resp[1])
		return
	}
	return
}

func ClientRequest(conn *SocksConn, req *SocksRequest) (reply *SocksReply, err error) {
	conn.SetWriteDeadline(time.Now().Add(conn.Timeout))
	_, err = WriteSocksRequest(conn, req)
	if err != nil {
		return
	}
	conn.SetReadDeadline(time.Now().Add(conn.Timeout))
	reply, err = ReadSocksReply(conn)
	return
}
