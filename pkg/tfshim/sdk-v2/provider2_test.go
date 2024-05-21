package sdkv2

import (
	"context"
	"strconv"
	"testing"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpgradeResourceState(t *testing.T) {
	const tfToken = "test_token"
	for _, tc := range []struct {
		name     string
		state    cty.Value
		rSchema  *schema.Resource
		expected cty.Value
	}{
		{
			name: "no upgrade",
			state: cty.ObjectVal(map[string]cty.Value{
				"id": cty.StringVal("1"),
				"x":  cty.NumberIntVal(1),
			}),
			rSchema: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"x": {Type: schema.TypeInt, Optional: true},
				},
			},
			expected: cty.ObjectVal(map[string]cty.Value{
				"id": cty.StringVal("1"),
				"x":  cty.NumberIntVal(1),
			}),
		},
		{
			name: "basic upgrade type",
			state: cty.ObjectVal(map[string]cty.Value{
				"id": cty.StringVal("1"),
				"x":  cty.StringVal("1"),
			}),
			rSchema: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"x": {Type: schema.TypeInt, Optional: true},
				},
				StateUpgraders: []schema.StateUpgrader{
					{
						Version: 0,
						Type:    cty.Object(map[string]cty.Type{"x": cty.String}),
						Upgrade: func(
							ctx context.Context, rawState map[string]interface{}, meta interface{},
						) (map[string]interface{}, error) {
							newVal, err := strconv.ParseInt(rawState["x"].(string), 10, 64)
							if err != nil {
								return nil, err
							}
							rawState["x"] = int(newVal)
							return rawState, nil
						},
					},
				},
			},
			expected: cty.ObjectVal(map[string]cty.Value{
				"id": cty.StringVal("1"),
				"x":  cty.NumberIntVal(1),
			}),
		},
		{
			name: "rename property",
			state: cty.ObjectVal(map[string]cty.Value{
				"id": cty.StringVal("1"),
				"x":  cty.StringVal("1"),
			}),
			rSchema: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"y": {Type: schema.TypeString, Optional: true},
				},
				StateUpgraders: []schema.StateUpgrader{
					{
						Version: 0,
						Type:    cty.Object(map[string]cty.Type{"x": cty.String}),
						Upgrade: func(
							ctx context.Context, rawState map[string]interface{}, meta interface{},
						) (map[string]interface{}, error) {
							rawState["y"] = rawState["x"]
							delete(rawState, "x")
							return rawState, nil
						},
					},
				},
			},
			expected: cty.ObjectVal(map[string]cty.Value{
				"id": cty.StringVal("1"),
				"y":  cty.StringVal("1"),
			}),
		},
		{
			name: "flat prop to collection",
			state: cty.ObjectVal(map[string]cty.Value{
				"id": cty.StringVal("1"),
				"x":  cty.StringVal("1"),
			}),
			rSchema: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"x": {Type: schema.TypeList, Optional: true, Elem: &schema.Schema{Type: schema.TypeString}},
				},
				StateUpgraders: []schema.StateUpgrader{
					{
						Version: 0,
						Type:    cty.Object(map[string]cty.Type{"x": cty.String}),
						Upgrade: func(
							ctx context.Context, rawState map[string]interface{}, meta interface{},
						) (map[string]interface{}, error) {
							rawState["x"] = []interface{}{rawState["x"]}
							return rawState, nil
						},
					},
				},
			},
			expected: cty.ObjectVal(map[string]cty.Value{
				"id": cty.StringVal("1"),
				"x":  cty.ListVal([]cty.Value{cty.StringVal("1")}),
			}),
		},
		{
			name: "collection to flat prop",
			state: cty.ObjectVal(map[string]cty.Value{
				"id": cty.StringVal("1"),
				"x":  cty.ListVal([]cty.Value{cty.StringVal("1")}),
			}),
			rSchema: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"x": {Type: schema.TypeString, Optional: true},
				},
				StateUpgraders: []schema.StateUpgrader{
					{
						Version: 0,
						Type:    cty.Object(map[string]cty.Type{"x": cty.List(cty.String)}),
						Upgrade: func(
							ctx context.Context, rawState map[string]interface{}, meta interface{},
						) (map[string]interface{}, error) {
							rawState["x"] = rawState["x"].([]interface{})[0]
							return rawState, nil
						},
					},
				},
			},
			expected: cty.ObjectVal(map[string]cty.Value{
				"id": cty.StringVal("1"),
				"x":  cty.StringVal("1"),
			}),
		},
		{
			name: "change list to set",
			state: cty.ObjectVal(map[string]cty.Value{
				"id": cty.StringVal("1"),
				"x":  cty.ListVal([]cty.Value{cty.StringVal("1"), cty.StringVal("2")}),
			}),
			rSchema: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"x": {
						Type:     schema.TypeSet,
						Optional: true,
						Elem:     &schema.Schema{Type: schema.TypeString},
					},
				},
				StateUpgraders: []schema.StateUpgrader{
					{
						Version: 0,
						Type:    cty.Object(map[string]cty.Type{"x": cty.List(cty.String)}),
						Upgrade: func(
							ctx context.Context, rawState map[string]interface{}, meta interface{},
						) (map[string]interface{}, error) {
							return rawState, nil
						},
					},
				},
			},
			expected: cty.ObjectVal(map[string]cty.Value{
				"id": cty.StringVal("1"),
				"x":  cty.SetVal([]cty.Value{cty.StringVal("1"), cty.StringVal("2")}),
			}),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tf := &schema.Provider{
				ResourcesMap: map[string]*schema.Resource{
					tfToken: tc.rSchema,
				},
			}
			actual, err := (&planResourceChangeImpl{
				tf:     tf,
				server: &grpcServer{schema.NewGRPCProviderServer(tf)},
			}).upgradeState(context.Background(), tfToken, &v2InstanceState2{
				resourceType: tfToken,
				stateValue:   tc.state,
			})
			require.NoError(t, err)

			assert.Equal(t, tc.expected, actual.(*v2InstanceState2).stateValue)
		})
	}
}

func TestNormalizeBlockCollections(t *testing.T) {
	for _, tc := range []struct {
		name     string
		res      *schema.Resource
		input    cty.Value
		expected cty.Value
	}{
		{
			name: "basic",
			input: cty.ObjectVal(
				map[string]cty.Value{
					"prop": cty.StringVal("val"),
				},
			),
			res: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"prop": {Type: schema.TypeString, Optional: true},
				},
			},
			expected: cty.ObjectVal(
				map[string]cty.Value{
					"prop": cty.StringVal("val"),
				},
			),
		},
		{
			name: "list attr",
			input: cty.ObjectVal(
				map[string]cty.Value{
					"prop": cty.ListVal([]cty.Value{cty.StringVal("val")}),
				},
			),
			res: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"prop": {
						Type:     schema.TypeList,
						Optional: true,
						Elem:     &schema.Schema{Type: schema.TypeString},
					},
				},
			},
			expected: cty.ObjectVal(
				map[string]cty.Value{
					"prop": cty.ListVal([]cty.Value{cty.StringVal("val")}),
				},
			),
		},
		{
			name: "list block with val",
			input: cty.ObjectVal(
				map[string]cty.Value{
					"prop": cty.ListVal(
						[]cty.Value{
							cty.ObjectVal(
								map[string]cty.Value{"field": cty.StringVal("val")},
							),
						},
					),
				},
			),
			res: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"prop": {
						Type:     schema.TypeList,
						Optional: true,
						Elem: &schema.Resource{
							Schema: map[string]*schema.Schema{
								"field": {Type: schema.TypeString, Optional: true},
							},
						},
					},
				},
			},
			expected: cty.ObjectVal(
				map[string]cty.Value{
					"prop": cty.ListVal(
						[]cty.Value{
							cty.ObjectVal(
								map[string]cty.Value{"field": cty.StringVal("val")},
							),
						},
					),
				},
			),
		},
		{
			name: "list block no val",
			input: cty.ObjectVal(
				map[string]cty.Value{
					"prop": cty.NullVal(
						cty.List(cty.Object(map[string]cty.Type{"field": cty.String})),
					),
				},
			),
			res: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"prop": {
						Type:     schema.TypeList,
						Optional: true,
						Elem: &schema.Resource{
							Schema: map[string]*schema.Schema{
								"field": {Type: schema.TypeString, Optional: true},
							},
						},
					},
				},
			},
			expected: cty.ObjectVal(
				map[string]cty.Value{
					"prop": cty.ListValEmpty(cty.Object(map[string]cty.Type{"field": cty.String})),
				},
			),
		},
		{
			name: "list block no val config mode attr",
			input: cty.ObjectVal(
				map[string]cty.Value{
					"prop": cty.NullVal(
						cty.List(cty.Object(map[string]cty.Type{"field": cty.String})),
					),
				},
			),
			res: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"prop": {
						Type:       schema.TypeList,
						Optional:   true,
						ConfigMode: schema.SchemaConfigModeAttr,
						Elem: &schema.Resource{
							Schema: map[string]*schema.Schema{
								"field": {Type: schema.TypeString, Optional: true},
							},
						},
					},
				},
			},
			expected: cty.ObjectVal(
				map[string]cty.Value{
					"prop": cty.NullVal(cty.List(cty.Object(map[string]cty.Type{"field": cty.String}))),
				},
			),
		},
		{
			name: "set block no val",
			input: cty.ObjectVal(
				map[string]cty.Value{
					"prop": cty.NullVal(
						cty.Set(cty.Object(map[string]cty.Type{"field": cty.String})),
					),
				},
			),
			res: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"prop": {
						Type:     schema.TypeSet,
						Optional: true,
						Elem: &schema.Resource{
							Schema: map[string]*schema.Schema{
								"field": {Type: schema.TypeString, Optional: true},
							},
						},
					},
				},
			},
			expected: cty.ObjectVal(
				map[string]cty.Value{
					"prop": cty.SetValEmpty(cty.Object(map[string]cty.Type{"field": cty.String})),
				},
			),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.res.InternalValidate(nil, false)
			require.NoError(t, err)

			res := normalizeBlockCollections(
				tc.input,
				tc.res,
			)
			if !tc.expected.Equals(res).True() {
				t.Logf("Expect: %s", tc.expected.GoString())
				t.Logf("Actual: %s", res.GoString())
				t.FailNow()
			}
		})
	}
}
