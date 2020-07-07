package addons

import (
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"k8s.io/klog"
)

const (
	FluentdVariablesFile = "/etc/default/td-agent"
)

// This add-on configures fluentd on AWS to send logs to CloudWatch.
type FluentdAWSAddon struct {
}

func init() {
	Registry["fluentd-aws"] = &FluentdAWSAddon{}
}

func autoDetectRegion() string {
	session, err := session.NewSession()
	if err != nil {
		klog.Warningf("creating session to autodetect AWS region: %v", err)
		return ""
	}
	client := ec2metadata.New(session)
	region, err := client.Region()
	if err != nil {
		klog.Warningf("trying to autodetect AWS region: %v", err)
		return ""
	}
	klog.V(2).Infof("detected AWS region: %q", region)
	return region
}

func configureVariables(clusterName, region string) {
	f, err := os.Create(FluentdVariablesFile)
	if err != nil {
		klog.Warningf("opening %s: %v", FluentdVariablesFile, err)
		return
	}
	defer f.Close()
	_, err = f.WriteString(fmt.Sprintf("CLUSTER_NAME=%s\n", clusterName))
	if err != nil {
		klog.Warningf("writing to %s: %v", FluentdVariablesFile, err)
		return
	}
	_, err = f.WriteString(fmt.Sprintf("REGION=%s\n", region))
	if err != nil {
		klog.Warningf("writing to %s: %v", FluentdVariablesFile, err)
		return
	}
}

func (f *FluentdAWSAddon) Run(config map[string]string) error {
	clusterName := ""
	region := ""
	for k, v := range config {
		if k == "fluentdAWSClusterName" {
			clusterName = v
		} else if k == "fluentdAWSRegion" {
			region = v
		}
	}
	if region == "" {
		region = autoDetectRegion()
	}
	if clusterName == "" || region == "" {
		return nil
	}
	configureVariables(clusterName, region)
	return nil
}
