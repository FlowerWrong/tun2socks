package util

import (
	"bytes"
	"log"
	"os"
	"os/exec"
	"strings"
)

func ExecCommand(name, sargs string) error {
	args := strings.Split(sargs, " ")
	cmd := exec.Command(name, args...)
	log.Printf("exec command: %s %s", name, sargs)
	return cmd.Run()
}

func ExecShell(s string) {
	cmd := exec.Command("/bin/bash", "-c", s)
	var out bytes.Buffer

	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		log.Println("Run shell command failed", err)
	}
	log.Println(out.String())
}

// Exit tun2socks
func Exit(tunName string) {
	UpdateDNSServers(false, tunName)
	os.Exit(0)
}
