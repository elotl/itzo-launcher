package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"k8s.io/klog"
)

//itzo_url_file="/tmp/itzo/itzo_url"
//itzo_url="http://itzo-download.s3.amazonaws.com"
//if [[ -f \$itzo_url_file ]]; then
//    itzo_url=\$(head -n 1 \$itzo_url_file)
//fi
//itzo_version_file="/tmp/itzo/itzo_version"
//itzo_version="latest"
//if [[ -f \$itzo_version_file ]]; then
//    itzo_version=\$(head -n 1 \$itzo_version_file)
//fi
//itzo_full_url="\${itzo_url}/itzo-\${itzo_version}"
//itzo_path="\${itzo_dir}/itzo"
//rm -f \$itzo_path
//while true; do
//    echo "\$(date) downloading itzo from \$itzo_full_url" >> /var/log/itzo/itzo_download.log 2>&1
//    wget --timeout=3 \$itzo_full_url -O \$itzo_path && break >> /var/log/itzo/itzo_download.log 2>&1
//    sleep 1
//done
//chmod 755 \$itzo_path
//\${itzo_dir}/itzo >> /var/log/itzo/itzo.log 2>&1

const (
	LogDir             = "/var/log/itzo"
	ItzoDir            = "/tmp/itzo"
	ItzoPath           = "/usr/local/bin/itzo"
	ItzoDefaultURL     = "https://itzo-kip-download.s3.amazonaws.com"
	ItzoDefaultVersion = "latest"
	ItzoURLFile        = ItzoDir + "/itzo_url"
	ItzoVersionFile    = ItzoDir + "/itzo_version"
)

func DownloadUserData() error {
	logfile, err := os.OpenFile(
		LogDir+"/itzo-cloud-init.log", os.O_RDWR|os.O_APPEND|os.O_CREATE, 0600)
	if err != nil {
		return fmt.Errorf("opening itzo-cloud-init logfile: %v", err)
	}
	defer logfile.Close()
	cmd := exec.Command(
		"itzo-cloud-init",
		"--from-metadata-service",
		"--from-waagent",
		"/var/lib/waagent",
		"--from-gce-metadata",
		"http://metadata.google.internal",
	)
	cmd.Stdout = logfile
	cmd.Stderr = logfile
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("starting %v: %v", cmd, err)
	}
	klog.V(2).Infof("%v finished", cmd)
	return nil
}

func DownloadItzo() error {
	itzoURL := ItzoDefaultURL
	contents, err := ioutil.ReadFile(ItzoURLFile)
	if err != nil && !os.IsNotExist(err) {
		klog.Warningf("reading %s: %v; using defaults", ItzoURLFile, err)
	} else {
		itzoURL = strings.TrimSpace(string(contents))
	}
	itzoVersion := ItzoDefaultVersion
	contents, err = ioutil.ReadFile(ItzoVersionFile)
	if err != nil && !os.IsNotExist(err) {
		klog.Warningf("reading %s: %v; using defaults", ItzoVersionFile, err)
	} else {
		itzoVersion = strings.TrimSpace(string(contents))
	}
	itzoDownloadURL := fmt.Sprintf("%s/itzo-%s", itzoURL, itzoVersion)
	binDir := filepath.Dir(ItzoPath)
	err = os.MkdirAll(binDir, 0755)
	if err != nil {
		return fmt.Errorf("ensuring %s exists: %v", binDir, err)
	}
	out, err := os.OpenFile(ItzoPath, os.O_WRONLY|os.O_CREATE, 0755)
	if err != nil {
		return fmt.Errorf("opening %s: %v", ItzoPath, err)
	}
	defer out.Close()
	resp, err := http.Get(itzoDownloadURL)
	if err != nil {
		return fmt.Errorf("downloading %s: %v", itzoDownloadURL, err)
	}
	defer resp.Body.Close()
	n, err := io.Copy(out, resp.Body)
	if err != nil {
		return fmt.Errorf("writing to %s: %v", ItzoPath, err)
	}
	klog.V(2).Infof("%s saved to %s, %d bytes", itzoDownloadURL, ItzoPath, n)
	return nil
}

func RunItzo() error {
	logfile, err := os.OpenFile(
		LogDir+"/itzo.log", os.O_RDWR|os.O_APPEND|os.O_CREATE, 0600)
	if err != nil {
		return fmt.Errorf("opening itzo logfile: %v", err)
	}
	defer logfile.Close()
	cmd := exec.Command(
		ItzoPath,
		"--v=5",
	)
	cmd.Stdout = logfile
	cmd.Stderr = logfile
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("running %v: %v", cmd, err)
	}
	klog.V(2).Infof("%v finished", cmd)
	return nil
}

func main() {
	err := os.MkdirAll(LogDir, 0755)
	if err != nil {
		klog.Fatalf("ensuring %s exists: %v", LogDir, err)
	}

	err = DownloadUserData()
	if err != nil {
		klog.Fatalf("downloading cloud-init user data: %v", err)
	}

	err = DownloadItzo()
	if err != nil {
		klog.Fatalf("downloading itzo: %v", err)
	}

	err = RunItzo()
	if err != nil {
		klog.Fatalf("running itzo: %v", err)
	}
}
