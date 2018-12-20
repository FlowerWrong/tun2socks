package main

import (
	"bufio"
	"flag"
	"io"
	"log"
	"math/rand"
	"os"
	"strings"
	"time"
)

// Reading files requires checking most calls for errors.
// This helper will streamline our error checks below.
func check(e error) {
	if e != nil {
		panic(e)
	}
}

func main() {
	rand.Seed(time.Now().UnixNano())
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	var configFile string
	flag.StringVar(&configFile, "c", "", "config file")
	flag.Parse()

	log.Println(configFile)

	in, err := os.Open(configFile)
	check(err)
	defer in.Close()

	var domainSuffixDirect []string
	var domainSuffixProxy []string
	var domainSuffixReject []string

	var domainKeywordDirect []string
	var domainKeywordProxy []string
	var domainKeywordReject []string

	var domainDirect []string
	var domainProxy []string
	var domainReject []string

	var ipCidrDirect []string
	var ipCidrProxy []string
	var ipCidrReject []string

	var geoipDirect []string
	var geoipProxy []string
	var geoipReject []string

	br := bufio.NewReader(in)
	for {
		lineBytes, _, err := br.ReadLine()
		if err == io.EOF {
			break
		}
		line := string(lineBytes)

		// https://manual.nssurge.com/rule/domain-based.html
		// DOMAIN-SUFFIX DOMAIN-KEYWORD DOMAIN
		// IP-CIDR GEOIP
		// DIRECT PROXY REJECT
		if strings.HasPrefix(line, "DOMAIN-SUFFIX") || strings.HasPrefix(line, "DOMAIN-KEYWORD") || strings.HasPrefix(line, "DOMAIN") || strings.HasPrefix(line, "IP-CIDR") || strings.HasPrefix(line, "GEOIP") {
			sp := strings.Split(line, ",")
			rule, host, policy := sp[0], sp[1], sp[2]

			if rule == "DOMAIN-SUFFIX" {
				switch policy {
				case "DIRECT":
					domainSuffixDirect = append(domainSuffixDirect, host)
				case "PROXY":
					domainSuffixProxy = append(domainSuffixProxy, host)
				case "REJECT":
					domainSuffixReject = append(domainSuffixReject, host)
				}
			}

			if rule == "DOMAIN-KEYWORD" {
				switch policy {
				case "DIRECT":
					domainKeywordDirect = append(domainKeywordDirect, host)
				case "PROXY":
					domainKeywordProxy = append(domainKeywordProxy, host)
				case "REJECT":
					domainKeywordReject = append(domainKeywordReject, host)
				}
			}

			if rule == "DOMAIN" {
				switch policy {
				case "DIRECT":
					domainDirect = append(domainDirect, host)
				case "PROXY":
					domainProxy = append(domainProxy, host)
				case "REJECT":
					domainReject = append(domainReject, host)
				}
			}

			if rule == "IP-CIDR" {
				switch policy {
				case "DIRECT":
					ipCidrDirect = append(ipCidrDirect, host)
				case "PROXY":
					ipCidrProxy = append(ipCidrProxy, host)
				case "REJECT":
					ipCidrReject = append(ipCidrReject, host)
				}
			}

			if rule == "GEOIP" {
				switch policy {
				case "DIRECT":
					geoipDirect = append(geoipDirect, host)
				case "PROXY":
					geoipProxy = append(geoipProxy, host)
				case "REJECT":
					geoipReject = append(geoipReject, host)
				}
			}
		}
	}

	out, err := os.Create("config_from_surge.txt")
	check(err)
	defer out.Close()

	writeAndCheck(out, "session_name smart")
	writeAndCheck(out, "welcome_info 欢迎使用smart!")
	writeAndCheck(out, "debug true")
	writeAndCheck(out, "")

	writeAndCheck(out, "ip 172.25.0.1")
	writeAndCheck(out, "dns 223.5.5.5")
	writeAndCheck(out, "dns 223.6.6.6")
	writeAndCheck(out, "dns_ttl 60")
	writeAndCheck(out, "route 0.0.0.0/0")
	writeAndCheck(out, "mtu 1500")
	writeAndCheck(out, "")

	writeAndCheck(out, "## rules with action, there are 3 actions, proxy, direct and block.")

	writeAndCheck(out, "# domain rule")
	writeToFile(out, domainProxy, "proxy_domain")
	writeToFile(out, domainDirect, "direct_domain")
	writeToFile(out, domainReject, "block_domain")
	writeAndCheck(out, "")

	writeAndCheck(out, "# domain suffix rule")
	writeToFile(out, domainSuffixProxy, "proxy_domain_suffix")
	writeToFile(out, domainSuffixDirect, "direct_domain_suffix")
	writeToFile(out, domainSuffixReject, "block_domain_suffix")
	writeAndCheck(out, "")

	writeAndCheck(out, "# domain keyword rule")
	writeToFile(out, domainKeywordProxy, "proxy_domain_keyword")
	writeToFile(out, domainKeywordDirect, "direct_domain_keyword")
	writeToFile(out, domainKeywordReject, "block_domain_keyword")
	writeAndCheck(out, "")

	writeAndCheck(out, "# ip country rule")
	writeToFile(out, geoipProxy, "proxy_ip_country")
	writeToFile(out, geoipDirect, "direct_ip_country")
	writeToFile(out, geoipReject, "block_ip_country")
	writeAndCheck(out, "")

	writeAndCheck(out, "# ip cidr rule")
	writeToFile(out, ipCidrProxy, "proxy_ip_cidr")
	writeToFile(out, ipCidrDirect, "direct_ip_cidr")
	writeToFile(out, ipCidrReject, "block_ip_cidr")
	writeAndCheck(out, "")

	os.Exit(0)
}

func writeAndCheck(f *os.File, content string) {
	_, err := f.WriteString(content + "\n")
	check(err)
}

func writeToFile(f *os.File, rules []string, pattern string) {
	writeAndCheck(f, pattern+" "+strings.Join(rules[:], " "))
}
