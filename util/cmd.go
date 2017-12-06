package util

import (
	"bytes"
	"log"
	"os/exec"
	"strings"
)

func ExecCommand(name, sargs string) error {
	args := strings.Split(sargs, " ")
	cmd := exec.Command(name, args...)
	log.Println("exec command: %s %s", name, sargs)
	return cmd.Run()
}

func ExecShell(s string) {
	cmd := exec.Command("/bin/bash", "-c", s)
	var out bytes.Buffer

	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		log.Fatal(err)
	}
	log.Println(out.String())
}
