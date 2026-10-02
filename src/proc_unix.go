//go:build !windows

package main

import (
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

func detach() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setsid: true}
}

var verified = map[int]bool{}

func sshAlive(pid int) bool {
	if syscall.Kill(pid, 0) != nil {
		delete(verified, pid)
		return false
	}
	if !verified[pid] {
		out, _ := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "comm=").Output()
		verified[pid] = strings.HasSuffix(strings.TrimSpace(string(out)), "ssh")
	}
	return verified[pid]
}
