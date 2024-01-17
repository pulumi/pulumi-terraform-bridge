// Copyright 2016-2022, Pulumi Corporation.
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

package testprovider

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	shimv2 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v2"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"

	testproviderdata "github.com/pulumi/pulumi-terraform-bridge/v3/internal/testprovider"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

// Minified variant of pulumi-aws provider extracted from
// pulumi/pulumi-terraform-bridge#611 issue.
func ProviderRegress611() tfbridge.ProviderInfo {
	awsMod := "index"
	awsPkg := "aws"
	iamMod := "Iam"

	p := shimv2.NewProvider(testproviderdata.ProviderRegress611())

	// awsMember manufactures a type token for the AWS package and
	// the given module, file name, and type.
	awsMember := func(moduleTitle string, fn string, mem string, experimental bool) tokens.ModuleMember {
		if experimental {
			moduleTitle = fmt.Sprintf("x/%s", moduleTitle)
		}

		moduleName := strings.ToLower(moduleTitle)
		if fn != "" {
			moduleName += "/" + fn
		}
		return tokens.ModuleMember(awsPkg + ":" + moduleName + ":" + mem)
	}

	// awsType manufactures a type token for the AWS package and
	// the given module, file name, and type.
	awsType := func(mod string, fn string, typ string) tokens.Type {
		return tokens.Type(awsMember(mod, fn, typ, false))
	}

	// awsResource manufactures a standard resource token given a
	// module and resource name. It automatically uses the AWS
	// package and names the file by simply lower casing the
	// type's first character.
	awsTypeDefaultFile := func(mod string, typ string) tokens.Type {
		fn := string(unicode.ToLower(rune(typ[0]))) + typ[1:]
		return awsType(mod, fn, typ)
	}

	// awsResource manufactures a standard resource token given a
	// module and resource name.
	awsResource := func(mod string, res string) tokens.Type {
		return awsTypeDefaultFile(mod, res)
	}

	// awsDataSource manufactures a standard resource token given
	// a module and resource name. It automatically uses the AWS
	// package and names the file by simply lower casing the data
	// source's first character.
	awsDataSource := func(mod string, res string) tokens.ModuleMember {
		fn := string(unicode.ToLower(rune(res[0]))) + res[1:]
		return awsMember(mod, fn, res, false)
	}

	awsExperimentalDataSource := func(mod string, res string) tokens.ModuleMember {
		fn := string(unicode.ToLower(rune(res[0]))) + res[1:]
		return awsMember(mod, fn, res, true)
	}

	prov := tfbridge.ProviderInfo{
		P:           p,
		Name:        "aws",
		Description: "A Pulumi package for creating and managing Amazon Web Services (AWS) cloud resources.",
		Keywords:    []string{"pulumi", "aws"},
		License:     "Apache-2.0",
		Homepage:    "https://pulumi.io",
		Repository:  "https://github.com/phillipedwards/pulumi-aws",
		Version:     "0.0.2",
		GitHubOrg:   "hashicorp",
		Config: map[string]*tfbridge.SchemaInfo{
			"region": {
				Type: awsTypeDefaultFile(awsMod, "Region"),
				Default: &tfbridge.DefaultInfo{
					EnvVars: []string{"AWS_REGION", "AWS_DEFAULT_REGION"},
				},
			},
			"skip_get_ec2_platforms": {
				Default: &tfbridge.DefaultInfo{
					Value: true,
				},
			},
			"skip_region_validation": {
				Default: &tfbridge.DefaultInfo{
					Value: true,
				},
			},
			"skip_credentials_validation": {
				Default: &tfbridge.DefaultInfo{
					// This is required to now be false! When this is true, we defer
					// the AWS credentials validation check to happen at resource
					// creation time. Although it may be a little slower validating
					// this upfront, we genuinely need to do this to ensure a good
					// user experience. If we don't validate upfront, then we can
					// be in a situation where a user can be waiting for a resource
					// creation timeout (default up to 30mins) to find out that they
					// have not got valid credentials
					Value: false,
				},
			},
			"skip_metadata_api_check": {
				Type: "boolean",
				Default: &tfbridge.DefaultInfo{
					Value: true,
				},
			},
		},
		Resources: map[string]*tfbridge.ResourceInfo{

			// Identity and Access Management (IAM)
			"aws_iam_access_key": {Tok: awsResource(iamMod, "AccessKey")},
			"aws_iam_account_alias": {
				Tok: awsResource(iamMod, "AccountAlias"),
				Fields: map[string]*tfbridge.SchemaInfo{
					"account_alias": {
						CSharpName: "Alias",
					},
				},
			},
			"aws_iam_account_password_policy": {Tok: awsResource(iamMod, "AccountPasswordPolicy")},
			"aws_iam_group_policy": {
				Tok: awsResource(iamMod, "GroupPolicy"),
				Fields: map[string]*tfbridge.SchemaInfo{
					"policy": {
						Type:      "string",
						AltTypes:  []tokens.Type{awsType(iamMod, "documents", "PolicyDocument")},
						Transform: tfbridge.TransformJSONDocument,
					},
				},
			},
			"aws_iam_group":            {Tok: awsResource(iamMod, "Group")},
			"aws_iam_group_membership": {Tok: awsResource(iamMod, "GroupMembership")},
			"aws_iam_group_policy_attachment": {
				Tok: awsResource(iamMod, "GroupPolicyAttachment"),
				Fields: map[string]*tfbridge.SchemaInfo{
					"group": {
						Type:     "string",
						AltTypes: []tokens.Type{awsTypeDefaultFile(iamMod, "Group")},
					},
					"policy_arn": {
						Name: "policyArn",
						Type: awsTypeDefaultFile(awsMod, "ARN"),
					},
				},
				// We pass delete-before-replace: this is a leaf node and a create followed by a delete actually
				// deletes the same attachment we just created, since it is structurally equivalent!
				DeleteBeforeReplace: true,
			},
			"aws_iam_instance_profile": {
				Tok: awsResource(iamMod, "InstanceProfile"),
				Fields: map[string]*tfbridge.SchemaInfo{
					"role": {
						Type:     "string",
						AltTypes: []tokens.Type{awsTypeDefaultFile(iamMod, "Role")},
					},
				},
			},
			"aws_iam_openid_connect_provider": {Tok: awsResource(iamMod, "OpenIdConnectProvider")},
			"aws_iam_policy": {
				Tok: awsResource(iamMod, "Policy"),
				Fields: map[string]*tfbridge.SchemaInfo{
					"policy": {
						Type:       "string",
						AltTypes:   []tokens.Type{awsType(iamMod, "documents", "PolicyDocument")},
						Transform:  tfbridge.TransformJSONDocument,
						CSharpName: "PolicyDocument",
					},
				},
			},
			"aws_iam_policy_attachment": {
				Tok: awsResource(iamMod, "PolicyAttachment"),
				Fields: map[string]*tfbridge.SchemaInfo{
					"users": {
						Elem: &tfbridge.SchemaInfo{
							Type:     "string",
							AltTypes: []tokens.Type{awsTypeDefaultFile(iamMod, "User")},
						},
					},
					"roles": {
						Elem: &tfbridge.SchemaInfo{
							Type:     "string",
							AltTypes: []tokens.Type{awsTypeDefaultFile(iamMod, "Role")},
						},
					},
					"groups": {
						Elem: &tfbridge.SchemaInfo{
							Type:     "string",
							AltTypes: []tokens.Type{awsTypeDefaultFile(iamMod, "Group")},
						},
					},
					"policy_arn": {
						Name: "policyArn",
						Type: awsTypeDefaultFile(awsMod, "ARN"),
					},
				},
				// We pass delete-before-replace: this is a leaf node and a create followed by a delete actually
				// deletes the same attachment we just created, since it is structurally equivalent!
				DeleteBeforeReplace: true,
			},
			"aws_iam_role_policy_attachment": {
				Tok: awsResource(iamMod, "RolePolicyAttachment"),
				Fields: map[string]*tfbridge.SchemaInfo{
					"role": {
						Type:     "string",
						AltTypes: []tokens.Type{awsTypeDefaultFile(iamMod, "Role")},
					},
					"policy_arn": {
						Name: "policyArn",
						Type: awsTypeDefaultFile(awsMod, "ARN"),
					},
				},
				// We pass delete-before-replace: this is a leaf node and a create followed by a delete actually
				// deletes the same attachment we just created, since it is structurally equivalent!
				DeleteBeforeReplace: true,
			},
			"aws_iam_role_policy": {
				Tok: awsResource(iamMod, "RolePolicy"),
				Fields: map[string]*tfbridge.SchemaInfo{
					"role": {
						Type:     "string",
						AltTypes: []tokens.Type{awsTypeDefaultFile(iamMod, "Role")},
					},
					"policy": {
						Type:      "string",
						AltTypes:  []tokens.Type{awsType(iamMod, "documents", "PolicyDocument")},
						Transform: tfbridge.TransformJSONDocument,
					},
				},
			},
			"aws_iam_role": {
				Tok: awsResource(iamMod, "Role"),
				Fields: map[string]*tfbridge.SchemaInfo{
					"name": tfbridge.AutoName("name", 64, "-"),
					"assume_role_policy": {
						Type:      "string",
						AltTypes:  []tokens.Type{awsType(iamMod, "documents", "PolicyDocument")},
						Transform: tfbridge.TransformJSONDocument,
					},
				},
			},
			"aws_iam_saml_provider":         {Tok: awsResource(iamMod, "SamlProvider")},
			"aws_iam_server_certificate":    {Tok: awsResource(iamMod, "ServerCertificate")},
			"aws_iam_service_linked_role":   {Tok: awsResource(iamMod, "ServiceLinkedRole")},
			"aws_iam_user_group_membership": {Tok: awsResource(iamMod, "UserGroupMembership")},
			"aws_iam_user_policy_attachment": {
				Tok: awsResource(iamMod, "UserPolicyAttachment"),
				Fields: map[string]*tfbridge.SchemaInfo{
					"user": {
						Type:     "string",
						AltTypes: []tokens.Type{awsTypeDefaultFile(iamMod, "User")},
					},
					"policy_arn": {
						Name: "policyArn",
						Type: awsTypeDefaultFile(awsMod, "ARN"),
					},
				},
				// We pass delete-before-replace: this is a leaf node and a create followed by a delete actually
				// deletes the same attachment we just created, since it is structurally equivalent!
				DeleteBeforeReplace: true,
			},
			"aws_iam_user_policy": {
				Tok: awsResource(iamMod, "UserPolicy"),
				Fields: map[string]*tfbridge.SchemaInfo{
					"policy": {
						Type:      "string",
						AltTypes:  []tokens.Type{awsType(iamMod, "documents", "PolicyDocument")},
						Transform: tfbridge.TransformJSONDocument,
					},
				},
			},
			"aws_iam_user_ssh_key":                {Tok: awsResource(iamMod, "SshKey")},
			"aws_iam_user":                        {Tok: awsResource(iamMod, "User")},
			"aws_iam_user_login_profile":          {Tok: awsResource(iamMod, "UserLoginProfile")},
			"aws_iam_service_specific_credential": {Tok: awsResource(iamMod, "ServiceSpecificCredential")},
			"aws_iam_signing_certificate":         {Tok: awsResource(iamMod, "SigningCertificate")},
			"aws_iam_virtual_mfa_device":          {Tok: awsResource(iamMod, "VirtualMfaDevice")},
		},
		DataSources: map[string]*tfbridge.DataSourceInfo{
			// AWS
			"aws_arn":                     {Tok: awsDataSource(awsMod, "getArn")},
			"aws_availability_zone":       {Tok: awsDataSource(awsMod, "getAvailabilityZone")},
			"aws_availability_zones":      {Tok: awsDataSource(awsMod, "getAvailabilityZones")},
			"aws_billing_service_account": {Tok: awsDataSource(awsMod, "getBillingServiceAccount")},
			"aws_caller_identity":         {Tok: awsDataSource(awsMod, "getCallerIdentity")},
			"aws_ip_ranges":               {Tok: awsDataSource(awsMod, "getIpRanges")},
			"aws_partition":               {Tok: awsDataSource(awsMod, "getPartition")},
			"aws_region":                  {Tok: awsDataSource(awsMod, "getRegion")},
			"aws_regions":                 {Tok: awsDataSource(awsMod, "getRegions")},
			"aws_default_tags":            {Tok: awsDataSource(awsMod, "getDefaultTags")},
			"aws_service":                 {Tok: awsDataSource(awsMod, "getService")},

			// IAM
			"aws_iam_account_alias":           {Tok: awsDataSource(iamMod, "getAccountAlias")},
			"aws_iam_group":                   {Tok: awsDataSource(iamMod, "getGroup")},
			"aws_iam_instance_profile":        {Tok: awsDataSource(iamMod, "getInstanceProfile")},
			"aws_iam_policy":                  {Tok: awsDataSource(iamMod, "getPolicy")},
			"aws_iam_policy_document":         {Tok: awsExperimentalDataSource(iamMod, "getPolicyDocument")},
			"aws_iam_role":                    {Tok: awsDataSource(iamMod, "getRole")},
			"aws_iam_server_certificate":      {Tok: awsDataSource(iamMod, "getServerCertificate")},
			"aws_iam_user":                    {Tok: awsDataSource(iamMod, "getUser")},
			"aws_iam_users":                   {Tok: awsDataSource(iamMod, "getUsers")},
			"aws_iam_session_context":         {Tok: awsDataSource(iamMod, "getSessionContext")},
			"aws_iam_roles":                   {Tok: awsDataSource(iamMod, "getRoles")},
			"aws_iam_user_ssh_key":            {Tok: awsDataSource(iamMod, "getUserSshKey")},
			"aws_iam_openid_connect_provider": {Tok: awsDataSource(iamMod, "getOpenidConnectProvider")},
			"aws_iam_saml_provider":           {Tok: awsDataSource(iamMod, "getSamlProvider")},
			"aws_iam_instance_profiles":       {Tok: awsDataSource(iamMod, "getInstanceProfiles")},
		},
	}

	prov.SetAutonaming(255, "-")

	for _, r := range prov.Resources {
		// Suppress unresolved ID mapping errors.
		r.ComputeID = func(state resource.PropertyMap) (resource.ID, error) {
			return resource.ID("ID"), nil
		}
	}

	return prov
}
