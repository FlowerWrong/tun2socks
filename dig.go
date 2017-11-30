package main

import (
	"github.com/lixiangzhong/dnsutil"
	"log"
)

func main() {
	dig := new(dnsutil.Dig)
	dig.SetDNS("10.0.0.2") // or ns.xxx.com
	// dig.SetEDNS0ClientSubnet("1.1.1.1")
	a, err := dig.A("baidu.com")
	log.Println(a)
	log.Println(err)
}
