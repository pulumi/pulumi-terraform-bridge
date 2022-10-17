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
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// Minified variant of pulumi-aws provider extracted from
// pulumi/pulumi-terraform-bridge#611 issue.
func ProviderRegress611() *schema.Provider {
	return &schema.Provider{
		Schema: map[string]*schema.Schema{
			"region": {
				Type:     schema.TypeString,
				Optional: true,
				Description: `The region where AWS operations will take place. Examples
are us-east-1, us-west-2, etc.`,
			},
			"skip_credentials_validation": {
				Type:     schema.TypeBool,
				Optional: true,
				Description: "Skip the credentials validation via STS API. " +
					"Used for AWS API implementations that do not have STS available/implemented.",
			},
			"skip_get_ec2_platforms": {
				Type:     schema.TypeBool,
				Optional: true,
				Description: "Skip getting the supported EC2 platforms. " +
					"Used by users that don't have ec2:DescribeAccountAttributes permissions.",
			},
			"skip_metadata_api_check": {
				Type:     schema.TypeString,
				Optional: true,
				Description: "Skip the AWS Metadata API check. " +
					"Used for AWS API implementations that do not have a metadata api endpoint.",
			},
			"skip_region_validation": {
				Type:     schema.TypeBool,
				Optional: true,
				Description: "Skip static validation of region name. " +
					"Used by users of alternative AWS-like APIs or users w/ access to regions that are not public (yet).",
			},
		},
		ResourcesMap: map[string]*schema.Resource{
			"aws_iam_access_key": {Schema: map[string]*schema.Schema{
				"create_date": {
					Type:     schema.TypeString,
					Computed: true,
				},
				"encrypted_secret": {
					Type:     schema.TypeString,
					Computed: true,
				},
				"encrypted_ses_smtp_password_v4": {
					Type:     schema.TypeString,
					Computed: true,
				},
				"key_fingerprint": {
					Type:     schema.TypeString,
					Computed: true,
				},
				"pgp_key": {
					Type:     schema.TypeString,
					Optional: true,
					ForceNew: true,
				},
				"secret": {
					Type:      schema.TypeString,
					Computed:  true,
					Sensitive: true,
				},
				"ses_smtp_password_v4": {
					Type:      schema.TypeString,
					Computed:  true,
					Sensitive: true,
				},
				"status": {
					Type:     schema.TypeString,
					Optional: true,
					Default:  "Active",
				},
				"user": {
					Type:     schema.TypeString,
					Required: true,
					ForceNew: true,
				},
			}},
			"aws_iam_account_alias": {Schema: map[string]*schema.Schema{"account_alias": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			}}},
			"aws_iam_account_password_policy": {Schema: map[string]*schema.Schema{
				"allow_users_to_change_password": {
					Type:     schema.TypeBool,
					Optional: true,
					Default:  true,
				},
				"expire_passwords": {
					Type:     schema.TypeBool,
					Computed: true,
				},
				"hard_expiry": {
					Type:     schema.TypeBool,
					Optional: true,
					Computed: true,
				},
				"max_password_age": {
					Type:     schema.TypeInt,
					Optional: true,
					Computed: true,
				},
				"minimum_password_length": {
					Type:     schema.TypeInt,
					Optional: true,
					Default:  6,
				},
				"password_reuse_prevention": {
					Type:     schema.TypeInt,
					Optional: true,
					Computed: true,
				},
				"require_lowercase_characters": {
					Type:     schema.TypeBool,
					Optional: true,
					Computed: true,
				},
				"require_numbers": {
					Type:     schema.TypeBool,
					Optional: true,
					Computed: true,
				},
				"require_symbols": {
					Type:     schema.TypeBool,
					Optional: true,
					Computed: true,
				},
				"require_uppercase_characters": {
					Type:     schema.TypeBool,
					Optional: true,
					Computed: true,
				},
			}},
			"aws_iam_group": {Schema: map[string]*schema.Schema{
				"arn": {
					Type:     schema.TypeString,
					Computed: true,
				},
				"name": {
					Type:     schema.TypeString,
					Required: true,
				},
				"path": {
					Type:     schema.TypeString,
					Optional: true,
					Default:  "/",
				},
				"unique_id": {
					Type:     schema.TypeString,
					Computed: true,
				},
			}},
			"aws_iam_group_membership": {Schema: map[string]*schema.Schema{
				"group": {
					Type:     schema.TypeString,
					Required: true,
					ForceNew: true,
				},
				"name": {
					Type:     schema.TypeString,
					Required: true,
					ForceNew: true,
				},
				"users": {
					Type:     schema.TypeSet,
					Required: true,
					Elem:     &schema.Schema{Type: schema.TypeString},
				},
			}},
			"aws_iam_group_policy": {Schema: map[string]*schema.Schema{
				"group": {
					Type:     schema.TypeString,
					Required: true,
					ForceNew: true,
				},
				"name": {
					Type:          schema.TypeString,
					Optional:      true,
					Computed:      true,
					ForceNew:      true,
					ConflictsWith: []string{"name_prefix"},
				},
				"name_prefix": {
					Type:          schema.TypeString,
					Optional:      true,
					ForceNew:      true,
					ConflictsWith: []string{"name"},
				},
				"policy": {
					Type:     schema.TypeString,
					Required: true,
				},
			}},
			"aws_iam_group_policy_attachment": {Schema: map[string]*schema.Schema{
				"group": {
					Type:     schema.TypeString,
					Required: true,
					ForceNew: true,
				},
				"policy_arn": {
					Type:     schema.TypeString,
					Required: true,
					ForceNew: true,
				},
			}},
			"aws_iam_instance_profile": {Schema: map[string]*schema.Schema{
				"arn": {
					Type:     schema.TypeString,
					Computed: true,
				},
				"create_date": {
					Type:     schema.TypeString,
					Computed: true,
				},
				"name": {
					Type:          schema.TypeString,
					Optional:      true,
					Computed:      true,
					ForceNew:      true,
					ConflictsWith: []string{"name_prefix"},
				},
				"name_prefix": {
					Type:          schema.TypeString,
					Optional:      true,
					ForceNew:      true,
					ConflictsWith: []string{"name"},
				},
				"path": {
					Type:     schema.TypeString,
					Optional: true,
					ForceNew: true,
					Default:  "/",
				},
				"role": {
					Type:     schema.TypeString,
					Optional: true,
				},
				"tags": {
					Type:     schema.TypeMap,
					Optional: true,
					Elem:     &schema.Schema{Type: schema.TypeString},
				},
				"tags_all": {
					Type:     schema.TypeMap,
					Computed: true,
					Elem:     &schema.Schema{Type: schema.TypeString},
				},
				"unique_id": {
					Type:     schema.TypeString,
					Computed: true,
				},
			}},
			"aws_iam_openid_connect_provider": {Schema: map[string]*schema.Schema{
				"arn": {
					Type:     schema.TypeString,
					Computed: true,
				},
				"client_id_list": {
					Type:     schema.TypeList,
					Required: true,
					ForceNew: true,
					Elem:     &schema.Schema{Type: schema.TypeString},
				},
				"tags": {
					Type:     schema.TypeMap,
					Optional: true,
					Elem:     &schema.Schema{Type: schema.TypeString},
				},
				"tags_all": {
					Type:     schema.TypeMap,
					Computed: true,
					Elem:     &schema.Schema{Type: schema.TypeString},
				},
				"thumbprint_list": {
					Type:     schema.TypeList,
					Required: true,
					Elem:     &schema.Schema{Type: schema.TypeString},
				},
				"url": {
					Type:     schema.TypeString,
					Required: true,
					ForceNew: true,
				},
			}},
			"aws_iam_policy": {Schema: map[string]*schema.Schema{
				"arn": {
					Type:     schema.TypeString,
					Computed: true,
				},
				"description": {
					Type:     schema.TypeString,
					Optional: true,
					ForceNew: true,
				},
				"name": {
					Type:          schema.TypeString,
					Optional:      true,
					Computed:      true,
					ForceNew:      true,
					ConflictsWith: []string{"name_prefix"},
				},
				"name_prefix": {
					Type:          schema.TypeString,
					Optional:      true,
					ForceNew:      true,
					ConflictsWith: []string{"name"},
				},
				"path": {
					Type:     schema.TypeString,
					Optional: true,
					ForceNew: true,
					Default:  "/",
				},
				"policy": {
					Type:     schema.TypeString,
					Required: true,
				},
				"policy_id": {
					Type:     schema.TypeString,
					Computed: true,
				},
				"tags": {
					Type:     schema.TypeMap,
					Optional: true,
					Elem:     &schema.Schema{Type: schema.TypeString},
				},
				"tags_all": {
					Type:     schema.TypeMap,
					Computed: true,
					Elem:     &schema.Schema{Type: schema.TypeString},
				},
			}},
			"aws_iam_policy_attachment": {Schema: map[string]*schema.Schema{
				"groups": {
					Type:     schema.TypeSet,
					Optional: true,
					Elem:     &schema.Schema{Type: schema.TypeString},
				},
				"name": {
					Type:     schema.TypeString,
					Required: true,
					ForceNew: true,
				},
				"policy_arn": {
					Type:     schema.TypeString,
					Required: true,
					ForceNew: true,
				},
				"roles": {
					Type:     schema.TypeSet,
					Optional: true,
					Elem:     &schema.Schema{Type: schema.TypeString},
				},
				"users": {
					Type:     schema.TypeSet,
					Optional: true,
					Elem:     &schema.Schema{Type: schema.TypeString},
				},
			}},
			"aws_iam_role": {Schema: map[string]*schema.Schema{
				"arn": {
					Type:     schema.TypeString,
					Computed: true,
				},
				"assume_role_policy": {
					Type:     schema.TypeString,
					Required: true,
				},
				"create_date": {
					Type:     schema.TypeString,
					Computed: true,
				},
				"description": {
					Type:     schema.TypeString,
					Optional: true,
				},
				"force_detach_policies": {
					Type:     schema.TypeBool,
					Optional: true,
					Default:  false,
				},
				"inline_policy": {
					Type:     schema.TypeSet,
					Optional: true,
					Computed: true,
					Elem: &schema.Resource{Schema: map[string]*schema.Schema{
						"name": {
							Type:     schema.TypeString,
							Optional: true,
						},
						"policy": {
							Type:     schema.TypeString,
							Optional: true,
						},
					}},
				},
				"managed_policy_arns": {
					Type:     schema.TypeSet,
					Optional: true,
					Computed: true,
					Elem:     &schema.Schema{Type: schema.TypeString},
				},
				"max_session_duration": {
					Type:     schema.TypeInt,
					Optional: true,
					Default:  3600,
				},
				"name": {
					Type:          schema.TypeString,
					Optional:      true,
					Computed:      true,
					ForceNew:      true,
					ConflictsWith: []string{"name_prefix"},
				},
				"name_prefix": {
					Type:          schema.TypeString,
					Optional:      true,
					Computed:      true,
					ForceNew:      true,
					ConflictsWith: []string{"name"},
				},
				"path": {
					Type:     schema.TypeString,
					Optional: true,
					ForceNew: true,
					Default:  "/",
				},
				"permissions_boundary": {
					Type:     schema.TypeString,
					Optional: true,
				},
				"tags": {
					Type:     schema.TypeMap,
					Optional: true,
					Elem:     &schema.Schema{Type: schema.TypeString},
				},
				"tags_all": {
					Type:     schema.TypeMap,
					Computed: true,
					Elem:     &schema.Schema{Type: schema.TypeString},
				},
				"unique_id": {
					Type:     schema.TypeString,
					Computed: true,
				},
			}},
			"aws_iam_role_policy": {Schema: map[string]*schema.Schema{
				"name": {
					Type:          schema.TypeString,
					Optional:      true,
					Computed:      true,
					ForceNew:      true,
					ConflictsWith: []string{"name_prefix"},
				},
				"name_prefix": {
					Type:          schema.TypeString,
					Optional:      true,
					ForceNew:      true,
					ConflictsWith: []string{"name"},
				},
				"policy": {
					Type:     schema.TypeString,
					Required: true,
				},
				"role": {
					Type:     schema.TypeString,
					Required: true,
					ForceNew: true,
				},
			}},
			"aws_iam_role_policy_attachment": {Schema: map[string]*schema.Schema{
				"policy_arn": {
					Type:     schema.TypeString,
					Required: true,
					ForceNew: true,
				},
				"role": {
					Type:     schema.TypeString,
					Required: true,
					ForceNew: true,
				},
			}},
			"aws_iam_saml_provider": {Schema: map[string]*schema.Schema{
				"arn": {
					Type:     schema.TypeString,
					Computed: true,
				},
				"name": {
					Type:     schema.TypeString,
					Required: true,
					ForceNew: true,
				},
				"saml_metadata_document": {
					Type:     schema.TypeString,
					Required: true,
				},
				"tags": {
					Type:     schema.TypeMap,
					Optional: true,
					Elem:     &schema.Schema{Type: schema.TypeString},
				},
				"tags_all": {
					Type:     schema.TypeMap,
					Computed: true,
					Elem:     &schema.Schema{Type: schema.TypeString},
				},
				"valid_until": {
					Type:     schema.TypeString,
					Computed: true,
				},
			}},
			"aws_iam_server_certificate": {Schema: map[string]*schema.Schema{
				"arn": {
					Type:     schema.TypeString,
					Computed: true,
				},
				"certificate_body": {
					Type:     schema.TypeString,
					Required: true,
					ForceNew: true,
				},
				"certificate_chain": {
					Type:     schema.TypeString,
					Optional: true,
					ForceNew: true,
				},
				"expiration": {
					Type:     schema.TypeString,
					Computed: true,
				},
				"name": {
					Type:          schema.TypeString,
					Optional:      true,
					Computed:      true,
					ForceNew:      true,
					ConflictsWith: []string{"name_prefix"},
				},
				"name_prefix": {
					Type:          schema.TypeString,
					Optional:      true,
					ForceNew:      true,
					ConflictsWith: []string{"name"},
				},
				"path": {
					Type:     schema.TypeString,
					Optional: true,
					ForceNew: true,
					Default:  "/",
				},
				"private_key": {
					Type:      schema.TypeString,
					Required:  true,
					ForceNew:  true,
					Sensitive: true,
				},
				"tags": {
					Type:     schema.TypeMap,
					Optional: true,
					Elem:     &schema.Schema{Type: schema.TypeString},
				},
				"tags_all": {
					Type:     schema.TypeMap,
					Computed: true,
					Elem:     &schema.Schema{Type: schema.TypeString},
				},
				"upload_date": {
					Type:     schema.TypeString,
					Computed: true,
				},
			}},
			"aws_iam_service_linked_role": {Schema: map[string]*schema.Schema{
				"arn": {
					Type:     schema.TypeString,
					Computed: true,
				},
				"aws_service_name": {
					Type:     schema.TypeString,
					Required: true,
					ForceNew: true,
				},
				"create_date": {
					Type:     schema.TypeString,
					Computed: true,
				},
				"custom_suffix": {
					Type:     schema.TypeString,
					Optional: true,
					ForceNew: true,
				},
				"description": {
					Type:     schema.TypeString,
					Optional: true,
				},
				"name": {
					Type:     schema.TypeString,
					Computed: true,
				},
				"path": {
					Type:     schema.TypeString,
					Computed: true,
				},
				"tags": {
					Type:     schema.TypeMap,
					Optional: true,
					Elem:     &schema.Schema{Type: schema.TypeString},
				},
				"tags_all": {
					Type:     schema.TypeMap,
					Computed: true,
					Elem:     &schema.Schema{Type: schema.TypeString},
				},
				"unique_id": {
					Type:     schema.TypeString,
					Computed: true,
				},
			}},
			"aws_iam_service_specific_credential": {Schema: map[string]*schema.Schema{
				"service_name": {
					Type:     schema.TypeString,
					Required: true,
					ForceNew: true,
				},
				"service_password": {
					Type:      schema.TypeString,
					Computed:  true,
					Sensitive: true,
				},
				"service_specific_credential_id": {
					Type:     schema.TypeString,
					Computed: true,
				},
				"service_user_name": {
					Type:     schema.TypeString,
					Computed: true,
				},
				"status": {
					Type:     schema.TypeString,
					Optional: true,
					Default:  "Active",
				},
				"user_name": {
					Type:     schema.TypeString,
					Required: true,
					ForceNew: true,
				},
			}},
			"aws_iam_signing_certificate": {Schema: map[string]*schema.Schema{
				"certificate_body": {
					Type:     schema.TypeString,
					Required: true,
					ForceNew: true,
				},
				"certificate_id": {
					Type:     schema.TypeString,
					Computed: true,
				},
				"status": {
					Type:     schema.TypeString,
					Optional: true,
					Default:  "Active",
				},
				"user_name": {
					Type:     schema.TypeString,
					Required: true,
					ForceNew: true,
				},
			}},
			"aws_iam_user": {Schema: map[string]*schema.Schema{
				"arn": {
					Type:     schema.TypeString,
					Computed: true,
				},
				"force_destroy": {
					Type:        schema.TypeBool,
					Optional:    true,
					Default:     false,
					Description: "Delete user even if it has non-Terraform-managed IAM access keys, login profile or MFA devices",
				},
				"name": {
					Type:     schema.TypeString,
					Required: true,
				},
				"path": {
					Type:     schema.TypeString,
					Optional: true,
					Default:  "/",
				},
				"permissions_boundary": {
					Type:     schema.TypeString,
					Optional: true,
				},
				"tags": {
					Type:     schema.TypeMap,
					Optional: true,
					Elem:     &schema.Schema{Type: schema.TypeString},
				},
				"tags_all": {
					Type:     schema.TypeMap,
					Computed: true,
					Elem:     &schema.Schema{Type: schema.TypeString},
				},
				"unique_id": {
					Type:     schema.TypeString,
					Computed: true,
				},
			}},
			"aws_iam_user_group_membership": {Schema: map[string]*schema.Schema{
				"groups": {
					Type:     schema.TypeSet,
					Required: true,
					Elem:     &schema.Schema{Type: schema.TypeString},
				},
				"user": {
					Type:     schema.TypeString,
					Required: true,
					ForceNew: true,
				},
			}},
			"aws_iam_user_login_profile": {Schema: map[string]*schema.Schema{
				"encrypted_password": {
					Type:     schema.TypeString,
					Computed: true,
				},
				"key_fingerprint": {
					Type:     schema.TypeString,
					Computed: true,
				},
				"password": {
					Type:     schema.TypeString,
					Computed: true,
				},
				"password_length": {
					Type:     schema.TypeInt,
					Optional: true,
					ForceNew: true,
					Default:  20,
				},
				"password_reset_required": {
					Type:     schema.TypeBool,
					Optional: true,
					Computed: true,
					ForceNew: true,
				},
				"pgp_key": {
					Type:     schema.TypeString,
					Optional: true,
					ForceNew: true,
				},
				"user": {
					Type:     schema.TypeString,
					Required: true,
					ForceNew: true,
				},
			}},
			"aws_iam_user_policy": {Schema: map[string]*schema.Schema{
				"name": {
					Type:          schema.TypeString,
					Optional:      true,
					Computed:      true,
					ForceNew:      true,
					ConflictsWith: []string{"name_prefix"},
				},
				"name_prefix": {
					Type:          schema.TypeString,
					Optional:      true,
					ForceNew:      true,
					ConflictsWith: []string{"name"},
				},
				"policy": {
					Type:     schema.TypeString,
					Required: true,
				},
				"user": {
					Type:     schema.TypeString,
					Required: true,
					ForceNew: true,
				},
			}},
			"aws_iam_user_policy_attachment": {Schema: map[string]*schema.Schema{
				"policy_arn": {
					Type:     schema.TypeString,
					Required: true,
					ForceNew: true,
				},
				"user": {
					Type:     schema.TypeString,
					Required: true,
					ForceNew: true,
				},
			}},
			"aws_iam_user_ssh_key": {Schema: map[string]*schema.Schema{
				"encoding": {
					Type:     schema.TypeString,
					Required: true,
					ForceNew: true,
				},
				"fingerprint": {
					Type:     schema.TypeString,
					Computed: true,
				},
				"public_key": {
					Type:     schema.TypeString,
					Required: true,
					ForceNew: true,
				},
				"ssh_public_key_id": {
					Type:     schema.TypeString,
					Computed: true,
				},
				"status": {
					Type:     schema.TypeString,
					Optional: true,
					Computed: true,
				},
				"username": {
					Type:     schema.TypeString,
					Required: true,
					ForceNew: true,
				},
			}},
			"aws_iam_virtual_mfa_device": {Schema: map[string]*schema.Schema{
				"arn": {
					Type:     schema.TypeString,
					Computed: true,
				},
				"base_32_string_seed": {
					Type:     schema.TypeString,
					Computed: true,
				},
				"path": {
					Type:     schema.TypeString,
					Optional: true,
					ForceNew: true,
					Default:  "/",
				},
				"qr_code_png": {
					Type:     schema.TypeString,
					Computed: true,
				},
				"tags": {
					Type:     schema.TypeMap,
					Optional: true,
					Elem:     &schema.Schema{Type: schema.TypeString},
				},
				"tags_all": {
					Type:     schema.TypeMap,
					Computed: true,
					Elem:     &schema.Schema{Type: schema.TypeString},
				},
				"virtual_mfa_device_name": {
					Type:     schema.TypeString,
					Required: true,
					ForceNew: true,
				},
			}},
		},
		DataSourcesMap: map[string]*schema.Resource{
			"aws_arn": {Schema: map[string]*schema.Schema{
				"account": {
					Type:     schema.TypeString,
					Computed: true,
				},
				"arn": {
					Type:     schema.TypeString,
					Required: true,
				},
				"partition": {
					Type:     schema.TypeString,
					Computed: true,
				},
				"region": {
					Type:     schema.TypeString,
					Computed: true,
				},
				"resource": {
					Type:     schema.TypeString,
					Computed: true,
				},
				"service": {
					Type:     schema.TypeString,
					Computed: true,
				},
			}},
			"aws_availability_zone": {Schema: map[string]*schema.Schema{
				"all_availability_zones": {
					Type:     schema.TypeBool,
					Optional: true,
				},
				"filter": {
					Type:     schema.TypeSet,
					Optional: true,
					Elem: &schema.Resource{Schema: map[string]*schema.Schema{
						"name": {
							Type:     schema.TypeString,
							Required: true,
						},
						"values": {
							Type:     schema.TypeSet,
							Required: true,
							Elem: &schema.Schema{
								Type: schema.TypeString,
							},
						},
					}},
				},
				"group_name": {
					Type:     schema.TypeString,
					Computed: true,
				},
				"name": {
					Type:     schema.TypeString,
					Optional: true,
					Computed: true,
				},
				"name_suffix": {
					Type:     schema.TypeString,
					Computed: true,
				},
				"network_border_group": {
					Type:     schema.TypeString,
					Computed: true,
				},
				"opt_in_status": {
					Type:     schema.TypeString,
					Computed: true,
				},
				"parent_zone_id": {
					Type:     schema.TypeString,
					Computed: true,
				},
				"parent_zone_name": {
					Type:     schema.TypeString,
					Computed: true,
				},
				"region": {
					Type:     schema.TypeString,
					Computed: true,
				},
				"state": {
					Type:     schema.TypeString,
					Optional: true,
					Computed: true,
				},
				"zone_id": {
					Type:     schema.TypeString,
					Optional: true,
					Computed: true,
				},
				"zone_type": {
					Type:     schema.TypeString,
					Computed: true,
				},
			}},
			"aws_availability_zones": {Schema: map[string]*schema.Schema{
				"all_availability_zones": {
					Type:     schema.TypeBool,
					Optional: true,
				},
				"exclude_names": {
					Type:     schema.TypeSet,
					Optional: true,
					Elem:     &schema.Schema{Type: schema.TypeString},
				},
				"exclude_zone_ids": {
					Type:     schema.TypeSet,
					Optional: true,
					Elem:     &schema.Schema{Type: schema.TypeString},
				},
				"filter": {
					Type:     schema.TypeSet,
					Optional: true,
					Elem: &schema.Resource{Schema: map[string]*schema.Schema{
						"name": {
							Type:     schema.TypeString,
							Required: true,
						},
						"values": {
							Type:     schema.TypeSet,
							Required: true,
							Elem:     &schema.Schema{Type: schema.TypeString},
						},
					}},
				},
				"group_names": {
					Type:     schema.TypeSet,
					Computed: true,
					Elem:     &schema.Schema{Type: schema.TypeString},
				},
				"names": {
					Type:     schema.TypeList,
					Computed: true,
					Elem:     &schema.Schema{Type: schema.TypeString},
				},
				"state": {
					Type:     schema.TypeString,
					Optional: true,
				},
				"zone_ids": {
					Type:     schema.TypeList,
					Computed: true,
					Elem:     &schema.Schema{Type: schema.TypeString},
				},
			}},
			"aws_billing_service_account": {Schema: map[string]*schema.Schema{"arn": {
				Type:     schema.TypeString,
				Computed: true,
			}}},
			"aws_caller_identity": {Schema: map[string]*schema.Schema{
				"account_id": {
					Type:     schema.TypeString,
					Computed: true,
				},
				"arn": {
					Type:     schema.TypeString,
					Computed: true,
				},
				"user_id": {
					Type:     schema.TypeString,
					Computed: true,
				},
			}},
			"aws_default_tags": {Schema: map[string]*schema.Schema{"tags": {
				Type:     schema.TypeMap,
				Optional: true,
				Computed: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
			}}},
			"aws_iam_account_alias": {Schema: map[string]*schema.Schema{"account_alias": {
				Type:     schema.TypeString,
				Computed: true,
			}}},
			"aws_iam_group": {Schema: map[string]*schema.Schema{
				"arn": {
					Type:     schema.TypeString,
					Computed: true,
				},
				"group_id": {
					Type:     schema.TypeString,
					Computed: true,
				},
				"group_name": {
					Type:     schema.TypeString,
					Required: true,
				},
				"path": {
					Type:     schema.TypeString,
					Computed: true,
				},
				"users": {
					Type:     schema.TypeList,
					Computed: true,
					Elem: &schema.Resource{Schema: map[string]*schema.Schema{
						"arn": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"path": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"user_id": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"user_name": {
							Type:     schema.TypeString,
							Computed: true,
						},
					}},
				},
			}},
			"aws_iam_instance_profile": {Schema: map[string]*schema.Schema{
				"arn": {
					Type:     schema.TypeString,
					Computed: true,
				},
				"create_date": {
					Type:     schema.TypeString,
					Computed: true,
				},
				"name": {
					Type:     schema.TypeString,
					Required: true,
				},
				"path": {
					Type:     schema.TypeString,
					Computed: true,
				},
				"role_arn": {
					Type:     schema.TypeString,
					Computed: true,
				},
				"role_id": {
					Type:     schema.TypeString,
					Computed: true,
				},
				"role_name": {
					Type:     schema.TypeString,
					Computed: true,
				},
			}},
			"aws_iam_instance_profiles": {Schema: map[string]*schema.Schema{
				"arns": {
					Type:     schema.TypeSet,
					Computed: true,
					Elem:     &schema.Schema{Type: schema.TypeString},
				},
				"names": {
					Type:     schema.TypeSet,
					Computed: true,
					Elem:     &schema.Schema{Type: schema.TypeString},
				},
				"paths": {
					Type:     schema.TypeSet,
					Computed: true,
					Elem:     &schema.Schema{Type: schema.TypeString},
				},
				"role_name": {
					Type:     schema.TypeString,
					Required: true,
				},
			}},
			"aws_iam_openid_connect_provider": {Schema: map[string]*schema.Schema{
				"arn": {
					Type:     schema.TypeString,
					Optional: true,
					Computed: true,
					ExactlyOneOf: []string{
						"arn",
						"url",
					},
				},
				"client_id_list": {
					Type:     schema.TypeList,
					Computed: true,
					Elem:     &schema.Schema{Type: schema.TypeString},
				},
				"tags": {
					Type:     schema.TypeMap,
					Optional: true,
					Computed: true,
					Elem:     &schema.Schema{Type: schema.TypeString},
				},
				"thumbprint_list": {
					Type:     schema.TypeList,
					Computed: true,
					Elem:     &schema.Schema{Type: schema.TypeString},
				},
				"url": {
					Type:     schema.TypeString,
					Optional: true,
					Computed: true,
					ExactlyOneOf: []string{
						"arn",
						"url",
					},
				},
			}},
			"aws_iam_policy": {Schema: map[string]*schema.Schema{
				"arn": {
					Type:     schema.TypeString,
					Optional: true,
					Computed: true,
					ConflictsWith: []string{
						"name",
						"path_prefix",
					},
				},
				"description": {
					Type:     schema.TypeString,
					Computed: true,
				},
				"name": {
					Type:          schema.TypeString,
					Optional:      true,
					Computed:      true,
					ConflictsWith: []string{"arn"},
				},
				"path": {
					Type:     schema.TypeString,
					Computed: true,
				},
				"path_prefix": {
					Type:          schema.TypeString,
					Optional:      true,
					ConflictsWith: []string{"arn"},
				},
				"policy": {
					Type:     schema.TypeString,
					Computed: true,
				},
				"policy_id": {
					Type:     schema.TypeString,
					Computed: true,
				},
				"tags": {
					Type:     schema.TypeMap,
					Optional: true,
					Computed: true,
					Elem:     &schema.Schema{Type: schema.TypeString},
				},
			}},
			"aws_iam_policy_document": {Schema: map[string]*schema.Schema{
				"json": {
					Type:     schema.TypeString,
					Computed: true,
				},
				"override_json": {
					Type:       schema.TypeString,
					Optional:   true,
					Deprecated: `Use the attribute "override_policy_documents" instead.`,
				},
				"override_policy_documents": {
					Type:     schema.TypeList,
					Optional: true,
					Elem:     &schema.Schema{Type: schema.TypeString},
				},
				"policy_id": {
					Type:     schema.TypeString,
					Optional: true,
				},
				"source_json": {
					Type:       schema.TypeString,
					Optional:   true,
					Deprecated: `Use the attribute "source_policy_documents" instead.`,
				},
				"source_policy_documents": {
					Type:     schema.TypeList,
					Optional: true,
					Elem:     &schema.Schema{Type: schema.TypeString},
				},
				"statement": {
					Type:     schema.TypeList,
					Optional: true,
					Elem: &schema.Resource{Schema: map[string]*schema.Schema{
						"actions": {
							Type:     schema.TypeSet,
							Optional: true,
							Elem:     &schema.Schema{Type: schema.TypeString},
						},
						"condition": {
							Type:     schema.TypeSet,
							Optional: true,
							Elem: &schema.Resource{Schema: map[string]*schema.Schema{
								"test": {
									Type:     schema.TypeString,
									Required: true,
								},
								"values": {
									Type:     schema.TypeList,
									Required: true,
									Elem:     &schema.Schema{Type: schema.TypeString},
								},
								"variable": {
									Type:     schema.TypeString,
									Required: true,
								},
							}},
						},
						"effect": {
							Type:     schema.TypeString,
							Optional: true,
							Default:  "Allow",
						},
						"not_actions": {
							Type:     schema.TypeSet,
							Optional: true,
							Elem:     &schema.Schema{Type: schema.TypeString},
						},
						"not_principals": {
							Type:     schema.TypeSet,
							Optional: true,
							Elem: &schema.Resource{Schema: map[string]*schema.Schema{
								"identifiers": {
									Type:     schema.TypeSet,
									Required: true,
									Elem:     &schema.Schema{Type: schema.TypeString},
								},
								"type": {
									Type:     schema.TypeString,
									Required: true,
								},
							}},
						},
						"not_resources": {
							Type:     schema.TypeSet,
							Optional: true,
							Elem:     &schema.Schema{Type: schema.TypeString},
						},
						"principals": {
							Type:     schema.TypeSet,
							Optional: true,
							Elem: &schema.Resource{Schema: map[string]*schema.Schema{
								"identifiers": {
									Type:     schema.TypeSet,
									Required: true,
									Elem:     &schema.Schema{Type: schema.TypeString},
								},
								"type": {
									Type:     schema.TypeString,
									Required: true,
								},
							}},
						},
						"resources": {
							Type:     schema.TypeSet,
							Optional: true,
							Elem:     &schema.Schema{Type: schema.TypeString},
						},
						"sid": {
							Type:     schema.TypeString,
							Optional: true,
						},
					}},
				},
				"version": {
					Type:     schema.TypeString,
					Optional: true,
					Default:  "2012-10-17",
				},
			}},
			"aws_iam_role": {Schema: map[string]*schema.Schema{
				"arn": {
					Type:     schema.TypeString,
					Computed: true,
				},
				"assume_role_policy": {
					Type:     schema.TypeString,
					Computed: true,
				},
				"create_date": {
					Type:     schema.TypeString,
					Computed: true,
				},
				"description": {
					Type:     schema.TypeString,
					Computed: true,
				},
				"max_session_duration": {
					Type:     schema.TypeInt,
					Computed: true,
				},
				"name": {
					Type:     schema.TypeString,
					Required: true,
				},
				"path": {
					Type:     schema.TypeString,
					Computed: true,
				},
				"permissions_boundary": {
					Type:     schema.TypeString,
					Computed: true,
				},
				"tags": {
					Type:     schema.TypeMap,
					Optional: true,
					Computed: true,
					Elem:     &schema.Schema{Type: schema.TypeString},
				},
				"unique_id": {
					Type:     schema.TypeString,
					Computed: true,
				},
			}},
			"aws_iam_roles": {Schema: map[string]*schema.Schema{
				"arns": {
					Type:     schema.TypeSet,
					Computed: true,
					Elem:     &schema.Schema{Type: schema.TypeString},
				},
				"name_regex": {
					Type:     schema.TypeString,
					Optional: true,
				},
				"names": {
					Type:     schema.TypeSet,
					Computed: true,
					Elem:     &schema.Schema{Type: schema.TypeString},
				},
				"path_prefix": {
					Type:     schema.TypeString,
					Optional: true,
				},
			}},
			"aws_iam_saml_provider": {Schema: map[string]*schema.Schema{
				"arn": {
					Type:     schema.TypeString,
					Required: true,
				},
				"create_date": {
					Type:     schema.TypeString,
					Computed: true,
				},
				"name": {
					Type:     schema.TypeString,
					Computed: true,
				},
				"saml_metadata_document": {
					Type:     schema.TypeString,
					Computed: true,
				},
				"tags": {
					Type:     schema.TypeMap,
					Optional: true,
					Computed: true,
					Elem:     &schema.Schema{Type: schema.TypeString},
				},
				"valid_until": {
					Type:     schema.TypeString,
					Computed: true,
				},
			}},
			"aws_iam_server_certificate": {Schema: map[string]*schema.Schema{
				"arn": {
					Type:     schema.TypeString,
					Computed: true,
				},
				"certificate_body": {
					Type:     schema.TypeString,
					Computed: true,
				},
				"certificate_chain": {
					Type:     schema.TypeString,
					Computed: true,
				},
				"expiration_date": {
					Type:     schema.TypeString,
					Computed: true,
				},
				"latest": {
					Type:     schema.TypeBool,
					Optional: true,
					Default:  false,
				},
				"name": {
					Type:          schema.TypeString,
					Optional:      true,
					Computed:      true,
					ConflictsWith: []string{"name_prefix"},
				},
				"name_prefix": {
					Type:          schema.TypeString,
					Optional:      true,
					ConflictsWith: []string{"name"},
				},
				"path": {
					Type:     schema.TypeString,
					Computed: true,
				},
				"path_prefix": {
					Type:     schema.TypeString,
					Optional: true,
				},
				"upload_date": {
					Type:     schema.TypeString,
					Computed: true,
				},
			}},
			"aws_iam_session_context": {Schema: map[string]*schema.Schema{
				"arn": {
					Type:     schema.TypeString,
					Required: true,
				},
				"issuer_arn": {
					Type:     schema.TypeString,
					Computed: true,
				},
				"issuer_id": {
					Type:     schema.TypeString,
					Computed: true,
				},
				"issuer_name": {
					Type:     schema.TypeString,
					Computed: true,
				},
				"session_name": {
					Type:     schema.TypeString,
					Computed: true,
				},
			}},
			"aws_iam_user": {Schema: map[string]*schema.Schema{
				"arn": {
					Type:     schema.TypeString,
					Computed: true,
				},
				"path": {
					Type:     schema.TypeString,
					Computed: true,
				},
				"permissions_boundary": {
					Type:     schema.TypeString,
					Computed: true,
				},
				"tags": {
					Type:     schema.TypeMap,
					Optional: true,
					Computed: true,
					Elem:     &schema.Schema{Type: schema.TypeString},
				},
				"user_id": {
					Type:     schema.TypeString,
					Computed: true,
				},
				"user_name": {
					Type:     schema.TypeString,
					Required: true,
				},
			}},
			"aws_iam_user_ssh_key": {Schema: map[string]*schema.Schema{
				"encoding": {
					Type:     schema.TypeString,
					Required: true,
				},
				"fingerprint": {
					Type:     schema.TypeString,
					Computed: true,
				},
				"public_key": {
					Type:     schema.TypeString,
					Computed: true,
				},
				"ssh_public_key_id": {
					Type:     schema.TypeString,
					Required: true,
				},
				"status": {
					Type:     schema.TypeString,
					Computed: true,
				},
				"username": {
					Type:     schema.TypeString,
					Required: true,
				},
			}},
			"aws_iam_users": {Schema: map[string]*schema.Schema{
				"arns": {
					Type:     schema.TypeSet,
					Computed: true,
					Elem:     &schema.Schema{Type: schema.TypeString},
				},
				"name_regex": {
					Type:     schema.TypeString,
					Optional: true,
				},
				"names": {
					Type:     schema.TypeSet,
					Computed: true,
					Elem:     &schema.Schema{Type: schema.TypeString},
				},
				"path_prefix": {
					Type:     schema.TypeString,
					Optional: true,
				},
			}},
			"aws_ip_ranges": {Schema: map[string]*schema.Schema{
				"cidr_blocks": {
					Type:     schema.TypeList,
					Computed: true,
					Elem:     &schema.Schema{Type: schema.TypeString},
				},
				"create_date": {
					Type:     schema.TypeString,
					Computed: true,
				},
				"ipv6_cidr_blocks": {
					Type:     schema.TypeList,
					Computed: true,
					Elem:     &schema.Schema{Type: schema.TypeString},
				},
				"regions": {
					Type:     schema.TypeSet,
					Optional: true,
					Elem:     &schema.Schema{Type: schema.TypeString},
				},
				"services": {
					Type:     schema.TypeSet,
					Required: true,
					Elem:     &schema.Schema{Type: schema.TypeString},
				},
				"sync_token": {
					Type:     schema.TypeInt,
					Computed: true,
				},
				"url": {
					Type:     schema.TypeString,
					Optional: true,
					Default:  "https://ip-ranges.amazonaws.com/ip-ranges.json",
				},
			}},
			"aws_partition": {Schema: map[string]*schema.Schema{
				"dns_suffix": {
					Type:     schema.TypeString,
					Computed: true,
				},
				"partition": {
					Type:     schema.TypeString,
					Computed: true,
				},
				"reverse_dns_prefix": {
					Type:     schema.TypeString,
					Computed: true,
				},
			}},
			"aws_region": {Schema: map[string]*schema.Schema{
				"description": {
					Type:     schema.TypeString,
					Computed: true,
				},
				"endpoint": {
					Type:     schema.TypeString,
					Optional: true,
					Computed: true,
				},
				"name": {
					Type:     schema.TypeString,
					Optional: true,
					Computed: true,
				},
			}},
			"aws_regions": {Schema: map[string]*schema.Schema{
				"all_regions": {
					Type:     schema.TypeBool,
					Optional: true,
				},
				"filter": {
					Type:     schema.TypeSet,
					Optional: true,
					Elem: &schema.Resource{Schema: map[string]*schema.Schema{
						"name": {
							Type:     schema.TypeString,
							Required: true,
						},
						"values": {
							Type:     schema.TypeList,
							Required: true,
							Elem:     &schema.Schema{Type: schema.TypeString},
						},
					}},
				},
				"names": {
					Type:     schema.TypeSet,
					Computed: true,
					Elem:     &schema.Schema{Type: schema.TypeString},
				},
			}},
			"aws_service": {Schema: map[string]*schema.Schema{
				"dns_name": {
					Type:     schema.TypeString,
					Optional: true,
					Computed: true,
					ExactlyOneOf: []string{
						"dns_name",
						"reverse_dns_name",
						"service_id",
					},
				},
				"partition": {
					Type:     schema.TypeString,
					Computed: true,
				},
				"region": {
					Type:     schema.TypeString,
					Optional: true,
					Computed: true,
					ConflictsWith: []string{
						"dns_name",
						"reverse_dns_name",
					},
				},
				"reverse_dns_name": {
					Type:     schema.TypeString,
					Optional: true,
					Computed: true,
					ExactlyOneOf: []string{
						"dns_name",
						"reverse_dns_name",
						"service_id",
					},
				},
				"reverse_dns_prefix": {
					Type:     schema.TypeString,
					Optional: true,
					Computed: true,
					ConflictsWith: []string{
						"dns_name",
						"reverse_dns_name",
					},
				},
				"service_id": {
					Type:     schema.TypeString,
					Optional: true,
					Computed: true,
					ExactlyOneOf: []string{
						"dns_name",
						"reverse_dns_name",
						"service_id",
					},
				},
				"supported": {
					Type:     schema.TypeBool,
					Computed: true,
				},
			}},
		},
	}
}
