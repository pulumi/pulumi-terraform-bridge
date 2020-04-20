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
	"fmt"
	"testing"

	"github.com/pulumi/pulumi-terraform-bridge/v2/pkg/tfbridge"
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
			Expected: "It's recommended to specify `createBeforeDestroy = true` in a [lifecycle][1] block to replace a certificate which is currently in use (eg, by `awsLbListener`).",                         // nolint: lll
		},
		{
			Input:    "The execution ARN to be used in [`lambda_permission`](/docs/providers/aws/r/lambda_permission.html)'s `source_arn`",                       // nolint: lll
			Expected: "The execution ARN to be used in [`lambdaPermission`](https://www.terraform.io/docs/providers/aws/r/lambda_permission.html)'s `sourceArn`", // nolint: lll
		},
		{
			Input:    "See google_container_node_pool for schema.",
			Expected: "See google.container.NodePool for schema.",
		},
	}

	g, err := newGenerator("google", "0.1.2", "nodejs", tfbridge.ProviderInfo{
		Name: "google",
		Resources: map[string]*tfbridge.ResourceInfo{
			"google_container_node_pool": {Tok: "google:container/nodePool:NodePool"},
		},
	}, "", "")
	assert.NoError(t, err)

	for _, test := range tests {
		text, _ := cleanupText(g, nil, test.Input)
		assert.Equal(t, test.Expected, text)
	}
}

func TestArgumentRegex(t *testing.T) {
	tests := []struct {
		input    []string
		expected map[string]*argument
	}{
		{
			input: []string{
				"* `iam_instance_profile` - (Optional) The IAM Instance Profile to",
				"launch the instance with. Specified as the name of the Instance Profile. Ensure your credentials have the correct permission to assign the instance profile according to the [EC2 documentation](http://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles_use_switch-role-ec2.html#roles-usingrole-ec2instance-permissions), notably `iam:PassRole`.",
				"* `ipv6_address_count`- (Optional) A number of IPv6 addresses to associate with the primary network interface. Amazon EC2 chooses the IPv6 addresses from the range of your subnet.",
				"* `ipv6_addresses` - (Optional) Specify one or more IPv6 addresses from the range of the subnet to associate with the primary network interface",
				"* `tags` - (Optional) A mapping of tags to assign to the resource.",
			},
			expected: map[string]*argument{
				"iam_instance_profile": &argument{
					description: "The IAM Instance Profile to" + "\n" +
						"launch the instance with. Specified as the name of the Instance Profile. Ensure your credentials have the correct permission to assign the instance profile according to the [EC2 documentation](http://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles_use_switch-role-ec2.html#roles-usingrole-ec2instance-permissions), notably `iam:PassRole`.",
				},
				"ipv6_address_count": &argument{
					description: "A number of IPv6 addresses to associate with the primary network interface. Amazon EC2 chooses the IPv6 addresses from the range of your subnet.",
				},
				"ipv6_addresses": &argument{
					description: "Specify one or more IPv6 addresses from the range of the subnet to associate with the primary network interface",
				},
				"tags": &argument{
					description: "A mapping of tags to assign to the resource.",
				},
			},
		},
		{
			input: []string{
				"* `jwt_configuration` - (Optional) The configuration of a JWT authorizer. Required for the `JWT` authorizer type.",
				"Supported only for HTTP APIs.",
				"",
				"The `jwt_configuration` object supports the following:",
				"",
				"* `audience` - (Optional) A list of the intended recipients of the JWT. A valid JWT must provide an aud that matches at least one entry in this list.",
				"* `issuer` - (Optional) The base domain of the identity provider that issues JSON Web Tokens, such as the `endpoint` attribute of the [`aws_cognito_user_pool`](/docs/providers/aws/r/cognito_user_pool.html) resource.",
			},
			expected: map[string]*argument{
				"jwt_configuration": &argument{
					description: "The configuration of a JWT authorizer. Required for the `JWT` authorizer type." + "\n" +
						"Supported only for HTTP APIs.",
					arguments: map[string]string{
						"audience": "A list of the intended recipients of the JWT. A valid JWT must provide an aud that matches at least one entry in this list.",
						"issuer":   "The base domain of the identity provider that issues JSON Web Tokens, such as the `endpoint` attribute of the [`aws_cognito_user_pool`](/docs/providers/aws/r/cognito_user_pool.html) resource.",
					},
				},
				"audience": &argument{
					description: "A list of the intended recipients of the JWT. A valid JWT must provide an aud that matches at least one entry in this list.",
					isNested:    true,
				},
				"issuer": &argument{
					description: "The base domain of the identity provider that issues JSON Web Tokens, such as the `endpoint` attribute of the [`aws_cognito_user_pool`](/docs/providers/aws/r/cognito_user_pool.html) resource.",
					isNested:    true,
				},
			},
		},
		{
			input: []string{
				"* `website` - (Optional) A website object (documented below).",
				"~> **NOTE:** You cannot use `acceleration_status` in `cn-north-1` or `us-gov-west-1`",
				"",
				"The `website` object supports the following:",
				"",
				"* `index_document` - (Required, unless using `redirect_all_requests_to`) Amazon S3 returns this index document when requests are made to the root domain or any of the subfolders.",
				"* `routing_rules` - (Optional) A json array containing [routing rules](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-s3-websiteconfiguration-routingrules.html)",
				"describing redirect behavior and when redirects are applied.",
			},
			expected: map[string]*argument{
				"website": &argument{
					description: "A website object (documented below)." + "\n" +
						"~> **NOTE:** You cannot use `acceleration_status` in `cn-north-1` or `us-gov-west-1`",
					arguments: map[string]string{
						"index_document": "Amazon S3 returns this index document when requests are made to the root domain or any of the subfolders.",
						"routing_rules": "A json array containing [routing rules](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-s3-websiteconfiguration-routingrules.html)" + "\n" +
							"describing redirect behavior and when redirects are applied.",
					},
				},
				"index_document": &argument{
					description: "Amazon S3 returns this index document when requests are made to the root domain or any of the subfolders.",
					isNested:    true,
				},
				"routing_rules": &argument{
					description: "A json array containing [routing rules](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-s3-websiteconfiguration-routingrules.html)" + "\n" +
						"describing redirect behavior and when redirects are applied.",
					isNested: true,
				},
			},
		},
		{
			input: []string{
				"* `action` - (Optional) The action that CloudFront or AWS WAF takes when a web request matches the conditions in the rule. Not used if `type` is `GROUP`.",
				"  * `type` - (Required) valid values are: `BLOCK`, `ALLOW`, or `COUNT`",
				"* `override_action` - (Optional) Override the action that a group requests CloudFront or AWS WAF takes when a web request matches the conditions in the rule. Only used if `type` is `GROUP`.",
				"  * `type` - (Required) valid values are: `BLOCK`, `ALLOW`, or `COUNT`",
			},
			// Note: This is the existing behavior and is indeed a bug. The type field should be nested within action and override_action.
			expected: map[string]*argument{
				"action": &argument{
					description: "The action that CloudFront or AWS WAF takes when a web request matches the conditions in the rule. Not used if `type` is `GROUP`.",
				},
				"override_action": &argument{
					description: "Override the action that a group requests CloudFront or AWS WAF takes when a web request matches the conditions in the rule. Only used if `type` is `GROUP`.",
				},
				"type": &argument{
					description: "valid values are: `BLOCK`, `ALLOW`, or `COUNT`",
				},
			},
		},
		{
			input: []string{
				"* `priority` - (Optional) The priority associated with the rule.",
				"",
				"* `priority` is optional (with a default value of `0`) but must be unique between multiple rules",
			},
			expected: map[string]*argument{
				"priority": &argument{
					description: "is optional (with a default value of `0`) but must be unique between multiple rules",
				},
			},
		},
		{
			input: []string{
				"* `allowed_audiences` (Optional) Allowed audience values to consider when validating JWTs issued by Azure Active Directory.",
				"* `retention_policy` - (Required) A `retention_policy` block as documented below.",
				"",
				"---",
				"* `retention_policy` supports the following:",
			},
			expected: map[string]*argument{
				"retention_policy": &argument{
					description: "A `retention_policy` block as documented below.",
				},
				"allowed_audiences": &argument{
					description: "Allowed audience values to consider when validating JWTs issued by Azure Active Directory.",
				},
			},
		},
	}

	for _, tt := range tests {
		ret := parsedDoc{
			Arguments: make(map[string]*argument),
		}
		processArgumentReferenceSection(tt.input, &ret)

		assert.Len(t, ret.Arguments, len(tt.expected))
		for k, v := range tt.expected {
			actualArg := ret.Arguments[k]
			assert.NotNil(t, actualArg, fmt.Sprintf("%s should not be nil", k))
			assert.Equal(t, v.description, actualArg.description)
			assert.Equal(t, v.isNested, actualArg.isNested)
			assert.Equal(t, v.arguments, actualArg.arguments)
		}
	}
}
