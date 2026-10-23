package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

var version = "dev"

const repo = "realChriss/fwdhub"

type updateMsg string

type updatedMsg struct {
	tag string
	err error
}

func exePath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(exe)
}

func checkUpdate() tea.Msg {
	if version == "dev" {
		return nil
	}
	out, err := curl("--max-time", "10", "https://api.github.com/repos/"+repo+"/releases/latest")
	if err != nil {
		return nil
	}
	var rel struct {
		Tag string `json:"tag_name"`
	}
	if err := json.Unmarshal(out, &rel); err != nil || slices.Compare(semver(rel.Tag), semver(version)) <= 0 {
		return nil
	}
	return updateMsg(rel.Tag)
}

func installUpdate(tag string) tea.Cmd {
	return func() tea.Msg {
		asset := "fwdhub-" + runtime.GOOS + "-" + runtime.GOARCH
		if runtime.GOOS == "windows" {
			asset += ".exe"
		}
		exe, err := exePath()
		if err == nil {
			var out []byte
			if out, err = curl("https://github.com/" + repo + "/releases/download/" + tag + "/" + asset); err == nil {
				err = replaceExe(exe, bytes.NewReader(out))
			}
		}
		return updatedMsg{tag, err}
	}
}

func curl(args ...string) ([]byte, error) {
	name := "curl"
	if runtime.GOOS == "windows" {
		name = "curl.exe"
	}
	out, err := exec.Command(name, append([]string{"-fsSL"}, args...)...).Output()
	if ee, ok := err.(*exec.ExitError); ok {
		err = errors.New(strings.TrimSpace(string(ee.Stderr)))
	}
	return out, err
}

func replaceExe(exe string, r io.Reader) error {
	f, err := os.OpenFile(exe+".new", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	_, err = io.Copy(f, r)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil && runtime.GOOS != "windows" {
		err = os.Rename(exe+".new", exe)
	} else if err == nil {
		if err = os.Rename(exe, exe+".old"); err == nil {
			if err = os.Rename(exe+".new", exe); err != nil {
				os.Rename(exe+".old", exe)
			}
		}
	}
	if err != nil {
		os.Remove(exe + ".new")
	}
	return err
}

func semver(v string) []int {
	var n []int
	for _, p := range strings.Split(strings.TrimPrefix(v, "v"), ".") {
		i, _ := strconv.Atoi(p)
		n = append(n, i)
	}
	return n
}
