// Copyright 2016-2024, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tfbridge

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Test environment variable parsing
func TestGetInconsistencyConfig(t *testing.T) {
	t.Parallel()
	// Save original environment
	origEnv := os.Getenv(EnvDetectInconsistentApply)
	origDetail := os.Getenv(EnvDetectInconsistentApplyDetail)
	origResources := os.Getenv(EnvDetectInconsistentApplyResources)
	defer func() {
		os.Setenv(EnvDetectInconsistentApply, origEnv)
		os.Setenv(EnvDetectInconsistentApplyDetail, origDetail)
		os.Setenv(EnvDetectInconsistentApplyResources, origResources)
	}()

	tests := []struct {
		name          string
		envEnabled    string
		envDetail     string
		envResources  string
		wantEnabled   bool
		wantDetail    string
		wantResources map[string]bool
	}{
		{
			name:          "Default settings",
			envEnabled:    "",
			envDetail:     "",
			envResources:  "",
			wantEnabled:   false,
			wantDetail:    "normal",
			wantResources: map[string]bool{},
		},
		{
			name:          "Enabled with defaults",
			envEnabled:    "true",
			envDetail:     "",
			envResources:  "",
			wantEnabled:   true,
			wantDetail:    "normal",
			wantResources: map[string]bool{},
		},
		{
			name:          "Debug detail level",
			envEnabled:    "true",
			envDetail:     "debug",
			envResources:  "",
			wantEnabled:   true,
			wantDetail:    "debug",
			wantResources: map[string]bool{},
		},
		{
			name:          "Invalid detail level falls back to normal",
			envEnabled:    "true",
			envDetail:     "invalid-level",
			envResources:  "",
			wantEnabled:   true,
			wantDetail:    "normal",
			wantResources: map[string]bool{},
		},
		{
			name:         "Specific resources",
			envEnabled:   "true",
			envDetail:    "",
			envResources: "aws_s3_bucket,aws_lambda_function",
			wantEnabled:  true,
			wantDetail:   "normal",
			wantResources: map[string]bool{
				"aws_s3_bucket":       true,
				"aws_lambda_function": true,
			},
		},
		{
			name:         "Resources with whitespace",
			envEnabled:   "true",
			envDetail:    "",
			envResources: " aws_s3_bucket , aws_lambda_function ",
			wantEnabled:  true,
			wantDetail:   "normal",
			wantResources: map[string]bool{
				"aws_s3_bucket":       true,
				"aws_lambda_function": true,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Reset cache before each test to ensure clean state
			resetConfigCacheForTesting()

			// Set environment for this test
			os.Setenv(EnvDetectInconsistentApply, test.envEnabled)
			os.Setenv(EnvDetectInconsistentApplyDetail, test.envDetail)
			os.Setenv(EnvDetectInconsistentApplyResources, test.envResources)

			// Get config
			config := GetInconsistencyConfig()

			// Check results
			assert.Equal(t, test.wantEnabled, config.Enabled)
			assert.Equal(t, test.wantDetail, config.DetailLevel)
			assert.Equal(t, len(test.wantResources), len(config.ResourceTypes))

			for resource := range test.wantResources {
				assert.True(t, config.ResourceTypes[resource],
					"Expected resource %s not found in config", resource)
			}
		})
	}
}

func TestShouldDetectInconsistentApply(t *testing.T) {
	t.Parallel()
	// Save original environment and restore after test
	origEnv := os.Getenv(EnvDetectInconsistentApply)
	origResources := os.Getenv(EnvDetectInconsistentApplyResources)
	defer func() {
		os.Setenv(EnvDetectInconsistentApply, origEnv)
		os.Setenv(EnvDetectInconsistentApplyResources, origResources)
	}()

	tests := []struct {
		name         string
		envEnabled   string
		envResources string
		resourceType string
		want         bool
	}{
		{
			name:         "Detection disabled",
			envEnabled:   "",
			resourceType: "aws_s3_bucket",
			want:         false,
		},
		{
			name:         "Detection enabled, all resources",
			envEnabled:   "true",
			resourceType: "aws_s3_bucket",
			want:         true,
		},
		{
			name:         "Data source should be skipped",
			envEnabled:   "true",
			resourceType: "data.aws_s3_bucket",
			want:         false,
		},
		{
			name:         "Empty resource type should be skipped",
			envEnabled:   "true",
			resourceType: "",
			want:         false,
		},
		{
			name:         "Resource not in allowed list",
			envEnabled:   "true",
			envResources: "aws_lambda_function,aws_iam_role",
			resourceType: "aws_s3_bucket",
			want:         false,
		},
		{
			name:         "Resource in allowed list",
			envEnabled:   "true",
			envResources: "aws_lambda_function,aws_s3_bucket",
			resourceType: "aws_s3_bucket",
			want:         true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Reset cache before each test to ensure clean state
			resetConfigCacheForTesting()

			os.Setenv(EnvDetectInconsistentApply, test.envEnabled)
			os.Setenv(EnvDetectInconsistentApplyResources, test.envResources)

			result := ShouldDetectInconsistentApply(test.resourceType)
			assert.Equal(t, test.want, result)
		})
	}
}

func TestIsNilOrEmpty(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		value interface{}
		want  bool
	}{
		{
			name:  "nil value",
			value: nil,
			want:  true,
		},
		{
			name:  "empty string",
			value: "",
			want:  true,
		},
		{
			name:  "non-empty string",
			value: "hello",
			want:  false,
		},
		{
			name:  "empty slice",
			value: []interface{}{},
			want:  true,
		},
		{
			name:  "non-empty slice",
			value: []interface{}{1, 2, 3},
			want:  false,
		},
		{
			name:  "empty map",
			value: map[string]interface{}{},
			want:  true,
		},
		{
			name:  "non-empty map",
			value: map[string]interface{}{"key": "value"},
			want:  false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := isNilOrEmpty(test.value)
			assert.Equal(t, test.want, result)
		})
	}
}

func TestNormalizeID(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "Empty string",
			input: "",
			want:  "",
		},
		{
			name:  "Lowercase simple ID",
			input: "myresource",
			want:  "myresource",
		},
		{
			name:  "Mixed case ID",
			input: "MyResource",
			want:  "myresource",
		},
		{
			name:  "ID with hyphens",
			input: "my-resource-123",
			want:  "myresource123",
		},
		{
			name:  "ID with underscores",
			input: "my_resource_123",
			want:  "myresource123",
		},
		{
			name:  "ID with multiple separator types",
			input: "my-resource_123:456.789",
			want:  "myresource123456789",
		},
		{
			name:  "ID with whitespace",
			input: "my resource 123",
			want:  "myresource123",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := normalizeID(test.input)
			assert.Equal(t, test.want, result)
		})
	}
}
