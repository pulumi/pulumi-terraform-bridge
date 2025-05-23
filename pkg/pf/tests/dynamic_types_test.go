// Copyright 2016-2023, Pulumi Corporation.
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

package tfbridgetests

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	pb "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/internal/providerbuilder"
)

func TestCreateResourceWithDynamicAttribute(t *testing.T) {
	t.Parallel()
	type testCase struct {
		name                     string                 // test case name
		manifestToSend           any                    // assumes a Pulumi YAML expression
		expectedManifestReceived basetypes.DynamicValue // how PF sees the decoded manifest
		expectedManifestMatches  func(v basetypes.DynamicValue) error
		expectedManifestOutput   any // value received back through the output machinery
		expectedStateMatches     func(t *testing.T, state apitype.UntypedDeployment)
	}

	testCases := []testCase{
		{
			name:           "null",
			manifestToSend: nil,
			expectedManifestMatches: func(v basetypes.DynamicValue) error {
				if !v.IsNull() {
					return errors.New("expected null dynamic value")
				}
				return nil
			},
			expectedManifestOutput: nil,
		},
		{
			name:                    "unknown",
			manifestToSend:          "${r0.u}",
			expectedManifestMatches: func(v basetypes.DynamicValue) error { return nil },
			expectedManifestOutput:  "U",
		},
		{
			name: "secret",
			manifestToSend: map[string]any{
				"fn::secret": "SECRET",
			},
			expectedManifestReceived: basetypes.NewDynamicValue(basetypes.NewStringValue("SECRET")),
			// First-class secrets are currently handled by the engine transparently without having to be
			// handled for dynamic types specifically; but as this might change it is important to have an
			// end-to-end test that makes sure they do not leak in the state.
			expectedStateMatches: func(t *testing.T, state apitype.UntypedDeployment) {
				bytes, err := json.MarshalIndent(state, "", "  ")
				require.NoError(t, err)
				var d3 apitype.DeploymentV3
				err = json.Unmarshal(bytes, &d3)
				require.NoError(t, err)

				for _, r := range d3.Resources {
					if r.Type != "pulumi:pulumi:Stack" {
						continue
					}
					manifest := r.Outputs["manifest"].(map[string]any)
					require.Equal(t, map[string]any{
						"4dabf18193072939515e22adb298388d": "1b47061264138c4ac30d75fd1eb44270",
						"plaintext":                        "\"SECRET\"",
					}, manifest)
				}
			},
			expectedManifestOutput: "SECRET",
		},
		{
			name:                     "string",
			manifestToSend:           "FOO",
			expectedManifestReceived: basetypes.NewDynamicValue(basetypes.NewStringValue("FOO")),
			expectedManifestOutput:   "FOO",
		},
		{
			name:                     "empty-string",
			manifestToSend:           "",
			expectedManifestReceived: basetypes.NewDynamicValue(basetypes.NewStringValue("")),
			expectedManifestOutput:   "",
		},
		{
			name:                     "true",
			manifestToSend:           true,
			expectedManifestReceived: basetypes.NewDynamicValue(basetypes.NewBoolValue(true)),
			expectedManifestOutput:   true,
		},
		{
			name:                     "false",
			manifestToSend:           false,
			expectedManifestReceived: basetypes.NewDynamicValue(basetypes.NewBoolValue(false)),
			expectedManifestOutput:   false,
		},
		{
			name:           "number",
			manifestToSend: float64(42.0),
			expectedManifestMatches: func(v basetypes.DynamicValue) error {
				var r big.Float
				vv, err := v.UnderlyingValue().ToTerraformValue(context.Background())
				if err != nil {
					return err
				}
				err = vv.As(&r)
				if err != nil {
					return err
				}
				f, _ := r.Float64()
				if f != 42.0 {
					return fmt.Errorf("expected 42.0, got %f", f)
				}
				return nil
			},
			expectedManifestOutput: float64(42.0),
		},
		{
			name:           "uniform-array",
			manifestToSend: []any{"a", "b", "c"},
			expectedManifestReceived: basetypes.NewDynamicValue(basetypes.NewListValueMust(
				types.StringType,
				[]attr.Value{
					basetypes.NewStringValue("a"),
					basetypes.NewStringValue("b"),
					basetypes.NewStringValue("c"),
				},
			)),
			expectedManifestOutput: []any{"a", "b", "c"},
		},
		{
			name:                    "heterogeneous-array",
			manifestToSend:          []any{"a", []any{"1", "2"}, map[string]any{"a": "1"}},
			expectedManifestMatches: func(v basetypes.DynamicValue) error { return nil },
			expectedManifestOutput:  []any{"a", []any{"1", "2"}, map[string]any{"a": "1"}},
		},
		{
			name:           "empty-array",
			manifestToSend: []any{},
			expectedManifestReceived: basetypes.NewDynamicValue(basetypes.NewListValueMust(
				types.DynamicType,
				[]attr.Value{},
			)),
			expectedManifestOutput: []any{},
		},
		{
			name: "uniform-map",
			manifestToSend: map[string]any{
				"a": "1",
				"b": "2",
			},
			expectedManifestMatches: func(v basetypes.DynamicValue) error {
				vv, err := v.ToTerraformValue(context.Background())
				if err != nil {
					return err
				}
				var parts map[string]tftypes.Value
				err = vv.As(&parts)
				if err != nil {
					return err
				}
				if parts["a"].String() != `tftypes.String<"1">` {
					return fmt.Errorf("expected a=1, got %s", parts["a"].String())
				}
				if parts["b"].String() != `tftypes.String<"2">` {
					return fmt.Errorf("expected b=2, got %s", parts["b"].String())
				}
				return nil
			},
			expectedManifestOutput: map[string]any{
				"a": "1",
				"b": "2",
			},
		},
		{
			name:           "empty-map",
			manifestToSend: map[string]any{},
			expectedManifestMatches: func(v basetypes.DynamicValue) error {
				vv, err := v.ToTerraformValue(context.Background())
				if err != nil {
					return err
				}
				var parts map[string]tftypes.Value
				err = vv.As(&parts)
				if err != nil {
					return err
				}
				if len(parts) != 0 {
					return fmt.Errorf("expected empty map, got %v", parts)
				}
				return nil
			},
			expectedManifestOutput: map[string]any{},
		},
		{
			name: "heterogeneous-map",
			manifestToSend: map[string]any{
				"a": "1",
				"b": []any{"x", "y"},
				"c": map[string]any{
					"aa": "1",
					"bb": "2",
				},
			},
			expectedManifestMatches: func(v basetypes.DynamicValue) error { return nil },
			expectedManifestOutput: map[string]any{
				"a": "1",
				"b": []any{"x", "y"},
				"c": map[string]any{
					"aa": "1",
					"bb": "2",
				},
			},
		},
	}

	type ResourceModel struct {
		ID       types.String  `tfsdk:"id"`
		Manifest types.Dynamic `tfsdk:"manifest"`
	}

	type ResourceModelR0 struct {
		ID types.String `tfsdk:"id"`
		U  types.String `tfsdk:"u"`
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r := pb.NewResource(pb.NewResourceArgs{
				Name: "r",
				CreateFunc: func(
					ctx context.Context,
					req resource.CreateRequest,
					resp *resource.CreateResponse,
				) {
					var model ResourceModel
					diags := req.Config.Get(ctx, &model)
					if tc.expectedManifestMatches != nil {
						if err := tc.expectedManifestMatches(model.Manifest); err != nil {
							diags.AddError(err.Error(), "")
						}
					} else {
						if !tc.expectedManifestReceived.Equal(model.Manifest) {
							diags.AddError(fmt.Sprintf("expected manifest does not equal observed one %+v, %+v", tc.expectedManifestReceived, model.Manifest), "")
						}
					}

					model.ID = basetypes.NewStringValue("id0")

					diags.Append(resp.State.Set(ctx, &model)...)
					resp.Diagnostics = append(resp.Diagnostics, diags...)
				},
				ResourceSchema: schema.Schema{
					Attributes: map[string]schema.Attribute{
						"manifest": schema.DynamicAttribute{
							Optional: true,
						},
					},
				},
			})

			// Define an aux resource to produce unknown values for testing.
			r0 := pb.NewResource(pb.NewResourceArgs{
				Name: "r0",
				CreateFunc: func(
					ctx context.Context,
					req resource.CreateRequest,
					resp *resource.CreateResponse,
				) {
					var model ResourceModelR0
					diags := req.Config.Get(ctx, &model)
					model.ID = basetypes.NewStringValue("r00")
					model.U = basetypes.NewStringValue("U")
					diags.Append(resp.State.Set(ctx, &model)...)
					resp.Diagnostics = append(resp.Diagnostics, diags...)
				},
				ResourceSchema: schema.Schema{
					Attributes: map[string]schema.Attribute{
						"u": schema.StringAttribute{
							Computed: true,
						},
					},
				},
			})

			p := pb.NewProvider(pb.NewProviderArgs{
				AllResources: []pb.Resource{r, r0},
			})

			program := map[string]any{
				"name":    "test-program",
				"runtime": "yaml",
				"resources": map[string]any{
					"r0": map[string]any{
						"type": "testprovider:index:R0",
					},
					"r1": map[string]any{
						"type": "testprovider:index:R",
						"properties": map[string]any{
							"manifest": tc.manifestToSend,
						},
					},
				},
				"outputs": map[string]any{
					"manifest": "${r1.manifest}",
				},
			}

			bytes, err := yaml.Marshal(program)
			require.NoError(t, err)
			pt := newPulumiTest(t, p, string(bytes))

			pt.Preview(t)

			res := pt.Up(t)

			m := res.Outputs["manifest"]

			if tc.expectedStateMatches != nil {
				// NOTE: ExportStack calls the CLI with --show-secrets.
				state := pt.ExportStack(t)
				tc.expectedStateMatches(t, state)
			}

			require.Equalf(t, m.Value, tc.expectedManifestOutput, "expected manifest to turnaround")
		})
	}
}
