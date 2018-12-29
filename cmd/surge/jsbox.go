// Package main https://github.com/lhie1/Rules
package main

import (
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"time"
	"unsafe"
)

var baseURL = "https://raw.githubusercontent.com/lhie1/Rules/master/"

var surgeRuleList = []string{
	"Apple",
	"REJECT",
	"PROXY",
	"DIRECT",
}

var surgeRewriteList = []string{
	"URL Rewrite",
	"URL REJECT",
	"Header Rewrite",
}

var filename = "surge.conf"

func fix(str string) string {
	str = strings.Replace(str, "🍃 Proxy", "PROXY", -1)
	str = strings.Replace(str, "🍂 Domestic", "DIRECT", -1)
	str = strings.Replace(str, "🍎 Only", "DIRECT", -1)
	str = strings.Replace(str, "☁️ Others", "DIRECT", -1)
	str = strings.Replace(str, "🏃 Auto", "PROXY", -1)
	return str
}

func writeToFile(file *os.File, name string) {
	response, err := http.Get(baseURL + "Auto/" + name + ".conf")
	if err != nil {
		log.Fatal(err)
	} else {
		defer response.Body.Close()

		bodyBytes, err := ioutil.ReadAll(response.Body)
		if err != nil {
			log.Fatal(err)
		}

		_, err = file.WriteString(fix(String(bodyBytes)))
		if err != nil {
			log.Fatal(err)
		}

		file.WriteString("\n\n")
	}
}

// String ...
func String(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}

func main() {
	rand.Seed(time.Now().UnixNano())
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	file, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatal("Open file", err)
	}
	defer file.Close()

	for _, f := range surgeRuleList {
		writeToFile(file, f)
	}

	file.WriteString("GEOIP,CN,DIRECT\n")
	file.WriteString("GEOIP,TW,DIRECT\n")
	file.WriteString("FINAL,PROXY\n")
	file.WriteString("\n\n")

	writeToFile(file, "HOST")

	file.WriteString("\n\n")

	for _, f := range surgeRewriteList {
		writeToFile(file, f)
	}

}
