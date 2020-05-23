package main

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/avast/retry-go"
	"github.com/elotl/itzo-launcher/pkg/addons"
	"github.com/elotl/itzo-launcher/pkg/cloudinit"
	"github.com/go-yaml/yaml"
	"github.com/hashicorp/go-multierror"
	"k8s.io/klog"
)

const (
	LogDir              = "/var/log/itzo"
	ItzoDir             = "/tmp/itzo"
	ItzoPath            = "/usr/local/bin/itzo"
	ItzoDefaultURL      = "https://itzo-kip-download.s3.amazonaws.com"
	ItzoDefaultVersion  = "latest"
	ItzoURLFile         = ItzoDir + "/itzo_url"
	ItzoVersionFile     = ItzoDir + "/itzo_version"
	ItzoDownloadTimeout = time.Duration(2 * time.Second)
	CellConfigFile      = ItzoDir + "/cell_config.yaml"
)

func ProcessUserData() error {
	klog.V(2).Infof("getting itzo files from cloud-init")
	err := cloudinit.WriteFiles(ItzoURLFile, ItzoVersionFile, CellConfigFile)
	if err != nil {
		return err
	}
	klog.V(2).Infof("wrote itzo files from cloud-init")
	return nil
}

func downloadItzo(url string, timeout time.Duration) (*http.Response, error) {
	transport := http.Transport{
		Dial: func(network, addr string) (net.Conn, error) {
			return net.DialTimeout(network, addr, timeout)
		},
	}
	client := http.Client{
		Transport: &transport,
	}
	return client.Get(url)
}

func DownloadItzo() error {
	klog.V(2).Infof("downloading itzo")
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
	var resp *http.Response
	err = retry.Do(
		func() error {
			resp, err = downloadItzo(itzoDownloadURL, ItzoDownloadTimeout)
			return err
		},
		retry.Attempts(10),
		retry.Delay(1*time.Second),
		retry.MaxJitter(1*time.Second),
	)
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
	klog.V(2).Infof("starting itzo")
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
	klog.Warningf("%v exited", cmd)
	return nil
}

func RunAddons() error {
	config := make(map[string]string)
	contents, err := ioutil.ReadFile(CellConfigFile)
	if err != nil {
		klog.Warningf("reading %s: %v", CellConfigFile, err)
	} else {
		err = yaml.Unmarshal(contents, &config)
		if err != nil {
			klog.Warningf("unmarshaling config %s: %v", contents, err)
		}
	}
	var errs error
	klog.Infof("found %d addon(s)", len(addons.Registry))
	for name, addon := range addons.Registry {
		klog.Infof("running addon %s", name)
		err := addon.Run(config)
		if err != nil {
			errs = multierror.Append(errs, err)
			klog.Errorf("running %s: %v", name, err)
		} else {
			klog.V(2).Infof("running %s: success", name)
		}
	}
	return errs
}

func HandleSignal(sig chan os.Signal) {
	s := <-sig
	klog.Fatalf("caught signal %v, exiting", s)
}

func main() {
	klog.InitFlags(nil)
	flag.Parse()

	klog.Infof("starting up")

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

	err = RunAddons()
	if err != nil {
		klog.Fatalf("running addons: %v", err)
	}

	err = DownloadItzo()
	if err != nil {
		klog.Fatalf("downloading itzo: %v", err)
	}

	err = RunItzo()
	if err != nil {
		klog.Fatalf("running itzo: %v", err)
	}

	klog.Infof("exiting")
}
