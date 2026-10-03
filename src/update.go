package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"
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
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("https://api.github.com/repos/" + repo + "/releases/latest")
	if err != nil {
		return
	}
	var rel struct {
		Tag string `json:"tag_name"`
	}
	err = json.NewDecoder(resp.Body).Decode(&rel)
	resp.Body.Close()
	if err != nil || slices.Compare(semver(rel.Tag), semver(version)) <= 0 {
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
	if err := download(exe, "https://github.com/"+repo+"/releases/download/"+rel.Tag+"/"+asset); err != nil {
		fmt.Fprintf(os.Stderr, "fwdhub: update failed: %v\n", err)
		return
	}
	fmt.Printf("Updated to %s. Start fwdhub again to use it.\n", rel.Tag)
	os.Exit(0)
}

func download(exe, url string) error {
	resp, err := (&http.Client{Timeout: 5 * time.Minute}).Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	return replaceExe(exe, resp.Body)
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
