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

type testcase struct {
	Input    string
	Expected string
}

func TestURLRewrite(t *testing.T) {
	tests := []testcase{
		{
			Input:    "The DNS name for the given subnet/AZ per [documented convention](http://docs.aws.amazon.com/efs/latest/ug/mounting-fs-mount-cmd-dns-name.html).", // nolint: lll
			Expected: "The DNS name for the given subnet/AZ per [documented convention](http://docs.aws.amazon.com/efs/latest/ug/mounting-fs-mount-cmd-dns-name.html).", // nolint: lll
		},
		{
			Input:    "It's recommended to specify `create_before_destroy = true` in a [lifecycle][1] block to replace a certificate which is currently in use (eg, by [`aws_lb_listener`](lb_listener.html)).", // nolint: lll
			Expected: "It's recommended to specify `create_before_destroy = true` in a [lifecycle][1] block to replace a certificate which is currently in use (eg, by `aws_lb_listener`).",                     // nolint: lll
		},
		{
			Input:    "The execution ARN to be used in [`lambda_permission`](/docs/providers/aws/r/lambda_permission.html)'s `source_arn`",                         // nolint: lll
			Expected: "The execution ARN to be used in [`lambda_permission`](https://www.terraform.io/docs/providers/aws/r/lambda_permission.html)'s `source_arn`", // nolint: lll
		},
	}

	for _, test := range tests {
		assert.Equal(t, test.Expected, cleanupText(test.Input))
	}
}
