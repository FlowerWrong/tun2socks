package main

import (
	"log"
	"net"
)

func main() {
	port := ":6666"
	protocol := "udp"

	udpAddr, err := net.ResolveUDPAddr(protocol, port)
	if err != nil {
		log.Println("Bad Address", err)
		return
	}

	udpConn, err := net.ListenUDP(protocol, udpAddr)
	if err != nil {
		log.Println("Listen UDP failed", err)
		return
	}
	defer udpConn.Close()
	log.Println("Listen UDP on 0.0.0.0", port)

	for {
		handleClient(udpConn)
	}
}

func handleClient(conn *net.UDPConn) {
	var buf [2048]byte
	n, remote, err := conn.ReadFromUDP(buf[0:])
	if err != nil {
		log.Println("Error Reading")
	} else {
		log.Println(string(buf[0:n]), "from", remote)
	}
	conn.WriteToUDP(buf[:n], remote)
}
