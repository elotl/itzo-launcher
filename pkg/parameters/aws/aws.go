package aws

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
	"gopkg.in/yaml.v2"
	"k8s.io/klog"
)

const (
	awsDetectionTimeout  = 1 * time.Second
	awsTimeout           = 10 * time.Second
	ssmMaxChunks         = 10
	ssmParameterBaseName = "config"
)

type AWSParameters struct {
	base string
	ssm  *ssm.SSM
}

func detectRegion() string {
	session, err := session.NewSession()
	if err != nil {
		klog.Warningf("creating session to autodetect AWS region: %v", err)
		return ""
	}
	client := ec2metadata.New(session)

	ctx, cancel := context.WithTimeout(aws.BackgroundContext(), awsDetectionTimeout)
	defer cancel()
	region, err := client.RegionWithContext(ctx)
	if err != nil {
		klog.Warningf("trying to autodetect AWS region: %v", err)
		return ""
	}

	klog.V(2).Infof("detected AWS region: %q", region)
	return region
}

func detectInstanceID() string {
	session, err := session.NewSession()
	if err != nil {
		klog.Warningf("creating session to autodetect AWS region: %v", err)
		return ""
	}
	client := ec2metadata.New(session)

	ctx, cancel := context.WithTimeout(aws.BackgroundContext(), awsDetectionTimeout)
	defer cancel()
	instanceID, err := client.GetMetadataWithContext(ctx, "instance-id")
	if err != nil {
		klog.Warningf("trying to autodetect AWS instance ID: %v", err)
		return ""
	}

	klog.V(2).Infof("detected AWS instance ID: %q", instanceID)
	return instanceID
}

func getAWSConfig() (*aws.Config, error) {
	region := os.Getenv("AWS_REGION")
	if region == "" {
		region = os.Getenv("AWS_DEFAULT_REGION")
	}
	if region == "" {
		region = detectRegion()
	}
	if region == "" {
		return nil, fmt.Errorf("failed to detect AWS region")
	}

	httpClient := &http.Client{
		Timeout: awsTimeout,
	}
	config := aws.NewConfig().WithHTTPClient(httpClient).WithRegion(region)
	return config, nil
}

func NewAWSParameters(base string, config *aws.Config) (*AWSParameters, error) {
	if config == nil {
		var err error
		config, err = getAWSConfig()
		if err != nil {
			return nil, fmt.Errorf("creating AWS config: %v", err)
		}
	}

	instanceID := detectInstanceID()
	if instanceID == "" {
		return nil, fmt.Errorf("failed to detect AWS instance ID, not running on AWS?")
	}

	sess, err := session.NewSession()
	if err != nil {
		return nil, err
	}
	ssmClient := ssm.New(sess, config)

	return &AWSParameters{
		base: filepath.Join(base, instanceID),
		ssm:  ssmClient,
	}, nil
}

func unmarshalParameters(params map[string]string) (map[string]string, error) {
	// This is either one single SSM parameter if it fit (<=4096 bytes), or
	// chunks of the serialized config e.g. "config-0", "config-1", etc. We
	// need to assemble the chunks in order, de-serialize and re-create the
	// original config map.
	if len(params) == 0 {
		return nil, fmt.Errorf("got no SSM parameters")
	}

	// The whole map in one parameter.
	if len(params) == 1 {
		for k, v := range params {
			configMap := make(map[string]string)
			klog.V(2).Infof("unmarshaling SSM parameter %q", k)
			err := yaml.Unmarshal([]byte(v), &configMap)
			if err != nil {
				return nil, err
			}
			return configMap, nil
		}
	}

	// Multiple chunks.
	paramList := make([]string, len(params))
	for k, v := range params {
		parts := strings.SplitN(k, "-", 2)
		if len(parts) < 2 {
			return nil, fmt.Errorf("invalid SSM parameter chunk key: %s", k)
		}
		n, err := strconv.ParseUint(parts[1], 10, 32)
		if err != nil {
			return nil, err
		}
		if int(n) >= len(paramList) {
			return nil, fmt.Errorf("invalid SSM parameter chunk key: %s", k)
		}
		paramList[n] = v
	}
	yml := ""
	for i := range paramList {
		yml = yml + paramList[i]
	}
	configMap := make(map[string]string)
	klog.V(2).Infof("unmarshaling %d SSM parameter chunks, %d bytes", len(paramList), len(yml))
	err := yaml.Unmarshal([]byte(yml), &configMap)
	if err != nil {
		return nil, err
	}
	return configMap, nil
}

func (a *AWSParameters) getParameter(name string) string {
	path := filepath.Join(a.base, name)
	in := &ssm.GetParameterInput{
		Name:           aws.String(path),
		WithDecryption: aws.Bool(true),
	}
	out, err := a.ssm.GetParameter(in)
	if err != nil {
		klog.V(2).Infof("getting SSM parameter %s: %v", name, err)
		return ""
	}
	if out.Parameter == nil {
		klog.V(2).Infof("got nil SSM parameter value for %s", name)
		return ""
	}
	return aws.StringValue(out.Parameter.Value)
}

func (a *AWSParameters) GetAllParameters() (map[string]string, error) {
	params := make(map[string]string)

	name := ssmParameterBaseName
	value := a.getParameter(name)
	if value != "" {
		params[name] = value
	}

	for i := 0; i < ssmMaxChunks; i++ {
		name = fmt.Sprintf("%s-%d", ssmParameterBaseName, i)
		value = a.getParameter(name)
		if value == "" {
			break
		}
		params[name] = value
	}

	configMap, err := unmarshalParameters(params)
	if err != nil {
		return nil, err
	}

	return configMap, nil
}
