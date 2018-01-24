package util

import (
	"bytes"
	"log"
	"os/exec"
	"strings"
)

// ExecCommand ...
func ExecCommand(name, sargs string) error {
	args := strings.Split(sargs, " ")
	cmd := exec.Command(name, args...)
	log.Printf("[command] %s %s", name, sargs)
	return cmd.Run()
}

// ExecCommandWithOutput ...
func ExecCommandWithOutput(name, sargs string) ([]byte, error) {
	args := strings.Split(sargs, " ")
	out, err := exec.Command(name, args...).Output()
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ExecShell ...
func ExecShell(s string) {
	cmd := exec.Command("bash", "-c", s)
	var out bytes.Buffer

	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		log.Println("[shell] run shell command failed", err)
	}
	log.Println("[shell]", out.String())
}
