/*
Copyright 2020 Elotl Inc

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package util

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"golang.org/x/mod/semver"
	"k8s.io/klog"
)

const (
	semverRegexFmt string = `v?([0-9]+)(\.[0-9]+)(\.[0-9]+)?` +
		`(-([0-9A-Za-z\-]+(\.[0-9A-Za-z\-]+)*))*` +
		`(\+([0-9A-Za-z\-]+(\.[0-9A-Za-z\-]+)*))?`
)

var (
	semverRegex = regexp.MustCompile("^" + semverRegexFmt + "$")
	DialTimeout = time.Duration(5 * time.Second)
)

func versionMatch(exe, minVersion, versionArg string) bool {
	cmd := exec.Command(exe, versionArg)
	output, err := cmd.CombinedOutput()
	if err != nil {
		klog.V(2).Infof("%q error getting version: %v", exe, err)
		return false
	}
	version := ""
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		for _, word := range strings.Fields(line) {
			word = strings.TrimRight(word, ",;.")
			if semverRegex.Match([]byte(word)) {
				version = word
				klog.V(2).Infof("%q found version %q (minimum requested: %q)",
					exe, version, minVersion)
				return semver.Compare(version, minVersion) >= 0
			}
		}
	}
	klog.V(2).Infof("%q not found version in output %q", exe, output)
	return false
}

func ensurePath(localPath string) {
	envPath := os.Getenv("PATH")
	for _, p := range strings.Split(envPath, ":") {
		if p == localPath {
			return
		}
	}
	os.Setenv("PATH", envPath+":"+localPath)
}

func EnsureProg(prog, downloadURL, minVersion, versionArg string) (string, error) {
	progBase := filepath.Base(prog)
	progDir := filepath.Dir(prog)
	if progDir == "" || progDir == "." {
		return "", fmt.Errorf("%q: need full path", prog)
	}
	ensurePath(progDir)
	exe, err := exec.LookPath(progBase)
	if err == nil {
		found := versionMatch(exe, minVersion, versionArg)
		klog.V(5).Infof("looking for %s %s: found %v", exe, minVersion, found)
		if found {
			return exe, nil
		}
	}
	exe = filepath.Join(progDir, progBase)
	err = InstallProg(downloadURL, exe)
	if err != nil {
		return "", err
	}
	return exe, nil
}

func InstallProg(url, path string) error {
	client := http.Client{
		Transport: &http.Transport{
			Dial: func(network, addr string) (net.Conn, error) {
				return net.DialTimeout(network, addr, DialTimeout)
			},
		},
	}
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("creating get request for %s: %+v", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("downloading %s: got status code %d",
			url, resp.StatusCode)
	}
	_ = os.MkdirAll(filepath.Dir(path), 0755)
	tmpPath := path + ".part"
	f, err := os.OpenFile(tmpPath, os.O_WRONLY|os.O_CREATE, 0755)
	if err != nil {
		return fmt.Errorf("opening %s for writing: %+v", path, err)
	}
	defer f.Close()
	_, err = io.Copy(f, resp.Body)
	if err != nil {
		return fmt.Errorf("writing %s: %+v", path, err)
	}
	err = os.Rename(tmpPath, path)
	if err != nil {
		return fmt.Errorf("renaming %s to %s: %+v", tmpPath, path, err)
	}
	return nil
}
