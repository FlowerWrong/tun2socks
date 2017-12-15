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

	readedLen := 0
	writedLen := 0

	for {
		var buf [1500]byte
		n, remote, err := udpConn.ReadFromUDP(buf[0:])
		if err != nil {
			log.Println("Error Reading", err)
		}
		readedLen += n
		m, err := udpConn.WriteToUDP(buf[0:n], remote)
		if err != nil {
			log.Println("Error Writing", err)
		}
		writedLen += m
		log.Println("readedLen", readedLen, " <->  writedLen", writedLen)
	}
}
