package sdkv2

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"

	"github.com/pulumi/pulumi-terraform-bridge/v3/internal/testprovider"
)

var awsSSMParameterSchema = &schema.Resource{
	Schema: map[string]*schema.Schema{
		"name": {
			Type:     schema.TypeString,
			Required: true,
			ForceNew: true,
		},
		"description": {
			Type:     schema.TypeString,
			Optional: true,
		},
		"tier": {
			Type:     schema.TypeString,
			Optional: true,
			Computed: true,
		},
		"type": {
			Type:     schema.TypeString,
			Required: true,
		},
		"value": {
			Type:      schema.TypeString,
			Required:  true,
			Sensitive: true,
		},
		"arn": {
			Type:     schema.TypeString,
			Optional: true,
			Computed: true,
		},
		"key_id": {
			Type:     schema.TypeString,
			Optional: true,
			Computed: true,
		},
		"data_type": {
			Type:     schema.TypeString,
			Optional: true,
			Computed: true,
		},
		"overwrite": {
			Type:     schema.TypeBool,
			Optional: true,
		},
		"allowed_pattern": {
			Type:     schema.TypeString,
			Optional: true,
		},
		"version": {
			Type:     schema.TypeInt,
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
	},
}

var auth0TenantSchema = &schema.Resource{
	Schema: map[string]*schema.Schema{
		"change_password": {
			Type:     schema.TypeList,
			Optional: true,
			MaxItems: 1,
			Computed: true,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"enabled": {
						Type:     schema.TypeBool,
						Required: true,
					},
					"html": {
						Type:     schema.TypeString,
						Required: true,
					},
				},
			},
		},
		"guardian_mfa_page": {
			Type:     schema.TypeList,
			Optional: true,
			MaxItems: 1,
			Computed: true,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"enabled": {
						Type:     schema.TypeBool,
						Required: true,
					},
					"html": {
						Type:     schema.TypeString,
						Required: true,
					},
				},
			},
		},
		"default_audience": {
			Type:     schema.TypeString,
			Optional: true,
			Computed: true,
		},
		"default_directory": {
			Type:     schema.TypeString,
			Optional: true,
			Computed: true,
		},
		"error_page": {
			Type:     schema.TypeList,
			Optional: true,
			Computed: true,
			MaxItems: 1,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"html": {
						Type:     schema.TypeString,
						Required: true,
					},
					"show_log_link": {
						Type:     schema.TypeBool,
						Required: true,
					},
					"url": {
						Type:     schema.TypeString,
						Required: true,
					},
				},
			},
		},
		"friendly_name": {
			Type:     schema.TypeString,
			Optional: true,
			Computed: true,
		},
		"picture_url": {
			Type:     schema.TypeString,
			Optional: true,
			Computed: true,
		},
		"support_email": {
			Type:     schema.TypeString,
			Optional: true,
			Computed: true,
		},
		"support_url": {
			Type:     schema.TypeString,
			Optional: true,
			Computed: true,
		},
		"allowed_logout_urls": {
			Type:     schema.TypeList,
			Elem:     &schema.Schema{Type: schema.TypeString},
			Optional: true,
			Computed: true,
		},
		"sandbox_version": {
			Type:     schema.TypeString,
			Optional: true,
			Computed: true,
		},
		"session_lifetime": {
			Type:     schema.TypeFloat,
			Optional: true,
			Default:  168,
		},
		"idle_session_lifetime": {
			Type:     schema.TypeFloat,
			Optional: true,
			Default:  72,
		},
		"enabled_locales": {
			Type:     schema.TypeList,
			Elem:     &schema.Schema{Type: schema.TypeString},
			Optional: true,
			Computed: true,
		},
		"flags": {
			Type:     schema.TypeList,
			Optional: true,
			Computed: true,
			MaxItems: 1,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"enable_client_connections": {
						Type:     schema.TypeBool,
						Optional: true,
						Computed: true,
					},
					"enable_apis_section": {
						Type:     schema.TypeBool,
						Optional: true,
						Computed: true,
					},
					"enable_pipeline2": {
						Type:     schema.TypeBool,
						Optional: true,
						Computed: true,
					},
					"enable_dynamic_client_registration": {
						Type:     schema.TypeBool,
						Optional: true,
						Computed: true,
					},
					"enable_custom_domain_in_emails": {
						Type:     schema.TypeBool,
						Optional: true,
						Computed: true,
					},
					"universal_login": {
						Type:     schema.TypeBool,
						Optional: true,
						Computed: true,
					},
					"enable_legacy_logs_search_v2": {
						Type:     schema.TypeBool,
						Optional: true,
						Computed: true,
					},
					"disable_clickjack_protection_headers": {
						Type:     schema.TypeBool,
						Optional: true,
						Computed: true,
					},
					"enable_public_signup_user_exists_error": {
						Type:     schema.TypeBool,
						Optional: true,
						Computed: true,
					},
					"use_scope_descriptions_for_consent": {
						Type:     schema.TypeBool,
						Optional: true,
						Computed: true,
					},
					"allow_legacy_delegation_grant_types": {
						Type:     schema.TypeBool,
						Optional: true,
						Computed: true,
					},
					"allow_legacy_ro_grant_types": {
						Type:     schema.TypeBool,
						Optional: true,
						Computed: true,
					},
					"allow_legacy_tokeninfo_endpoint": {
						Type:     schema.TypeBool,
						Optional: true,
						Computed: true,
					},
					"enable_legacy_profile": {
						Type:     schema.TypeBool,
						Optional: true,
						Computed: true,
					},
					"enable_idtoken_api2": {
						Type:     schema.TypeBool,
						Optional: true,
						Computed: true,
					},
					"no_disclose_enterprise_connections": {
						Type:     schema.TypeBool,
						Optional: true,
						Computed: true,
					},
					"disable_management_api_sms_obfuscation": {
						Type:     schema.TypeBool,
						Optional: true,
						Computed: true,
					},
					"enable_adfs_waad_email_verification": {
						Type:     schema.TypeBool,
						Optional: true,
						Computed: true,
					},
					"revoke_refresh_token_grant": {
						Type:     schema.TypeBool,
						Optional: true,
						Computed: true,
					},
					"dashboard_log_streams_next": {
						Type:     schema.TypeBool,
						Optional: true,
						Computed: true,
					},
					"dashboard_insights_view": {
						Type:     schema.TypeBool,
						Optional: true,
						Computed: true,
					},
					"disable_fields_map_fix": {
						Type:     schema.TypeBool,
						Optional: true,
						Computed: true,
					},
				},
			},
		},
		"universal_login": {
			Type:     schema.TypeList,
			Optional: true,
			Computed: true,
			MaxItems: 1,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"colors": {
						Type:     schema.TypeList,
						Optional: true,
						MaxItems: 1,
						Elem: &schema.Resource{
							Schema: map[string]*schema.Schema{
								"primary": {
									Type:     schema.TypeString,
									Optional: true,
									Computed: true,
								},
								"page_background": {
									Type:     schema.TypeString,
									Optional: true,
									Computed: true,
								},
							},
						},
					},
				},
			},
		},
		"default_redirection_uri": {
			Type:     schema.TypeString,
			Optional: true,
			Computed: true,
		},
		"session_cookie": {
			Type:     schema.TypeList,
			Optional: true,
			Computed: true,
			MaxItems: 1,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"mode": {
						Type:        schema.TypeString,
						Optional:    true,
						Description: "Behavior of tenant session cookie. Accepts either \"persistent\" or \"non-persistent\"",
					},
				},
			},
		},
	},
}

func TestMakeResourceRawConfig(t *testing.T) {
	cases := []struct {
		name     string
		schema   *schema.Resource
		config   map[string]interface{}
		expected cty.Value
		skip     bool
	}{
		{
			// Equivalent TF config:
			//
			// resource "aws_ssm_parameter" "parameter" {
			//     name = "/someParam"
			//     type = "String"
			//     value = "foo"
			// }
			name:   "AWS SSM Parameter",
			schema: awsSSMParameterSchema,
			config: map[string]interface{}{
				"name":  "/someParam",
				"type":  "String",
				"value": "foo",
			},
			expected: cty.ObjectVal(map[string]cty.Value{
				"allowed_pattern": cty.NullVal(cty.String),
				"tier":            cty.NullVal(cty.String),
				"version":         cty.NullVal(cty.Number),
				"data_type":       cty.NullVal(cty.String),
				"key_id":          cty.NullVal(cty.String),
				"name":            cty.StringVal("/someParam"),
				"overwrite":       cty.NullVal(cty.Bool),
				"tags_all":        cty.NullVal(cty.Map(cty.String)),
				"tags":            cty.NullVal(cty.Map(cty.String)),
				"type":            cty.StringVal("String"),
				"arn":             cty.NullVal(cty.String),
				"id":              cty.NullVal(cty.String),
				"description":     cty.NullVal(cty.String),
				"value":           cty.StringVal("foo"),
			}),
		},
		{
			// Equivalent TF config:
			//
			// resource "auth0_tenant" "tenant" {
			//     friendly_name = "Tenant Name"
			// }
			name:   "Auth0 Tenant",
			schema: auth0TenantSchema,
			config: map[string]interface{}{
				"friendly_name": "Tenant Name",
			},
			expected: cty.ObjectVal(map[string]cty.Value{
				"allowed_logout_urls": cty.NullVal(cty.List(cty.String)),
				"change_password": cty.NullVal(cty.List(cty.Object(map[string]cty.Type{
					"enabled": cty.Bool,
					"html":    cty.String,
				}))),
				"default_audience":        cty.NullVal(cty.String),
				"default_directory":       cty.NullVal(cty.String),
				"default_redirection_uri": cty.NullVal(cty.String),
				"enabled_locales":         cty.NullVal(cty.List(cty.String)),
				"error_page": cty.NullVal(cty.List(cty.Object(map[string]cty.Type{
					"html":          cty.String,
					"show_log_link": cty.Bool,
					"url":           cty.String,
				}))),
				"flags": cty.NullVal(cty.List(cty.Object(map[string]cty.Type{
					"allow_legacy_delegation_grant_types":    cty.Bool,
					"allow_legacy_ro_grant_types":            cty.Bool,
					"allow_legacy_tokeninfo_endpoint":        cty.Bool,
					"dashboard_insights_view":                cty.Bool,
					"dashboard_log_streams_next":             cty.Bool,
					"disable_clickjack_protection_headers":   cty.Bool,
					"disable_fields_map_fix":                 cty.Bool,
					"disable_management_api_sms_obfuscation": cty.Bool,
					"enable_adfs_waad_email_verification":    cty.Bool,
					"enable_apis_section":                    cty.Bool,
					"enable_client_connections":              cty.Bool,
					"enable_custom_domain_in_emails":         cty.Bool,
					"enable_dynamic_client_registration":     cty.Bool,
					"enable_idtoken_api2":                    cty.Bool,
					"enable_legacy_logs_search_v2":           cty.Bool,
					"enable_legacy_profile":                  cty.Bool,
					"enable_pipeline2":                       cty.Bool,
					"enable_public_signup_user_exists_error": cty.Bool,
					"no_disclose_enterprise_connections":     cty.Bool,
					"revoke_refresh_token_grant":             cty.Bool,
					"universal_login":                        cty.Bool,
					"use_scope_descriptions_for_consent":     cty.Bool,
				}))),
				"friendly_name": cty.StringVal("Tenant Name"),
				"guardian_mfa_page": cty.NullVal(cty.List(cty.Object(map[string]cty.Type{
					"enabled": cty.Bool,
					"html":    cty.String,
				}))),
				"id":                    cty.NullVal(cty.String),
				"idle_session_lifetime": cty.NullVal(cty.Number),
				"picture_url":           cty.NullVal(cty.String),
				"sandbox_version":       cty.NullVal(cty.String),
				"session_cookie": cty.NullVal(cty.List(cty.Object(map[string]cty.Type{
					"mode": cty.String,
				}))),
				"session_lifetime": cty.NullVal(cty.Number),
				"support_email":    cty.NullVal(cty.String),
				"support_url":      cty.NullVal(cty.String),
				"universal_login": cty.NullVal(cty.List(cty.Object(map[string]cty.Type{
					"colors": cty.List(cty.Object(map[string]cty.Type{
						"page_background": cty.String,
						"primary":         cty.String,
					})),
				}))),
			}),
		},
		{
			// Equivalent TF config:
			//
			// resource "auth0_tenant" "tenant" {
			//     friendly_name = "Tenant Name"
			//
			//     flags {
			//         universal_login = true
			//     }
			//
			//     universal_login {
			//         colors {
			//             primary = "#3385ff"
			//             page_background = "#000000"
			//         }
			//     }
			// }
			name:   "Auth0 Tenant With Flags",
			schema: auth0TenantSchema,
			config: map[string]interface{}{
				"friendly_name": "Tenant Name",
				"flags": []interface{}{
					map[string]interface{}{
						"universal_login": true,
					},
				},
				"universal_login": []interface{}{
					map[string]interface{}{
						"colors": []interface{}{
							map[string]interface{}{
								"primary":         "#3385ff",
								"page_background": "#000000",
							},
						},
					},
				},
			},
			expected: cty.ObjectVal(map[string]cty.Value{
				"allowed_logout_urls": cty.NullVal(cty.List(cty.String)),
				"change_password": cty.NullVal(cty.List(cty.Object(map[string]cty.Type{
					"enabled": cty.Bool,
					"html":    cty.String,
				}))),
				"default_audience":        cty.NullVal(cty.String),
				"default_directory":       cty.NullVal(cty.String),
				"default_redirection_uri": cty.NullVal(cty.String),
				"enabled_locales":         cty.NullVal(cty.List(cty.String)),
				"error_page": cty.NullVal(cty.List(cty.Object(map[string]cty.Type{
					"html":          cty.String,
					"show_log_link": cty.Bool,
					"url":           cty.String,
				}))),
				"flags": cty.ListVal([]cty.Value{
					cty.ObjectVal(map[string]cty.Value{
						"allow_legacy_delegation_grant_types":    cty.NullVal(cty.Bool),
						"allow_legacy_ro_grant_types":            cty.NullVal(cty.Bool),
						"allow_legacy_tokeninfo_endpoint":        cty.NullVal(cty.Bool),
						"dashboard_insights_view":                cty.NullVal(cty.Bool),
						"dashboard_log_streams_next":             cty.NullVal(cty.Bool),
						"disable_clickjack_protection_headers":   cty.NullVal(cty.Bool),
						"disable_fields_map_fix":                 cty.NullVal(cty.Bool),
						"disable_management_api_sms_obfuscation": cty.NullVal(cty.Bool),
						"enable_adfs_waad_email_verification":    cty.NullVal(cty.Bool),
						"enable_apis_section":                    cty.NullVal(cty.Bool),
						"enable_client_connections":              cty.NullVal(cty.Bool),
						"enable_custom_domain_in_emails":         cty.NullVal(cty.Bool),
						"enable_dynamic_client_registration":     cty.NullVal(cty.Bool),
						"enable_idtoken_api2":                    cty.NullVal(cty.Bool),
						"enable_legacy_logs_search_v2":           cty.NullVal(cty.Bool),
						"enable_legacy_profile":                  cty.NullVal(cty.Bool),
						"enable_pipeline2":                       cty.NullVal(cty.Bool),
						"enable_public_signup_user_exists_error": cty.NullVal(cty.Bool),
						"no_disclose_enterprise_connections":     cty.NullVal(cty.Bool),
						"revoke_refresh_token_grant":             cty.NullVal(cty.Bool),
						"universal_login":                        cty.True,
						"use_scope_descriptions_for_consent":     cty.NullVal(cty.Bool),
					}),
				}),
				"friendly_name": cty.StringVal("Tenant Name"),
				"guardian_mfa_page": cty.NullVal(cty.List(cty.Object(map[string]cty.Type{
					"enabled": cty.Bool,
					"html":    cty.String,
				}))),
				"id":                    cty.NullVal(cty.String),
				"idle_session_lifetime": cty.NullVal(cty.Number),
				"picture_url":           cty.NullVal(cty.String),
				"sandbox_version":       cty.NullVal(cty.String),
				"session_cookie": cty.NullVal(cty.List(cty.Object(map[string]cty.Type{
					"mode": cty.String,
				}))),
				"session_lifetime": cty.NullVal(cty.Number),
				"support_email":    cty.NullVal(cty.String),
				"support_url":      cty.NullVal(cty.String),
				"universal_login": cty.ListVal([]cty.Value{
					cty.ObjectVal(map[string]cty.Value{
						"colors": cty.ListVal([]cty.Value{
							cty.ObjectVal(map[string]cty.Value{
								"page_background": cty.StringVal("#000000"),
								"primary":         cty.StringVal("#3385ff"),
							}),
						}),
					}),
				}),
			}),
		},
		{
			name:   "Regress an issue with set values being parsed as tuples",
			schema: testprovider.ProviderV2().ResourcesMap["example_resource"],
			config: (func() map[string]interface{} {
				data := map[string]interface{}{
					"string_property_value": "foo",
					"set_property_value":    []interface{}{"e1", "e2"},
				}
				return data
			})(),
			expected: cty.ObjectVal(map[string]cty.Value{
				"string_property_value": cty.StringVal("foo"),
				"set_property_value": cty.SetVal([]cty.Value{
					cty.StringVal("e1"),
					cty.StringVal("e2"),
				}),
			}),
		},
		{
			name: "Regress aws 3094",
			schema: func() *schema.Resource {
				securityGroupRuleNestedBlock := &schema.Resource{
					Schema: map[string]*schema.Schema{
						"from_port": {
							Type:     schema.TypeInt,
							Required: true,
						},
						"to_port": {
							Type:     schema.TypeInt,
							Required: true,
						},
						"cidr_blocks": {
							Type:     schema.TypeList,
							Optional: true,
							Elem: &schema.Schema{
								Type: schema.TypeString,
							},
						},
					},
				}
				ingress := &schema.Schema{
					Type:       schema.TypeSet,
					Optional:   true,
					Computed:   true,
					ConfigMode: schema.SchemaConfigModeAttr,
					Elem:       securityGroupRuleNestedBlock,
					Set: func(i interface{}) int {
						return 0
					},
				}
				return &schema.Resource{
					Schema: map[string]*schema.Schema{
						"arn": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"ingress": ingress,
					},
				}
			}(),
			config: map[string]interface{}{
				"ingress": []interface{}{
					map[string]interface{}{
						"to_port":   terraformUnknownVariableValue,
						"from_port": terraformUnknownVariableValue,
					},
				},
			},
			expected: cty.ObjectVal(map[string]cty.Value{
				"ingress": cty.SetVal([]cty.Value{
					cty.ObjectVal(map[string]cty.Value{
						"cidr_blocks": cty.NullVal(cty.List(cty.String)),
						"from_port":   cty.UnknownVal(cty.Number),
						"to_port":     cty.UnknownVal(cty.Number),
					}),
				}),
			}),
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if c.skip {
				t.Skip()
			}

			resourceConfig := &terraform.ResourceConfig{Raw: c.config}
			config := makeResourceRawConfig(PlanState, resourceConfig, c.schema)

			actualMap := config.AsValueMap()
			expectedMap := c.expected.AsValueMap()

			keys := map[string]bool{}

			for k := range actualMap {
				keys[k] = true
			}

			for k := range expectedMap {
				keys[k] = true
			}

			for k := range keys {
				ev, gotEv := expectedMap[k]
				av, gotAv := actualMap[k]
				switch {
				case gotAv && !gotEv && !av.IsNull(): // tolerate Null to avoid spelling it out in expected
					t.Errorf("Key %q has an unexpected entry %v", k, av.GoString())
				case !gotAv && gotEv:
					t.Errorf("Key %q is missing an entry, expecting %v", k, av.GoString())
				case gotAv && gotEv && !av.RawEquals(ev):
					t.Errorf("Key %q expected to have a value %v but got %v", k, ev.GoString(), av.GoString())
				}
			}
		})
	}
}

func TestRecoverCtyValue(t *testing.T) {
	type testCase struct {
		name   string
		dT     cty.Type
		value  any
		expect cty.Value
	}

	cases := []testCase{
		{"null", cty.EmptyObject, nil, cty.NullVal(cty.EmptyObject)},
		{
			"object with mismatched fields",
			cty.Object(map[string]cty.Type{
				"x": cty.String,
				"y": cty.Bool,
			}),
			map[string]any{
				"y": true,
				"z": "ignored",
			},
			cty.ObjectVal(map[string]cty.Value{
				"x": cty.NullVal(cty.String),
				"y": cty.BoolVal(true),
			}),
		},
		{
			"tuple",
			cty.Tuple([]cty.Type{cty.String, cty.Number}),
			[]interface{}{"A", 42},
			cty.TupleVal([]cty.Value{cty.StringVal("A"), cty.NumberIntVal(42)}),
		},
		{
			"empty object",
			cty.EmptyObject,
			map[string]interface{}{},
			cty.EmptyObjectVal,
		},
		{
			"empty tuple",
			cty.EmptyTuple,
			[]interface{}{},
			cty.EmptyTupleVal,
		},
		{
			"empty map",
			cty.Map(cty.String),
			map[string]interface{}{},
			cty.MapValEmpty(cty.String),
		},
		{
			"empty list",
			cty.List(cty.String),
			[]any{},
			cty.ListValEmpty(cty.String),
		},
		{
			"empty set",
			cty.Set(cty.String),
			[]interface{}{},
			cty.SetValEmpty(cty.String),
		},
		{"int", cty.Number, int(42), cty.NumberIntVal(42)},
		{"int64", cty.Number, int64(42), cty.NumberIntVal(42)},
		{"uint8", cty.Number, uint8(42), cty.NumberIntVal(42)},
		{"uint16", cty.Number, uint16(42), cty.NumberIntVal(42)},
		{"uint32", cty.Number, uint32(42), cty.NumberIntVal(42)},
		{"uint64", cty.Number, uint64(42), cty.NumberIntVal(42)},
		{"int8", cty.Number, int8(42), cty.NumberIntVal(42)},
		{"int16", cty.Number, int16(42), cty.NumberIntVal(42)},
		{"int32", cty.Number, int32(42), cty.NumberIntVal(42)},
		{"int64", cty.Number, int64(42), cty.NumberIntVal(42)},
		{"float64", cty.Number, float64(1.42), cty.NumberFloatVal(1.42)},
		{"float32", cty.Number, float32(1.42), cty.NumberFloatVal(float64(float32(1.42)))},
		{"big.Float", cty.Number, big.NewFloat(1.42), cty.NumberVal(big.NewFloat(1.42))},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			r, err := recoverCtyValue(tc.dT, tc.value)
			require.NoError(t, err)
			require.Truef(t, tc.expect.RawEquals(r), "expected %s to equal %s",
				r.GoString(), tc.expect.GoString())
		})
	}
}
