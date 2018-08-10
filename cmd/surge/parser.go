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

	out, err := os.Create("config_from_surge.ini")
	check(err)
	defer out.Close()

	writeToFile(out, domainSuffixDirect, "direct-website-suffix", "DOMAIN-SUFFIX")
	writeToFile(out, domainSuffixProxy, "proxy-website-suffix", "DOMAIN-SUFFIX")
	writeToFile(out, domainSuffixReject, "reject-website-suffix", "DOMAIN-SUFFIX")

	writeToFile(out, domainKeywordDirect, "direct-website-keyword", "DOMAIN-KEYWORD")
	writeToFile(out, domainKeywordProxy, "proxy-website-keyword", "DOMAIN-KEYWORD")
	writeToFile(out, domainKeywordReject, "reject-website-keyword", "DOMAIN-KEYWORD")

	writeToFile(out, domainDirect, "direct-website-domain", "DOMAIN-SUFFIX")
	writeToFile(out, domainProxy, "proxy-website-domain", "DOMAIN-SUFFIX")
	writeToFile(out, domainReject, "reject-website-domain", "DOMAIN-SUFFIX")

	writeToFile(out, ipCidrDirect, "direct-website-ipcidr", "IP-CIDR")
	writeToFile(out, ipCidrProxy, "proxy-website-ipcidr", "IP-CIDR")
	writeToFile(out, ipCidrReject, "reject-website-ipcidr", "IP-CIDR")

	writeToFile(out, geoipDirect, "direct-website-geoip", "IP-COUNTRY")
	writeToFile(out, geoipProxy, "proxy-website-geoip", "IP-COUNTRY")
	writeToFile(out, geoipReject, "reject-website-geoip", "IP-COUNTRY")

	writeAndCheck(out, "# rules define the order of checking pattern")
	writeAndCheck(out, "[rule]")
	writeAndCheck(out, "pattern = direct-website-suffix")
	writeAndCheck(out, "pattern = proxy-website-suffix")
	writeAndCheck(out, "pattern = reject-website-suffix")
	writeAndCheck(out, "pattern = direct-website-keyword")
	writeAndCheck(out, "pattern = proxy-website-keyword")
	writeAndCheck(out, "pattern = reject-website-keyword")
	writeAndCheck(out, "pattern = direct-website-domain")
	writeAndCheck(out, "pattern = proxy-website-domain")
	writeAndCheck(out, "pattern = reject-website-domain")
	writeAndCheck(out, "pattern = direct-website-ipcidr")
	writeAndCheck(out, "pattern = proxy-website-ipcidr")
	writeAndCheck(out, "pattern = reject-website-ipcidr")
	writeAndCheck(out, "pattern = direct-website-geoip")
	writeAndCheck(out, "pattern = proxy-website-geoip")
	writeAndCheck(out, "pattern = reject-website-geoip")

	_, err = out.WriteString("\n")
	check(err)
	writeAndCheck(out, "# set to a proxy for domain that don't match any pattern")
	writeAndCheck(out, "# DEFAULT VALUE: \"\"")
	writeAndCheck(out, "final = B")

	os.Exit(0)
}

func writeAndCheck(f *os.File, content string) {
	_, err := f.WriteString(content + "\n")
	check(err)
}

func writeToFile(f *os.File, rules []string, pattern, scheme string) {
	writeAndCheck(f, "[pattern \""+pattern+"\"]")
	if strings.HasPrefix(pattern, "reject") {
		writeAndCheck(f, "proxy = block")
	} else if strings.HasPrefix(pattern, "proxy") {
		writeAndCheck(f, "proxy = B")
	}
	writeAndCheck(f, "scheme = "+scheme)
	for _, host := range rules {
		_, err := f.WriteString("v = " + host + "\n")
		check(err)
	}
	writeAndCheck(f, "\n\n")
}
