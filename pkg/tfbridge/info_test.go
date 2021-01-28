package tfbridge

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestGetModuleMajorVersion(t *testing.T) {
	type testcase struct {
		version              string
		expectedMajorVersion string
	}

	tests := []testcase{
		{
			"v0.0.100",
			"",
		},
		{
			"v1.0.0-alpha.sdqq.dirty",
			"",
		},
		{
			"v2.0.0",
			"v2",
		},
		{
			"v3.147.2-alpha.sdqq.dirty",
			"v3",
		},
	}

	for _, test := range tests {
		majorVersion := GetModuleMajorVersion(test.version)
		assert.Equal(t, test.expectedMajorVersion, majorVersion)
	}
}
