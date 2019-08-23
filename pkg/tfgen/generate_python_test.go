// Copyright 2016-2018, Pulumi Corporation.
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

package tfgen

import (
	"testing"

	"github.com/stretchr/testify/assert"

	pycodegen "github.com/pulumi/pulumi/pkg/codegen/python"
)

// Tests that we convert from CamelCase resource names to pythonic
// snake_case correctly.
func TestPyName(t *testing.T) {
	tests := [][]string{
		{"IAMPolicy", "iam_policy"},
		{"SubscriptionIAMBinding", "subscription_iam_binding"},
		{"Route", "route"},
		{"InstanceGroup", "instance_group"},
		{"Ipv4Thingy", "ipv4_thingy"},
		{"Sha256Hash", "sha256_hash"},
		{"SHA256Hash", "sha256_hash"},
		{"LongACME7Name", "long_acme7_name"},
		{"foo4Bar", "foo4_bar"},
	}

	for _, testCase := range tests {
		t.Run(testCase[0], func(tt *testing.T) {
			out := pycodegen.PyName(testCase[0])
			assert.Equal(tt, testCase[1], out)
		})
	}
}

// Tests that we properly transform some Python reserved keywords.
func TestPyKeywords(t *testing.T) {
	assert.Equal(t, pycodegen.PyName("if"), "if_")
	assert.Equal(t, pycodegen.PyName("lambda"), "lambda_")
	assert.Equal(t, pyClassName("True"), "True_")
}

// Tests our PEP440 to Semver conversion.
func TestVersionConversion(t *testing.T) {
	ver, err := pep440VersionToSemver("1.0.0")
	assert.NoError(t, err)
	assert.Equal(t, ver.String(), "1.0.0")

	ver, err = pep440VersionToSemver("1.0.0a123")
	assert.NoError(t, err)
	assert.Equal(t, ver.String(), "1.0.0-alpha.123")

	ver, err = pep440VersionToSemver("1.0.0b123")
	assert.NoError(t, err)
	assert.Equal(t, ver.String(), "1.0.0-beta.123")

	ver, err = pep440VersionToSemver("1.0.0rc123")
	assert.NoError(t, err)
	assert.Equal(t, ver.String(), "1.0.0-rc.123")

	ver, err = pep440VersionToSemver("1.0.0.dev123")
	assert.NoError(t, err)
	assert.Equal(t, ver.String(), "1.0.0-dev.123")
}
