package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net"
	"os"
	"sync"
	"time"
)

var r *rand.Rand

const BufSize = 1500

func init() {
	r = rand.New(rand.NewSource(time.Now().UnixNano()))
}

func RandomString(strlen int) []byte {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	result := make([]byte, strlen)
	for i := range result {
		result[i] = chars[r.Intn(len(chars))]
	}
	return result
}

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
		data := make([]byte, BufSize)
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
		sendData := RandomString(1000 * 300)
		dataLen := len(sendData)
		log.Println("Total len is", dataLen)
		writedLen := 0
		for {
			if dataLen <= 0 {
				log.Println("write success")
				break
			}
			if dataLen > BufSize {
				_, err = conn.Write(sendData[0:BufSize])
				if err != nil {
					fmt.Println("failed:", err)
					break
				}
				sendData = sendData[BufSize:]
				dataLen -= BufSize
				writedLen += BufSize
				time.Sleep(time.Millisecond * 10)
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
