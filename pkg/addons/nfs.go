package addons

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"k8s.io/klog"
)

var (
	ImageDir     = "/tmp/tosi"
	ImageSubDirs = [4]string{"configs", "manifests", "layers", "overlays"}
)

type NFSAddon struct {
	endpoint string
}

func init() {
	Registry["nfs"] = &NFSAddon{}
}

func (n *NFSAddon) createLinks(mountDir string) error {
	klog.V(5).Infof("mount dir %s, %d subdirs", mountDir, len(ImageSubDirs))
	for _, subdir := range ImageSubDirs {
		klog.V(5).Infof("checking subdir %s", subdir)
		dest := filepath.Join(ImageDir, subdir)
		err := os.MkdirAll(dest, 0755)
		if err != nil {
			return nil
		}
		src := filepath.Join(mountDir, subdir)
		err = os.MkdirAll(src, 0755)
		if err != nil {
			return nil
		}
		klog.V(5).Infof("subdir %s -> %s", src, dest)
		fis, err := ioutil.ReadDir(src)
		if err != nil {
			return nil
		}
		for _, fi := range fis {
			name := fi.Name()
			klog.V(5).Infof("found %s in %s", name, subdir)
			if len(name) > 0 && name[0] == '.' {
				continue
			}
			oldName := filepath.Join(src, name)
			newName := filepath.Join(dest, name)
			klog.V(5).Infof("linking %s -> %s", oldName, newName)
			err = os.Symlink(oldName, newName)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (n *NFSAddon) Run(config map[string]string) error {
	endpoint := ""
	mountDir := "/nfs"
	mountOpts := "-o ro"
	for k, v := range config {
		if k == "imageCacheEndpoint" {
			endpoint = v
		} else if k == "imageCacheMountDir" {
			mountDir = v
		} else if k == "imageCacheMountOpts" {
			mountOpts = v
		}
	}
	if endpoint == "" {
		return nil
	}
	n.endpoint = endpoint
	err := os.MkdirAll(mountDir, 0755)
	if err != nil {
		return nil
	}
	opts := strings.Fields(mountOpts)
	args := []string{
		"-t",
		"nfs",
		endpoint,
	}
	args = append(args, opts...)
	args = append(args, mountDir)
	cmd := exec.Command("mount", args...)
	buf, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf(
			"mounting %s: %v, output:\n%s", endpoint, err, string(buf))
	}
	err = n.createLinks(mountDir)
	if err != nil {
		return fmt.Errorf("creating links: %v", err)
	}
	return nil
}
