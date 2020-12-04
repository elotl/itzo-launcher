package aws

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

//func unmarshalParameters(params map[string]string) (map[string]string, error) {
func TestUnmarshalParameters(t *testing.T) {
	testCases := []struct {
		inputMap  map[string]string
		outputMap map[string]string
		failure   bool
	}{
		{
			// Basic case, one key that contains the whole map.
			inputMap: map[string]string{
				"config": "param1: value1\nparam2: value2\n",
			},
			outputMap: map[string]string{
				"param1": "value1",
				"param2": "value2",
			},
			failure: false,
		},
		{
			// Map split into two chunks.
			inputMap: map[string]string{
				"config-0": "param1: value1\nparam2: val",
				"config-1": "ue2\nparam3: value3\n",
			},
			outputMap: map[string]string{
				"param1": "value1",
				"param2": "value2",
				"param3": "value3",
			},
			failure: false,
		},
		{
			// Not a simple map.
			inputMap: map[string]string{
				"config": "list1:\n- elem1\n- elem2\nparam2: value2\n",
			},
			outputMap: nil,
			failure:   true,
		},
		{
			// Key without index.
			inputMap: map[string]string{
				"config":   "param1: value1\n",
				"config-0": "param1: value1\n",
				"config-1": "param2: value2\n",
			},
			outputMap: nil,
			failure:   true,
		},
		{
			// Key with invalid index.
			inputMap: map[string]string{
				"config-0": "param1: value1\n",
				"config-2": "param2: value2\n",
			},
			outputMap: nil,
			failure:   true,
		},
		{
			// Invalid input.
			inputMap: map[string]string{
				"config": "an escaped \\' single quote",
			},
			outputMap: nil,
			failure:   true,
		},
	}

	for _, tc := range testCases {
		output, err := unmarshalParameters(tc.inputMap)
		if tc.failure {
			assert.Error(t, err)
		} else {
			assert.NoError(t, err)
		}
		assert.Equal(t, tc.outputMap, output)
	}
}
