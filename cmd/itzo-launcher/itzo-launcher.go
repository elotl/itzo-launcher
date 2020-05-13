package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/elotl/itzo-launcher/pkg/cloudinit"
	"k8s.io/klog"
)

const (
	LogDir             = "/var/log/itzo"
	ItzoDir            = "/tmp/itzo"
	ItzoPath           = "/usr/local/bin/itzo"
	ItzoDefaultURL     = "https://itzo-kip-download.s3.amazonaws.com"
	ItzoDefaultVersion = "latest"
	ItzoURLFile        = ItzoDir + "/itzo_url"
	ItzoVersionFile    = ItzoDir + "/itzo_version"
)

func ProcessUserData() error {
	err := cloudinit.WriteFiles(ItzoURLFile, ItzoVersionFile)
	if err != nil {
		return err
	}
	klog.V(2).Infof("wrote itzo files from cloud-init")
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
	klog.Infof("%s saved to %s, %d bytes", itzoDownloadURL, ItzoPath, n)
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
	klog.Infof("running %v", cmd)
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("running %v: %v", cmd, err)
	}
	klog.Warningf("%v finished", cmd)
	return nil
}

func HandleSignal(sig chan os.Signal) {
	s := <-sig
	klog.Fatalf("caught signal %v, exiting", s)
}

func main() {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go HandleSignal(sig)

	err := os.MkdirAll(LogDir, 0755)
	if err != nil {
		klog.Fatalf("ensuring %s exists: %v", LogDir, err)
	}

	err = ProcessUserData()
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
