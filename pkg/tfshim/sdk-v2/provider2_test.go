package sdkv2

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/hexops/autogold/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/unstable/logging"
)

func TestProvider2UpgradeResourceState(t *testing.T) {
	t.Parallel()
	const tfToken = "test_token"
	for _, tc := range []struct {
		name      string
		state     cty.Value
		rSchema   *schema.Resource
		expected  cty.Value
		expectErr func(*testing.T, error)
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
		{
			name: "large number",
			state: func() cty.Value {
				n, err := cty.ParseNumberVal("641577219598130723")
				require.NoError(t, err)
				return cty.ObjectVal(map[string]cty.Value{
					"x":  n,
					"id": cty.StringVal("id"),
				})
			}(),
			expected: func() cty.Value {
				n, err := cty.ParseNumberVal("641577219598130723")
				require.NoError(t, err)

				// We set the precision so it agrees with the test. We
				// don't have any semantic requirement that the precision
				// is 64.
				n = cty.NumberVal(n.AsBigFloat().SetPrec(64))
				return cty.ObjectVal(map[string]cty.Value{
					"x":  n,
					"id": cty.StringVal("id"),
				})
			}(),
			rSchema: &schema.Resource{
				UseJSONNumber: true,
				Schema: map[string]*schema.Schema{
					"x": {Type: schema.TypeInt, Optional: true},
				},
			},
		},
		{
			name: "handle errors",
			state: func() cty.Value {
				return cty.ObjectVal(map[string]cty.Value{
					"compute_resources": cty.ObjectVal(map[string]cty.Value{
						"ec2_configuration": cty.ObjectVal(map[string]cty.Value{
							"image_id_override": cty.StringVal("override"),
						}),
					}),
					"id": cty.StringVal("id"),
				})
			}(),
			expectErr: func(t *testing.T, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, "missing expected [")
			},
			rSchema: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"compute_resources": {
						Type:     schema.TypeList,
						Optional: true,
						ForceNew: true,
						MinItems: 0,
						MaxItems: 1,
						Elem: &schema.Resource{
							Schema: map[string]*schema.Schema{
								"ec2_configuration": {
									Type:     schema.TypeList,
									Optional: true,
									Computed: true,
									ForceNew: true,
									MaxItems: 2,
									Elem: &schema.Resource{
										Schema: map[string]*schema.Schema{
											"image_id_override": {
												Type:     schema.TypeString,
												Optional: true,
												Computed: true,
											},
											"image_type": {
												Type:     schema.TypeString,
												Optional: true,
											},
										},
									},
								},
							},
						},
					},
				},
			},
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
			if tc.expectErr != nil {
				tc.expectErr(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.expected, actual.(*v2InstanceState2).stateValue)
			}
		})
	}
}

func TestNormalizeBlockCollections(t *testing.T) {
	t.Parallel()
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

func TestConfigWithTimeouts(t *testing.T) {
	t.Parallel()
	type testCase struct {
		name                  string
		topts                 shim.TimeoutOptions
		configWithoutTimeouts map[string]any
		expected              map[string]any
		expectedWarnings      autogold.Value
		rschema               schema.Resource
	}

	sec20 := 20 * time.Second

	testCases := []testCase{
		{
			name: "customize create timeout",
			rschema: schema.Resource{
				Timeouts: &schema.ResourceTimeout{Create: &sec20},
				Schema: map[string]*schema.Schema{
					"x": {Type: schema.TypeInt, Optional: true},
				},
			},
			topts: shim.TimeoutOptions{TimeoutOverrides: map[shim.TimeoutKey]time.Duration{
				shim.TimeoutCreate: 1 * time.Second,
			}},
			configWithoutTimeouts: map[string]any{"x": 1},
			expected:              map[string]any{"x": 1, "timeouts": map[string]any{"create": "1s"}},
		},
		{
			name: "customize update timeout",
			rschema: schema.Resource{
				Timeouts: &schema.ResourceTimeout{Update: &sec20},
				Schema: map[string]*schema.Schema{
					"x": {Type: schema.TypeInt, Optional: true},
				},
			},
			topts: shim.TimeoutOptions{TimeoutOverrides: map[shim.TimeoutKey]time.Duration{
				shim.TimeoutUpdate: 1 * time.Second,
			}},
			configWithoutTimeouts: map[string]any{"x": 1},
			expected:              map[string]any{"x": 1, "timeouts": map[string]any{"update": "1s"}},
		},
		{
			name: "customize delete timeout",
			rschema: schema.Resource{
				Timeouts: &schema.ResourceTimeout{Delete: &sec20},
				Schema: map[string]*schema.Schema{
					"x": {Type: schema.TypeInt, Optional: true},
				},
			},
			topts: shim.TimeoutOptions{TimeoutOverrides: map[shim.TimeoutKey]time.Duration{
				shim.TimeoutDelete: 1 * time.Second,
			}},
			configWithoutTimeouts: map[string]any{"x": 1},
			expected:              map[string]any{"x": 1, "timeouts": map[string]any{"delete": "1s"}},
		},
		{
			name: "pass through when no overrides specified",
			rschema: schema.Resource{
				Schema: map[string]*schema.Schema{
					"x": {Type: schema.TypeInt, Optional: true},
				},
			},
			topts:                 shim.TimeoutOptions{},
			configWithoutTimeouts: map[string]any{"x": 1},
			expected:              map[string]any{"x": 1},
		},
		{
			name: "warn when customizing create timeouts is not supported",
			rschema: schema.Resource{
				Schema: map[string]*schema.Schema{
					"x": {Type: schema.TypeInt, Optional: true},
				},
			},
			topts: shim.TimeoutOptions{TimeoutOverrides: map[shim.TimeoutKey]time.Duration{
				shim.TimeoutCreate: 1 * time.Second,
				shim.TimeoutDelete: 2 * time.Second,
			}},
			configWithoutTimeouts: map[string]any{"x": 1},
			//nolint:lll
			expectedWarnings: autogold.Expect([]string{"WARN: Resource does not support customTimeouts, ignoring: create=1s, delete=2s"}),
		},
		{
			name: "warn when customizing create timeouts against a custom timeout schema",
			rschema: schema.Resource{
				Schema: map[string]*schema.Schema{
					"x": {Type: schema.TypeInt, Optional: true},
					"timeouts": {
						Type: schema.TypeList,
						Elem: &schema.Resource{
							Schema: map[string]*schema.Schema{
								"appTimeout": {Type: schema.TypeInt, Optional: true},
							},
						},
						MaxItems: 1,
					},
				},
			},
			topts: shim.TimeoutOptions{TimeoutOverrides: map[shim.TimeoutKey]time.Duration{
				shim.TimeoutCreate: 1 * time.Second,
			}},
			configWithoutTimeouts: map[string]any{"x": 1},

			expectedWarnings: autogold.Expect([]string{"WARN: Resource does not support customTimeouts, ignoring: create=1s"}),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			logger := &testLogger{}
			ctx := context.WithValue(context.Background(), logging.CtxKey, logger)
			i := &planResourceChangeImpl{}
			ty := tc.rschema.CoreConfigSchema().ImpliedType()
			// Quick short-cut here, taking advantage of the fact that configWithTimeouts only reads
			// cfg.tf.Config, set only that. If this needs to be revised, can construct ResourceConfig in
			// the usual way instead.
			cfg := v2ResourceConfig{
				tf: &terraform.ResourceConfig{
					Config: tc.configWithoutTimeouts,
				},
			}
			actual := i.configWithTimeouts(ctx, cfg, tc.topts, ty)
			if tc.expected != nil {
				assert.Equal(t, tc.expected, actual)
			} else {
				tc.expectedWarnings.Equal(t, logger.messages)
			}
		})
	}
}

type testLogger struct {
	messages []string
}

func (l *testLogger) Debug(msg string) {
	l.messages = append(l.messages, fmt.Sprintf("DEBUG: %s", msg))
}

func (l *testLogger) Info(msg string) {
	l.messages = append(l.messages, fmt.Sprintf("INFO: %s", msg))
}

func (l *testLogger) Warn(msg string) {
	l.messages = append(l.messages, fmt.Sprintf("WARN: %s", msg))
}

func (l *testLogger) Error(msg string) {
	l.messages = append(l.messages, fmt.Sprintf("ERROR: %s", msg))
}

func (*testLogger) StatusUntyped() any {
	return "?"
}
