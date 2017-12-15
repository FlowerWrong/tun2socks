package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"sync"
	"time"
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

	var wg sync.WaitGroup

	wg.Add(1)
	go func(conn *net.UDPConn) {
		recvLen := 0
		data := make([]byte, 1500)
		for {
			conn.SetReadDeadline(time.Now().Add(time.Second * 10))
			n, err := conn.Read(data)
			if err != nil {
				log.Println("failed to read UDP msg because of", err)
				break
			}
			recvLen += n
			conn.SetReadDeadline(time.Time{})
			log.Println("recvLen", recvLen, "read", n, "bytes")
		}
		log.Println("Total recv len", recvLen)
		wg.Done()
	}(conn)

	wg.Add(1)
	go func(conn *net.UDPConn) {
		sendData, err := ioutil.ReadFile("GitHub.html")
		if err != nil {
			log.Println("read file failed", err)
		}
		dataLen := len(sendData)
		log.Println("Total len is", dataLen)
		writeLen := 1500
		writedLen := 0
		for {
			if dataLen <= 0 {
				log.Println("write success")
				break
			}
			if dataLen > 1500 {
				_, err = conn.Write(sendData[0:writeLen])
				if err != nil {
					fmt.Println("failed:", err)
					break
				}
				sendData = sendData[writeLen:]
				dataLen -= writeLen
				writedLen += writeLen
				// FIXME 写入大文件太快，会丢包
				// time.Sleep(time.Millisecond * 10)
			} else {
				_, err = conn.Write(sendData[0:dataLen])
				if err != nil {
					fmt.Println("failed:", err)
					break
				}
				writedLen += dataLen
				dataLen -= dataLen
			}
			log.Println("writedLen", writedLen, " dataLen", dataLen)
		}
		wg.Done()
	}(conn)

	wg.Wait()
	os.Exit(0)
}
