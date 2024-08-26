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
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	pb "github.com/pulumi/pulumi-terraform-bridge/pf/tests/internal/providerbuilder"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestCreateResourceWithDynamicAttribute(t *testing.T) {
	type testCase struct {
		name                     string                 // test case name
		manifestToSend           any                    // assumes a Pulumi YAML expression
		expectedManifestReceived basetypes.DynamicValue // how PF sees the decoded manifest
		expectedManifestOutput   any                    // value received back through the output machinery
	}

	testCases := []testCase{
		{
			name:                     "string",
			manifestToSend:           "FOO",
			expectedManifestReceived: basetypes.NewDynamicValue(basetypes.NewStringValue("FOO")),
			expectedManifestOutput:   "FOO",
		},
	}

	type ResourceModel struct {
		Id       types.String  `tfsdk:"id"`
		Manifest types.Dynamic `tfsdk:"manifest"`
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			r := pb.Resource{
				Name: "r",
				CreateFunc: func(
					ctx context.Context,
					req resource.CreateRequest,
					resp *resource.CreateResponse,
				) {
					var model ResourceModel
					diags := req.Config.Get(ctx, &model)
					if diags.HasError() {
						for _, d := range diags {
							t.Logf("%s: %s", d.Summary(), d.Detail())
						}
						panic("req.Config.Get failed")
					}
					require.Equal(t, tc.expectedManifestReceived, model.Manifest)

					t.Logf("Create called with manifest as: %+v", model.Manifest)

					model.Id = basetypes.NewStringValue("id0")

					diags = resp.State.Set(ctx, &model)
					resp.Diagnostics = append(resp.Diagnostics, diags...)
				},
				ResourceSchema: schema.Schema{
					Attributes: map[string]schema.Attribute{
						"manifest": schema.DynamicAttribute{
							Optional: true,
						},
					},
				},
			}

			p := pb.NewProvider(pb.NewProviderArgs{
				AllResources: []pb.Resource{r},
			})

			program := map[string]any{
				"name":    "test-program",
				"runtime": "yaml",
				"resources": map[string]any{
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
			res := pt.Up()

			m := res.Outputs["manifest"]

			require.Equalf(t, m.Value, tc.expectedManifestOutput, "expected manifest to turnaround")
		})
	}
}
