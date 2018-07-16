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
			out := pyName(testCase[0])
			assert.Equal(tt, testCase[1], out)
		})
	}
}

// Tests that we properly transform some Python reserved keywords.
func TestPyKeywords(t *testing.T) {
	assert.Equal(t, pyName("if"), "if_")
	assert.Equal(t, pyName("lambda"), "lambda_")
	assert.Equal(t, pyClassName("True"), "True_")
}
