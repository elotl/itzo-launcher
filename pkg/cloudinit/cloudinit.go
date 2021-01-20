package cloudinit

// This based on from elotl/cloud-init.

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/elotl/cloud-init/config"
	"github.com/elotl/cloud-init/config/validate"
	"github.com/elotl/cloud-init/datasource"
	"github.com/elotl/cloud-init/datasource/metadata/ec2"
	"github.com/elotl/cloud-init/datasource/metadata/gce"
	"github.com/elotl/cloud-init/datasource/waagent"
	"github.com/elotl/cloud-init/pkg"
	"k8s.io/klog"
)

const (
	datasourceInterval    = 100 * time.Millisecond
	datasourceMaxInterval = 1 * time.Second
	datasourceTimeout     = 5 * time.Minute
)

func WriteFiles(fileDir string, paths ...string) error {
	dss := getDatasources()
	if len(dss) == 0 {
		return fmt.Errorf("no datasources configured")
	}

	ds := selectDatasource(dss)
	if ds == nil {
		return fmt.Errorf("no datasources available in time")
	}

	klog.V(2).Infof("fetching user-data from datasource of type %q\n", ds.Type())
	userdataBytes, err := ds.FetchUserdata()
	if err != nil {
		return fmt.Errorf("fetching user-data from datasource: %v", err)
	}
	userdataBytes, err = decompressIfGzip(userdataBytes)
	if err != nil {
		return fmt.Errorf("decompressing user-data from datasource: %v. Continuing...\n", err)
	}

	if report, err := validate.Validate(userdataBytes); err == nil {
		for _, e := range report.Entries() {
			klog.Infof("validating userdata: %v", e)
		}
	} else {
		return fmt.Errorf("validating userdata: %v", err)
	}

	klog.V(2).Infof("fetching meta-data from datasource of type %q", ds.Type())

	cc, err := config.NewCloudConfig(string(userdataBytes))
	if err != nil {
		return err
	}
	if err := cc.Decode(); err != nil {
		return err
	}

	// ensure directory which holds files exists before attempting write
	err = os.MkdirAll(fileDir, os.ModeDir)
	if err != nil {
		return err
	}

	for _, wf := range cc.WriteFiles {
		for _, p := range paths {
			if wf.Path == p {
				permStr := wf.RawFilePermissions
				perm, err := strconv.ParseInt(permStr, 0, 32)
				if err != nil {
					klog.Warningf("parsing permission %s: %v", permStr, err)
					perm = 0644
				}
				err = ioutil.WriteFile(p, []byte(wf.Content), os.FileMode(perm))
				if err != nil {
					return err
				}
				klog.Infof("saved %s", p)
			}
		}
	}

	return nil
}

// getDatasources creates a slice of possible Datasources for cloudinit based
// on the different source command-line flags.
func getDatasources() []datasource.Datasource {
	dss := make([]datasource.Datasource, 0, 5)
	dss = append(dss, ec2.NewDatasource(ec2.DefaultAddress))
	dss = append(dss, gce.NewDatasource("http://metadata.google.internal"))
	dss = append(dss, waagent.NewDatasource("/var/lib/waagent"))
	return dss
}

// selectDatasource attempts to choose a valid Datasource to use based on its
// current availability. The first Datasource to report to be available is
// returned. Datasources will be retried if possible if they are not
// immediately available. If all Datasources are permanently unavailable or
// datasourceTimeout is reached before one becomes available, nil is returned.
func selectDatasource(sources []datasource.Datasource) datasource.Datasource {
	ds := make(chan datasource.Datasource)
	stop := make(chan struct{})
	var wg sync.WaitGroup

	for _, s := range sources {
		wg.Add(1)
		go func(s datasource.Datasource) {
			defer wg.Done()

			duration := datasourceInterval
			for {
				klog.V(2).Infof("checking availability of %q\n", s.Type())
				if s.IsAvailable() {
					ds <- s
					return
				} else if !s.AvailabilityChanges() {
					return
				}
				select {
				case <-stop:
					return
				case <-time.After(duration):
					duration = pkg.ExpBackoff(duration, datasourceMaxInterval)
				}
			}
		}(s)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	var s datasource.Datasource
	select {
	case s = <-ds:
	case <-done:
	case <-time.After(datasourceTimeout):
	}

	close(stop)
	return s
}

const gzipMagicBytes = "\x1f\x8b"

func decompressIfGzip(userdataBytes []byte) ([]byte, error) {
	if !bytes.HasPrefix(userdataBytes, []byte(gzipMagicBytes)) {
		return userdataBytes, nil
	}
	gzr, err := gzip.NewReader(bytes.NewReader(userdataBytes))
	if err != nil {
		return nil, err
	}
	defer gzr.Close()
	return ioutil.ReadAll(gzr)
}
