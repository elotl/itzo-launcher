package addons

import (
	"fmt"
	"io/ioutil"
	"os/exec"
	"strings"

	"k8s.io/klog"
)

const (
	AWSCWAgentConfig = "/opt/aws/amazon-cloudwatch-agent/etc/amazon-cloudwatch-agent.json"
)

// This add-on configures the AWS CW Agent.
type AWSCWAgentAddon struct {
}

func init() {
	Registry["aws-cw-agent"] = &AWSCWAgentAddon{}
}

func replaceVariables(vars map[string]string) error {
	buf, err := ioutil.ReadFile(AWSCWAgentConfig)
	if err != nil {
		return fmt.Errorf("reading %s: %v", AWSCWAgentConfig, err)
	}
	contents := string(buf)
	for k, v := range vars {
		contents = strings.ReplaceAll(contents, "{{"+k+"}}", v)
	}
	err = ioutil.WriteFile(AWSCWAgentConfig, []byte(contents), 0644)
	if err != nil {
		return fmt.Errorf("writing %s: %v", AWSCWAgentConfig, err)
	}
	return nil
}

func restartAWSCWAgent() error {
	cmd := exec.Command("systemctl", "restart", "amazon-cloudwatch-agent.service")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("restarting amazon-cloudwatch-agent: %v; output:\n%s", err, output)
	}
	return nil
}

func (a *AWSCWAgentAddon) Run(config map[string]string) error {
	vars := make(map[string]string)
	for k, v := range config {
		if strings.HasPrefix(k, "awsCWAgent") && len(k) > 10 {
			vars[k[10:]] = v
		}
	}
	if len(vars) == 0 {
		klog.V(2).Infof("no AWS CW agent configuration found")
		return nil
	}
	err := replaceVariables(vars)
	if err != nil {
		klog.Errorf("%v", err)
		return err
	}
	err = restartAWSCWAgent()
	if err != nil {
		klog.Errorf("%v", err)
		return err
	}
	return nil
}
