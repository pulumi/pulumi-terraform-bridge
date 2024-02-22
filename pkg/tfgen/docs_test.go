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

//nolint:lll
package tfgen

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"text/template"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfgen/internal/testprovider"
)

var (
	accept = cmdutil.IsTruthy(os.Getenv("PULUMI_ACCEPT"))
)

type testcase struct {
	Input    string
	Expected string
}

func TestReformatText(t *testing.T) {
	tests := []testcase{
		{
			Input:    "The DNS name for the given subnet/AZ per [documented convention](http://docs.aws.amazon.com/efs/latest/ug/mounting-fs-mount-cmd-dns-name.html).", //nolint:lll
			Expected: "The DNS name for the given subnet/AZ per [documented convention](http://docs.aws.amazon.com/efs/latest/ug/mounting-fs-mount-cmd-dns-name.html).", //nolint:lll
		},
		{
			Input:    "It's recommended to specify `create_before_destroy = true` in a [lifecycle][1] block to replace a certificate which is currently in use (eg, by [`aws_lb_listener`](lb_listener.html)).", //nolint:lll
			Expected: "It's recommended to specify `createBeforeDestroy = true` in a [lifecycle][1] block to replace a certificate which is currently in use (eg, by `awsLbListener`).",                         //nolint:lll
		},
		{
			Input:    "The execution ARN to be used in [`lambda_permission`](/docs/providers/aws/r/lambda_permission.html)'s `source_arn`",                       //nolint:lll
			Expected: "The execution ARN to be used in [`lambdaPermission`](https://www.terraform.io/docs/providers/aws/r/lambda_permission.html)'s `sourceArn`", //nolint:lll
		},
		{
			Input:    "See google_container_node_pool for schema.",
			Expected: "See google.container.NodePool for schema.",
		},
		{
			Input:    "\n(Required)\nThe app_ip of name of the Firebase webApp.",
			Expected: "The appIp of name of the Firebase webApp.",
		},
		{
			Input:    "An example username is jdoa@hashicorp.com",
			Expected: "",
		},
		{
			Input:    "An example passowrd is Terraform-secret",
			Expected: "",
		},
	}

	infoCtx := infoContext{
		pkg:      "google",
		language: "nodejs",
		info: tfbridge.ProviderInfo{
			Name: "google",
			Resources: map[string]*tfbridge.ResourceInfo{
				"google_container_node_pool": {Tok: "google:container/nodePool:NodePool"},
			},
		},
	}

	for _, test := range tests {
		text, elided := reformatText(infoCtx, test.Input, nil)
		assert.Equal(t, test.Expected, text)
		assert.Equalf(t, text == "", elided,
			"We should only see an empty result for non-empty inputs if we have elided text")
	}
}

func TestArgumentRegex(t *testing.T) {
	tests := []struct {
		input    []string
		expected map[docsPath]*argumentDocs
	}{
		{
			input: []string{
				"* `iam_instance_profile` - (Optional) The IAM Instance Profile to",
				"launch the instance with. Specified as the name of the Instance Profile. Ensure your credentials have the correct permission to assign the instance profile according to the [EC2 documentation](http://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles_use_switch-role-ec2.html#roles-usingrole-ec2instance-permissions), notably `iam:PassRole`.",
				"* `ipv6_address_count`- (Optional) A number of IPv6 addresses to associate with the primary network interface. Amazon EC2 chooses the IPv6 addresses from the range of your subnet.",
				"* `ipv6_addresses` - (Optional) Specify one or more IPv6 addresses from the range of the subnet to associate with the primary network interface",
				"* `tags` - (Optional) A mapping of tags to assign to the resource.",
			},
			expected: map[docsPath]*argumentDocs{
				"iam_instance_profile": {
					description: "The IAM Instance Profile to" + "\n" +
						"launch the instance with. Specified as the name of the Instance Profile. Ensure your credentials have the correct permission to assign the instance profile according to the [EC2 documentation](http://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles_use_switch-role-ec2.html#roles-usingrole-ec2instance-permissions), notably `iam:PassRole`.",
				},
				"ipv6_address_count": {
					description: "A number of IPv6 addresses to associate with the primary network interface. Amazon EC2 chooses the IPv6 addresses from the range of your subnet.",
				},
				"ipv6_addresses": {
					description: "Specify one or more IPv6 addresses from the range of the subnet to associate with the primary network interface",
				},
				"tags": {
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
			expected: map[docsPath]*argumentDocs{
				"jwt_configuration": {
					description: "The configuration of a JWT authorizer. Required for the `JWT` authorizer type." + "\n" +
						"Supported only for HTTP APIs.",
				},
				"jwt_configuration.audience": {
					description: "A list of the intended recipients of the JWT. A valid JWT must provide an aud that matches at least one entry in this list.",
				},
				"jwt_configuration.issuer": {
					description: "The base domain of the identity provider that issues JSON Web Tokens, such as the `endpoint` attribute of the [`aws_cognito_user_pool`](/docs/providers/aws/r/cognito_user_pool.html) resource.",
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
			expected: map[docsPath]*argumentDocs{
				"website": {
					description: "A website object (documented below)." + "\n" +
						"~> **NOTE:** You cannot use `acceleration_status` in `cn-north-1` or `us-gov-west-1`",
				},
				"website.index_document": {
					description: "Amazon S3 returns this index document when requests are made to the root domain or any of the subfolders.",
				},
				"website.routing_rules": {
					description: "A json array containing [routing rules](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-s3-websiteconfiguration-routingrules.html)" + "\n" +
						"describing redirect behavior and when redirects are applied.",
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
			expected: map[docsPath]*argumentDocs{
				"action": {
					description: "The action that CloudFront or AWS WAF takes when a web request matches the conditions in the rule. Not used if `type` is `GROUP`.",
				},
				"override_action": {
					description: "Override the action that a group requests CloudFront or AWS WAF takes when a web request matches the conditions in the rule. Only used if `type` is `GROUP`.",
				},
				"type": {
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
			expected: map[docsPath]*argumentDocs{
				"priority": {
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
			expected: map[docsPath]*argumentDocs{
				"retention_policy": {
					description: "A `retention_policy` block as documented below.",
				},
				"allowed_audiences": {
					description: "Allowed audience values to consider when validating JWTs issued by Azure Active Directory.",
				},
			},
		},
		{
			input: []string{
				"* `launch_template_config` - (Optional) Launch template configuration block. See [Launch Template Configs](#launch-template-configs) below for more details. Conflicts with `launch_specification`. At least one of `launch_specification` or `launch_template_config` is required.",
				"* `spot_maintenance_strategies` - (Optional) Nested argument containing maintenance strategies for managing your Spot Instances that are at an elevated risk of being interrupted. Defined below.",
				"* `spot_price` - (Optional; Default: On-demand price) The maximum bid price per unit hour.",
				"* `wait_for_fulfillment` - (Optional; Default: false) If set, Terraform will",
				"  wait for the Spot Request to be fulfilled, and will throw an error if the",
				"  timeout of 10m is reached.",
				"* `target_capacity` - The number of units to request. You can choose to set the",
				"  target capacity in terms of instances or a performance characteristic that is",
				"  important to your application workload, such as vCPUs, memory, or I/O.",
				"* `allocation_strategy` - Indicates how to allocate the target capacity across",
				"  the Spot pools specified by the Spot fleet request. The default is",
				"  `lowestPrice`.",
				"* `instance_pools_to_use_count` - (Optional; Default: 1)",
				"  The number of Spot pools across which to allocate your target Spot capacity.",
				"  Valid only when `allocation_strategy` is set to `lowestPrice`. Spot Fleet selects",
				"  the cheapest Spot pools and evenly allocates your target Spot capacity across",
				"  the number of Spot pools that you specify.",
			},
			expected: map[docsPath]*argumentDocs{
				"launch_template_config": {
					description: "Launch template configuration block. See [Launch Template Configs](#launch-template-configs) below for more details. Conflicts with `launch_specification`. At least one of `launch_specification` or `launch_template_config` is required.",
				},
				"spot_maintenance_strategies": {
					description: "Nested argument containing maintenance strategies for managing your Spot Instances that are at an elevated risk of being interrupted. Defined below.",
				},
				"spot_price": {
					description: "The maximum bid price per unit hour.",
				},
				"wait_for_fulfillment": {
					description: "If set, Terraform will\nwait for the Spot Request to be fulfilled, and will throw an error if the\ntimeout of 10m is reached.",
				},
				"target_capacity": {
					description: "The number of units to request. You can choose to set the\ntarget capacity in terms of instances or a performance characteristic that is\nimportant to your application workload, such as vCPUs, memory, or I/O.",
				},
				"allocation_strategy": {
					description: "Indicates how to allocate the target capacity across\nthe Spot pools specified by the Spot fleet request. The default is\n`lowestPrice`.",
				},
				"instance_pools_to_use_count": {
					description: "\nThe number of Spot pools across which to allocate your target Spot capacity.\nValid only when `allocation_strategy` is set to `lowestPrice`. Spot Fleet selects\nthe cheapest Spot pools and evenly allocates your target Spot capacity across\nthe number of Spot pools that you specify.",
				},
			},
		},
		{
			input: []string{
				"The following arguments are supported:",
				"",
				"- `zone_id` - (Required) The DNS zone ID to which the page rule should be added.",
				"- `target` - (Required) The URL pattern to target with the page rule.",
				"- `actions` - (Required) The actions taken by the page rule, options given below.",
				"",
				"Action blocks support the following:",
				"",
				"- `always_use_https` - (Optional) Boolean of whether this action is enabled. Default: false.",
				"",
			},
			expected: map[docsPath]*argumentDocs{
				"zone_id": {description: "The DNS zone ID to which the page rule should be added."},
				"target":  {description: "The URL pattern to target with the page rule."},
				"actions": {description: "The actions taken by the page rule, options given below."},
				// Note: We parse this as an argument, but it is then discarded when assembling *argumetDocs
				// because it doesn't correspond to a top level resource property.
				"always_use_https": {description: "Boolean of whether this action is enabled. Default: false."},
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run("", func(t *testing.T) {
			ret := entityDocs{
				Arguments: make(map[docsPath]*argumentDocs),
			}
			parseArgReferenceSection(tt.input, &ret)

			assert.Equal(t, tt.expected, ret.Arguments)

			//assert.Len(t, parser.ret.Arguments, len(tt.expected))
			//for k, v := range tt.expected {
			//	actualArg := parser.ret.Arguments[k]
			//	assert.NotNil(t, actualArg, fmt.Sprintf("%s should not be nil", k))
			//	assert.Equal(t, v.description, actualArg.description)
			//	assert.Equal(t, v.isNested, actualArg.isNested)
			//	assert.Equal(t, v.arguments, actualArg.arguments)
			//}
		})
	}
}

func TestGetFooterLinks(t *testing.T) {
	input := `## Attributes Reference

For **environment** the following attributes are supported:

[1]: https://docs.aws.amazon.com/lambda/latest/dg/welcome.html
[2]: https://docs.aws.amazon.com/lambda/latest/dg/walkthrough-s3-events-adminuser-create-test-function-create-function.html
[3]: https://docs.aws.amazon.com/lambda/latest/dg/walkthrough-custom-events-create-test-function.html`

	expected := map[string]string{
		"[1]": "https://docs.aws.amazon.com/lambda/latest/dg/welcome.html",
		"[2]": "https://docs.aws.amazon.com/lambda/latest/dg/walkthrough-s3-events-adminuser-create-test-function-create-function.html",
		"[3]": "https://docs.aws.amazon.com/lambda/latest/dg/walkthrough-custom-events-create-test-function.html",
	}

	actual := getFooterLinks(input)

	assert.Equal(t, expected, actual)
}

func TestReplaceFooterLinks(t *testing.T) {
	inputText := `# Resource: aws_lambda_function

	Provides a Lambda Function resource. Lambda allows you to trigger execution of code in response to events in AWS, enabling serverless backend solutions. The Lambda Function itself includes source code and runtime configuration.

	For information about Lambda and how to use it, see [What is AWS Lambda?][1]
	* (Required) The function [entrypoint][3] in your code.`
	footerLinks := map[string]string{
		"[1]": "https://docs.aws.amazon.com/lambda/latest/dg/welcome.html",
		"[2]": "https://docs.aws.amazon.com/lambda/latest/dg/walkthrough-s3-events-adminuser-create-test-function-create-function.html",
		"[3]": "https://docs.aws.amazon.com/lambda/latest/dg/walkthrough-custom-events-create-test-function.html",
	}

	expected := `# Resource: aws_lambda_function

	Provides a Lambda Function resource. Lambda allows you to trigger execution of code in response to events in AWS, enabling serverless backend solutions. The Lambda Function itself includes source code and runtime configuration.

	For information about Lambda and how to use it, see [What is AWS Lambda?](https://docs.aws.amazon.com/lambda/latest/dg/welcome.html)
	* (Required) The function [entrypoint](https://docs.aws.amazon.com/lambda/latest/dg/walkthrough-custom-events-create-test-function.html) in your code.`
	actual := replaceFooterLinks(inputText, footerLinks)
	assert.Equal(t, expected, actual)

	// Test when there are no footer link.
	actual = replaceFooterLinks(inputText, nil)
	assert.Equal(t, inputText, actual)
}

func TestFixExamplesHeaders(t *testing.T) {
	codeFence := "```"
	t.Run("WithCodeFences", func(t *testing.T) {
		markdown := `
# digitalocean\_cdn

Provides a DigitalOcean CDN Endpoint resource for use with Spaces.

## Example Usage

#### Basic Example

` + codeFence + `typescript
// Some code.
` + codeFence + `
## Argument Reference`

		var processedMarkdown string
		groups := splitGroupLines(markdown, "## ")
		for _, lines := range groups {
			fixExampleTitles(lines)
			for _, line := range lines {
				processedMarkdown += line
			}
		}

		assert.NotContains(t, processedMarkdown, "#### Basic Example")
		assert.Contains(t, processedMarkdown, "### Basic Example")
	})

	t.Run("WithoutCodeFences", func(t *testing.T) {
		markdown := `
# digitalocean\_cdn

Provides a DigitalOcean CDN Endpoint resource for use with Spaces.

## Example Usage

#### Basic Example

Misleading example title without any actual code fences. We should not modify the title.

## Argument Reference`

		var processedMarkdown string
		groups := splitGroupLines(markdown, "## ")
		for _, lines := range groups {
			fixExampleTitles(lines)
			for _, line := range lines {
				processedMarkdown += line
			}
		}

		assert.Contains(t, processedMarkdown, "#### Basic Example")
	})
}

func TestExtractExamples(t *testing.T) {
	basic := `Previews a CIDR from an IPAM address pool. Only works for private IPv4.

~> **NOTE:** This functionality is also encapsulated in a resource sharing the same name. The data source can be used when you need to use the cidr in a calculation of the same Root module, count for example. However, once a cidr range has been allocated that was previewed, the next refresh will find a **new** cidr and may force new resources downstream. Make sure to use Terraform's lifecycle ignore_changes policy if this is undesirable.

## Example Usage
Basic usage:`
	assert.Equal(t, "## Example Usage\nBasic usage:", extractExamples(basic))

	noExampleUsages := `Something mentioning Terraform`
	assert.Equal(t, "", extractExamples(noExampleUsages))

	// This use case is not known to exist in the wild, but we want to make sure our handling here is conservative given that there's no strictly defined schema to TF docs.
	multipleExampleUsages := `Something mentioning Terraform

	## Example Usage
	Some use case

	## Example Usage
	Some other use case
`
	assert.Equal(t, "", extractExamples(multipleExampleUsages))
}

func TestReformatExamples(t *testing.T) {
	runTest := func(input string, expected [][]string) {
		inputSections := splitGroupLines(input, "## ")
		output := reformatExamples(inputSections)

		assert.ElementsMatch(t, expected, output)
	}

	// This is a simple use case. We expect no changes to the original doc:
	simpleDoc := `description

## Example Usage

example usage content`

	simpleDocExpected := [][]string{
		{
			"description",
			"",
		},
		{
			"## Example Usage",
			"",
			"example usage content",
		},
	}

	runTest(simpleDoc, simpleDocExpected)

	// This use case demonstrates 2 examples at the same H2 level: a canonical Example Usage and another example
	// for a specific use case. We expect these to be transformed into a canonical H2 "Example Usage" with an H3 for
	// the specific use case.
	// This scenario is common in the pulumi-gcp provider:
	gcpDoc := `description

## Example Usage

example usage content

## Example Usage - Specific Case

specific case content`

	gcpDocExpected := [][]string{
		{
			"description",
			"",
		},
		{
			"## Example Usage",
			"",
			"example usage content",
			"",
			"### Specific Case",
			"",
			"specific case content",
		},
	}

	runTest(gcpDoc, gcpDocExpected)

	// This use case demonstrates 2 no canonical Example Usage/basic case and 2 specific use cases. We expect the
	// function to add a canonical Example Usage section with the 2 use cases as H3's beneath the canonical section.
	// This scenario is common in the pulumi-gcp provider:
	gcpDoc2 := `description

## Example Usage - 1

content 1

## Example Usage - 2

content 2`

	gcpDoc2Expected := [][]string{
		{
			"description",
			"",
		},
		{
			"## Example Usage",
			"### 1",
			"",
			"content 1",
			"",
			"### 2",
			"",
			"content 2",
		},
	}

	runTest(gcpDoc2, gcpDoc2Expected)
}

func TestFormatEntityName(t *testing.T) {
	assert.Equal(t, "'prov_entity'", formatEntityName("prov_entity"))
	assert.Equal(t, "'prov_entity' (aliased or renamed)", formatEntityName("prov_entity_legacy"))
}

func TestHclConversionsToString(t *testing.T) {
	input := map[string]string{
		"typescript": "var foo = bar;",
		"java":       "FooFactory fooFactory = new FooFactory();",
		"go":         "foo := bar",
		"python":     "foo = bar",
		"yaml":       "# Good enough YAML example",
		"csharp":     "var fooFactory = barProvider.Baz();",
		"pcl":        "# Good enough PCL example",
		"haskell":    "", // i.e., a language we could not convert, which should not appear in the output
	}

	// We use a template because we cannot escape backticks within a herestring, and concatenating this output would be
	// very difficult without using a herestring.
	expectedOutputTmpl := `{{ .CodeFences }}typescript
var foo = bar;
{{ .CodeFences }}
{{ .CodeFences }}python
foo = bar
{{ .CodeFences }}
{{ .CodeFences }}csharp
var fooFactory = barProvider.Baz();
{{ .CodeFences }}
{{ .CodeFences }}go
foo := bar
{{ .CodeFences }}
{{ .CodeFences }}java
FooFactory fooFactory = new FooFactory();
{{ .CodeFences }}
{{ .CodeFences }}pcl
# Good enough PCL example
{{ .CodeFences }}
{{ .CodeFences }}yaml
# Good enough YAML example
{{ .CodeFences }}`

	outputTemplate, _ := template.New("dummy").Parse(expectedOutputTmpl)
	data := struct {
		CodeFences string
	}{
		CodeFences: "```",
	}

	var buf = bytes.Buffer{}
	_ = outputTemplate.Execute(&buf, data)

	assert.Equal(t, buf.String(), hclConversionsToString(input))
}

func TestGroupLines(t *testing.T) {
	input := `description

## subtitle 1

subtitle 1 content

## subtitle 2

subtitle 2 content
`
	expected := [][]string{
		{
			"description",
			"",
		},
		{
			"## subtitle 1",
			"",
			"subtitle 1 content",
			"",
		},
		{
			"## subtitle 2",
			"",
			"subtitle 2 content",
			"",
		},
	}

	assert.Equal(t, expected, groupLines(strings.Split(input, "\n"), "## "))
}

func TestParseArgFromMarkdownLine(t *testing.T) {
	//nolint:lll
	tests := []struct {
		input         string
		expectedName  string
		expectedDesc  string
		expectedFound bool
	}{
		{"* `name` - (Required) A unique name to give the role.", "name", "A unique name to give the role.", true},
		{"* `key_vault_key_id` - (Optional) The Key Vault key URI for CMK encryption. Changing this forces a new resource to be created.", "key_vault_key_id", "The Key Vault key URI for CMK encryption. Changing this forces a new resource to be created.", true},
		{"* `urn` - The uniform resource name of the Droplet", "urn", "The uniform resource name of the Droplet", true},
		{"* `name`- The name of the Droplet", "name", "The name of the Droplet", true},
		{"* `jumbo_frame_capable` -Indicates whether jumbo frames (9001 MTU) are supported.", "jumbo_frame_capable", "Indicates whether jumbo frames (9001 MTU) are supported.", true},
		{"* `ssl_support_method`: Specifies how you want CloudFront to serve HTTPS", "ssl_support_method", "Specifies how you want CloudFront to serve HTTPS", true},
		{"* `principal_tags`: (Optional: []) - String to string map of variables.", "principal_tags", "String to string map of variables.", true},
		// In rare cases, we may have a match where description is empty like the following, taken from https://github.com/hashicorp/terraform-provider-aws/blob/main/website/docs/r/spot_fleet_request.html.markdown
		{"* `instance_pools_to_use_count` - (Optional; Default: 1)", "instance_pools_to_use_count", "", true},
		{"", "", "", false},
		{"Most of these arguments directly correspond to the", "", "", false},
	}

	for _, test := range tests {
		name, desc, found := parseArgFromMarkdownLine(test.input)
		assert.Equal(t, test.expectedName, name)
		assert.Equal(t, test.expectedDesc, desc)
		assert.Equal(t, test.expectedFound, found)
	}
}

func TestParseAttributesReferenceSection(t *testing.T) {
	ret := entityDocs{
		Arguments:  make(map[docsPath]*argumentDocs),
		Attributes: make(map[string]string),
	}
	parseAttributesReferenceSection([]string{
		"The following attributes are exported:",
		"",
		"* `id` - The ID of the Droplet",
		"* `urn` - The uniform resource name of the Droplet",
		"* `name`- The name of the Droplet",
		"* `region` - The region of the Droplet",
	}, &ret)
	assert.Len(t, ret.Attributes, 4)
}

func TestGetNestedBlockName(t *testing.T) {
	var tests = []struct {
		input, expected string
	}{
		{"", ""},
		{"The `website` object supports the following:", "website"},
		{"The optional `settings.location_preference` subblock supports:", "location_preference"},
		{"The optional `settings.ip_configuration.authorized_networks[]` sublist supports:", "authorized_networks"},
		{"#### result_configuration Argument Reference", "result_configuration"},
		{"### advanced_security_options", "advanced_security_options"},
		{"### `server_side_encryption`", "server_side_encryption"},
		{"### Failover Routing Policy", "failover_routing_policy"},
		{"##### `log_configuration`", "log_configuration"},
		{"### data_format_conversion_configuration", "data_format_conversion_configuration"},
		// This is a common starting line of base arguments, so should result in zero value:
		{"The following arguments are supported:", ""},
		{"* `kms_key_id` - ...", ""},
		{"## Import", ""},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, getNestedBlockName(tt.input))
	}
}

func TestOverlayAttributesToAttributes(t *testing.T) {
	source := entityDocs{
		Attributes: map[string]string{
			"overwrite_me": "overwritten_desc",
			"source_only":  "source_only_desc",
		},
	}

	dest := entityDocs{
		Attributes: map[string]string{
			"overwrite_me": "original_desc",
			"dest_only":    "dest_only_desc",
		},
	}

	expected := entityDocs{
		Attributes: map[string]string{
			"overwrite_me": "overwritten_desc",
			"source_only":  "source_only_desc",
			"dest_only":    "dest_only_desc",
		},
	}

	overlayAttributesToAttributes(source, dest)

	assert.Equal(t, expected, dest)
}

func TestOverlayArgsToAttributes(t *testing.T) {
	source := entityDocs{
		Arguments: map[docsPath]*argumentDocs{
			"overwrite_me": {
				description: "overwritten_desc",
			},
			"source_only": {
				description: "source_only_desc",
			},
		},
	}

	dest := entityDocs{
		Attributes: map[string]string{
			"overwrite_me": "original_desc",
			"dest_only":    "dest_only_desc",
		},
	}

	expected := entityDocs{
		Attributes: map[string]string{
			"overwrite_me": "overwritten_desc",
			"source_only":  "source_only_desc",
			"dest_only":    "dest_only_desc",
		},
	}

	overlayArgsToAttributes(source, dest)

	assert.Equal(t, expected, dest)
}

func TestOverlayArgsToArgs(t *testing.T) {
	source := entityDocs{
		Arguments: map[docsPath]*argumentDocs{
			"overwrite_me":                     {description: "overwritten_desc"},
			"overwrite_me.nested_source_only":  {description: "nested_source_only_desc"},
			"overwrite_me.nested_overwrite_me": {description: "nested_overwrite_me_overwritten_desc"},

			"source_only": {description: "source_only_desc"},
		},
	}

	dest := entityDocs{
		Arguments: map[docsPath]*argumentDocs{
			"overwrite_me":                     {description: "original_desc"},
			"overwrite_me.nested_dest_only":    {description: "should not appear"},
			"overwrite_me.nested_overwrite_me": {description: "nested_overwrite_me original desc"},

			"dest_only": {description: "dest_only_desc"},
		},
	}

	expected := entityDocs{
		Arguments: map[docsPath]*argumentDocs{
			"overwrite_me":                     {description: "overwritten_desc"},
			"overwrite_me.nested_source_only":  {description: "nested_source_only_desc"},
			"overwrite_me.nested_overwrite_me": {description: "nested_overwrite_me_overwritten_desc"},

			"source_only": {description: "source_only_desc"},
			"dest_only":   {description: "dest_only_desc"},
		},
	}

	overlayArgsToArgs(source, &dest)

	assert.Equal(t, expected, dest)
}

func TestParseImports_NoOverrides(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skipf("Skippping on windows - tests cases need to be made robust to newline handling")
	}
	var tests = []struct {
		input        []string
		token        tokens.Token
		expected     string
		expectedFile string
	}{
		{
			input: []string{
				"",
				"Import is supported using the following syntax:", // This is intentionally discarded
				"",
				"```shell", // This has several variations upstream
				"# format is account name | | | privilege | true/false for with_grant_option", // Ensure we remove the shell comment to avoid rendering as H1 in Markdown
				"terraform import snowflake_account_grant.example 'accountName|||USAGE|true'",
				"```",
				"",
			},
			token:    "snowflake:index/accountGrant:AccountGrant",
			expected: "## Import\n\nformat is account name | | | privilege | true/false for with_grant_option\n\n```sh\n$ pulumi import snowflake:index/accountGrant:AccountGrant example 'accountName|||USAGE|true'\n```\n\n",
		},
		{
			input: []string{
				"",
				"Import is supported using the following syntax:", // This is intentionally discarded
				"",
				"```sh", // This has several variations upstream
				"terraform import snowflake_api_integration.example name",
				"```",
				"",
			},
			token:    "snowflake:index/apiIntegration:ApiIntegration",
			expected: "## Import\n\n```sh\n$ pulumi import snowflake:index/apiIntegration:ApiIntegration example name\n```\n\n",
		},
		{
			input: []string{
				"",
				"This is a first line in a multi-line import section",
				"* `{{name}}`",
				"* `{{id}}`",
				"For example:",
				"```sh", // This has several variations upstream
				"terraform import gcp_accesscontextmanager_access_level.example name",
				"```",
				"",
			},
			token:    "gcp:accesscontextmanager/accessLevel:AccessLevel",
			expected: "## Import\n\nThis is a first line in a multi-line import section\n\n* `{{name}}`\n\n* `{{id}}`\n\nFor example:\n\n```sh\n$ pulumi import gcp:accesscontextmanager/accessLevel:AccessLevel example name\n```\n\n",
		},
		{
			input:        readlines(t, "test_data/parse-imports/accessanalyzer.md"),
			token:        "aws:accessanalyzer/analyzer:Analyzer",
			expectedFile: "test_data/parse-imports/accessanalyzer-expected.md",
		},
		{
			input:        readlines(t, "test_data/parse-imports/gameliftconfig.md"),
			token:        "aws:gamelift/matchmakingConfiguration:MatchmakingConfiguration",
			expectedFile: "test_data/parse-imports/gameliftconfig-expected.md",
		},
		{
			input:        readlines(t, "test_data/parse-imports/lambdalayer.md"),
			token:        "aws:lambda/layerVersion:LayerVersion",
			expectedFile: "test_data/parse-imports/lambdalayer-expected.md",
		},
	}

	for _, tt := range tests {
		parser := tfMarkdownParser{
			info: &mockResource{
				token: tt.token,
			},
		}
		parser.parseImports(tt.input)
		actual := parser.ret.Import
		if tt.expectedFile != "" {
			if accept {
				writefile(t, tt.expectedFile, []byte(actual))
			}
			tt.expected = readfile(t, tt.expectedFile)
		}
		assert.Equal(t, tt.expected, actual)
	}
}

func TestParseImports_WithOverride(t *testing.T) {
	parser := tfMarkdownParser{
		info: &mockResource{
			docs: tfbridge.DocInfo{
				ImportDetails: "overridden import details",
			},
		},
	}

	parser.parseImports([]string{"this doesn't matter because we are overriding it"})

	assert.Equal(t, "## Import\n\noverridden import details", parser.ret.Import)
}

func TestConvertExamples(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skipf("Skipping on windows to avoid failing on incorrect newline handling")
	}

	inmem := afero.NewMemMapFs()
	info := testprovider.ProviderMiniRandom()
	g, err := NewGenerator(GeneratorOptions{
		Package:      info.Name,
		Version:      info.Version,
		Language:     Schema,
		ProviderInfo: info,
		Root:         inmem,
		Sink: diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{
			Color: colors.Never,
		}),
	})
	assert.NoError(t, err)

	type testCase struct {
		name string
		path examplePath

		needsProviders map[string]pluginDesc
	}

	testCases := []testCase{
		{
			name: "wavefront_dashboard_json",
			path: examplePath{
				fullPath: "#/resources/wavefront:index/dashboardJson:DashboardJson",
				token:    "wavefront:index/dashboardJson:DashboardJson",
			},
			needsProviders: map[string]pluginDesc{
				"wavefront": {version: "3.0.0"},
			},
		},
		{
			name: "equinix_fabric_connection",
			path: examplePath{
				fullPath: "#/resources/equinix:fabric:Connection",
				token:    "equinix:fabric:Connection",
			},
			needsProviders: map[string]pluginDesc{
				"equinix": {
					pluginDownloadURL: "github://api.github.com/equinix",
					version:           "0.6.0",
				},
			},
		},
		{
			name: "aws_lambda_function",
			path: examplePath{
				fullPath: "#/resources/aws:lambda/function:Function",
				token:    "aws:lambda/function:Function",
			},
			needsProviders: map[string]pluginDesc{
				"aws": {
					pluginDownloadURL: "github://api.github.com/pulumi",
					version:           "6.21.0",
				},
			},
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(fmt.Sprintf("%s/setup", tc.name), func(t *testing.T) {
			ensureProvidersInstalled(t, tc.needsProviders)
		})

		t.Run(tc.name, func(t *testing.T) {
			docs, err := os.ReadFile(filepath.Join("test_data", "convertExamples",
				fmt.Sprintf("%s.md", tc.name)))
			require.NoError(t, err)
			result := g.convertExamples(string(docs), tc.path)

			out := filepath.Join("test_data", "convertExamples",
				fmt.Sprintf("%s_out.md", tc.name))
			if accept {
				err = os.WriteFile(out, []byte(result), 0600)
				require.NoError(t, err)
			}
			expect, err := os.ReadFile(out)
			require.NoError(t, err)
			assert.Equal(t, string(expect), result)
		})
	}
}

//func TestConvertExamplesInner(t *testing.T) {
//	if runtime.GOOS == "windows" {
//		t.Skipf("Skipping on windows to avoid failing on incorrect newline handling")
//	}
//
//	inmem := afero.NewMemMapFs()
//	info := testprovider.ProviderMiniRandom()
//	g, err := NewGenerator(GeneratorOptions{
//		Package:      info.Name,
//		Version:      info.Version,
//		Language:     Schema,
//		ProviderInfo: info,
//		Root:         inmem,
//		Sink: diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{
//			Color: colors.Never,
//		}),
//	})
//	assert.NoError(t, err)
//
//	type testCase struct {
//		name           string
//		path           examplePath
//		needsProviders map[string]pluginDesc
//	}
//
//	testCases := []testCase{
//		{
//			name: "aws_lambda_function",
//			path: examplePath{
//				fullPath: "#/resources/aws:lambda/function:Function",
//				token:    "aws:lambda/function:Function",
//			},
//			needsProviders: map[string]pluginDesc{
//				"aws": {
//					pluginDownloadURL: "github://api.github.com/pulumi",
//					version:           "5.35.0",
//				},
//			},
//		},
//		{
//			name: "displays valid JSON ",
//			path: examplePath{
//				fullPath: "#/resources/aws:lambda/function:Function",
//				token:    "aws:lambda/function:Function",
//			},
//			needsProviders: map[string]pluginDesc{
//				"aws": {
//					pluginDownloadURL: "github://api.github.com/pulumi",
//					version:           "5.35.0",
//				},
//			},
//		},
//	}
//
//	for _, tc := range testCases {
//		tc := tc
//
//		t.Run(fmt.Sprintf("%s/setup", tc.name), func(t *testing.T) {
//			ensureProvidersInstalled(t, tc.needsProviders)
//		})
//
//		t.Run(tc.name, func(t *testing.T) {
//			docs, err := os.ReadFile(filepath.Join("test_data", "convertExamples",
//				fmt.Sprintf("%s.md", tc.name)))
//			require.NoError(t, err)
//			result := g.convertExamplesInner(string(docs), tc.path, g.convertHCL, false)
//
//			out := filepath.Join("test_data", "convertExamples",
//				fmt.Sprintf("%s_out.md", tc.name))
//			if accept {
//				err = os.WriteFile(out, []byte(result), 0600)
//				require.NoError(t, err)
//			}
//			expect, err := os.ReadFile(out)
//			require.NoError(t, err)
//			assert.Equal(t, string(expect), result)
//		})
//	}
//}

type pluginDesc struct {
	version           string
	pluginDownloadURL string
}

func TestFindFencesAndHeaders(t *testing.T) {
	type testCase struct {
		name     string
		path     string
		expected []codeBlock
	}

	testCases := []testCase{
		{
			name: "finds locations of all fences and headers in a long doc",
			path: filepath.Join("test_data", "parse-inner-docs",
				"aws_lambda_function_description.md"),
			expected: []codeBlock{
				{start: 1966, end: 2977, headerStart: 1947},
				{start: 3001, end: 3224, headerStart: 2982},
				{start: 3387, end: 4105, headerStart: 3229},
				{start: 4358, end: 5953, headerStart: 4110},
				{start: 6622, end: 8041, headerStart: 6421},
				{start: 9151, end: 9238, headerStart: 9052},
			},
		},
		{
			name: "finds locations when there are no headers",
			path: filepath.Join("test_data", "parse-inner-docs",
				"starts-with-code-block.md"),
			expected: []codeBlock{
				{start: 0, end: 46, headerStart: -1},
			},
		},
		{
			name: "starts with an h2 header",
			path: filepath.Join("test_data", "parse-inner-docs",
				"starts-with-h2.md"),
			expected: []codeBlock{
				{start: 91, end: 142, headerStart: 0},
			},
		},
		{
			name: "starts with an h3 header",
			path: filepath.Join("test_data", "parse-inner-docs",
				"starts-with-h3.md"),
			expected: []codeBlock{
				{start: 92, end: 114, headerStart: 0},
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			testDocBytes, err := os.ReadFile(tc.path)
			require.NoError(t, err)
			testDoc := string(testDocBytes)
			actual := findFencesAndHeaders(testDoc)
			assert.Equal(t, tc.expected, actual)
		})

	}

}

func ensureProvidersInstalled(t *testing.T, needsProviders map[string]pluginDesc) {
	pulumi, err := exec.LookPath("pulumi")
	require.NoError(t, err)

	t.Logf("pulumi plugin ls --json")
	cmd := exec.Command(pulumi, "plugin", "ls", "--json")
	var buf bytes.Buffer
	cmd.Stdout = &buf
	err = cmd.Run()
	require.NoError(t, err)

	type plugin struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}

	var installedPlugins []plugin
	err = json.Unmarshal(buf.Bytes(), &installedPlugins)
	require.NoError(t, err)

	for name, desc := range needsProviders {
		count := 0
		matched := false

		for _, p := range installedPlugins {
			if p.Name == name {
				count++
			}
			if p.Name == name && p.Version == desc.version {
				matched = true
			}
		}

		alreadyInstalled := count == 1 && matched
		if alreadyInstalled {
			continue
		}

		if count > 0 {
			t.Logf("pulumi plugin rm resource %s", name)
			err = exec.Command(pulumi, "plugin", "rm", "resource", name).Run()
			require.NoError(t, err)
		}

		args := []string{"plugin", "install", "resource", name, desc.version}
		if desc.pluginDownloadURL != "" {
			args = append(args, "--server", desc.pluginDownloadURL)
		}
		cmd := exec.Command(pulumi, args...)
		t.Logf("Exec: %s", cmd)
		err = cmd.Run()
		require.NoError(t, err)
	}
}

func TestExampleGeneration(t *testing.T) {
	info := testprovider.ProviderMiniRandom()

	markdown := []byte(`
## Examples

There is some more code in here.

~~~java
throw new Exception("!");
~~~
`)

	markdown = bytes.ReplaceAll(markdown, []byte("~~~"), []byte("```"))

	info.Resources["random_integer"].Docs = &tfbridge.DocInfo{
		Markdown: markdown,
	}

	inmem := afero.NewMemMapFs()

	g, err := NewGenerator(GeneratorOptions{
		Package:      info.Name,
		Version:      info.Version,
		Language:     Schema,
		ProviderInfo: info,
		Root:         inmem,
		Sink: diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{
			Color: colors.Never,
		}),
	})
	assert.NoError(t, err)

	err = g.Generate()
	assert.NoError(t, err)

	f, err := inmem.Open("schema.json")
	assert.NoError(t, err)

	schemaBytes, err := io.ReadAll(f)
	assert.NoError(t, err)

	assert.NotContains(t, string(schemaBytes), "{{% //examples %}}")
}

func TestParseTFMarkdown(t *testing.T) {
	t.Parallel()

	type testCase struct {
		// The name of the test case.
		//
		// The name of the folder for input and expected output is derived from
		// `name`.
		name string

		info         tfbridge.ResourceOrDataSourceInfo
		providerInfo tfbridge.ProviderInfo
		kind         DocKind
		rawName      string

		fileName string
	}

	// Assert that file contents match the expected description.
	test := func(name string, configure ...func(tc *testCase)) testCase {
		tc := testCase{
			name:    name,
			kind:    ResourceDocs,
			rawName: "pkg_mod1_res1",

			fileName: "mod1_res1.md",
		}
		for _, c := range configure {
			c(&tc)
		}
		return tc
	}

	tests := []testCase{
		test("simple"),
		test("link"),
		test("azurerm-sql-firewall-rule"),
		test("address_map"),

		test("custom-replaces", func(tc *testCase) {
			rule := tfbridge.DocsEdit{
				Path: "*",
				Edit: func(path string, content []byte) ([]byte, error) {
					assert.Equal(t, "mod1_res1.md", path)
					return bytes.ReplaceAll(content,
						[]byte(`CUSTOM_REPLACES`),
						[]byte(`checking custom replaces`)), nil
				},
			}

			tc.providerInfo.DocRules = &tfbridge.DocRuleInfo{
				EditRules: func(defaults []tfbridge.DocsEdit) []tfbridge.DocsEdit {
					return append([]tfbridge.DocsEdit{rule}, defaults...)
				},
			}
		}),
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.NotZero(t, tt.name)
			input := testFilePath(tt.name, "input.md")
			expected := filepath.Join(tt.name, "expected.json")
			p := &tfMarkdownParser{
				sink:             mockSink{t},
				info:             tt.info,
				kind:             tt.kind,
				markdownFileName: tt.fileName,
				rawname:          tt.rawName,

				infoCtx: infoContext{
					language: Schema,
					pkg:      "pkg",
					info:     tt.providerInfo,
				},
				editRules: getEditRules(tt.providerInfo.DocRules),
			}

			inputBytes, err := os.ReadFile(input)
			require.NoError(t, err)

			actual, err := p.parse(inputBytes)
			require.NoError(t, err)

			actualBytes, err := json.MarshalIndent(actual, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			compareTestFile(t, expected, string(actualBytes), assert.JSONEq)
		})
	}
}

func TestErrorMissingDocs(t *testing.T) {
	tests := []struct {
		docs                 tfbridge.DocInfo
		forbidMissingDocsEnv string
		source               DocsSource
		expectErr            bool
	}{
		// No Error, since the docs can be found
		{source: mockSource{"raw_name": "some-docs"}},
		{
			source:               mockSource{"raw_name": "some-docs"},
			forbidMissingDocsEnv: "true",
		},

		// Docs are missing, but we don't ask to error on missing
		{source: mockSource{}},

		// Docs are missing, and we ask to error on missing, so error
		{
			source:               mockSource{},
			forbidMissingDocsEnv: "true",
			expectErr:            true,
		},

		// Docs are missing and we ask globally to error on missing, but we
		// override locally, so no error
		{
			source:               mockSource{},
			docs:                 tfbridge.DocInfo{AllowMissing: true},
			forbidMissingDocsEnv: "true",
		},
		// DocInfo is nil so we error because docs are missing
		{
			source:               mockSource{},
			forbidMissingDocsEnv: "true",
			expectErr:            true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run("", func(t *testing.T) {
			g := &Generator{
				sink: mockSink{t},
			}
			rawName := "raw_name"
			t.Setenv("PULUMI_MISSING_DOCS_ERROR", tt.forbidMissingDocsEnv)
			_, err := getDocsForResource(g, tt.source, ResourceDocs, rawName, &mockResource{
				token: tokens.Token(rawName),
				docs:  tt.docs,
			})
			if tt.expectErr {
				assert.NotNil(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
func TestErrorNilDocs(t *testing.T) {
	t.Run("", func(t *testing.T) {
		g := &Generator{
			sink: mockSink{t},
		}
		rawName := "nil_docs"
		t.Setenv("PULUMI_MISSING_DOCS_ERROR", "true")
		info := mockNilDocsResource{token: tokens.Token(rawName)}
		_, err := getDocsForResource(g, mockSource{}, ResourceDocs, rawName, &info)
		assert.NotNil(t, err)
	})
}

type mockSource map[string]string

func (m mockSource) getResource(rawname string, info *tfbridge.DocInfo) (*DocFile, error) {
	f, ok := m[rawname]
	if !ok {
		return nil, nil
	}
	return &DocFile{
		Content:  []byte(f),
		FileName: rawname + ".md",
	}, nil
}
func (m mockSource) getDatasource(rawname string, info *tfbridge.DocInfo) (*DocFile, error) {
	return nil, nil
}

type mockSink struct{ t *testing.T }

func (mockSink) warn(string, ...interface{})                                  {}
func (mockSink) debug(string, ...interface{})                                 {}
func (mockSink) error(string, ...interface{})                                 {}
func (mockSink) Logf(sev diag.Severity, diag *diag.Diag, args ...interface{}) {}
func (mockSink) Debugf(diag *diag.Diag, args ...interface{})                  {}
func (mockSink) Infof(diag *diag.Diag, args ...interface{})                   {}
func (mockSink) Infoerrf(diag *diag.Diag, args ...interface{})                {}
func (mockSink) Errorf(diag *diag.Diag, args ...interface{})                  {}
func (mockSink) Warningf(diag *diag.Diag, args ...interface{})                {}

func (mockSink) Stringify(sev diag.Severity, diag *diag.Diag, args ...interface{}) (string, string) {
	return "", ""
}

type mockResource struct {
	docs  tfbridge.DocInfo
	token tokens.Token
}

func (r *mockResource) GetFields() map[string]*tfbridge.SchemaInfo {
	return map[string]*tfbridge.SchemaInfo{}
}

func (r *mockResource) ReplaceExamplesSection() bool {
	return false
}

func (r *mockResource) GetDocs() *tfbridge.DocInfo {
	return &r.docs
}

func (r *mockResource) GetTok() tokens.Token {
	return r.token
}

type mockNilDocsResource struct {
	token tokens.Token
	mockResource
}

func (nr *mockNilDocsResource) GetDocs() *tfbridge.DocInfo {
	return nil
}

func readfile(t *testing.T, file string) string {
	t.Helper()
	bytes, err := os.ReadFile(file)
	require.NoError(t, err)
	return string(bytes)
}

func writefile(t *testing.T, file string, bytes []byte) {
	t.Helper()
	err := os.WriteFile(file, bytes, 0600)
	require.NoError(t, err)
}

func readlines(t *testing.T, file string) []string {
	t.Helper()
	f, err := os.Open(file)
	require.NoError(t, err)
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	return lines
}

func TestFixupImports(t *testing.T) {
	tests := []struct{ text, expected string }{
		{
			"% terraform import thing",
			"% pulumi import thing",
		},
		{
			"% Terraform import thing",
			"% Pulumi import thing",
		},
		{
			"% FOO import thing",
			"% FOO import thing",
		},
		{
			"`terraform import`",
			"`pulumi import`",
		},
		{
			"`Terraform import`",
			"`pulumi import`",
		},
		{
			`% terraform import has-terraform-name`,
			`% pulumi import has-pulumi-name`,
		},
		{
			text: "In Terraform v1.5.0 and later, use an `import` block to import Transfer Workflows using the `id`. For example:\n" +
				"\n" +
				"```terraform" + `
		import {
		to = aws_verifiedaccess_trust_provider.example
		id = "vatp-8012925589"
		}` + "\n```\n" + `post text:
		` + "```yaml" + `
		foo: bar
		` + "```\n",
			expected: `post text:
		` + "```yaml" + `
		foo: bar
		` + "```\n",
		},
		{
			"In terraform v1.5.0 and later, use an `import` block to import Transfer Workflows using the `id`. For example:\n" +
				"\n" +
				"```terraform" + `
		import {
		to = aws_verifiedaccess_trust_provider.example
		id = "vatp-8012925589"
		}` + "\n```\n" + `post text:
		` + "```yaml" + `
		foo: bar
		` + "```\n",
			`post text:
		` + "```yaml" + `
		foo: bar
		` + "```\n",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.text, func(t *testing.T) {
			t.Parallel()
			myRule := fixupImports()
			actual, err := myRule.Edit("*", []byte(tt.text))
			require.NoError(t, err)
			assert.Equal(t, tt.expected, string(actual))
		})
	}
}
