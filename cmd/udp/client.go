package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
)

func main() {
	var host = flag.String("host", "localhost", "host")
	var port = flag.String("port", "6666", "port")
	flag.Parse()
	addr, err := net.ResolveUDPAddr("udp", *host+":"+*port)
	if err != nil {
		log.Println("Can't resolve address:", err)
		os.Exit(1)
	}
	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		log.Println("Can't dial:", err)
		os.Exit(1)
	}
	defer conn.Close()

	_, err = conn.Write([]byte("hello"))
	if err != nil {
		fmt.Println("failed:", err)
		os.Exit(1)
	}
	data := make([]byte, 1024)
	n, err := conn.Read(data)
	if err != nil {
		log.Println("failed to read UDP msg because of", err)
		os.Exit(1)
	}
	log.Println(string(data[0:n]))
	os.Exit(0)
}
