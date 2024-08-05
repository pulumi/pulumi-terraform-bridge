// Copyright 2016-2024, Pulumi Corporation.
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

package check

import (
	"bytes"
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	sdkschema "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hexops/autogold/v2"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	property "github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"

	pfbridge "github.com/pulumi/pulumi-terraform-bridge/pf/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	sdkv2 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v2"
)

type idSchema struct {
	computed bool
	required bool
	optional bool
}
type mockSchema struct {
	shim.Schema
	idSchema
}

func (m *mockSchema) Optional() bool {
	return m.optional
}

func (m *mockSchema) Required() bool {
	return m.required
}

func (m *mockSchema) Computed() bool {
	return m.computed
}

func TestIsInputProperty(t *testing.T) {
	tests := []struct {
		name    string
		schema  shim.Schema
		isInput bool
	}{
		{
			name:    "Optional+Computed",
			schema:  &mockSchema{idSchema: idSchema{optional: true, computed: true, required: false}},
			isInput: true,
		},
		{
			name:    "Optional",
			schema:  &mockSchema{idSchema: idSchema{optional: true, computed: false, required: false}},
			isInput: true,
		},
		{
			name:    "Required",
			schema:  &mockSchema{idSchema: idSchema{optional: false, computed: false, required: true}},
			isInput: true,
		},
		{
			name:    "Computed",
			schema:  &mockSchema{idSchema: idSchema{optional: false, computed: true, required: false}},
			isInput: false,
		},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.isInput, isInputProperty(tc.schema))
		})
	}
}

func TestMissingIDProperty(t *testing.T) {
	stderr, err := test(t, tfbridge.ProviderInfo{
		P: pfbridge.ShimProvider(testProvider{missingID: true}),
	})

	assert.Equal(t, "error: Resource test_res has a problem: no \"id\" attribute. "+
		"To map this resource consider specifying ResourceInfo.ComputeID\n", stderr)

	assert.ErrorContains(t, err, "There were 1 unresolved ID mapping errors")
}

func TestMissingIDWithOverride(t *testing.T) {
	stderr, err := test(t, tfbridge.ProviderInfo{
		P: pfbridge.ShimProvider(testProvider{missingID: true}),
		Resources: map[string]*tfbridge.ResourceInfo{
			"test_res": {ComputeID: func(context.Context, property.PropertyMap) (property.ID, error) {
				panic("ComputeID")
			}},
		},
	})

	assert.Empty(t, stderr)
	assert.NoError(t, err)
}

func TestSensitiveID(t *testing.T) {
	stderr, err := test(t, tfbridge.ProviderInfo{
		P: pfbridge.ShimProvider(testProvider{sensitiveID: true}),
	})

	//nolint:lll
	assert.Equal(t, "error: Resource test_res has a problem: \"id\" attribute is sensitive, but cannot be kept secret. To accept exposing ID, set `ProviderInfo.Resources[\"test_res\"].Fields[\"id\"].Secret = tfbridge.True()`\n", stderr)
	assert.ErrorContains(t, err, "There were 1 unresolved ID mapping errors")
}

func TestSensitiveIDWithOverride(t *testing.T) {
	t.Run("false", func(t *testing.T) {
		stderr, err := test(t, tfbridge.ProviderInfo{
			P: pfbridge.ShimProvider(testProvider{sensitiveID: true}),
			Resources: map[string]*tfbridge.ResourceInfo{
				"test_res": {Fields: map[string]*tfbridge.SchemaInfo{
					"id": {Secret: tfbridge.False()},
				}},
			},
		})
		assert.Empty(t, stderr)
		assert.NoError(t, err)
	})
	t.Run("true (no-op)", func(t *testing.T) {
		stderr, err := test(t, tfbridge.ProviderInfo{
			P: pfbridge.ShimProvider(testProvider{sensitiveID: true}),
			Resources: map[string]*tfbridge.ResourceInfo{
				"test_res": {Fields: map[string]*tfbridge.SchemaInfo{
					"id": {Secret: tfbridge.True()},
				}},
			},
		})
		assert.NotEmpty(t, stderr)
		assert.Error(t, err)
	})
}

func TestInvalidInputID(t *testing.T) {
	tests := []struct {
		name            string
		idSchema        idSchema
		isInputProperty bool
	}{
		{name: "Required", idSchema: idSchema{required: true, optional: false, computed: false}, isInputProperty: true},
		{name: "Optional", idSchema: idSchema{required: false, optional: true, computed: false}, isInputProperty: true},
		{name: "Computed", idSchema: idSchema{required: false, optional: false, computed: true}, isInputProperty: false},
		{
			name:            "Optional+Computed",
			idSchema:        idSchema{required: false, optional: true, computed: true},
			isInputProperty: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name+" no overrides", func(t *testing.T) {
			stderr, err := test(t, tfbridge.ProviderInfo{
				P: pfbridge.ShimProvider(testProvider{withID: &tc.idSchema}),
			})
			if tc.isInputProperty {
				assert.Error(t, err)
				autogold.Expect(`error: Resource test_res has a problem: an "id" input attribute is not allowed. `+
					`To map this resource specify SchemaInfo.Name and ResourceInfo.ComputeID
`).Equal(t, stderr)
			} else {
				assert.Empty(t, stderr)
				assert.NoError(t, err)
			}
		})

		// "id" is a required output property so if we remap "id" -> "otherId" now we
		// don't have an "id" output. We must create one by using `ComputeID`
		t.Run(tc.name+" overrides with Name and missing ComputeID", func(t *testing.T) {
			stderr, err := test(t, tfbridge.ProviderInfo{
				P: pfbridge.ShimProvider(testProvider{withID: &tc.idSchema}),
				Resources: map[string]*tfbridge.ResourceInfo{
					"test_res": {Fields: map[string]*tfbridge.SchemaInfo{
						"id": {Name: "otherId"},
					}},
				},
			})
			assert.Error(t, err)
			autogold.Expect(`error: Resource test_res has a problem: an "id" attribute with SchemaInfo.Name `+
				`must also specify ResourceInfo.ComputeID
`).Equal(t, stderr)
		})

		// Providing `ComputeID` handles remapping the output "id" to a different property, but
		// we still may have an input property "id"
		t.Run(tc.name+" overrides with ComputeID and missing Name", func(t *testing.T) {
			stderr, err := test(t, tfbridge.ProviderInfo{
				P: pfbridge.ShimProvider(testProvider{withID: &tc.idSchema}),
				Resources: map[string]*tfbridge.ResourceInfo{
					"test_res": {Fields: map[string]*tfbridge.SchemaInfo{
						"id": {},
					}, ComputeID: func(ctx context.Context, state property.PropertyMap) (property.ID, error) {
						panic("ComputeID")
					}},
				},
			})
			if tc.isInputProperty {
				assert.Error(t, err)
				autogold.Expect(`error: Resource test_res has a problem: an "id" input attribute is not allowed. `+
					`To map this resource specify SchemaInfo.Name and ResourceInfo.ComputeID
`).Equal(t, stderr)
			} else {
				assert.Empty(t, stderr)
				assert.NoError(t, err)
			}
		})
		// While remapping "id" may not make sense for an output property, it is still valid
		t.Run(tc.name+"no error (with override)", func(t *testing.T) {
			stderr, err := test(t, tfbridge.ProviderInfo{
				P: pfbridge.ShimProvider(testProvider{withID: &tc.idSchema}),
				Resources: map[string]*tfbridge.ResourceInfo{
					"test_res": {Fields: map[string]*tfbridge.SchemaInfo{
						"id": {Name: "otherId"},
					}, ComputeID: func(ctx context.Context, state property.PropertyMap) (property.ID, error) {
						panic("ComputeID")
					}},
				},
			})
			assert.Empty(t, stderr)
			assert.NoError(t, err)
		})
	}
}

func TestMuxedProvider(t *testing.T) {
	stderr, err := test(t, tfbridge.ProviderInfo{
		P: pfbridge.MuxShimWithPF(context.Background(),
			sdkv2.NewProvider(testSDKv2Provider()),
			testProvider{missingID: true}),
		Resources: map[string]*tfbridge.ResourceInfo{
			"test_res": {ComputeID: func(context.Context, property.PropertyMap) (property.ID, error) {
				panic("ComputeID")
			}},
		},
	})

	assert.Empty(t, stderr)
	assert.NoError(t, err)
}

func test(t *testing.T, info tfbridge.ProviderInfo) (string, error) {
	var stdout, stderr bytes.Buffer
	sink := diag.DefaultSink(&stdout, &stderr, diag.FormatOptions{
		Color: colors.Never,
	})

	err := Provider(sink, info)

	// We should not write diags to stdout
	assert.Empty(t, stdout.String())

	return stderr.String(), err
}

type (
	testProvider struct {
		provider.Provider

		missingID, sensitiveID bool
		withID                 *idSchema
	}
	testMissingIDResource   struct{ resource.Resource }
	testSensitiveIDResource struct{ resource.Resource }
	testIDResource          struct {
		resource.Resource
		idSchema
	}
)

func returnT[T any](t T) func() T { return func() T { return t } }

func (p testProvider) Resources(context.Context) []func() resource.Resource {
	var resources []func() resource.Resource
	if p.missingID {
		resources = append(resources, returnT[resource.Resource](testMissingIDResource{}))
	}
	if p.sensitiveID {
		resources = append(resources, returnT[resource.Resource](testSensitiveIDResource{}))
	}
	if p.withID != nil {
		resources = append(resources, returnT[resource.Resource](testIDResource{idSchema: *p.withID}))
	}
	return resources
}

func (testProvider) DataSources(context.Context) []func() datasource.DataSource { return nil }

func (testProvider) Metadata(_ context.Context, _ provider.MetadataRequest, req *provider.MetadataResponse) {
	req.TypeName = "test"
}

func (testMissingIDResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{}
}

func (testMissingIDResource) Metadata(
	_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse,
) {
	resp.TypeName = req.ProviderTypeName + "_res"
}

func testSDKv2Provider() *sdkschema.Provider {
	return &sdkschema.Provider{
		ResourcesMap: map[string]*sdkschema.Resource{
			"test_sdk": {},
		},
		DataSourcesMap: map[string]*sdkschema.Resource{},
	}
}

func (testSensitiveIDResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true, Sensitive: true},
		},
	}
}

func (testSensitiveIDResource) Metadata(
	_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse,
) {
	resp.TypeName = req.ProviderTypeName + "_res"
}

func (i testIDResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: i.idSchema.computed,
				Required: i.idSchema.required,
				Optional: i.idSchema.optional,
			},
		},
	}
}

func (testIDResource) Metadata(
	_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse,
) {
	resp.TypeName = req.ProviderTypeName + "_res"
}
