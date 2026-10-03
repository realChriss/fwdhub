package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
)

var version = "dev"

const repo = "realChriss/fwdhub"

func checkUpdate() {
	exe, err := os.Executable()
	if err == nil {
		exe, err = filepath.EvalSymlinks(exe)
	}
	if err != nil {
		return
	}
	os.Remove(exe + ".old")

	if version == "dev" {
		return
	}
	out, err := curl("--max-time", "3", "https://api.github.com/repos/"+repo+"/releases/latest")
	if err != nil {
		return
	}
	var rel struct {
		Tag string `json:"tag_name"`
	}
	if err := json.Unmarshal(out, &rel); err != nil || slices.Compare(semver(rel.Tag), semver(version)) <= 0 {
		return
	}

	fmt.Printf("fwdhub %s is available (you have %s). Update now? [y/N] ", rel.Tag, version)
	var answer string
	fmt.Scanln(&answer)
	if a := strings.ToLower(answer); a != "y" && a != "yes" {
		return
	}

	asset := "fwdhub-" + runtime.GOOS + "-" + runtime.GOARCH
	if runtime.GOOS == "windows" {
		asset += ".exe"
	}
	fmt.Println("Downloading " + asset + "...")
	out, err = curl("https://github.com/" + repo + "/releases/download/" + rel.Tag + "/" + asset)
	if err == nil {
		err = replaceExe(exe, bytes.NewReader(out))
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "fwdhub: update failed: %v\n", err)
		return
	}
	fmt.Printf("Updated to %s. Start fwdhub again to use it.\n", rel.Tag)
	os.Exit(0)
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
