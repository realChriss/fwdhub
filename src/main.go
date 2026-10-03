package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	tea "github.com/charmbracelet/bubbletea"
)

var appDir string

type config struct {
	Tunnels []Tunnel `json:"tunnels"`
}

func loadJSON(name string, v any) error {
	b, err := os.ReadFile(filepath.Join(appDir, name))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}

func saveJSON(name string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(appDir, name)
	if err := os.WriteFile(path+".tmp", b, 0o600); err != nil {
		return err
	}
	return os.Rename(path+".tmp", path)
}

func fatal(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "fwdhub: "+format+"\n", a...)
	os.Exit(1)
}

func main() {
	if _, err := exec.LookPath("ssh"); err != nil {
		hint := map[string]string{
			"windows": "Run as admin: Add-WindowsCapability -Online -Name OpenSSH.Client~~~~0.0.1.0",
			"linux":   "Install the openssh-client package.",
		}[runtime.GOOS]
		fatal("ssh not found in PATH. %s", hint)
	}

	checkUpdate()

	base, err := os.UserConfigDir()
	if err != nil {
		fatal("%v", err)
	}
	appDir = filepath.Join(base, "fwdhub")
	if err := os.MkdirAll(filepath.Join(appDir, "logs"), 0o700); err != nil {
		fatal("%v", err)
	}

	var cfg config
	if err := loadJSON("tunnels.json", &cfg); err != nil {
		fatal("%s: %v", filepath.Join(appDir, "tunnels.json"), err)
	}
	seen := map[string]bool{}
	for _, t := range cfg.Tunnels {
		if err := t.validate(); err != nil {
			fatal("%s: tunnel %q: %v", filepath.Join(appDir, "tunnels.json"), t.Name, err)
		}
		if seen[t.Name] {
			fatal("%s: tunnel %q: name used twice", filepath.Join(appDir, "tunnels.json"), t.Name)
		}
		seen[t.Name] = true
	}
	runs := map[string]run{}
	if err := loadJSON("state.json", &runs); err != nil {
		fatal("%s: %v", filepath.Join(appDir, "state.json"), err)
	}
	if runs == nil {
		runs = map[string]run{}
	}

	if _, err := tea.NewProgram(newModel(cfg.Tunnels, runs), tea.WithAltScreen()).Run(); err != nil {
		fatal("%v", err)
	}
}
