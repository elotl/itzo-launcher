package util

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestGetItzoFlags(t *testing.T) {
	testCases := []struct {
		name          string
		config        map[string]string
		expectedFlags []string
	}{
		{
			name: "no itzo flags",
			config: map[string]string{
				"dummy": "dummy",
			},
			expectedFlags: DefaultItzoFlags,
		},
		{
			name: "itzo flags passed",
			config: map[string]string{
				"dummy":               "dummy",
				"itzoFlag-use-podman": "true",
			},
			expectedFlags: []string{"-use-podman", "true", "--v", "5"}, // --v 5 are default Flags
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			flags := GetItzoFlags(testCase.config)
			assert.Equal(t, testCase.expectedFlags, flags)
		})
	}
}
