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
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"text/template"

	"github.com/hexops/autogold/v2"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfgen/internal/testprovider"
)

var accept = cmdutil.IsTruthy(os.Getenv("PULUMI_ACCEPT"))

func TestReformatText(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name            string
		input           string
		assertPreserved bool
	}{
		{
			name:  "No changes on valid links",
			input: "The DNS name for the given subnet/AZ per [documented convention](http://docs.aws.amazon.com/efs/latest/ug/mounting-fs-mount-cmd-dns-name.html).", //nolint:lll
		},
		{
			name:  "Translates input options to Pulumi formats",
			input: "It's recommended to specify `create_before_destroy = true` in a [lifecycle][1] block to replace a certificate which is currently in use (eg, by [`aws_lb_listener`](lb_listener.html)).", //nolint:lll
		},
		{
			name:  "Fixes up link refs",
			input: "The execution ARN to be used in [`lambda_permission`](/docs/providers/aws/r/lambda_permission.html)'s `source_arn`", //nolint:lll
		},
		{
			name:  "Translates resource names to Pulumi formats",
			input: "See google_container_node_pool for schema.",
		},
		{
			name:  "Translates property types to Pulumi formats",
			input: "\n(Required)\nThe app_ip of name of the Firebase webApp.",
		},
		{
			name:            "Preserves text with @hashicorp.com",
			input:           "An example username is jdoa@hashicorp.com",
			assertPreserved: true,
		},
		{
			name:            "Preserves text with Terraform",
			input:           "An example password is Terraform-secret",
			assertPreserved: true,
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

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			text := reformatText(infoCtx, tc.input, nil)
			autogold.ExpectFile(t, autogold.Raw(text))
			if tc.assertPreserved {
				assert.NotEmpty(t, text, "Terraform/Hashicorp cleanup should preserve transformed content")
			}
		})
	}
}

func TestReformatImportText(t *testing.T) {
	t.Parallel()
	infoCtx := infoContext{
		pkg:      "aws",
		language: "nodejs",
		info: tfbridge.ProviderInfo{
			Name: "aws",
		},
	}
	input := "### Identity Schema\n\n#### Required\n\n- `load_balancer_name` (String) Name."
	text := reformatImportText(infoCtx, input, nil)
	assert.Contains(t, text, "`load_balancer_name`")
	assert.Contains(t, text, "pulumi-lang-nodejs")
}

func TestArgumentRegex(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    []string
		expected map[docsPath]*argumentDocs
	}{
		{
			name: "Discovers * bullet descriptions",
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
			name: "Parses nested arguments via `object supports the following`",
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
			name: "Renders ~> **NOTE:** and continues parsing as expected",
			input: []string{
				"* `website` - (Optional) A website object (documented below).",
				"~> **NOTE:** You cannot use `acceleration_status` in `cn-north-1` or `us-gov-west-1`",
				"",
				"The `website` and `webpage` objects support the following:",
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
				"webpage.index_document": {
					description: "Amazon S3 returns this index document when requests are made to the root domain or any of the subfolders.",
				},
				"webpage.routing_rules": {
					description: "A json array containing [routing rules](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-s3-websiteconfiguration-routingrules.html)" + "\n" +
						"describing redirect behavior and when redirects are applied.",
				},
			},
		},
		{
			name: "Displays in-text backticked values as given",
			input: []string{
				"* `action` - (Optional) The action that CloudFront or AWS WAF takes when a web request matches the conditions in the rule. Not used if `type` is `GROUP`.",
				"  * `type` - (Required) valid values are: `BLOCK`, `ALLOW`, or `COUNT`",
				"* `override_action` - (Optional) Override the action that a group requests CloudFront or AWS WAF takes when a web request matches the conditions in the rule. Only used if `type` is `GROUP`.",
				"  * `type` - (Required) valid values are: `BLOCK`, `ALLOW`, or `COUNT`",
			},
			expected: map[docsPath]*argumentDocs{
				"action": {
					description: "The action that CloudFront or AWS WAF takes when a web request matches the conditions in the rule. Not used if `type` is `GROUP`.",
				},
				"override_action": {
					description: "Override the action that a group requests CloudFront or AWS WAF takes when a web request matches the conditions in the rule. Only used if `type` is `GROUP`.",
				},
				"override_action.type": {
					description: "valid values are: `BLOCK`, `ALLOW`, or `COUNT`",
				},
				"action.type": {
					description: "valid values are: `BLOCK`, `ALLOW`, or `COUNT`",
				},
			},
		},
		{
			name: "Retains second mention of property if named twice",
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
			name: "Correctly handles markdown sectioning",
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
			name: "Cleans up `TF Optional; Default` parentheses",
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
			name: "Doesn't associate unbackticked properties in supports block regexp",
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
		{
			name: "Parses `property1`, `property2`, and `property3`'s `subproperty` object supports the following",
			input: []string{
				"The `grpc_route`, `http_route` and `http2_route`'s `action` object supports the following:",
				"",
				"- `target` - (Required) Target that traffic is routed to when a request matches the gateway route.",
				"",
				"The `target` object supports the following:",
				"",
				"- `port` - (Optional) The port number that corresponds to the target for Virtual Service provider port. This is required when the provider (router or node) of the Virtual Service has multiple listeners.",
				"- `virtual_service` - (Required) Virtual service gateway route target.",
				"",
				"The `grpc_route`'s `match` object supports the following:",
				"",
				"- `service_name` - (Required) Fully qualified domain name for the service to match from the request.",
				"- `port` - (Optional) The port number to match from the request.",
			},
			expected: map[docsPath]*argumentDocs{
				"grpc_route.action.target":      {description: "Target that traffic is routed to when a request matches the gateway route."},
				"http_route.action.target":      {description: "Target that traffic is routed to when a request matches the gateway route."},
				"http2_route.action.target":     {description: "Target that traffic is routed to when a request matches the gateway route."},
				"target.port":                   {description: "The port number that corresponds to the target for Virtual Service provider port. This is required when the provider (router or node) of the Virtual Service has multiple listeners."},
				"target.virtual_service":        {description: "Virtual service gateway route target."},
				"grpc_route.match.port":         {description: "The port number to match from the request."},
				"grpc_route.match.service_name": {description: "Fully qualified domain name for the service to match from the request."},
			},
		},
		{
			name: "Parses H3 and H4 headers and their bullets as nested properties",
			input: []string{
				"### certificate_authority_configuration",
				"",
				"* `key_algorithm` - (Required) Type of the public key algorithm and size, in bits, of the key pair that your key pair creates when it issues a certificate. Valid values can be found in the [ACM PCA Documentation](https://docs.aws.amazon.com/privateca/latest/APIReference/API_CertificateAuthorityConfiguration.html).",
				"* `signing_algorithm` - (Required) Name of the algorithm your private CA uses to sign certificate requests. Valid values can be found in the [ACM PCA Documentation](https://docs.aws.amazon.com/privateca/latest/APIReference/API_CertificateAuthorityConfiguration.html).",
				"* `subject` - (Required) Nested argument that contains X.500 distinguished name information. At least one nested attribute must be specified.",
				"",
				"#### subject",
				"",
				"Contains information about the certificate subject. Identifies the entity that owns or controls the public key in the certificate. The entity can be a user, computer, device, or service.",
				"",
				"* `common_name` - (Optional) Fully qualified domain name (FQDN) associated with the certificate subject. Must be less than or equal to 64 characters in length.",
				"* `country` - (Optional) Two digit code that specifies the country in which the certificate subject located. Must be less than or equal to 2 characters in length.",
			},
			expected: map[docsPath]*argumentDocs{
				"certificate_authority_configuration.key_algorithm":     {description: "Type of the public key algorithm and size, in bits, of the key pair that your key pair creates when it issues a certificate. Valid values can be found in the [ACM PCA Documentation](https://docs.aws.amazon.com/privateca/latest/APIReference/API_CertificateAuthorityConfiguration.html)."},
				"certificate_authority_configuration.signing_algorithm": {description: "Name of the algorithm your private CA uses to sign certificate requests. Valid values can be found in the [ACM PCA Documentation](https://docs.aws.amazon.com/privateca/latest/APIReference/API_CertificateAuthorityConfiguration.html)."},
				"certificate_authority_configuration.subject":           {description: "Nested argument that contains X.500 distinguished name information. At least one nested attribute must be specified."},
				"subject.common_name":                                   {description: "Fully qualified domain name (FQDN) associated with the certificate subject. Must be less than or equal to 64 characters in length."},
				"subject.country":                                       {description: "Two digit code that specifies the country in which the certificate subject located. Must be less than or equal to 2 characters in length."},
			},
		},
		{
			name: "Appends information on newlines to correct nested description",
			input: []string{
				"* `header` - (Optional) Contains additional header parameters for the connection. Each parameter can contain the following:",
				"  * `key` - (Required) The key for the parameter.",
				"  			There is an extra line description here for reasons.",
				"  * `value` - (Required) The value associated with the key. Created and stored in AWS Secrets Manager if is secret.",
				"  * `is_value_secret` - (Optional) Specified whether the value is secret.",
			},
			expected: map[docsPath]*argumentDocs{
				"header":                 {description: "Contains additional header parameters for the connection. Each parameter can contain the following:"},
				"header.key":             {description: "The key for the parameter.\nThere is an extra line description here for reasons."},
				"header.value":           {description: "The value associated with the key. Created and stored in AWS Secrets Manager if is secret."},
				"header.is_value_secret": {description: "Specified whether the value is secret."},
			},
		},
		{
			name: "Finds all levels of nested lists",
			input: []string{
				"* `rules` - (Required) Collection of real time alert rules",
				"  * `type` - (Required) Rule type.",
				"  * `issue_detection_configuration` - (Optional) Configuration for an issue detection rule.",
				"    * `rule_name` - (Required) Rule name.",
				"      * `spec` - A spec for the issue detection configuration rule name.",
				"  * `keyword_match_configuration` - (Optional) Configuration for a keyword match rule.",
				"    * `rule_name` - (Required) Rule name.",
				"    * `keywords` - (Required) Collection of keywords to match.",
				"    * `negate` - (Optional) Negate the rule.",
				"  * `sentiment_configuration` - (Optional) Configuration for a sentiment rule.",
				"    * `rule_name` - (Required) Rule name.",
				"    * `sentiment_type` - (Required) Sentiment type to match.",
				"    * `time_period` - (Optional) Analysis interval.",
				"* `disabled` - (Optional) Disables real time alert rules.",
			},
			expected: map[docsPath]*argumentDocs{
				"rules":                               {description: "Collection of real time alert rules"},
				"rules.type":                          {description: "Rule type."},
				"rules.issue_detection_configuration": {description: "Configuration for an issue detection rule."},
				"rules.issue_detection_configuration.rule_name":      {description: "Rule name."},
				"rules.issue_detection_configuration.rule_name.spec": {description: "A spec for the issue detection configuration rule name."},
				"rules.keyword_match_configuration":                  {description: "Configuration for a keyword match rule."},
				"rules.keyword_match_configuration.rule_name":        {description: "Rule name."},
				"rules.keyword_match_configuration.keywords":         {description: "Collection of keywords to match."},
				"rules.keyword_match_configuration.negate":           {description: "Negate the rule."},
				"rules.sentiment_configuration":                      {description: "Configuration for a sentiment rule."},
				"rules.sentiment_configuration.rule_name":            {description: "Rule name."},
				"rules.sentiment_configuration.sentiment_type":       {description: "Sentiment type to match."},
				"rules.sentiment_configuration.time_period":          {description: "Analysis interval."},
				"disabled": {description: "Disables real time alert rules."},
			},
		},
		{
			name: "Handles different whitespace increments for nesting in either direction",
			input: []string{
				"* `rules` - (Required) Collection of real time alert rules",
				"  * `type` - (Required) Rule type.",
				"     * `rule_name` - (Required) Rule name.",
				"  * `sentiment_configuration` - (Optional) Configuration for a sentiment rule.",
				"* `disabled` - (Optional) Disables real time alert rules.",
			},
			expected: map[docsPath]*argumentDocs{
				"rules":                         {description: "Collection of real time alert rules"},
				"rules.type":                    {description: "Rule type."},
				"rules.type.rule_name":          {description: "Rule name."},
				"rules.sentiment_configuration": {description: "Configuration for a sentiment rule."},
				"disabled":                      {description: "Disables real time alert rules."},
			},
		},
		{
			name: "Parses four-space indents for nested lists",
			input: []string{
				"* `keyword_match_configuration` - (Optional) Configuration for a keyword match rule.",
				"    * `rule_name` - (Required) Rule name.",
				"    * `keywords` - (Required) Collection of keywords to match.",
				"    * `negate` - (Optional) Negate the rule.",
			},
			expected: map[docsPath]*argumentDocs{
				"keyword_match_configuration":           {description: "Configuration for a keyword match rule."},
				"keyword_match_configuration.rule_name": {description: "Rule name."},
				"keyword_match_configuration.keywords":  {description: "Collection of keywords to match."},
				"keyword_match_configuration.negate":    {description: "Negate the rule."},
			},
		},
		{
			name: "Ignores top-level indent for lists",
			input: []string{
				"  * `keyword_match_configuration` - (Optional) Configuration for a keyword match rule.",
				"      * `rule_name` - (Required) Rule name.",
				"      * `keywords` - (Required) Collection of keywords to match.",
				"      * `negate` - (Optional) Negate the rule.",
			},
			expected: map[docsPath]*argumentDocs{
				"keyword_match_configuration":           {description: "Configuration for a keyword match rule."},
				"keyword_match_configuration.rule_name": {description: "Rule name."},
				"keyword_match_configuration.keywords":  {description: "Collection of keywords to match."},
				"keyword_match_configuration.negate":    {description: "Negate the rule."},
			},
		},
		{
			name: "Tracker resets on a new list",
			input: []string{
				"* `keyword_match_configuration` - (Optional) Configuration for a keyword match rule.",
				"  * `rule_name` - (Required) Rule name.",
				"---",
				"  * `keywords` - (Required) Collection of keywords to match.",
				"  * `negate` - (Optional) Negate the rule.",
			},
			expected: map[docsPath]*argumentDocs{
				"keyword_match_configuration":           {description: "Configuration for a keyword match rule."},
				"keyword_match_configuration.rule_name": {description: "Rule name."},
				"keywords":                              {description: "Collection of keywords to match."},
				"negate":                                {description: "Negate the rule."},
			},
		},

		{
			name: "Cleans up tabs",
			input: []string{
				"* `node_pool_config` (Input only) The configuration for the GKE node pool. ",
				"       If specified, Dataproc attempts to create a node pool with the specified shape. ",
				"       If one with the same name already exists, it is verified against all specified fields. ",
				"       If a field differs, the virtual cluster creation will fail.",
			},
			expected: map[docsPath]*argumentDocs{
				"node_pool_config": {
					description: "The configuration for the GKE node pool. \nIf specified, " +
						"Dataproc attempts to create a node pool with the specified shape.\nIf one with the same name " +
						"already exists, it is verified against all specified fields.\nIf a field differs, the virtual " +
						"cluster creation will fail.",
				},
			},
		},
		{
			name: "Parses subblock regexp",
			input: []string{
				"The optional `settings.location_preference` subblock supports:",
				"",
				"* `follow_gae_application` - (Optional) A GAE application whose zone to remain",
				"in. Must be in the same region as this instance.",
				"",
				"* `zone` - (Optional) The preferred compute engine",
				"[zone](https://cloud.google.com/compute/docs/zones?hl=en).",
				"",
				"The optional `settings.maintenance_window` subblock for instances declares a one-hour",
				"[maintenance window](https://cloud.google.com/sql/docs/instance-settings?hl=en#maintenance-window-2ndgen)",
				"when an Instance can automatically restart to apply updates. The maintenance window is specified in UTC time. It supports:",
				"",
				"* `day` - (Optional) Day of week (`1-7`), starting on Monday",
				"",
				"* `hour` - (Optional) Hour of day (`0-23`), ignored if `day` not set",
				"",
				"* `update_track` - (Optional) Receive updates earlier (`canary`) or later",
			},
			expected: map[docsPath]*argumentDocs{
				"settings.location_preference.follow_gae_application": {description: "A GAE application whose zone to remain\nin. Must be in the same region as this instance."},
				"settings.location_preference.zone":                   {description: "The preferred compute engine\n[zone](https://cloud.google.com/compute/docs/zones?hl=en)."},
				"settings.maintenance_window.day":                     {description: "Day of week (`1-7`), starting on Monday"},
				"settings.maintenance_window.hour":                    {description: "Hour of day (`0-23`), ignored if `day` not set"},
				"settings.maintenance_window.update_track":            {description: "Receive updates earlier (`canary`) or later"},
			},
		},
		{
			name: "Parses block regexp",
			input: []string{
				"The optional `settings.location_preference` subblock supports:",
				"",
				"* `follow_gae_application` - (Optional) A GAE application whose zone to remain",
				"in. Must be in the same region as this instance.",
				"",
				"The optional `settings.maintenance_window` block for instances declares a one-hour",
				"[maintenance window](https://cloud.google.com/sql/docs/instance-settings?hl=en#maintenance-window-2ndgen)",
				"when an Instance can automatically restart to apply updates. The maintenance window is specified in UTC time. It supports:",
				"",
				"* `day` - (Optional) Day of week (`1-7`), starting on Monday",
			},
			expected: map[docsPath]*argumentDocs{
				"settings.location_preference.follow_gae_application": {description: "A GAE application whose zone to remain\nin. Must be in the same region as this instance."},
				"settings.maintenance_window.day":                     {description: "Day of week (`1-7`), starting on Monday"},
			},
		},
		{
			name: "Parses sublist regexp",
			input: []string{
				"The optional `settings.location_preference` subblock supports:",
				"",
				"* `follow_gae_application` - (Optional) A GAE application whose zone to remain",
				"in. Must be in the same region as this instance.",
				"",
				"The optional `settings.maintenance_window` sublist for instances declares a one-hour",
				"[maintenance window](https://cloud.google.com/sql/docs/instance-settings?hl=en#maintenance-window-2ndgen)",
				"when an Instance can automatically restart to apply updates. The maintenance window is specified in UTC time. It supports:",
				"",
				"* `day` - (Optional) Day of week (`1-7`), starting on Monday",
			},
			expected: map[docsPath]*argumentDocs{
				"settings.location_preference.follow_gae_application": {description: "A GAE application whose zone to remain\nin. Must be in the same region as this instance."},
				"settings.maintenance_window.day":                     {description: "Day of week (`1-7`), starting on Monday"},
			},
		},

		{
			name: "All caps bullet points are not parsed as TF properties",
			input: []string{
				"* `status` - Status of the AWS PrivateLink connection.",
				"	Returns one of the following values:",
				"	* `AVAILABLE` Atlas created the load balancer and the Private Link Service.",
				"	* `INITIATING` Atlas is creating the network load balancer and VPC endpoint service.",
				"	* `WAITING_FOR_USER` The Atlas network load balancer and VPC endpoint service are created and ready to receive connection requests. " +
					"When you receive this status, create an interface endpoint to continue configuring the AWS PrivateLink connection.",
				"	* `FAILED` A system failure has occurred.",
				"	* `DELETING` The Private Link service is being deleted.",
			},
			expected: map[docsPath]*argumentDocs{
				"status": {description: "Status of the AWS PrivateLink connection.\n" +
					"Returns one of the following values:\n" +
					"* `AVAILABLE` Atlas created the load balancer and the Private Link Service.\n" +
					"* `INITIATING` Atlas is creating the network load balancer and VPC endpoint service.\n" +
					"* `WAITING_FOR_USER` The Atlas network load balancer and VPC endpoint service are created and ready to receive connection requests. " +
					"When you receive this status, create an interface endpoint to continue configuring the AWS PrivateLink connection.\n" +
					"* `FAILED` A system failure has occurred.\n*" +
					" `DELETING` The Private Link service is being deleted."},
			},
		},
		{
			name: "Bullet points in backticks containing any uppercase letters are not parsed as TF properties",
			input: []string{
				"* `status` - Status of the AWS PrivateLink connection.",
				"	Returns one of the following values:",
				"	* `Available` Atlas created the load balancer and the Private Link Service.",
				"	* `Initiating` Atlas is creating the network load balancer and VPC endpoint service.",
			},
			expected: map[docsPath]*argumentDocs{
				"status": {description: "Status of the AWS PrivateLink connection.\n" +
					"Returns one of the following values:\n" +
					"* `Available` Atlas created the load balancer and the Private Link Service.\n" +
					"* `Initiating` Atlas is creating the network load balancer and VPC endpoint service."},
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
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

func TestArgumentRegexAuto(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    []string
		expected autogold.Value
	}{
		{
			name: "newline after bullet",
			input: []string{
				"",
				"The following arguments are supported:",
				"",
				"* `versioning` - (Optional) A state of [versioning](https://www.scaleway.com/en/docs/storage/object/how-to/use-bucket-versioning/). The `versioning` object supports the following:",
				"",
				"    * `enabled` - (Optional) Enable versioning. Once you version-enable a bucket, it can never return to an unversioned state. You can, however, suspend versioning on that bucket.",
				"",
			},
			expected: autogold.Expect(map[docsPath]*argumentDocs{docsPath("versioning.enabled"): {
				description: "Enable versioning. Once you version-enable a bucket, it can never return to an unversioned state. You can, however, suspend versioning on that bucket.",
			}}),
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			ret := entityDocs{
				Arguments: make(map[docsPath]*argumentDocs),
			}
			parseArgReferenceSection(tt.input, &ret)
			tt.expected.Equal(t, ret.Arguments)
		})
	}
}

func TestGetFooterLinks(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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

func TestSplitByMarkdownHeaders(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		level    int
		expected autogold.Value
	}{
		{
			input: `Section1
## H2
Section2
## H2
Section3
`,
			level: 2,
			expected: autogold.Expect([][]string{
				{
					"Section1",
				},
				{
					"## H2",
					"Section2",
				},
				{
					"## H2",
					"Section3",
					"",
				},
			}),
		},
		{
			input: `# hi
h1 content
`,
			level: 1,
			expected: autogold.Expect([][]string{{
				"# hi",
				"h1 content",
				"",
			}}),
		},
		{
			input: `
only 1 section - no headers
`,
			level: 2,
			expected: autogold.Expect([][]string{{
				"",
				"only 1 section - no headers",
				"",
			}}),
		},
		{
			input: `
##

No content for the header
`,
			expected: autogold.Expect([][]string{{
				"",
				"##",
				"",
				"No content for the header",
				"",
			}}),
		},
		{
			input: `
## *emph content*
foo
`,
			level: 2,
			expected: autogold.Expect([][]string{{
				"",
				"## *emph content*",
				"foo",
				"",
			}}),
		},
		{
			input: `## Real header
` + "```" + `
## Fake header
` + "```" + `
## Another real header
content
`,
			level: 2,
			expected: autogold.Expect([][]string{
				{
					"## Real header",
					"```",
					"## Fake header",
					"```",
				},
				{
					"## Another real header",
					"content",
					"",
				},
			}),
		},
		{
			input: readTestFile(t, "alternative_header.md"),
			level: 2,
			expected: autogold.Expect([][]string{
				{
					"---",
					`subcategory: "Batch"`,
					`layout: "azurerm"`,
					`page_title: "Azure Resource Manager: azurerm_batch_account"`,
					"description: |-",
					"  Manages an Azure Batch account.",
					"",
					"---",
					"",
					"# azurerm_batch_account",
					"",
				},
				{
					"## Argument Reference",
					"",
					"An `account_access` block supports the following:",
					"",
					"* `default_action` - (Optional) Specifies the default action for the account access. Possible values are `Allow` and `Deny`. Defaults to `Deny`.",
					"",
					"* alternative rule (we don't parse this correctly now, but we should)",
					"---",
					"",
					"",
				},
			}),
		},
		{
			input: readTestFile(t, "container_app_environment_custom_domain.md"),
			level: 2,
			expected: autogold.Expect([][]string{
				{
					"---",
					`subcategory: "Container Apps"`,
					`layout: "azurerm"`,
					`page_title: "Azure Resource Manager: azurerm_container_app_environment_custom_domain"`,
					"description: |-",
					"  Manages a Container App Environment Custom Domain.",
					"---",
					"",
					"# azurerm_container_app_environment_custom_domain",
					"",
					"Manages a Container App Environment Custom Domain Suffix.",
					"",
				},
				{
					"## Example Usage",
					"",
					"```hcl",
					`resource "azurerm_resource_group" "example" {`,
					`  name     = "example-resources"`,
					`  location = "West Europe"`,
					"}",
					"",
					`resource "azurerm_log_analytics_workspace" "example" {`,
					`  name                = "acctest-01"`,
					"  location            = azurerm_resource_group.example.location",
					"  resource_group_name = azurerm_resource_group.example.name",
					`  sku                 = "PerGB2018"`,
					"  retention_in_days   = 30",
					"}",
					"",
					`resource "azurerm_container_app_environment" "example" {`,
					`  name                       = "my-environment"`,
					"  location                   = azurerm_resource_group.example.location",
					"  resource_group_name        = azurerm_resource_group.example.name",
					"  log_analytics_workspace_id = azurerm_log_analytics_workspace.example.id",
					"}",
					"",
					`resource "azurerm_container_app_environment_custom_domain" "example" {`,
					"  container_app_environment_id = azurerm_container_app_environment.example.id",
					`  certificate_blob_base64      = filebase64("testacc.pfx")`,
					`  certificate_password         = "TestAcc"`,
					`  dns_suffix                   = "acceptancetest.contoso.com"`,
					"}",
					"```",
					"",
				},
				{
					"## Arguments Reference",
					"",
					"The following arguments are supported:",
					"",
					"* `container_app_environment_id` - (Required) The ID of the Container Apps Managed Environment. Changing this forces a new resource to be created.",
					"",
					"* `certificate_blob_base64` - (Required) The bundle of Private Key and Certificate for the Custom DNS Suffix as a base64 encoded PFX or PEM.",
					"",
					"* `certificate_password` - (Required) The password for the Certificate bundle.",
					"",
					"* `dns_suffix` - (Required) Custom DNS Suffix for the Container App Environment.",
				},
				{
					"## Timeouts",
					"",
					"The `timeouts` block allows you to specify [timeouts](https://www.terraform.io/docs/configuration/resources.html#timeouts) for certain actions:",
					"",
					"* `create` - (Defaults to 30 minutes) Used when creating the Container App Environment.",
					"* `update` - (Defaults to 30 minutes) Used when updating the Container App Environment.",
					"* `read` - (Defaults to 5 minutes) Used when retrieving the Container App Environment.",
					"* `delete` - (Defaults to 30 minutes) Used when deleting the Container App Environment.",
					"",
				},
				{
					"## Import",
					"",
					"A Container App Environment Custom Domain Suffix can be imported using the `resource id` of its parent container ontainer App Environment , e.g.",
					"",
					"```shell",
					`terraform import azurerm_container_app_environment_custom_domain.example "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/resGroup1/providers/Microsoft.App/managedEnvironments/myEnvironment"`,
					"```",
					"",
				},
			}),
		},
		{
			input: `## Header 1
content 1
## 
content 2
## header 3
content 3
`,
			level: 2,
			expected: autogold.Expect([][]string{
				{
					"## Header 1",
					"content 1",
					"## ",
					"content 2",
				},
				{
					"## header 3",
					"content 3",
					"",
				},
			}),
		},
		{
			level: 2,
			input: `## Header 1
content
## Header 2
`,
			expected: autogold.Expect([][]string{
				{
					"## Header 1",
					"content",
				},
				{
					"## Header 2",
					"",
				},
			}),
		},
		{
			level: 2,
			input: readTestFile(t, "service_directory_namespace.md"),
			expected: autogold.Expect([][]string{
				{
					"---",
					"# ----------------------------------------------------------------------------",
					"#",
					"#     ***     AUTO GENERATED CODE    ***    Type: MMv1     ***",
					"#",
					"# ----------------------------------------------------------------------------",
					"#",
					"#     This file is automatically generated by Magic Modules and manual",
					"#     changes will be clobbered when the file is regenerated.",
					"#",
					"#     Please read more about how to change this file in",
					"#     .github/CONTRIBUTING.md.",
					"#",
					"# ----------------------------------------------------------------------------",
					`subcategory: "Service Directory"`,
					"description: |-",
					"  A container for `services`.",
					"---",
					"",
					"# google_service_directory_namespace",
					"",
					"A container for `services`. Namespaces allow administrators to group services",
					"together and define permissions for a collection of services.",
					"",
					"To get more information about Namespace, see:",
					"",
					"* [API documentation](https://cloud.google.com/service-directory/docs/reference/rest/v1beta1/projects.locations.namespaces)",
					"* How-to Guides",
					"    * [Configuring a namespace](https://cloud.google.com/service-directory/docs/configuring-service-directory#configuring_a_namespace)",
					"",
					`<div class = "oics-button" style="float: right; margin: 0 0 -15px">`,
					`  <a href="https://console.cloud.google.com/cloudshell/open?cloudshell_git_repo=https%3A%2F%2Fgithub.com%2Fterraform-google-modules%2Fdocs-examples.git&cloudshell_image=gcr.io%2Fcloudshell-images%2Fcloudshell%3Alatest&cloudshell_print=.%2Fmotd&cloudshell_tutorial=.%2Ftutorial.md&cloudshell_working_dir=service_directory_namespace_basic&open_in_editor=main.tf" target="_blank">`,
					`    <img alt="Open in Cloud Shell" src="//gstatic.com/cloudssh/images/open-btn.svg" style="max-height: 44px; margin: 32px auto; max-width: 100%;">`,
					"  </a>",
					"</div>",
				},
				{
					"## Example Usage - Service Directory Namespace Basic",
					"",
					"",
					"```hcl",
					`resource "google_service_directory_namespace" "example" {`,
					"  provider     = google-beta",
					`  namespace_id = "example-namespace"`,
					`  location     = "us-central1"`,
					"",
					"  labels = {",
					`    key = "value"`,
					`    foo = "bar"`,
					"  }",
					"}",
					"```",
					"",
				},
				{
					"## Argument Reference",
					"",
					"The following arguments are supported:",
					"",
					"",
					"* `location` -",
					"  (Required)",
					"  The location for the Namespace.",
					"  A full list of valid locations can be found by running",
					"  `gcloud beta service-directory locations list`.",
					"",
					"* `namespace_id` -",
					"  (Required)",
					"  The Resource ID must be 1-63 characters long, including digits,",
					"  lowercase letters or the hyphen character.",
					"",
					"",
					"- - -",
					"",
					"",
					"* `labels` -",
					"  (Optional)",
					"  Resource labels associated with this Namespace. No more than 64 user",
					"  labels can be associated with a given resource. Label keys and values can",
					"  be no longer than 63 characters.",
					"",
					"  **Note**: This field is non-authoritative, and will only manage the labels present in your configuration.",
					"  Please refer to the field `effective_labels` for all of the labels present on the resource.",
					"",
					"* `project` - (Optional) The ID of the project in which the resource belongs.",
					"    If it is not provided, the provider project is used.",
					"",
					"",
				},
				{
					"## Attributes Reference",
					"",
					"In addition to the arguments listed above, the following computed attributes are exported:",
					"",
					"* `id` - an identifier for the resource with format `{{name}}`",
					"",
					"* `name` -",
					"  The resource name for the namespace",
					"  in the format `projects/*/locations/*/namespaces/*`.",
					"",
					"* `terraform_labels` -",
					"  The combination of labels configured directly on the resource",
					"   and default labels configured on the provider.",
					"",
					"* `effective_labels` -",
					"  All of labels (key/value pairs) present on the resource in GCP, including the labels configured through Terraform, other clients and services.",
					"",
					"",
				},
				{
					"## Timeouts",
					"",
					"This resource provides the following",
					"[Timeouts](https://developer.hashicorp.com/terraform/plugin/sdkv2/resources/retries-and-customizable-timeouts) configuration options:",
					"",
					"- `create` - Default is 20 minutes.",
					"- `update` - Default is 20 minutes.",
					"- `delete` - Default is 20 minutes.",
					"",
				},
				{
					"## Import",
					"",
					"",
					"Namespace can be imported using any of these accepted formats:",
					"",
					"* `projects/{{project}}/locations/{{location}}/namespaces/{{namespace_id}}`",
					"* `{{project}}/{{location}}/{{namespace_id}}`",
					"* `{{location}}/{{namespace_id}}`",
					"",
					"",
					"In Terraform v1.5.0 and later, use an [`import` block](https://developer.hashicorp.com/terraform/language/import) to import Namespace using one of the formats above. For example:",
					"",
					"```tf",
					"import {",
					`  id = "projects/{{project}}/locations/{{location}}/namespaces/{{namespace_id}}"`,
					"  to = google_service_directory_namespace.default",
					"}",
					"```",
					"",
					"When using the [`terraform import` command](https://developer.hashicorp.com/terraform/cli/commands/import), Namespace can be imported using one of the formats above. For example:",
					"",
					"```",
					"$ terraform import google_service_directory_namespace.default projects/{{project}}/locations/{{location}}/namespaces/{{namespace_id}}",
					"$ terraform import google_service_directory_namespace.default {{project}}/{{location}}/{{namespace_id}}",
					"$ terraform import google_service_directory_namespace.default {{location}}/{{namespace_id}}",
					"```",
					"",
				},
			}),
		},
		{
			level: 2,
			input: readTestFile(t, "dataplex_entry_type_iam.md"),
			expected: autogold.Expect([][]string{
				{
					"---",
					"# ----------------------------------------------------------------------------",
					"#",
					"#     ***     AUTO GENERATED CODE    ***    Type: MMv1     ***",
					"#",
					"# ----------------------------------------------------------------------------",
					"#",
					"#     This file is automatically generated by Magic Modules and manual",
					"#     changes will be clobbered when the file is regenerated.",
					"#",
					"#     Please read more about how to change this file in",
					"#     .github/CONTRIBUTING.md.",
					"#",
					"# ----------------------------------------------------------------------------",
					`subcategory: "Dataplex"`,
					"description: |-",
					"  Collection of resources to manage IAM policy for Dataplex EntryType",
					"---",
					"",
					"# IAM policy for Dataplex EntryType",
					"Three different resources help you manage your IAM policy for Dataplex EntryType. Each of these resources serves a different use case:",
					"",
					"* `google_dataplex_entry_type_iam_policy`: Authoritative. Sets the IAM policy for the entrytype and replaces any existing policy already attached.",
					"* `google_dataplex_entry_type_iam_binding`: Authoritative for a given role. Updates the IAM policy to grant a role to a list of members. Other roles within the IAM policy for the entrytype are preserved.",
					"* `google_dataplex_entry_type_iam_member`: Non-authoritative. Updates the IAM policy to grant a role to a new member. Other members for the role for the entrytype are preserved.",
					"",
					"A data source can be used to retrieve policy data in advent you do not need creation",
					"",
					"* `google_dataplex_entry_type_iam_policy`: Retrieves the IAM policy for the entrytype",
					"",
					"~> **Note:** `google_dataplex_entry_type_iam_policy` **cannot** be used in conjunction with `google_dataplex_entry_type_iam_binding` and `google_dataplex_entry_type_iam_member` or they will fight over what your policy should be.",
					"",
					"~> **Note:** `google_dataplex_entry_type_iam_binding` resources **can be** used in conjunction with `google_dataplex_entry_type_iam_member` resources **only if** they do not grant privilege to the same role.",
					"",
					"",
					"",
				},
				{
					"## google_dataplex_entry_type_iam_policy",
					"",
					"```hcl",
					`data "google_iam_policy" "admin" {`,
					"  binding {",
					`    role = "roles/viewer"`,
					"    members = [",
					`      "user:jane@example.com",`,
					"    ]",
					"  }",
					"}",
					"",
					`resource "google_dataplex_entry_type_iam_policy" "policy" {`,
					"  project = google_dataplex_entry_type.test_entry_type_basic.project",
					"  location = google_dataplex_entry_type.test_entry_type_basic.location",
					"  entry_type_id = google_dataplex_entry_type.test_entry_type_basic.entry_type_id",
					"  policy_data = data.google_iam_policy.admin.policy_data",
					"}",
					"```",
					"",
				},
				{
					"## google_dataplex_entry_type_iam_binding",
					"",
					"```hcl",
					`resource "google_dataplex_entry_type_iam_binding" "binding" {`,
					"  project = google_dataplex_entry_type.test_entry_type_basic.project",
					"  location = google_dataplex_entry_type.test_entry_type_basic.location",
					"  entry_type_id = google_dataplex_entry_type.test_entry_type_basic.entry_type_id",
					`  role = "roles/viewer"`,
					"  members = [",
					`    "user:jane@example.com",`,
					"  ]",
					"}",
					"```",
					"",
				},
				{
					"## google_dataplex_entry_type_iam_member",
					"",
					"```hcl",
					`resource "google_dataplex_entry_type_iam_member" "member" {`,
					"  project = google_dataplex_entry_type.test_entry_type_basic.project",
					"  location = google_dataplex_entry_type.test_entry_type_basic.location",
					"  entry_type_id = google_dataplex_entry_type.test_entry_type_basic.entry_type_id",
					`  role = "roles/viewer"`,
					`  member = "user:jane@example.com"`,
					"}",
					"```",
					"",
					"",
				},
				{
					"## Argument Reference",
					"",
					"The following arguments are supported:",
					"",
					"* `location` - (Optional) The location where entry type will be created in.",
					" Used to find the parent resource to bind the IAM policy to. If not specified,",
					"  the value will be parsed from the identifier of the parent resource. If no location is provided in the parent identifier and no",
					"  location is specified, it is taken from the provider configuration.",
					"",
					"* `project` - (Optional) The ID of the project in which the resource belongs.",
					"    If it is not provided, the project will be parsed from the identifier of the parent resource. If no project is provided in the parent identifier and no project is specified, the provider project is used.",
					"",
					"* `member/members` - (Required) Identities that will be granted the privilege in `role`.",
					"  Each entry can have one of the following values:",
					"  * **allUsers**: A special identifier that represents anyone who is on the internet; with or without a Google account.",
					"  * **allAuthenticatedUsers**: A special identifier that represents anyone who is authenticated with a Google account or a service account.",
					"  * **user:{emailid}**: An email address that represents a specific Google account. For example, alice@gmail.com or joe@example.com.",
					"  * **serviceAccount:{emailid}**: An email address that represents a service account. For example, my-other-app@appspot.gserviceaccount.com.",
					"  * **group:{emailid}**: An email address that represents a Google group. For example, admins@example.com.",
					"  * **domain:{domain}**: A G Suite domain (primary, instead of alias) name that represents all the users of that domain. For example, google.com or example.com.",
					`  * **projectOwner:projectid**: Owners of the given project. For example, "projectOwner:my-example-project"`,
					`  * **projectEditor:projectid**: Editors of the given project. For example, "projectEditor:my-example-project"`,
					`  * **projectViewer:projectid**: Viewers of the given project. For example, "projectViewer:my-example-project"`,
					"",
					"* `role` - (Required) The role that should be applied. Only one",
					"    `google_dataplex_entry_type_iam_binding` can be used per role. Note that custom roles must be of the format",
					"    `[projects|organizations]/{parent-name}/roles/{role-name}`.",
					"",
					"* `policy_data` - (Required only by `google_dataplex_entry_type_iam_policy`) The policy data generated by",
					"  a `google_iam_policy` data source.",
					"",
				},
				{
					"## Attributes Reference",
					"",
					"In addition to the arguments listed above, the following computed attributes are",
					"exported:",
					"",
					"* `etag` - (Computed) The etag of the IAM policy.",
					"",
				},
				{
					"## Import",
					"",
					`For all import syntaxes, the "resource in question" can take any of the following forms:`,
					"",
					"* projects/{{project}}/locations/{{location}}/entryTypes/{{entry_type_id}}",
					"* {{project}}/{{location}}/{{entry_type_id}}",
					"* {{location}}/{{entry_type_id}}",
					"* {{entry_type_id}}",
					"",
					"Any variables not passed in the import command will be taken from the provider configuration.",
					"",
					"Dataplex entrytype IAM resources can be imported using the resource identifiers, role, and member.",
					"",
					"IAM member imports use space-delimited identifiers: the resource in question, the role, and the member identity, e.g.",
					"```",
					`$ terraform import google_dataplex_entry_type_iam_member.editor "projects/{{project}}/locations/{{location}}/entryTypes/{{entry_type_id}} roles/viewer user:jane@example.com"`,
					"```",
					"",
					"IAM binding imports use space-delimited identifiers: the resource in question and the role, e.g.",
					"```",
					`$ terraform import google_dataplex_entry_type_iam_binding.editor "projects/{{project}}/locations/{{location}}/entryTypes/{{entry_type_id}} roles/viewer"`,
					"```",
					"",
					"IAM policy imports use the identifier of the resource in question, e.g.",
					"```",
					"$ terraform import google_dataplex_entry_type_iam_policy.editor projects/{{project}}/locations/{{location}}/entryTypes/{{entry_type_id}}",
					"```",
					"",
					"-> **Custom Roles**: If you're importing a IAM resource with a custom role, make sure to use the",
					" full name of the custom role, e.g. `[projects/my-project|organizations/my-org]/roles/my-custom-role`.",
					"",
				},
				{
					"## User Project Overrides",
					"",
					"This resource supports [User Project Overrides](https://registry.terraform.io/providers/hashicorp/google/latest/docs/guides/provider_reference#user_project_override).",
					"",
				},
			}),
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run("", func(t *testing.T) {
			actual := splitByMarkdownHeaders(tt.input, tt.level)
			tt.expected.Equal(t, actual)
		})
	}
}

func TestFixExamplesHeaders(t *testing.T) {
	t.Parallel()
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
		groups := splitByMarkdownHeaders(markdown, 2)
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
		groups := splitByMarkdownHeaders(markdown, 2)
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
	t.Parallel()
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
	t.Parallel()
	runTest := func(input string, expected [][]string) {
		inputSections := splitByMarkdownHeaders(input, 2)
		actual := reformatExamples(inputSections)

		assert.Equal(t, expected, actual)
	}

	// This is a simple use case. We expect no changes to the original doc:
	t.Run("no-op", func(t *testing.T) {
		input := `description

## Example Usage

example usage content`

		expected := [][]string{
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

		runTest(input, expected)
	})

	// This use case demonstrates 2 examples at the same H2 level: a canonical Example
	// Usage and another example for a specific use case. We expect these to be
	// transformed into a canonical H2 "Example Usage" with an H3 for the specific use
	// case.
	//
	// This scenario is common in the pulumi-gcp provider.
	t.Run("multiple-examples-same-level", func(t *testing.T) {
		input := `description

## Example Usage

example usage content

## Example Usage - Specific Case

specific case content`

		expected := [][]string{
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

		runTest(input, expected)
	})

	// This use case demonstrates 2 no canonical Example Usage/basic case and 2
	// specific use cases. We expect the function to add a canonical Example Usage
	// section with the 2 use cases as H3's beneath the canonical section.
	//
	// This scenario is common in the pulumi-gcp provider.
	t.Run("no-canonical-example-header", func(t *testing.T) {
		input := `description

## Example Usage - 1

content 1

## Example Usage - 2

content 2`

		expected := [][]string{
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

		runTest(input, expected)
	})

	t.Run("misformatted-docs-dont-panic", func(t *testing.T) {
		input := `## jetstream_kv_entry Resource
content
### Example
content`

		expected := [][]string{
			{
				"## jetstream_kv_entry Resource",
				"content",
				"### Example",
				"content",
			},
		}

		runTest(input, expected)
	})
}

func TestFormatEntityName(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "'prov_entity'", formatEntityName("prov_entity"))
	assert.Equal(t, "'prov_entity' (aliased or renamed)", formatEntityName("prov_entity_legacy"))
}

func TestHclConversionsToString(t *testing.T) {
	t.Parallel()
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

	buf := bytes.Buffer{}
	_ = outputTemplate.Execute(&buf, data)

	assert.Equal(t, buf.String(), hclConversionsToString(input))
}

func TestParseArgFromMarkdownLine(t *testing.T) {
	t.Parallel()
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
		{"  * `id` - The id of the property", "id", "The id of the property", true},
		{"  * id - The id of the property", "", "", false},
		// In rare cases, we may have a match where description is empty like the following, taken from https://github.com/hashicorp/terraform-provider-aws/blob/main/website/docs/r/spot_fleet_request.html.markdown
		{"* `instance_pools_to_use_count` - (Optional; Default: 1)", "instance_pools_to_use_count", "", true},
		{"", "", "", false},
		{"Most of these arguments directly correspond to the", "", "", false},
	}

	for _, test := range tests {
		parsedLine := parseArgFromMarkdownLine(test.input)
		assert.Equal(t, test.expectedName, parsedLine.name)
		assert.Equal(t, test.expectedDesc, parsedLine.desc)
		assert.Equal(t, test.expectedFound, parsedLine.isFound)
	}
}

func TestParseAttributesReferenceSection(t *testing.T) {
	t.Parallel()
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

func TestParseAttributesReferenceSectionParsesNested(t *testing.T) {
	t.Parallel()
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
		"* `region.zone` - The zone of the Droplet region",
	}, &ret)
	assert.Len(t, ret.Attributes, 5)
}

func TestParseAttributesReferenceSectionParsesNestedOrderAgnostic(t *testing.T) {
	t.Parallel()
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
		"* `region.zone` - The zone of the Droplet region",
		"* `region` - The region of the Droplet",
	}, &ret)
	assert.Len(t, ret.Attributes, 5)
}

func TestParseAttributesReferenceSectionFlattensListAttributes(t *testing.T) {
	t.Parallel()
	ret := entityDocs{
		Arguments:  make(map[docsPath]*argumentDocs),
		Attributes: make(map[string]string),
	}
	expected := entityDocs{
		Attributes: map[string]string{
			"region":      "The region of the Droplet",
			"region.zone": "The zone of the Droplet region",
		},
	}
	parseAttributesReferenceSection([]string{
		"The following attributes are exported:",
		"",
		"* `region` - The region of the Droplet",
		"* `region.0.zone` - The zone of the Droplet region",
	}, &ret)
	assert.Equal(t, expected.Attributes, ret.Attributes)
}

func TestGetNestedBlockName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input    string
		expected []string
	}{
		{"", []string(nil)},
		{"The `website` object supports the following:", []string{"website"}},
		{"The `website` and `pages` objects support the following:", []string{"website", "pages"}},
		{"The optional `settings.location_preference` subblock supports:", []string{"settings.location_preference"}},
		{"The optional `settings.ip_configuration.authorized_networks[]` sublist supports:", []string{"settings.ip_configuration.authorized_networks"}},
		{"#### result_configuration Argument Reference", []string{"result_configuration"}},
		{"### advanced_security_options", []string{"advanced_security_options"}},
		{"### `server_side_encryption`", []string{"server_side_encryption"}},
		{"### Failover Routing Policy", []string{"failover_routing_policy"}},
		{"##### `log_configuration`", []string{"log_configuration"}},
		{"### data_format_conversion_configuration", []string{"data_format_conversion_configuration"}},
		{"#### build_batch_config: restrictions", []string{"build_batch_config.restrictions"}},
		{"#### logs_config: s3_logs", []string{"logs_config.s3_logs"}},
		{"###### S3 Input Format Config", []string{"s3_input_format_config"}},
		// This is a common starting line of base arguments, so should result in nil value:
		{"The following arguments are supported:", []string(nil)},
		{"* `kms_key_id` - ...", []string(nil)},
		{"## Import", []string(nil)},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, getNestedBlockNames(tt.input))
	}
}

func TestOverlayAttributesToAttributes(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skipf("Skippping on windows - tests cases need to be made robust to newline handling")
	}
	tests := []struct {
		input        string
		token        tokens.Token
		expected     string
		expectedFile string
	}{
		{
			input: strings.Join([]string{
				"",
				"Import is supported using the following syntax:", // This is intentionally discarded
				"",
				"```shell", // This has several variations upstream
				"# format is account name | | | privilege | true/false for with_grant_option", // Ensure we remove the shell comment to avoid rendering as H1 in Markdown
				"terraform import snowflake_account_grant.example 'accountName|||USAGE|true'",
				"```",
				"",
			}, "\n"),
			token:    "snowflake:index/accountGrant:AccountGrant",
			expected: "## Import\n\nformat is account name | | | privilege | true/false for with_grant_option\n\n```sh\n$ pulumi import snowflake:index/accountGrant:AccountGrant example 'accountName|||USAGE|true'\n```\n\n",
		},
		{
			input: strings.Join([]string{
				"",
				"Import is supported using the following syntax:", // This is intentionally discarded
				"",
				"```sh", // This has several variations upstream
				"terraform import snowflake_api_integration.example name",
				"```",
				"",
			}, "\n"),
			token:    "snowflake:index/apiIntegration:ApiIntegration",
			expected: "## Import\n\n```sh\n$ pulumi import snowflake:index/apiIntegration:ApiIntegration example name\n```\n\n",
		},
		{
			// The following test case contains a `console` codeblock that should be gone
			input: strings.Join([]string{
				"```console",
				"terraform import snowflake_api_integration.example name",
				"```",
			}, "\n"),
			token:    "snowflake:index/apiIntegration:ApiIntegration",
			expected: "## Import\n\n```sh\n$ pulumi import snowflake:index/apiIntegration:ApiIntegration example name\n```\n\n",
		},
		{
			input: strings.Join([]string{
				"",
				"This is a first line in a multi-line import section",
				"* `{{name}}`",
				"* `{{id}}`",
				"For example:",
				"```sh", // This has several variations upstream
				"terraform import gcp_accesscontextmanager_access_level.example name",
				"```",
				"",
			}, "\n"),
			token:    "gcp:accesscontextmanager/accessLevel:AccessLevel",
			expected: "## Import\n\nThis is a first line in a multi-line import section\n* `{{name}}`\n* `{{id}}`\nFor example:\n```sh\n$ pulumi import gcp:accesscontextmanager/accessLevel:AccessLevel example name\n```\n\n",
		},
		{
			input:        readfile(t, "test_data/parse-imports/accessanalyzer.md"),
			token:        "aws:accessanalyzer/analyzer:Analyzer",
			expectedFile: "test_data/parse-imports/accessanalyzer-expected.md",
		},
		{
			input:        readfile(t, "test_data/parse-imports/aws-iam-role.md"),
			token:        "aws:iam/role:Role",
			expectedFile: "test_data/parse-imports/aws-iam-role-expected.md",
		},
		{
			input:        readfile(t, "test_data/parse-imports/random-id.md"),
			token:        "random:index/id:Id",
			expectedFile: "test_data/parse-imports/random-id-expected.md",
		},
		{
			input:        readfile(t, "test_data/parse-imports/gameliftconfig.md"),
			token:        "aws:gamelift/matchmakingConfiguration:MatchmakingConfiguration",
			expectedFile: "test_data/parse-imports/gameliftconfig-expected.md",
		},
		{
			input:        readfile(t, "test_data/parse-imports/lambdalayer.md"),
			token:        "aws:lambda/layerVersion:LayerVersion",
			expectedFile: "test_data/parse-imports/lambdalayer-expected.md",
		},
		{
			input:        readfile(t, "test_data/parse-imports/auth0pages.md"),
			token:        "auth0/index/pages:Pages",
			expectedFile: "test_data/parse-imports/auth0pages-expected.md",
		},
		{
			input: strings.Join([]string{
				"",
				"### This is a sub-section",
				"",
				"```shell",
				`terraform import auth0_pages.my_pages "22f4f21b-017a-319d-92e7-2291c1ca36c4"`,
				"```",
				"",
			}, "\n"),
			token:    "auth0/index/pages:Pages",
			expected: "## Import\n\n### This is a sub-section\n\n```sh\n$ pulumi import auth0/index/pages:Pages my_pages \"22f4f21b-017a-319d-92e7-2291c1ca36c4\"\n```\n\n",
		},
		{
			input: strings.Join([]string{
				"",
				"### Identity Schema",
				"",
				"#### Required",
				"",
				"- `arn` (String) Amazon Resource Name (ARN) of the load balancer.",
				"",
				"Using `pulumi import`, import LBs using their ARN. For example:",
				"",
				"```console",
				"% terraform import aws_lb.bar arn:aws:elasticloadbalancing:us-west-2:123456789012:loadbalancer/app/my-load-balancer/50dc6c495c0c9188",
				"```",
				"",
			}, "\n"),
			token: "aws:lb/loadBalancer:LoadBalancer",
			expected: "## Import\n\n### Identity Schema\n\n#### Required\n\n" +
				"- `arn` (String) Amazon Resource Name (ARN) of the load balancer.\n\n" +
				"Using `pulumi import`, import LBs using their ARN. For example:\n\n" +
				"```sh\n$ pulumi import aws:lb/loadBalancer:LoadBalancer bar " +
				"arn:aws:elasticloadbalancing:us-west-2:123456789012:loadbalancer/app/my-load-balancer/50dc6c495c0c9188\n```\n\n",
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

func TestParseImports_ImportOnlyFence(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skipf("Skippping on windows - tests cases need to be made robust to newline handling")
	}
	t.Parallel()
	input := readfile(t, "test_data/parse-imports/import-only.md")
	expected := readfile(t, "test_data/parse-imports/import-only-expected.md")
	actual := parseImportsNoOverrides(t, input, "pkg:mod/name:Type")
	assert.Equal(t, expected, actual)
}

func TestParseImports_WithOverride(t *testing.T) {
	t.Parallel()
	parser := tfMarkdownParser{
		info: &mockResource{
			docs: tfbridge.DocInfo{
				ImportDetails: "overridden import details",
			},
		},
	}

	parser.parseImports("this doesn't matter because we are overriding it")

	assert.Equal(t, "## Import\n\noverridden import details", parser.ret.Import)
}

func TestParseImports_EndToEnd(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skipf("Skippping on windows - tests cases need to be made robust to newline handling")
	}

	tests := []struct {
		name         string
		token        tokens.Token
		rawname      string
		markdownName string
		pkg          tokens.Package
		expectedFile string
	}{
		{
			name:         "aws_iam_role",
			token:        "aws:iam/role:Role",
			rawname:      "role",
			markdownName: "aws-iam-role-full.md",
			pkg:          "aws",
			expectedFile: "test_data/parse-imports/aws-iam-role-full-expected.md",
		},
		{
			name:         "random_string",
			token:        "random:index/string:String",
			rawname:      "string",
			markdownName: "random-string-full.md",
			pkg:          "random",
			expectedFile: "test_data/parse-imports/random-string-full-expected.md",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			parser := tfMarkdownParser{
				sink:             mockSink{t},
				info:             &mockResource{token: tt.token},
				kind:             ResourceDocs,
				markdownFileName: tt.markdownName,
				rawname:          tt.rawname,
				infoCtx: infoContext{
					pkg:      tt.pkg,
					language: "nodejs",
					info:     tfbridge.ProviderInfo{Name: string(tt.pkg)},
				},
				editRules: defaultEditRules(),
			}

			input := readfile(t, "test_data/parse-imports/"+tt.markdownName)
			doc, err := parser.parse([]byte(input))
			require.NoError(t, err)
			actual := doc.Import

			if accept {
				writefile(t, tt.expectedFile, []byte(actual))
			}
			expected := readfile(t, tt.expectedFile)
			assert.Equal(t, expected, actual)
		})
	}
}

func ref[T any](t T) *T { return &t }

func TestConvertExamples(t *testing.T) {
	t.Setenv("PULUMI_CONVERT", "0")
	if runtime.GOOS == "windows" {
		t.Skipf("Skipping on windows to avoid failing on incorrect newline handling")
	}

	type testCase struct {
		name string
		path examplePath

		language *Language
	}

	testCases := []testCase{
		{
			name: "wavefront_dashboard_json",
			path: examplePath{
				fullPath: "#/resources/wavefront:index/dashboardJson:DashboardJson",
				token:    "wavefront:index/dashboardJson:DashboardJson",
			},
		},
		{
			name: "golang_wavefront_dashboard_json",
			path: examplePath{
				fullPath: "#/resources/wavefront:index/dashboardJson:DashboardJson",
				token:    "wavefront:index/dashboardJson:DashboardJson",
			},
			language: ref(Golang),
		},
		{
			name: "equinix_fabric_connection",
			path: examplePath{
				fullPath: "#/resources/equinix:fabric:Connection",
				token:    "equinix:fabric:Connection",
			},
		},
		{
			name: "aws_lambda_function",
			path: examplePath{
				fullPath: "#/resources/aws:lambda/function:Function",
				token:    "aws:lambda/function:Function",
			},
		},
		{
			name: "outscale_volume",
			path: examplePath{
				fullPath: "#/resources/outscale:index/volume:Volume",
				token:    "outscale:index/volume:Volume",
			},
		},
		{
			name: "random_string",
			path: examplePath{
				token:    "random:index/randomString:RandomString",
				fullPath: "#/resources/random:index/randomString:RandomString",
			},
		},
		{
			name: "auth0_pages",
			path: examplePath{
				token:    "auth0:index/pages:Pages",
				fullPath: "#/resources/auth0:index/pages:Pages",
			},
		},
		{
			name: "google_service_account_id_token",
			path: examplePath{
				token:    "gcp:serviceaccount/getAccountIdToken:getAccountIdToken",
				fullPath: "#/datasources/gcp:serviceaccount/getAccountIdToken:getAccountIdToken",
			},
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			inmem := afero.NewMemMapFs()
			info := testprovider.ProviderMiniRandom()
			language := Schema
			if tc.language != nil {
				language = *tc.language
			}
			g, err := NewGenerator(GeneratorOptions{
				Package:      info.Name,
				Version:      info.Version,
				Language:     language,
				ProviderInfo: info,
				Root:         inmem,
				Sink: diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{
					Color: colors.Never,
				}),
			})
			assert.NoError(t, err)

			docs, err := os.ReadFile(filepath.Join("test_data", "convertExamples",
				fmt.Sprintf("%s.md", tc.name)))
			require.NoError(t, err)
			result := g.convertExamples(string(docs), tc.path)

			out := filepath.Join("test_data", "convertExamples",
				fmt.Sprintf("%s_out.md", tc.name))
			if accept {
				err = os.WriteFile(out, []byte(result), 0o600)
				require.NoError(t, err)
			}
			expect, err := os.ReadFile(out)
			require.NoError(t, err)
			assert.Equal(t, string(expect), result)
		})
	}
}

func TestConvertExamplesInner(t *testing.T) {
	t.Parallel()
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
	}

	testCases := []testCase{
		{
			name: "code_tagged_json_stays_in_description",
			path: examplePath{
				fullPath: "#/resources/fake:module/resource:Resource",
				token:    "fake:module/resource:Resource",
			},
		},
		{
			name: "inline_fences_are_preserved",
			path: examplePath{
				fullPath: "#/resources/fake:module/resource:Resource",
				token:    "fake:module/resource:Resource",
			},
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			docs, err := os.ReadFile(filepath.Join("test_data", "convertExamples",
				fmt.Sprintf("%s.md", tc.name)))
			require.NoError(t, err)
			result := g.convertExamplesInner(string(docs), tc.path, g.convertHCL, false)

			out := filepath.Join("test_data", "convertExamples",
				fmt.Sprintf("%s_out.md", tc.name))
			if accept {
				err = os.WriteFile(out, []byte(result), 0o600)
				require.NoError(t, err)
			}
			expect, err := os.ReadFile(out)
			require.NoError(t, err)
			assert.Equal(t, string(expect), result)
		})
	}
}

func TestFalsePositiveCodeFences(t *testing.T) {
	t.Parallel()

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

	input := `

# H1

` + "```inner block```" + `

More comments
`

	panicOnUse := func(*Example, string, string, []string) (string, error) {
		panic("Should not be called")
	}
	s := g.convertExamplesInner(input, examplePath{}, panicOnUse, false)

	assert.Equal(t, input, s)
}

func TestSkipLastCodeFenceAfterError(t *testing.T) {
	t.Parallel()

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

	input := `

# H1

Some text

# Examples

` + "```hcl" + `
<invalid>
` + "```" + `

Skipped (we don't want a newline after the last code fence)

` + "```hcl" + `
<skipped>
` + "```"

	panicOnUse := func(_ *Example, code string, _ string, _ []string) (string, error) {
		switch strings.TrimSpace(code) {
		case "<invalid>":
			return "", errors.New("invalid HCL")
		case "<skipped>":
			t.Fatalf("This shouldn't have been called - it should have been skipped")
			fallthrough
		default:
			panic("unknown test: " + code)
		}
	}
	s := g.convertExamplesInner(input, examplePath{}, panicOnUse, false)

	assert.Equal(t, `

# H1

Some text

`, s)
}

func TestFindFencesAndHeaders(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skipf("Skipping on windows to avoid failing on incorrect newline handling")
	}
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
				{start: 1966, end: 2977, headerStart: 1947, language: "terraform"},
				{start: 3001, end: 3224, headerStart: 2982, language: "terraform"},
				{start: 3387, end: 4105, headerStart: 3229, language: "terraform"},
				{start: 4358, end: 5953, headerStart: 4110, language: "terraform"},
				{start: 6622, end: 8041, headerStart: 6421, language: "terraform"},
				{start: 9151, end: 9238, headerStart: 9052, language: "sh"},
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
		{
			name: "indented code fences",
			path: filepath.Join("test_data", "convertExamples",
				"google_service_account_id_token.md"),
			expected: []codeBlock{
				{start: 858, end: 1617, headerStart: 435, language: "hcl"},
				{start: 1897, end: 2245, headerStart: 1624, language: "hcl"},
			},
		},
		{
			name: "handles empty code blocks without panic",
			path: filepath.Join("test_data", "parse-inner-docs",
				"empty-code-blocks.md"),
			expected: []codeBlock{
				{start: 0, end: 113, headerStart: -1, language: "terraform"},
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			testDocBytes, err := os.ReadFile(tc.path)
			require.NoError(t, err)
			testDoc := string(testDocBytes)
			actual := findCodeBlocks([]byte(testDoc))
			assert.Equal(t, tc.expected, actual)
		})
	}
}

func TestExampleGeneration(t *testing.T) {
	t.Parallel()
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

	_, err = g.Generate()
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

		readFileFunc func(string) ([]byte, error)
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

	editRule := func(edit func(string, []byte) ([]byte, error)) func(*testCase) {
		rule := tfbridge.DocsEdit{
			Path: "*",
			Edit: edit,
		}
		return func(tc *testCase) {
			tc.providerInfo.DocRules = &tfbridge.DocRuleInfo{
				EditRules: func(defaults []tfbridge.DocsEdit) []tfbridge.DocsEdit {
					return append([]tfbridge.DocsEdit{rule}, defaults...)
				},
			}
		}
	}

	tests := []testCase{
		test("simple"),
		test("link"),
		test("azurerm-sql-firewall-rule"),
		test("address_map"),
		test("signalfx-log-timeline"),
		test("custom-replaces",
			editRule(func(path string, content []byte) ([]byte, error) {
				assert.Equal(t, "mod1_res1.md", path)
				return bytes.ReplaceAll(content,
					[]byte(`CUSTOM_REPLACES`),
					[]byte(`checking custom replaces`)), nil
			})),
		test("codeblock-header"),
		test("replace-examples",
			func(tc *testCase) {
				tc.readFileFunc = func(name string) ([]byte, error) {
					switch {
					// This test works on windows if and only if we use the correct separator.
					case strings.HasSuffix(name, filepath.Join("docs", "resource", "pkg_mod1_res1.examples.md")):
						return []byte(`## REPLACEMENT TEXT
This should be interpolated in.
`), nil
					default:
						return nil, fmt.Errorf("invalid path %q", name)
					}
				}
				tc.info = &tfbridge.ResourceInfo{Docs: &tfbridge.DocInfo{
					ReplaceExamplesSection: true,
				}}
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
				editRules:    getEditRules(tt.providerInfo.DocRules),
				readFileFunc: tt.readFileFunc,
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

func (m mockSource) getInstallation(info *tfbridge.DocInfo) (*DocFile, error) {
	f, ok := m["index.md"]
	if !ok {
		return nil, nil
	}
	return &DocFile{
		Content:  []byte(f),
		FileName: "index.md",
	}, nil
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
	return r.docs.ReplaceExamplesSection
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

func parseImportsNoOverrides(t *testing.T, input string, token tokens.Token) string {
	t.Helper()
	parser := tfMarkdownParser{
		info: &mockResource{
			token: token,
		},
	}
	parser.parseImports(input)
	return parser.ret.Import
}

func writefile(t *testing.T, file string, bytes []byte) {
	t.Helper()
	err := os.WriteFile(file, bytes, 0o600)
	require.NoError(t, err)
}

func TestFixupImports(t *testing.T) {
	t.Parallel()
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
		{
			text: "In Terraform v1.12.0 and later, the `import` block can be used with the `identity` attribute. For example:\n" +
				"\n" +
				"```terraform" + `
		import {
		to = aws_lb.example
		identity = {
		"arn" = "arn:aws:elasticloadbalancing:us-west-2:123456789012:loadbalancer/app/my-load-balancer/50dc6c495c0c9188"
		}
		}` + "\n```\n" + `post text:
		` + "```yaml" + `
		foo: bar
		` + "```\n",
			expected: `post text:
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

func TestGuessIsHCL(t *testing.T) {
	t.Parallel()
	type testCase struct {
		code string
		hcl  bool
	}
	testCases := []testCase{
		{
			code: `
data "aws_ami_ids" "ubuntu" {
  owners = ["099720109477"]

  filter {
    name   = "name"
    values = ["ubuntu/images/ubuntu-*-*-amd64-server-*"]
  }
}
			`,
			hcl: true,
		},
		{
			code: `
resource "aws_ami" "example" {
  name                = "terraform-example"
  virtualization_type = "hvm"
  root_device_name    = "/dev/xvda"
  imds_support        = "v2.0" # Enforce usage of IMDSv2.
  ebs_block_device {
    device_name = "/dev/xvda"
    snapshot_id = "snap-xxxxxxxx"
    volume_size = 8
  }
}
`,
			hcl: true,
		},
		{
			code: `
    Valid names:
      * amazon-web-services
      * amd
      * nvidia
      * xilinx
`,
			hcl: false,
		},
	}
	for i, tc := range testCases {
		tc := tc
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			actual := guessIsHCL(tc.code)
			assert.Equal(t, tc.hcl, actual)
		})
	}
}

func TestFixupPropertyReference(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name     string
		input    string
		expected string
		ctx      infoContext
	}

	tests := []testCase{
		{
			name:     "resource name with backticks",
			input:    "Use the `random_pet` resource to generate pet names.",
			expected: "Use the <span pulumi-lang-nodejs=\"`random.RandomPet`\" pulumi-lang-dotnet=\"`random.RandomPet`\" pulumi-lang-go=\"`RandomPet`\" pulumi-lang-python=\"`RandomPet`\" pulumi-lang-yaml=\"`random.RandomPet`\" pulumi-lang-java=\"`random.RandomPet`\">`random.RandomPet`</span> resource to generate pet names.",
			ctx: infoContext{
				pkg: "random",
				info: tfbridge.ProviderInfo{
					Resources: map[string]*tfbridge.ResourceInfo{
						"random_pet": {Tok: "random:index/randomPet:RandomPet"},
					},
				},
			},
		},
		{
			name:     "data source name with backticks",
			input:    "Use the `random_id` data source to get random IDs.",
			expected: "Use the <span pulumi-lang-nodejs=\"`random.RandomId`\" pulumi-lang-dotnet=\"`random.RandomId`\" pulumi-lang-go=\"`RandomId`\" pulumi-lang-python=\"`random_id`\" pulumi-lang-yaml=\"`random.RandomId`\" pulumi-lang-java=\"`random.RandomId`\">`random.RandomId`</span> data source to get random IDs.",
			ctx: infoContext{
				pkg: "random",
				info: tfbridge.ProviderInfo{
					DataSources: map[string]*tfbridge.DataSourceInfo{
						"random_id": {Tok: "random:index/randomId:RandomId"},
					},
				},
			},
		},
		{
			name:     "property name with backticks",
			input:    "The `length` property controls the output length.",
			expected: "The <span pulumi-lang-nodejs=\"`length`\" pulumi-lang-dotnet=\"`Length`\" pulumi-lang-go=\"`length`\" pulumi-lang-python=\"`length`\" pulumi-lang-yaml=\"`length`\" pulumi-lang-java=\"`length`\">`length`</span> property controls the output length.",
			ctx: infoContext{
				pkg:  "random",
				info: tfbridge.ProviderInfo{},
			},
		},
		{
			name:     "property name with underscores",
			input:    "The length must also be greater than `min_upper`.",
			expected: "The length must also be greater than <span pulumi-lang-nodejs=\"`minUpper`\" pulumi-lang-dotnet=\"`MinUpper`\" pulumi-lang-go=\"`minUpper`\" pulumi-lang-python=\"`min_upper`\" pulumi-lang-yaml=\"`minUpper`\" pulumi-lang-java=\"`minUpper`\">`min_upper`</span>.",
			ctx: infoContext{
				pkg:  "random",
				info: tfbridge.ProviderInfo{},
			},
		},
		{
			name:     "resource name without backticks",
			input:    "Use random_pet resource to generate pet names.",
			expected: "Use<span pulumi-lang-nodejs=\" random.RandomPet \" pulumi-lang-dotnet=\" random.RandomPet \" pulumi-lang-go=\" RandomPet \" pulumi-lang-python=\" RandomPet \" pulumi-lang-yaml=\" random.RandomPet \" pulumi-lang-java=\" random.RandomPet \"> random.RandomPet </span>resource to generate pet names.",
			ctx: infoContext{
				pkg: "random",
				info: tfbridge.ProviderInfo{
					Resources: map[string]*tfbridge.ResourceInfo{
						"random_pet": {Tok: "random:index/randomPet:RandomPet"},
					},
				},
			},
		},
		{
			name:     "multiple resource references",
			input:    "Use `random_pet` and `random_id` together.",
			expected: "Use <span pulumi-lang-nodejs=\"`random.RandomPet`\" pulumi-lang-dotnet=\"`random.RandomPet`\" pulumi-lang-go=\"`RandomPet`\" pulumi-lang-python=\"`RandomPet`\" pulumi-lang-yaml=\"`random.RandomPet`\" pulumi-lang-java=\"`random.RandomPet`\">`random.RandomPet`</span> and <span pulumi-lang-nodejs=\"`random.RandomId`\" pulumi-lang-dotnet=\"`random.RandomId`\" pulumi-lang-go=\"`RandomId`\" pulumi-lang-python=\"`random_id`\" pulumi-lang-yaml=\"`random.RandomId`\" pulumi-lang-java=\"`random.RandomId`\">`random.RandomId`</span> together.",
			ctx: infoContext{
				pkg: "random",
				info: tfbridge.ProviderInfo{
					Resources: map[string]*tfbridge.ResourceInfo{
						"random_pet": {Tok: "random:index/randomPet:RandomPet"},
					},
					DataSources: map[string]*tfbridge.DataSourceInfo{
						"random_id": {Tok: "random:index/randomId:RandomId"},
					},
				},
			},
		},
		{
			name:     "returns no span for registry docs",
			input:    "Use random_pet resource to generate pet names.",
			expected: "Use random.RandomPet resource to generate pet names.",
			ctx: infoContext{
				pkg: "random",
				info: tfbridge.ProviderInfo{
					Resources: map[string]*tfbridge.ResourceInfo{
						"random_pet": {Tok: "random:index/randomPet:RandomPet"},
					},
				},
				language: RegistryDocs,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			actual := tt.ctx.fixupPropertyReference(tt.input)
			assert.Equal(t, tt.expected, actual)
		})
	}
}
