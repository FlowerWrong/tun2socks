package gosocks

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"
)

const (
	DefaultPort = 1080
)

type Handler interface {
	ServeSocks(conn *SocksConn)
	Quit()
}

type ServerAuthenticator interface {
	ServerAuthenticate(conn *SocksConn) error
}

type Server struct {
	addr    string
	timeout time.Duration
	handler Handler
	auth    ServerAuthenticator
	msgCh   chan interface{}
	quit    chan bool
}

type UDPPacket struct {
	Addr *net.UDPAddr
	Data []byte
}

func (svr *Server) ListenAndServe() error {
	addr := svr.addr
	if addr == "" {
		addr = fmt.Sprintf(":%d", DefaultPort)
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	defer ln.Close()
	return svr.Serve(ln)
}

func (svr *Server) GetTimeout() time.Duration {
	return svr.timeout
}

// ChangeAuth safely changes authenticator when server is running
func (svr *Server) ChangeAuth(auth ServerAuthenticator) {
	select {
	case svr.msgCh <- auth:
	case <-svr.quit:
	}
}

func (svr *Server) ChangeHandler(handler Handler) {
	select {
	case svr.msgCh <- handler:
	case <-svr.quit:
	}
}

type ret struct {
	conn net.Conn
	err  error
}

func (svr *Server) Serve(ln net.Listener) error {
	// close quit channel after loop ends, so that attempts to change
	// authenticator or handler do not block.
	defer func() {
		close(svr.quit)
		svr.handler.Quit()
	}()

	ch := make(chan ret)
	go func() {
		// how long to sleep on accept failure
		var tempDelay time.Duration
		for {
			conn, e := ln.Accept()
			if e != nil {
				if ne, ok := e.(net.Error); ok && ne.Temporary() {
					if tempDelay == 0 {
						tempDelay = 5 * time.Millisecond
					} else {
						tempDelay *= 2
					}
					if max := 1 * time.Second; tempDelay > max {
						tempDelay = max
					}
					time.Sleep(tempDelay)
					continue
				} else {
					ch <- ret{nil, e}
					return
				}
			}
			tempDelay = 0
			ch <- ret{conn, e}
		}
	}()

	for {
		select {
		case r := <-ch:
			if r.err != nil {
				return r.err
			}

			go func(c net.Conn, to time.Duration, auth ServerAuthenticator, handler Handler) {
				socks := &SocksConn{Conn: c, Timeout: to}
				// if svr.Auth is nil, Handler should process authenticate.
				if auth != nil {
					if auth.ServerAuthenticate(socks) != nil {
						socks.Close()
						return
					}
				}
				handler.ServeSocks(socks)
			}(r.conn, svr.timeout, svr.auth, svr.handler)
		case msg := <-svr.msgCh:
			switch msg.(type) {
			case ServerAuthenticator:
				svr.auth = msg.(ServerAuthenticator)
			case Handler:
				svr.handler = msg.(Handler)
			}
		}
	}
}

type AnonymousServerAuthenticator struct{}

func (a *AnonymousServerAuthenticator) ServerAuthenticate(conn *SocksConn) (err error) {
	conn.SetReadDeadline(time.Now().Add(conn.Timeout))
	var h [smallBufSize]byte
	r := bufio.NewReader(conn)
	_, err = io.ReadFull(r, h[:2])
	if err != nil {
		return
	}

	if h[0] != SocksVersion {
		err = fmt.Errorf("Unsupported version 0x%02x", h[0])
		return
	}

	n := int(h[1])
	_, err = io.ReadFull(r, h[2:(2+n)])
	if err != nil {
		return
	}

	conn.SetWriteDeadline(time.Now().Add(conn.Timeout))
	var buf [2]byte
	buf[0] = SocksVersion
	for i := 0; i < n; i++ {
		if h[i+3] == SocksNoAuthentication {
			buf[1] = SocksNoAuthentication
			_, err = conn.Write(buf[:])
			return
		}
	}
	buf[1] = SocksNoAcceptableMethods
	_, err = conn.Write(buf[:])
	if err == nil {
		err = fmt.Errorf("NoAuthentication(0x%02x) not found in claimed methods", SocksNoAuthentication)
	}
	return
}

type BasicSocksHandler struct{}

func (h *BasicSocksHandler) HandleCmdConnect(req *SocksRequest, conn *SocksConn) {
	addr := SockAddrString(req.DstHost, req.DstPort)
	log.Printf("connect: %s", addr)
	remote, err := net.DialTimeout("tcp", addr, conn.Timeout)
	if err != nil {
		log.Printf("error in connecting remote target %s: %s", addr, err)
		ReplyGeneralFailure(conn, req)
		conn.Close()
		return
	}

	localAddr := remote.LocalAddr()
	hostType, host, port := NetAddrToSocksAddr(localAddr)
	conn.SetWriteDeadline(time.Now().Add(conn.Timeout))
	_, err = WriteSocksReply(conn, &SocksReply{SocksSucceeded, hostType, host, port})
	if err != nil {
		log.Printf("error in sending reply: %s", err)
		conn.Close()
		remote.Close()
		return
	}

	CopyLoopTimeout(conn, remote, conn.Timeout)
	log.Printf("TCP connection done")
}

func (h *BasicSocksHandler) UDPAssociateFirstPacket(req *SocksRequest, conn *SocksConn) (*net.UDPConn, *net.UDPAddr, *UDPRequest, *net.UDPAddr, error) {
	log.Printf("udp associate: %s:%d", req.DstHost, req.DstPort)
	socksAddr := conn.LocalAddr().(*net.TCPAddr)
	// create one UDP to recv/send packets from client
	clientBind, err := net.ListenUDP("udp", &net.UDPAddr{
		IP:   socksAddr.IP,
		Port: 0,
		Zone: socksAddr.Zone,
	})
	if err != nil {
		log.Printf("error in binding local UDP: %s", err)
		ReplyGeneralFailure(conn, req)
		return nil, nil, nil, nil, err
	}

	bindAddr := clientBind.LocalAddr()
	hostType, host, port := NetAddrToSocksAddr(bindAddr)
	log.Printf("UDP bind local address: %s", bindAddr.String())
	conn.SetWriteDeadline(time.Now().Add(conn.Timeout))
	_, err = WriteSocksReply(conn, &SocksReply{SocksSucceeded, hostType, host, port})
	if err != nil {
		log.Printf("error in sending reply: %s", err)
		clientBind.Close()
		return nil, nil, nil, nil, err
	}
	clientAssociate := SocksAddrToNetAddr("udp", req.DstHost, req.DstPort).(*net.UDPAddr)

	clientBind.SetReadDeadline(time.Now().Add(conn.Timeout))
	var udpReq *UDPRequest
	var buf [largeBufSize]byte
	var clientAddr *net.UDPAddr
loop:
	for {
		n, addr, err := clientBind.ReadFromUDP(buf[:])
		if err != nil {
			log.Printf("error in reading UDP packet from client: %s", err)
			clientBind.Close()
			return nil, nil, nil, nil, err
		}
		// validation
		// 1) RFC1928 Section-7
		if !LegalClientAddr(clientAssociate, addr) {
			log.Printf("illegal addr: %s vs %s", clientAssociate.IP.String(), addr.String())
			continue
		}
		// 2) format
		udpReq, err = ParseUDPRequest(buf[:n])
		if err != nil {
			log.Printf("error to parse UDP packet: %s", err)
			clientBind.Close()
			return nil, nil, nil, nil, err
		}
		// 3) no fragment
		if udpReq.Frag != SocksNoFragment {
			continue
		}
		clientAddr = addr
		break loop
	}
	return clientBind, clientAssociate, udpReq, clientAddr, nil
}

func (h *BasicSocksHandler) UDPAssociateForwarding(conn *SocksConn, clientBind *net.UDPConn, clientAssociate *net.UDPAddr, firstPkt *UDPRequest, clientAddr *net.UDPAddr) {
	forwardingAddr := SocksAddrToNetAddr("udp", firstPkt.DstHost, firstPkt.DstPort).(*net.UDPAddr)
	c, err := net.DialUDP("udp", nil, forwardingAddr)
	if err != nil {
		log.Printf("error to connect UDP target (%s):%s", forwardingAddr.String(), err)
		clientBind.Close()
		conn.Close()
		return
	}
	uaddr := c.LocalAddr().(*net.UDPAddr)
	uaddr.Port = 0
	c.Close()
	forwardingBind, _ := net.ListenUDP("udp", uaddr)
	_, err = forwardingBind.WriteToUDP(firstPkt.Data, forwardingAddr)
	if err != nil {
		log.Printf("error to send UDP packet to remote: %s", err)
		forwardingBind.Close()
		clientBind.Close()
		return
	}

	// monitoring socks connection, quit for any reading event
	quit := make(chan bool)
	go ConnMonitor(conn, quit)

	// read client UPD packets
	chClientUDP := make(chan *UDPPacket)
	go UDPReader(clientBind, chClientUDP, quit)

	// read remote UPD packets
	chRemoteUDP := make(chan *UDPPacket)
	go UDPReader(forwardingBind, chRemoteUDP, quit)

loop:
	for {
		t := time.NewTimer(conn.Timeout)
		select {
		// packets from client
		case pkt, ok := <-chClientUDP:
			t.Stop()
			if !ok {
				break loop
			}
			// validation
			// 1) RFC1928 Section-7
			if !LegalClientAddr(clientAssociate, pkt.Addr) {
				continue
			}
			// 2) format
			udpReq, err := ParseUDPRequest(pkt.Data)
			if err != nil {
				log.Printf("error to parse UDP packet: %s", err)
				break loop
			}
			// 3) no fragment
			if udpReq.Frag != SocksNoFragment {
				continue
			}

			// update clientAddr (not required)
			clientAddr = pkt.Addr
			forwardingAddr := SocksAddrToNetAddr("udp", udpReq.DstHost, udpReq.DstPort).(*net.UDPAddr)
			_, err = forwardingBind.WriteToUDP(udpReq.Data, forwardingAddr)
			if err != nil {
				log.Printf("error to send UDP packet to remote: %s", err)
				break loop
			}

		// packets from remote
		case pkt, ok := <-chRemoteUDP:
			t.Stop()
			if !ok {
				break loop
			}

			hostType, host, port := NetAddrToSocksAddr(pkt.Addr)
			data := PackUDPRequest(&UDPRequest{SocksNoFragment, hostType, host, port, pkt.Data})
			_, err := clientBind.WriteToUDP(data, clientAddr)
			if err != nil {
				log.Printf("error to send UDP packet to client: %s", err)
				break loop
			}

		case <-quit:
			t.Stop()
			log.Printf("UDP unexpected event from socks connection")
			break loop

		case <-t.C:
			log.Printf("UDP timeout")
			break loop
		}
		t.Stop()
	}

	conn.Close()
	clientBind.Close()
	forwardingBind.Close()

	// readeres may block on writing, try read to wake them so they
	// are aware that the underlying connection has closed.
	<-chClientUDP
	<-chRemoteUDP
}

func (h *BasicSocksHandler) HandleCmdUDPAssociate(req *SocksRequest, conn *SocksConn) {
	clientBind, clientAssociate, udpReq, clientAddr, err := h.UDPAssociateFirstPacket(req, conn)
	if err != nil {
		conn.Close()
		return
	}
	h.UDPAssociateForwarding(conn, clientBind, clientAssociate, udpReq, clientAddr)
	log.Printf("UDP connection done")
}

func UDPReader(u *net.UDPConn, ch chan<- *UDPPacket, quit chan bool) {
	u.SetDeadline(time.Time{})
	var buf [largeBufSize]byte
loop:
	for {
		n, addr, err := u.ReadFromUDP(buf[:])
		if err != nil {
			break loop
		}
		b := make([]byte, n)
		copy(b, buf[:n])
		select {
		case ch <- &UDPPacket{addr, b}:
		case <-quit:
			break loop
		}
	}
	close(ch)
}

func ConnMonitor(c net.Conn, quit chan bool) {
	var buf [1]byte
	c.SetDeadline(time.Time{})
	r := bufio.NewReader(c)
	r.Read(buf[:])
	close(quit)
}

type timeoutConn struct {
	c net.Conn
	t time.Duration
}

func (tc timeoutConn) Read(buf []byte) (int, error) {
	tc.c.SetDeadline(time.Now().Add(tc.t))
	return tc.c.Read(buf)
}

func (tc timeoutConn) Write(buf []byte) (int, error) {
	tc.c.SetDeadline(time.Now().Add(tc.t))
	return tc.c.Write(buf)
}

func (tc timeoutConn) Close() {
	tc.c.Close()
}

func CopyLoopTimeout(c1 net.Conn, c2 net.Conn, timeout time.Duration) {
	tc1 := timeoutConn{c: c1, t: timeout}
	tc2 := timeoutConn{c: c2, t: timeout}
	var wg sync.WaitGroup
	copyer := func(dst timeoutConn, src timeoutConn) {
		defer wg.Done()
		_, e := io.Copy(dst, src)
		dst.Close()
		if e != nil {
			src.Close()
		}
	}
	wg.Add(2)
	go copyer(tc1, tc2)
	go copyer(tc2, tc1)
	wg.Wait()
}

func (h *BasicSocksHandler) Quit() {}

func (h *BasicSocksHandler) ServeSocks(conn *SocksConn) {
	conn.SetReadDeadline(time.Now().Add(conn.Timeout))
	req, err := ReadSocksRequest(conn)
	if err != nil {
		log.Printf("error in ReadSocksRequest: %s", err)
		return
	}

	switch req.Cmd {
	case SocksCmdConnect:
		h.HandleCmdConnect(req, conn)
		return
	case SocksCmdUDPAssociate:
		h.HandleCmdUDPAssociate(req, conn)
		return
	case SocksCmdBind:
		conn.Close()
		return
	default:
		return
	}
}

func NewBasicServer(addr string, to time.Duration) *Server {
	return &Server{
		addr:    addr,
		timeout: to,
		handler: &BasicSocksHandler{},
		auth:    &AnonymousServerAuthenticator{},
		msgCh:   make(chan interface{}),
		quit:    make(chan bool),
	}
}

func NewServer(addr string, to time.Duration, handler Handler, auth ServerAuthenticator) *Server {
	return &Server{
		addr:    addr,
		timeout: to,
		handler: handler,
		auth:    auth,
		msgCh:   make(chan interface{}),
		quit:    make(chan bool),
	}
}
