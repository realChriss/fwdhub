package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func TestForward(t *testing.T) {
	cases := map[string]Tunnel{
		"-L 5432:db.internal:5432":       {Type: "local", LocalPort: 5432, RemoteHost: "db.internal", RemotePort: 5432},
		"-R 0.0.0.0:9000:localhost:3000": {Type: "remote", LocalPort: 3000, RemoteHost: "0.0.0.0", RemotePort: 9000},
		"-D 1080":                        {Type: "dynamic", LocalPort: 1080},
	}
	for want, tun := range cases {
		if got := tun.forward(); got[0]+" "+got[1] != want {
			t.Errorf("got %v, want %s", got, want)
		}
	}
}

func TestValidate(t *testing.T) {
	ok := Tunnel{Name: "prod-db", Type: "local", Host: "me@bastion", LocalPort: 5432, RemoteHost: "db.internal", RemotePort: 5432}
	if err := ok.validate(); err != nil {
		t.Fatal(err)
	}
	bad := map[string]func(*Tunnel){
		"option as host":        func(x *Tunnel) { x.Host = "-oProxyCommand=calc" },
		"option as remote host": func(x *Tunnel) { x.RemoteHost = "-x" },
		"path in name":          func(x *Tunnel) { x.Name = "../x" },
		"port zero":             func(x *Tunnel) { x.LocalPort = 0 },
		"port too big":          func(x *Tunnel) { x.RemotePort = 70000 },
		"unknown type":          func(x *Tunnel) { x.Type = "" },
	}
	for name, change := range bad {
		x := ok
		change(&x)
		if x.validate() == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestFailReason(t *testing.T) {
	tun := Tunnel{LocalPort: 5432}
	cases := map[string]string{
		"bind [127.0.0.1]:5432: Permission denied\r\nchannel_setup_fwd_listener_tcpip: cannot listen to port: 5432": "port 5432 in use",
		"me@host: Permission denied (publickey).":                      "auth failed",
		"ssh: Could not resolve hostname nope: No such host is known.": "host unreachable",
		"ssh: connect to host 192.0.2.1 port 22: Connection timed out": "host unreachable",
		"Error: remote port forwarding failed for listen port 80":      "forward refused",
		"Host key verification failed.":                                "host key not trusted",
		"something new\nlast line":                                     "last line",
		"":                                                             "ssh exited",
	}
	for log, want := range cases {
		if got := failReason(log, tun); got != want {
			t.Errorf("%q: got %q, want %q", log, got, want)
		}
	}
}

func TestParseHosts(t *testing.T) {
	got := parseHosts("Host bastion vps\n  HostName 1.2.3.4\nhost=home\nHost *\nHost *.internal !x\n")
	if want := []string{"bastion", "vps", "home"}; !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestUptime(t *testing.T) {
	cases := map[time.Duration]string{45 * time.Second: "45s", 12 * time.Minute: "12m", 134 * time.Minute: "2h 14m"}
	for d, want := range cases {
		if got := uptime(d); got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	}
}

func TestStartStop(t *testing.T) {
	if _, err := exec.LookPath("ssh"); err != nil {
		t.Skip("ssh not installed")
	}
	appDir = t.TempDir()
	os.MkdirAll(filepath.Join(appDir, "logs"), 0o700)

	r, err := start(Tunnel{Name: "t", Type: "dynamic", Host: "192.0.2.1", LocalPort: 47811})
	if err != nil {
		t.Fatal(err)
	}
	if !sshAlive(r.PID) {
		t.Fatalf("ssh not running: %s", readLog("t"))
	}
	stop(r)
	for range 50 {
		if !sshAlive(r.PID) {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("ssh still running after stop")
}

func TestAddTunnel(t *testing.T) {
	appDir = t.TempDir()
	m := newModel(nil, map[string]run{})
	send := func(keys ...string) {
		for _, k := range keys {
			msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
			switch k {
			case "tab":
				msg = tea.KeyMsg{Type: tea.KeyTab}
			case "enter":
				msg = tea.KeyMsg{Type: tea.KeyEnter}
			case "down":
				msg = tea.KeyMsg{Type: tea.KeyDown}
			}
			m.Update(msg)
		}
	}
	send("a", "db", "tab", "tab", "bastion", "tab", "5432", "tab", "tab")
	m.View()
	send("enter", "enter")

	want := Tunnel{Name: "db", Type: "local", Host: "bastion", LocalPort: 5432, RemoteHost: "localhost", RemotePort: 5432}
	if m.mode != modeList || len(m.tunnels) != 1 || m.tunnels[0] != want {
		t.Fatalf("mode %d, err %q, tunnels %+v", m.mode, m.formErr, m.tunnels)
	}
	var saved config
	if err := loadJSON("tunnels.json", &saved); err != nil || len(saved.Tunnels) != 1 {
		t.Fatalf("saved %+v, err %v", saved, err)
	}
	if !strings.Contains(m.View(), "localhost:5432") {
		t.Fatal("tunnel missing from list view")
	}

	send("a", "web", "tab", "tab")
	m.hosts = []string{"bastion", "vps", "home"}
	if !strings.Contains(m.View(), "home") {
		t.Fatal("host list not shown")
	}
	send("v", "down", "tab", "80", "enter", "enter", "enter", "enter")
	if len(m.tunnels) != 2 || m.tunnels[1].Host != "vps" {
		t.Fatalf("err %q, tunnels %+v", m.formErr, m.tunnels)
	}
}

func TestReplaceExe(t *testing.T) {
	exe := filepath.Join(t.TempDir(), "fwdhub")
	if err := os.WriteFile(exe, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := replaceExe(exe, strings.NewReader("new")); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(exe); string(b) != "new" {
		t.Errorf("got %q, want new", b)
	}
	if _, err := os.Stat(exe + ".new"); err == nil {
		t.Error(".new left behind")
	}
}

func TestSemver(t *testing.T) {
	cases := []struct {
		tag, cur string
		want     bool
	}{
		{"v0.1.10", "v0.1.9", true},
		{"v0.2.0", "v0.1.99", true},
		{"v0.1.9", "v0.1.10", false},
		{"v0.1.5", "v0.1.5", false},
		{"", "v0.1.5", false},
	}
	for _, c := range cases {
		if got := slices.Compare(semver(c.tag), semver(c.cur)) > 0; got != c.want {
			t.Errorf("%s newer than %s: got %v", c.tag, c.cur, got)
		}
	}
}
