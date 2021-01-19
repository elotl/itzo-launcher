package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/elotl/itzo-launcher/pkg/addons"
	"github.com/elotl/itzo-launcher/pkg/cloudinit"
	"github.com/elotl/itzo-launcher/pkg/parameters/aws"
	"github.com/elotl/itzo-launcher/pkg/util"
	"github.com/go-yaml/yaml"
	"github.com/hashicorp/go-multierror"
	"k8s.io/klog"
)

const (
	ItzoDir                   = "/tmp/itzo"
	ItzoDefaultPath           = "/usr/local/bin/itzo"
	ItzoDefaultURL            = "https://itzo-kip-download.s3.amazonaws.com"
	ItzoDefaultVersion        = "latest"
	ItzoURLFile               = ItzoDir + "/itzo_url"
	ItzoVersionFile           = ItzoDir + "/itzo_version"
	CellConfigFile            = ItzoDir + "/cell_config.yaml"
	InstanceParameterBasePath = "/kip/cells"
)

var (
	BuildVersion = "N/A"
	BuildTime    = "N/A"
)

var (
	version    = flag.Bool("version", false, "print version and exit")
	itzoLogDir = flag.String("itzo-log-dir", "/var/log/itzo", "directory for itzo.log")
)

func ProcessInstanceParameters() error {
	klog.V(2).Infof("checking instance parameters")
	// For now, only AWS is supported. On other platforms we'll fall back to
	// cloud-init user data.
	params, err := aws.NewAWSParameters(InstanceParameterBasePath, nil)
	if err != nil {
		return err
	}
	allParameters, err := params.GetAllParameters()
	if err != nil {
		return fmt.Errorf("getting instance parameters: %v", err)
	}
	err = os.MkdirAll(ItzoDir, 0755)
	if err != nil {
		return fmt.Errorf("ensuring %s exists: %v", ItzoDir, err)
	}
	for name, value := range allParameters {
		ppath := filepath.Join(ItzoDir, name)
		err = ioutil.WriteFile(ppath, []byte(value), 0600)
		if err != nil {
			return fmt.Errorf("writing instance parameter %s to %s: %v", name, ppath, err)
		}
	}
	klog.V(2).Infof("retrieved %d instance parameters", len(allParameters))
	return nil
}

func ProcessUserData() error {
	klog.V(2).Infof("getting itzo files from cloud-init")
	err := os.MkdirAll(ItzoDir, os.ModeDir)
	if err != nil {
		return err
	}
	err = cloudinit.WriteFiles(ItzoURLFile, ItzoVersionFile, CellConfigFile)
	if err != nil {
		return err
	}
	klog.V(2).Infof("wrote itzo files from cloud-init")
	return nil
}

func EnsureItzo() (string, error) {
	klog.V(2).Infof("downloading itzo")
	itzoURL := ItzoDefaultURL
	contents, err := ioutil.ReadFile(ItzoURLFile)
	if err != nil && os.IsNotExist(err) {
		klog.Warningf("reading %s: %v; using defaults", ItzoURLFile, err)
	} else if err != nil {
		err = fmt.Errorf("reading %s: %v", ItzoURLFile, err)
		klog.Errorf("%v", err)
		return "", err
	} else {
		itzoURL = strings.TrimSpace(string(contents))
	}
	itzoVersion := ItzoDefaultVersion
	contents, err = ioutil.ReadFile(ItzoVersionFile)
	if err != nil && os.IsNotExist(err) {
		klog.Warningf("reading %s: %v; using defaults", ItzoVersionFile, err)
	} else if err != nil {
		err = fmt.Errorf("reading %s: %v", ItzoVersionFile, err)
		klog.Errorf("%v", err)
		return "", err
	} else {
		itzoVersion = strings.TrimSpace(string(contents))
	}
	if itzoVersion == "" {
		// Set it to 0.0.0, so if itzo is already installed, it will be used,
		// whatever version it is.
		itzoVersion = "0.0.0"
		klog.Warningf("empty itzo version, using %q", itzoVersion)
	}
	itzoDownloadURL := fmt.Sprintf("%s/itzo-%s", itzoURL, itzoVersion)
	itzoPath := ItzoDefaultPath
	if itzoVersion != ItzoDefaultVersion {
		itzoPath, err = util.EnsureProg(ItzoDefaultPath, itzoDownloadURL, itzoVersion, "--version")
		if err != nil {
			klog.Errorf("ensuring itzo version %q: %v", itzoVersion, err)
			return "", err
		}
	} else {
		err = util.InstallProg(itzoDownloadURL, ItzoDefaultPath)
		if err != nil {
			klog.Errorf("downloading itzo version %q: %v", itzoVersion, err)
			return "", err
		}
	}
	klog.V(2).Infof("itzo is installed at %s", itzoPath)
	return itzoPath, nil
}

func RunItzo(itzoPath, logDir string) error {
	klog.V(2).Infof("starting itzo")

	config, err := readCellConfig()
	if err != nil {
		klog.Warningf("cannot read cell config to get extra itzo flags: %v", err)
	}
	klog.V(5).Info(config)

	logfile, err := os.OpenFile(
		logDir+"/itzo.log", os.O_RDWR|os.O_APPEND|os.O_CREATE, 0600)
	if err != nil {
		return fmt.Errorf("opening itzo logfile: %v", err)
	}
	defer logfile.Close()

	// here we get itzo flags from cell_config.yaml
	cmdArgs := util.GetItzoFlags(config)
	cmd := exec.Command(
		itzoPath,
		cmdArgs...,
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

func readCellConfig() (map[string]string, error) {
	config := make(map[string]string)
	contents, err := ioutil.ReadFile(CellConfigFile)
	if err != nil {
		klog.Warningf("reading %s: %v", CellConfigFile, err)
		return nil, err
	} else {
		err = yaml.Unmarshal(contents, &config)
		if err != nil {
			klog.Warningf("unmarshaling config %s: %v", contents, err)
			return nil, err
		}
	}
	return config, nil
}

func RunAddons() error {
	config, err := readCellConfig()
	if err != nil {
		config = map[string]string{}
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

	if *version {
		fmt.Printf("%s version %s built on %s\n", filepath.Base(os.Args[0]), BuildVersion, BuildTime)
		os.Exit(0)
	}

	klog.Infof("starting up")

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go HandleSignal(sig)

	err := os.MkdirAll(*itzoLogDir, 0755)
	if err != nil {
		klog.Fatalf("ensuring %s exists: %v", *itzoLogDir, err)
	}

	err = ProcessInstanceParameters()
	if err != nil {
		klog.Warningf("failed to process instance parameters: %v, falling back to user-data", err)
		err = ProcessUserData()
		if err != nil {
			klog.Fatalf("processing cloud-init user data: %v", err)
		}
	}

	err = RunAddons()
	if err != nil {
		klog.Warningf("running addons: %v", err)
	}

	itzoPath, err := EnsureItzo()
	if err != nil {
		klog.Fatalf("downloading itzo: %v", err)
	}

	err = RunItzo(itzoPath, *itzoLogDir)
	if err != nil {
		klog.Fatalf("running %q: %v", itzoPath, err)
	}

	klog.Infof("exiting")
}
