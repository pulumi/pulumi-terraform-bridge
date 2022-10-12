package testprovider

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	shimv2 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v2"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

// Minified variant of pulumi-aws provider extracted from
// pulumi/pulumi-terraform-bridge#611 issue.
func ProviderRegress611() tfbridge.ProviderInfo {
	awsMod := "index"
	awsPkg := "aws"
	iamMod := "Iam"

	tfProvider := &schema.Provider{
		Schema: map[string]*schema.Schema{
			"region": {
				Type:     schema.TypeString,
				Optional: true,
				Description: `The region where AWS operations will take place. Examples
are us-east-1, us-west-2, etc.`,
			},
			"skip_credentials_validation": {
				Type:        schema.TypeBool,
				Optional:    true,
				Description: "Skip the credentials validation via STS API. Used for AWS API implementations that do not have STS available/implemented.",
			},
			"skip_get_ec2_platforms": {
				Type:        schema.TypeBool,
				Optional:    true,
				Description: "Skip getting the supported EC2 platforms. Used by users that don't have ec2:DescribeAccountAttributes permissions.",
			},
			"skip_metadata_api_check": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Skip the AWS Metadata API check. Used for AWS API implementations that do not have a metadata api endpoint.",
			},
			"skip_region_validation": {
				Type:        schema.TypeBool,
				Optional:    true,
				Description: "Skip static validation of region name. Used by users of alternative AWS-like APIs or users w/ access to regions that are not public (yet).",
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

	p := shimv2.NewProvider(tfProvider)

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

	return prov
}
