package addons

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
			if len(name) > 0 && name[0] == '.' {
				continue
			}
			oldName := filepath.Join(src, name)
			newName := filepath.Join(dest, name)
			err = os.Symlink(oldName, newName)
			if err != nil {
				return err
			}
		}
	}
	return nil
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
	cmd := exec.Command("mount", args...)
	buf, err := cmd.CombinedOutput()
	output := string(buf)
	if err != nil {
		return output, err
	}
	err = n.createLinks()
	if err != nil {
		return "", fmt.Errorf("creating links: %v", err)
	}
	return output, nil
}
