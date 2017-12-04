package util

import (
	"strings"
	"os/exec"
	"log"
)

func ExecCommand(name, sargs string) error {
	args := strings.Split(sargs, " ")
	cmd := exec.Command(name, args...)
	log.Println("exec command: %s %s", name, sargs)
	return cmd.Run()
}
