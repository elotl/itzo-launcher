package mount

import (
	"fmt"
	"io/ioutil"
	"os/exec"
	"strings"
)

type Mount struct {
	Device  string
	Path    string
	FSType  string
	Options string
}

type Mounter interface {
	Mount(device, path, fstype, options string) error
	Umount(deviceOrPath, options string) error
	Mounts() ([]Mount, error)
}

type OSMounter struct {
}

func NewOSMounter() Mounter {
	return &OSMounter{}
}

func (m *OSMounter) Mount(device, path, fstype, options string) error {
	opts := strings.Fields(options)
	args := []string{
		"-t",
		fstype,
		device,
	}
	args = append(args, opts...)
	args = append(args, path)
	cmd := exec.Command("mount", args...)
	buf, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf(
			"mounting %s: %v, output:\n%s", device, err, string(buf))
	}
	return nil
}

func (m *OSMounter) Umount(deviceOrPath string, options string) error {
	opts := strings.Fields(options)
	args := []string{}
	args = append(args, opts...)
	args = append(args, deviceOrPath)
	cmd := exec.Command("umount", args...)
	buf, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf(
			"unmounting %s: %v, output:\n%s", deviceOrPath, err, string(buf))
	}
	return nil
}

func (m *OSMounter) Mounts() ([]Mount, error) {
	contents, err := ioutil.ReadFile("/proc/self/mounts")
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(contents), "\n")
	mounts := make([]Mount, 0, len(lines))
	for _, line := range lines {
		parts := strings.Fields(line)
		if len(parts) < 4 {
			continue
		}
		mounts = append(mounts, Mount{
			Device:  parts[0],
			Path:    parts[1],
			FSType:  parts[2],
			Options: parts[3],
		})
	}
	return mounts, nil
}
