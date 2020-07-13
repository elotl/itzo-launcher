package addons

import (
	"fmt"
	"os"
	"os/exec"
	"time"

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
	sess, err := session.NewSession()
	if err != nil {
		klog.Warningf("creating session to autodetect AWS region: %v", err)
		return ""
	}
	client := ec2metadata.New(sess)
	region, err := client.Region()
	if err != nil {
		klog.Warningf("trying to autodetect AWS region: %v", err)
		return ""
	}
	klog.V(2).Infof("detected AWS region: %q", region)
	return region
}

func configureVariables(clusterName, region string) error {
	f, err := os.Create(FluentdVariablesFile)
	if err != nil {
		return fmt.Errorf("opening %s: %v", FluentdVariablesFile, err)
	}
	defer f.Close()
	_, err = f.WriteString(fmt.Sprintf("CLUSTER_NAME=%s\n", clusterName))
	if err != nil {
		return fmt.Errorf("writing to %s: %v", FluentdVariablesFile, err)
	}
	_, err = f.WriteString(fmt.Sprintf("REGION=%s\n", region))
	if err != nil {
		return fmt.Errorf("writing to %s: %v", FluentdVariablesFile, err)
	}
	return nil
}

func restartUnit() error {
	cmd := exec.Command("systemctl", "restart", "td-agent")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("restarting fluentd: %v; output:\n%s", err, output)
	}
	return nil
}

func waitForIAMRole() {
	for {
		time.Sleep(3)
		klog.V(2).Infof("checking if IAM role for fluentd is now available")
		sess, err := session.NewSession()
		if err != nil {
			klog.Warningf("creating AWS session for checking credentials: %v", err)
			continue
		}
		client := ec2metadata.New(sess)
		content, err := client.GetMetadata("iam/security-credentials")
		if err != nil || len(content) < 1 {
			continue
		}
		klog.V(2).Infof("found IAM role for fluentd %q", content)
		restartUnit()
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
	err := configureVariables(clusterName, region)
	if err != nil {
		klog.Errorf("%v", err)
		return err
	}
	err = restartUnit()
	if err != nil {
		klog.Errorf("%v", err)
		return err
	}
	// The IAM role for fluentd only gets attached after pod dispatch, but the
	// AWS library the cloudwatch plugin uses only checks the role at startup.
	// To ensure credentials are configured for the plugin, we'll need to
	// restart fluentd after the role has been attached to the instance.
	go waitForIAMRole()
	return nil
}
