package nfs

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/elotl/itzo-launcher/pkg/addons"
)

const (
	ImageDir     = "/tmp/tosi"
	ImageSubDirs = [4]string{"configs", "manifests", "layers", "overlays"}
)

type NFSAddon struct {
	endpoint string
}

func init() {
	addons.Registry["nfs"] = &NFSAddon{}
}

func (n *NFSAddon) createLinks() error {
	for _, subdir := range ImageSubDirs {
		dest := filepath.Join(ImageDir, subdir)
		err := os.MkdirAll(dest, 0755)
		if err != nil {
			return nil
		}
		src := filepath.Join(n.endpoint, subdir)
		err = os.MkdirAll(src, 0755)
		if err != nil {
			return nil
		}
		fis, err := ioutil.ReadDir(src)
		if err != nil {
			return nil
		}
		for _, fi := range fis {
			name := fi.Name()
			if len(name) > 0 && name[0] == "." {
				continue
			}
			oldName := filepath.Join(src, name)
			newName := filepath.Join(dst, name)
			err = os.Symlink(oldName, newName)
			if err != nil {
				return err
			}
		}
	}
}

func (n *NFSAddon) Run(config map[string]string) (string, error) {
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
		return "", nil
	}
	n.endpoint = endpoint
	opts := strings.Fields(mountOpts)
	args := []string{
		"-t",
		"nfs",
	}
	args = append(args, opts...)
	args = append(args, mountDir)
	cmd := exec.Command(args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return output, err
	}
	err = n.createLinks()
	if err != nil {
		return "", fmt.Errorf("creating links: %v", err)
	}
	return output, nil
}
