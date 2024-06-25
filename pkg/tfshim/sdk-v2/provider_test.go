package sdkv2

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/hexops/autogold/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProvider1UpgradeResourceState(t *testing.T) {
	t.Parallel()

	type tc struct {
		name   string
		schema *schema.Resource
		input  func() *terraform.InstanceState
		expect func(t *testing.T, actual *terraform.InstanceState, tc tc)
	}

	tests := []tc{
		{
			name: "roundtrip int64",
			schema: &schema.Resource{
				UseJSONNumber: true,
				Schema: map[string]*schema.Schema{
					"x": {Type: schema.TypeInt, Optional: true},
				},
			},
			input: func() *terraform.InstanceState {
				n, err := cty.ParseNumberVal("641577219598130723")
				require.NoError(t, err)
				v := cty.ObjectVal(map[string]cty.Value{"x": n})
				s := terraform.NewInstanceStateShimmedFromValue(v, 0)
				s.Meta["schema_version"] = "0"
				s.ID = "id"
				s.RawState = v
				s.Attributes["id"] = s.ID
				return s
			},
			expect: func(t *testing.T, actual *terraform.InstanceState, tc tc) {
				assert.Equal(t, tc.input().Attributes, actual.Attributes)
			},
		},
		{
			name: "type change",
			schema: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"x1": {Type: schema.TypeInt, Optional: true},
				},
				SchemaVersion: 1,
				StateUpgraders: []schema.StateUpgrader{{
					Version: 0,
					Type: cty.Object(map[string]cty.Type{
						"id": cty.String,
						"x0": cty.String,
					}),
					Upgrade: func(_ context.Context, rawState map[string]any, _ interface{}) (map[string]any, error) {
						return map[string]any{
							"id": rawState["id"],
							"x1": len(rawState["x0"].(string)),
						}, nil
					},
				}},
			},
			input: func() *terraform.InstanceState {
				s := terraform.NewInstanceStateShimmedFromValue(cty.ObjectVal(map[string]cty.Value{
					"x0": cty.StringVal("123"),
				}), 0)
				s.Meta["schema_version"] = "0"
				s.ID = "id"
				return s
			},
			expect: func(t *testing.T, actual *terraform.InstanceState, tc tc) {
				t.Logf("Actual = %#v", actual)
				assert.Equal(t, map[string]string{
					"id": "id",
					"x1": "3",
				}, actual.Attributes)
			},
		},
	}

	const tfToken = "test_token"

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()

			require.NoError(t, tt.schema.InternalValidate(tt.schema.Schema, true))

			p := &schema.Provider{ResourcesMap: map[string]*schema.Resource{tfToken: tt.schema}}

			actual, err := upgradeResourceState(ctx, tfToken, p, tt.schema, tt.input())
			require.NoError(t, err)

			tt.expect(t, actual, tt)
		})
	}
}

func TestProviderDetailedSchemaDump(t *testing.T) {
	prov := NewProvider(&schema.Provider{
		ResourcesMap: map[string]*schema.Resource{
			"test_resource": {
				Schema: map[string]*schema.Schema{
					"foo": {Type: schema.TypeString},
					"bar": {Type: schema.TypeInt},
				},
			},
		},
		DataSourcesMap: map[string]*schema.Resource{
			"test_data_source": {
				Schema: map[string]*schema.Schema{
					"foo": {Type: schema.TypeString},
					"bar": {Type: schema.TypeInt},
				},
			},
		},
		Schema: map[string]*schema.Schema{
			"test_schema": {Type: schema.TypeString},
		},
	})

	jsonArr := prov.DetailedSchemaDump()
	var out bytes.Buffer
	err := json.Indent(&out, jsonArr, "", "    ")
	require.NoError(t, err)

	autogold.Expect(`{
    "Schema": {
        "test_schema": {
            "Type": 4,
            "ConfigMode": 0,
            "Required": false,
            "Optional": false,
            "Computed": false,
            "ForceNew": false,
            "DiffSuppressFuncDefined": false,
            "DiffSuppressOnRefresh": false,
            "Default": null,
            "DefaultFuncDefined": false,
            "Description": "",
            "InputDefault": "",
            "StateFuncDefined": false,
            "Elem": null,
            "MaxItems": 0,
            "MinItems": 0,
            "SetDefined": false,
            "ComputedWhen": null,
            "ConflictsWith": null,
            "ExactlyOneOf": null,
            "AtLeastOneOf": null,
            "RequiredWith": null,
            "Deprecated": "",
            "ValidateFuncDefined": false,
            "ValidateDiagFuncDefined": false,
            "Sensitive": false
        }
    },
    "ResourcesMap": {
        "test_resource": {
            "Schema": {
                "bar": {
                    "Type": 2,
                    "ConfigMode": 0,
                    "Required": false,
                    "Optional": false,
                    "Computed": false,
                    "ForceNew": false,
                    "DiffSuppressFuncDefined": false,
                    "DiffSuppressOnRefresh": false,
                    "Default": null,
                    "DefaultFuncDefined": false,
                    "Description": "",
                    "InputDefault": "",
                    "StateFuncDefined": false,
                    "Elem": null,
                    "MaxItems": 0,
                    "MinItems": 0,
                    "SetDefined": false,
                    "ComputedWhen": null,
                    "ConflictsWith": null,
                    "ExactlyOneOf": null,
                    "AtLeastOneOf": null,
                    "RequiredWith": null,
                    "Deprecated": "",
                    "ValidateFuncDefined": false,
                    "ValidateDiagFuncDefined": false,
                    "Sensitive": false
                },
                "foo": {
                    "Type": 4,
                    "ConfigMode": 0,
                    "Required": false,
                    "Optional": false,
                    "Computed": false,
                    "ForceNew": false,
                    "DiffSuppressFuncDefined": false,
                    "DiffSuppressOnRefresh": false,
                    "Default": null,
                    "DefaultFuncDefined": false,
                    "Description": "",
                    "InputDefault": "",
                    "StateFuncDefined": false,
                    "Elem": null,
                    "MaxItems": 0,
                    "MinItems": 0,
                    "SetDefined": false,
                    "ComputedWhen": null,
                    "ConflictsWith": null,
                    "ExactlyOneOf": null,
                    "AtLeastOneOf": null,
                    "RequiredWith": null,
                    "Deprecated": "",
                    "ValidateFuncDefined": false,
                    "ValidateDiagFuncDefined": false,
                    "Sensitive": false
                }
            },
            "SchemaVersion": 0,
            "MigrateStateDefined": false,
            "StateUpgradersLen": 0,
            "CustomizeDiffDefined": false,
            "DeprecationMessage": "",
            "Description": "",
            "UseJSONNumber": false,
            "EnableLegacyTypeSystemApplyErrors": false,
            "EnableLegacyTypeSystemPlanErrors": false
        }
    },
    "DataSourcesMap": {
        "test_data_source": {
            "Schema": {
                "bar": {
                    "Type": 2,
                    "ConfigMode": 0,
                    "Required": false,
                    "Optional": false,
                    "Computed": false,
                    "ForceNew": false,
                    "DiffSuppressFuncDefined": false,
                    "DiffSuppressOnRefresh": false,
                    "Default": null,
                    "DefaultFuncDefined": false,
                    "Description": "",
                    "InputDefault": "",
                    "StateFuncDefined": false,
                    "Elem": null,
                    "MaxItems": 0,
                    "MinItems": 0,
                    "SetDefined": false,
                    "ComputedWhen": null,
                    "ConflictsWith": null,
                    "ExactlyOneOf": null,
                    "AtLeastOneOf": null,
                    "RequiredWith": null,
                    "Deprecated": "",
                    "ValidateFuncDefined": false,
                    "ValidateDiagFuncDefined": false,
                    "Sensitive": false
                },
                "foo": {
                    "Type": 4,
                    "ConfigMode": 0,
                    "Required": false,
                    "Optional": false,
                    "Computed": false,
                    "ForceNew": false,
                    "DiffSuppressFuncDefined": false,
                    "DiffSuppressOnRefresh": false,
                    "Default": null,
                    "DefaultFuncDefined": false,
                    "Description": "",
                    "InputDefault": "",
                    "StateFuncDefined": false,
                    "Elem": null,
                    "MaxItems": 0,
                    "MinItems": 0,
                    "SetDefined": false,
                    "ComputedWhen": null,
                    "ConflictsWith": null,
                    "ExactlyOneOf": null,
                    "AtLeastOneOf": null,
                    "RequiredWith": null,
                    "Deprecated": "",
                    "ValidateFuncDefined": false,
                    "ValidateDiagFuncDefined": false,
                    "Sensitive": false
                }
            },
            "SchemaVersion": 0,
            "MigrateStateDefined": false,
            "StateUpgradersLen": 0,
            "CustomizeDiffDefined": false,
            "DeprecationMessage": "",
            "Description": "",
            "UseJSONNumber": false,
            "EnableLegacyTypeSystemApplyErrors": false,
            "EnableLegacyTypeSystemPlanErrors": false
        }
    },
    "ConfigureFuncDefined": false,
    "TerraformVersion": ""
}`).Equal(t, out.String())
}
