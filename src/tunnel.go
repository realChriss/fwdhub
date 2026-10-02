package main

import (
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"
)

var types = []string{"local", "remote", "dynamic"}

type Tunnel struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	Host       string `json:"host"`
	LocalPort  int    `json:"local_port"`
	RemoteHost string `json:"remote_host,omitempty"`
	RemotePort int    `json:"remote_port,omitempty"`
}

type run struct {
	PID     int       `json:"pid"`
	Started time.Time `json:"started"`
}

var (
	nameRe   = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
	hostRe   = regexp.MustCompile(`^[A-Za-z0-9_.@:\[\]%][A-Za-z0-9_.@:\[\]%-]*$`)
	remoteRe = regexp.MustCompile(`^([A-Za-z0-9_.][A-Za-z0-9_.-]*|\[[0-9A-Za-z:.%]+\]|\*)$`)
)

func validPort(p int) bool { return p >= 1 && p <= 65535 }

func (t Tunnel) validate() error {
	switch {
	case !nameRe.MatchString(t.Name):
		return errors.New("name: use letters, digits, . _ -")
	case !hostRe.MatchString(t.Host):
		return errors.New("ssh host: use a config alias or user@host")
	case !validPort(t.LocalPort):
		return errors.New("local port: 1 to 65535")
	}
	switch t.Type {
	case "dynamic":
		return nil
	case "local", "remote":
		if !remoteRe.MatchString(t.RemoteHost) {
			return errors.New("remote host: host name, IPv4 or [IPv6]")
		}
		if !validPort(t.RemotePort) {
			return errors.New("remote port: 1 to 65535")
		}
		return nil
	}
	return errors.New("type: local, remote or dynamic")
}

func (t Tunnel) forward() []string {
	switch t.Type {
	case "remote":
		return []string{"-R", fmt.Sprintf("%s:%d:localhost:%d", t.RemoteHost, t.RemotePort, t.LocalPort)}
	case "dynamic":
		return []string{"-D", strconv.Itoa(t.LocalPort)}
	}
	return []string{"-L", fmt.Sprintf("%d:%s:%d", t.LocalPort, t.RemoteHost, t.RemotePort)}
}

func (t Tunnel) sshArgs() []string {
	args := []string{"-N",
		"-o", "BatchMode=yes",
		"-o", "ExitOnForwardFailure=yes",
		"-o", "ServerAliveInterval=15",
		"-o", "ServerAliveCountMax=3",
		"-o", "LogLevel=VERBOSE",
		"-o", "ControlPath=none",
	}
	args = append(args, t.forward()...)
	return append(args, "--", t.Host)
}

func logPath(name string) string { return filepath.Join(appDir, "logs", name+".log") }

func readLog(name string) string {
	b, _ := os.ReadFile(logPath(name))
	return strings.ReplaceAll(string(b), "\r\n", "\n")
}

func start(t Tunnel) (run, error) {
	if err := t.validate(); err != nil {
		return run{}, err
	}
	if t.Type != "remote" {
		l, err := net.Listen("tcp", net.JoinHostPort("localhost", strconv.Itoa(t.LocalPort)))
		if err != nil {
			const wsaeacces = 10013
			var errno syscall.Errno
			if errors.Is(err, os.ErrPermission) || errors.As(err, &errno) && errno == wsaeacces {
				return run{}, fmt.Errorf("port %d not permitted", t.LocalPort)
			}
			return run{}, fmt.Errorf("port %d in use", t.LocalPort)
		}
		l.Close()
	}
	f, err := os.Create(logPath(t.Name))
	if err != nil {
		return run{}, err
	}
	defer f.Close()

	cmd := exec.Command("ssh", t.sshArgs()...)
	cmd.Stdout, cmd.Stderr = f, f
	cmd.SysProcAttr = detach()
	if err := cmd.Start(); err != nil {
		return run{}, err
	}
	go cmd.Wait()
	return run{PID: cmd.Process.Pid, Started: time.Now()}, nil
}

func stop(r run) {
	if !sshAlive(r.PID) {
		return
	}
	if p, err := os.FindProcess(r.PID); err == nil {
		p.Kill()
		p.Release()
	}
}

const errHostKey = "host key not trusted"

func trustCmd(t Tunnel) *exec.Cmd {
	return exec.Command("ssh", "-o", "StrictHostKeyChecking=ask", "-o", "PasswordAuthentication=no", "--", t.Host, "exit")
}

func up(name string) bool { return strings.Contains(readLog(name), "Authenticated to") }

func failReason(log string, t Tunnel) string {
	low := strings.ToLower(log)
	has := func(subs ...string) bool {
		for _, s := range subs {
			if strings.Contains(low, s) {
				return true
			}
		}
		return false
	}
	switch {
	case has("address already in use", "cannot listen to port", "could not request local forwarding"):
		return fmt.Sprintf("port %d in use", t.LocalPort)
	case has("remote port forwarding failed"):
		return "forward refused"
	case has("host identification has changed"):
		return "host key changed"
	case has("host key verification failed"):
		return errHostKey
	case has("permission denied"):
		return "auth failed"
	case has("could not resolve hostname", "connection refused", "timed out", "no route to host", "network is unreachable"):
		return "host unreachable"
	}
	if lines := strings.Split(strings.TrimSpace(log), "\n"); lines[0] != "" {
		return lines[len(lines)-1]
	}
	return "ssh exited"
}

func sshHosts() []string {
	home, _ := os.UserHomeDir()
	b, _ := os.ReadFile(filepath.Join(home, ".ssh", "config"))
	return parseHosts(string(b))
}

func parseHosts(s string) []string {
	var hosts []string
	for _, line := range strings.Split(s, "\n") {
		f := strings.Fields(strings.ReplaceAll(line, "=", " "))
		if len(f) < 2 || !strings.EqualFold(f[0], "Host") {
			continue
		}
		for _, h := range f[1:] {
			if !strings.ContainsAny(h, "*?!") {
				hosts = append(hosts, h)
			}
		}
	}
	return hosts
}

func uptime(d time.Duration) string {
	h, m := int(d.Hours()), int(d.Minutes())%60
	switch {
	case h > 0:
		return fmt.Sprintf("%dh %dm", h, m)
	case m > 0:
		return fmt.Sprintf("%dm", m)
	}
	return fmt.Sprintf("%ds", int(d.Seconds()))
}
