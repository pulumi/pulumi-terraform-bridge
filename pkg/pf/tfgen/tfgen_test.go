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

package tfgen

import (
	"bytes"
	"context"
	"encoding/json"
	"runtime"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	pschema "github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hexops/autogold/v2"
	pulumiSchema "github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/internal/providerbuilder"
	pftfbridge "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
)

// Regressing an issue with AWS provider not recognizing that assume_role config setting is singular via
// listvalidator.SizeAtMost(1).
func TestMaxItemsOne(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := pschema.Schema{
		Blocks: map[string]pschema.Block{
			"assume_role": pschema.ListNestedBlock{
				Validators: []validator.List{
					listvalidator.SizeAtMost(1),
				},
				NestedObject: pschema.NestedBlockObject{
					Attributes: map[string]pschema.Attribute{
						"external_id": pschema.StringAttribute{
							Optional:    true,
							Description: "A unique identifier that might be required when you assume a role in another account.",
						},
					},
				},
			},
		},
	}
	res, err := GenerateSchema(ctx, GenerateSchemaOptions{
		ProviderInfo: tfbridge.ProviderInfo{
			Name: "testprovider",
			P:    pftfbridge.ShimProvider(&schemaTestProvider{schema: s}),
		},
	})
	require.NoError(t, err)

	var schema pulumiSchema.PackageSpec
	if err := json.Unmarshal(res.ProviderMetadata.PackageSchema, &schema); err != nil {
		t.Fatal(err)
	}

	require.Contains(t, schema.Config.Variables, "assumeRole")
	require.NotContains(t, schema.Config.Variables, "assumeRoles")
}

type schemaTestProvider struct {
	schema    pschema.Schema
	resources map[string]rschema.Schema
}

func (*schemaTestProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "test_"
}

func (p *schemaTestProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = p.schema
}

func (*schemaTestProvider) Configure(context.Context, provider.ConfigureRequest, *provider.ConfigureResponse) {
	panic("NOT IMPLEMENTED")
}

func (*schemaTestProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return nil
}

func (p *schemaTestProvider) Resources(context.Context) []func() resource.Resource {
	r := make([]func() resource.Resource, 0, len(p.resources))
	for k, v := range p.resources {
		r = append(r, makeTestResource(k, v))
	}
	return r
}

func makeTestResource(name string, schema rschema.Schema) func() resource.Resource {
	return func() resource.Resource { return schemaTestResource{name, schema} }
}

type schemaTestResource struct {
	name   string
	schema rschema.Schema
}

func (r schemaTestResource) Metadata(
	_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse,
) {
	resp.TypeName = req.ProviderTypeName + r.name
}

func (r schemaTestResource) Schema(
	_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse,
) {
	resp.Schema = r.schema
}

func (r schemaTestResource) Create(context.Context, resource.CreateRequest, *resource.CreateResponse) {
	panic(r.name)
}

func (r schemaTestResource) Read(context.Context, resource.ReadRequest, *resource.ReadResponse) {
	panic(r.name)
}

func (r schemaTestResource) Update(context.Context, resource.UpdateRequest, *resource.UpdateResponse) {
	panic(r.name)
}

func (r schemaTestResource) Delete(context.Context, resource.DeleteRequest, *resource.DeleteResponse) {
	panic(r.name)
}

func TestTypeOverride(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skipf("Skipping on windows - tests cases need to be made robust to newline handling")
	}

	tests := []struct {
		name          string
		schema        rschema.Schema
		info          *tfbridge.ResourceInfo
		expectedError autogold.Value
	}{
		{
			name: "no-override",
			schema: rschema.Schema{
				Attributes: map[string]rschema.Attribute{
					"a1": rschema.StringAttribute{Optional: true},
				},
				Blocks: map[string]rschema.Block{
					"b1": rschema.SingleNestedBlock{
						Attributes: map[string]rschema.Attribute{
							"a1": rschema.StringAttribute{Optional: true},
						},
					},
				},
			},
		},
		{
			name: "attr-single-nested-object-element",
			schema: rschema.Schema{
				Attributes: map[string]rschema.Attribute{
					"a1": rschema.SingleNestedAttribute{
						Optional: true,
						Attributes: map[string]rschema.Attribute{
							"n1": rschema.StringAttribute{Optional: true},
						},
					},
				},
			},
			info: &tfbridge.ResourceInfo{Fields: map[string]*tfbridge.SchemaInfo{
				"a1": {Elem: &tfbridge.SchemaInfo{
					Fields: map[string]*tfbridge.SchemaInfo{
						"n1": {Type: "number"},
					},
				}},
			}},
		},
		{
			// This test case reproduces https://github.com/pulumi/pulumi-terraform-bridge/issues/2185
			name: "attr-single-nested-object",
			schema: rschema.Schema{
				Attributes: map[string]rschema.Attribute{
					"a1": rschema.SingleNestedAttribute{
						Optional: true,
						Attributes: map[string]rschema.Attribute{
							"n1": rschema.StringAttribute{Optional: true},
						},
					},
				},
			},
			info: &tfbridge.ResourceInfo{Fields: map[string]*tfbridge.SchemaInfo{
				"a1": {Elem: &tfbridge.SchemaInfo{
					Type: "testprovider:index:SomeOtherType",
				}},
			}},
		},
		{
			name: "attr-list-nested-object-object",
			schema: rschema.Schema{
				Attributes: map[string]rschema.Attribute{
					"a1": rschema.ListNestedAttribute{
						NestedObject: rschema.NestedAttributeObject{
							Attributes: map[string]rschema.Attribute{
								"n1": rschema.StringAttribute{Optional: true},
							},
						},
					},
				},
			},
			info: &tfbridge.ResourceInfo{Fields: map[string]*tfbridge.SchemaInfo{
				"a1": {
					MaxItemsOne: tfbridge.True(),
					Elem: &tfbridge.SchemaInfo{
						Elem: &tfbridge.SchemaInfo{
							Fields: map[string]*tfbridge.SchemaInfo{
								"n1": {Name: "foo"},
							},
						},
					},
				},
			}},
		},
		{
			name: "invalid-attr-single-nested-object-fields",
			schema: rschema.Schema{
				Attributes: map[string]rschema.Attribute{
					"a1": rschema.SingleNestedAttribute{
						Optional: true,
						Attributes: map[string]rschema.Attribute{
							"n1": rschema.StringAttribute{Optional: true},
						},
					},
				},
			},
			info: &tfbridge.ResourceInfo{Fields: map[string]*tfbridge.SchemaInfo{
				"a1": {Fields: map[string]*tfbridge.SchemaInfo{
					"invalid": {},
				}},
			}},
			expectedError: autogold.Expect("test_res: [{a1}]: .Fields should be .Elem.Fields"),
		},
		{
			name: "attr-map-element",
			schema: rschema.Schema{
				Attributes: map[string]rschema.Attribute{
					"a1": rschema.MapAttribute{
						Optional:    true,
						ElementType: types.StringType,
					},
				},
			},
			info: &tfbridge.ResourceInfo{
				Fields: map[string]*tfbridge.SchemaInfo{
					"a1": {Elem: &tfbridge.SchemaInfo{
						Type: "number",
					}},
				},
			},
		},
		{
			name: "attr-map-object-element",
			schema: rschema.Schema{
				Attributes: map[string]rschema.Attribute{
					"a1": rschema.MapNestedAttribute{
						Optional: true,
						NestedObject: rschema.NestedAttributeObject{
							Attributes: map[string]rschema.Attribute{
								"n1": rschema.StringAttribute{Optional: true},
							},
						},
					},
				},
			},
			info: &tfbridge.ResourceInfo{
				Fields: map[string]*tfbridge.SchemaInfo{
					"a1": {Elem: &tfbridge.SchemaInfo{
						Elem: &tfbridge.SchemaInfo{
							Fields: map[string]*tfbridge.SchemaInfo{
								"n1": {Type: "number"},
							},
						},
					}},
				},
			},
		},
		{
			name: "invalid-attr-map-fields",
			schema: rschema.Schema{
				Attributes: map[string]rschema.Attribute{
					"a1": rschema.MapAttribute{
						Optional:    true,
						ElementType: types.StringType,
					},
				},
			},
			info: &tfbridge.ResourceInfo{
				Fields: map[string]*tfbridge.SchemaInfo{
					"a1": {Fields: map[string]*tfbridge.SchemaInfo{
						"invalid": {},
					}},
				},
			},
			expectedError: autogold.Expect("test_res: [{a1}]: cannot specify .Fields on a List[T], Set[T] or Map[T] type"),
		},
		{
			name: "invalid-attr-map-max-items-one",
			schema: rschema.Schema{
				Attributes: map[string]rschema.Attribute{
					"a1": rschema.MapAttribute{
						Optional:    true,
						ElementType: types.StringType,
					},
				},
			},
			info: &tfbridge.ResourceInfo{
				Fields: map[string]*tfbridge.SchemaInfo{
					"a1": {MaxItemsOne: tfbridge.True()},
				},
			},
			expectedError: autogold.Expect("test_res: [{a1}]: can only specify .MaxItemsOne on List[T] or Set[T] type"),
		},
		{
			name: "attr-set-element",
			schema: rschema.Schema{
				Attributes: map[string]rschema.Attribute{
					"a1": rschema.SetAttribute{
						Optional:    true,
						ElementType: types.StringType,
					},
				},
			},
			info: &tfbridge.ResourceInfo{
				Fields: map[string]*tfbridge.SchemaInfo{
					"a1": {Elem: &tfbridge.SchemaInfo{
						Type: "number",
					}},
				},
			},
		},
		{
			name: "invalid-attr-map-fields",
			schema: rschema.Schema{
				Attributes: map[string]rschema.Attribute{
					"a1": rschema.SetAttribute{
						Optional:    true,
						ElementType: types.StringType,
					},
				},
			},
			info: &tfbridge.ResourceInfo{
				Fields: map[string]*tfbridge.SchemaInfo{
					"a1": {Fields: map[string]*tfbridge.SchemaInfo{
						"invalid": {},
					}},
				},
			},
			expectedError: autogold.Expect("test_res: [{a1}]: cannot specify .Fields on a List[T], Set[T] or Map[T] type"),
		},
		{
			name: "attr-list-element",
			schema: rschema.Schema{
				Attributes: map[string]rschema.Attribute{
					"a1": rschema.ListAttribute{
						Optional:    true,
						ElementType: types.StringType,
					},
				},
			},
			info: &tfbridge.ResourceInfo{
				Fields: map[string]*tfbridge.SchemaInfo{
					"a1": {Elem: &tfbridge.SchemaInfo{
						Type: "number",
					}},
				},
			},
		},
		{
			name: "attr-list-max-items-one",
			schema: rschema.Schema{
				Attributes: map[string]rschema.Attribute{
					"a1": rschema.ListAttribute{
						Optional:    true,
						ElementType: types.StringType,
					},
				},
			},
			info: &tfbridge.ResourceInfo{
				Fields: map[string]*tfbridge.SchemaInfo{
					"a1": {MaxItemsOne: tfbridge.True()},
				},
			},
		},
		{
			name: "attr-override-map-fields",
			schema: rschema.Schema{
				Attributes: map[string]rschema.Attribute{
					"a1": rschema.ListAttribute{
						Optional:    true,
						ElementType: types.StringType,
					},
				},
			},
			info: &tfbridge.ResourceInfo{
				Fields: map[string]*tfbridge.SchemaInfo{
					"a1": {Fields: map[string]*tfbridge.SchemaInfo{
						"invalid": {},
					}},
				},
			},
			expectedError: autogold.Expect("test_res: [{a1}]: cannot specify .Fields on a List[T], Set[T] or Map[T] type"),
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			if tt.info == nil {
				tt.info = &tfbridge.ResourceInfo{}
			}
			tt.info.Tok = "testprovider:index:Res"
			tt.info.Docs = &tfbridge.DocInfo{Markdown: []byte{' '}}
			if _, ok := tt.schema.Attributes["id"]; !ok {
				tt.schema.Attributes["id"] = rschema.StringAttribute{Computed: true}
			}
			res, err := GenerateSchema(ctx, GenerateSchemaOptions{
				ProviderInfo: tfbridge.ProviderInfo{
					Name:             "testprovider",
					UpstreamRepoPath: ".", // no invalid mappings warnings
					P: pftfbridge.ShimProvider(&schemaTestProvider{
						resources: map[string]rschema.Schema{
							"res": tt.schema,
						},
					}),
					Resources: map[string]*tfbridge.ResourceInfo{
						"test_res": tt.info,
					},
					// Trim the schema for easier comparison
					SchemaPostProcessor: func(p *pulumiSchema.PackageSpec) {
						p.Language = nil
						p.Provider.Description = ""
					},
				},
			})
			if tt.expectedError != nil {
				require.Error(t, err)
				tt.expectedError.Equal(t, err.Error())
				return
			}
			require.NoError(t, err)
			var b bytes.Buffer
			require.NoError(t, json.Indent(&b, res.ProviderMetadata.PackageSchema, "", "    "))
			autogold.ExpectFile(t, autogold.Raw(b.String()))
		})
	}
}

func TestDataSourceAttributeVsBlockSchemaNameOverride(t *testing.T) {
	t.Parallel()

	attrRes := dschema.Schema{
		Attributes: map[string]dschema.Attribute{
			"id": dschema.StringAttribute{Computed: true},
			"collection": dschema.ListNestedAttribute{
				NestedObject: dschema.NestedAttributeObject{
					Attributes: map[string]dschema.Attribute{
						"prop": dschema.StringAttribute{
							Optional: true,
						},
					},
				},
			},
		},
	}

	blockRes := dschema.Schema{
		Attributes: map[string]dschema.Attribute{
			"id": dschema.StringAttribute{Computed: true},
		},
		Blocks: map[string]dschema.Block{
			"collection": dschema.ListNestedBlock{
				NestedObject: dschema.NestedBlockObject{
					Attributes: map[string]dschema.Attribute{
						"prop": dschema.StringAttribute{
							Optional: true,
						},
					},
				},
			},
		},
	}

	prov := providerbuilder.NewProvider(
		providerbuilder.NewProviderArgs{
			AllDataSources: []providerbuilder.DataSource{
				{
					Name:             "attr_res",
					DataSourceSchema: attrRes,
				},
				{
					Name:             "block_res",
					DataSourceSchema: blockRes,
				},
			},
		},
	)

	providerInfo := prov.ToProviderInfo()

	providerInfo.DataSources["testprovider_attr_res"].Fields = map[string]*tfbridge.SchemaInfo{
		"collection": {
			Name: "collection1",
			Elem: &tfbridge.SchemaInfo{
				Fields: map[string]*tfbridge.SchemaInfo{
					"prop": {Name: "prop1"},
				},
			},
		},
	}

	providerInfo.DataSources["testprovider_block_res"].Fields = map[string]*tfbridge.SchemaInfo{
		"collection": {
			Name: "collection1",
			Elem: &tfbridge.SchemaInfo{
				Fields: map[string]*tfbridge.SchemaInfo{
					"prop": {Name: "prop1"},
				},
			},
		},
	}

	res, err := GenerateSchema(context.Background(), GenerateSchemaOptions{
		ProviderInfo: providerInfo,
	})
	require.NoError(t, err)
	var prettyJSON bytes.Buffer
	if err := json.Indent(&prettyJSON, res.ProviderMetadata.PackageSchema, "", "    "); err != nil {
		t.Fatal(err)
	}

	autogold.ExpectFile(t, autogold.Raw(prettyJSON.String()))
}

func TestResourceAttributeVsBlockSchemaNameOverride(t *testing.T) {
	t.Skip("TODO[pulumi/pulumi-terraform-bridge#2803] Skip until fixed")
	t.Parallel()

	attrRes := rschema.Schema{
		Attributes: map[string]rschema.Attribute{
			"id": rschema.StringAttribute{Computed: true},
			"collection": rschema.ListNestedAttribute{
				NestedObject: rschema.NestedAttributeObject{
					Attributes: map[string]rschema.Attribute{
						"prop": rschema.StringAttribute{
							Optional: true,
						},
					},
				},
			},
		},
	}

	blockRes := rschema.Schema{
		Attributes: map[string]rschema.Attribute{
			"id": rschema.StringAttribute{Computed: true},
		},
		Blocks: map[string]rschema.Block{
			"collection": rschema.ListNestedBlock{
				NestedObject: rschema.NestedBlockObject{
					Attributes: map[string]rschema.Attribute{
						"prop": rschema.StringAttribute{
							Optional: true,
						},
					},
				},
			},
		},
	}

	prov := providerbuilder.NewProvider(
		providerbuilder.NewProviderArgs{
			AllResources: []providerbuilder.Resource{
				{
					Name:           "attr_res",
					ResourceSchema: attrRes,
				},
				{
					Name:           "block_res",
					ResourceSchema: blockRes,
				},
			},
		},
	)

	providerInfo := prov.ToProviderInfo()

	providerInfo.Resources["testprovider_attr_res"].Fields = map[string]*tfbridge.SchemaInfo{
		"collection": {
			Name: "collection1",
			Elem: &tfbridge.SchemaInfo{
				Fields: map[string]*tfbridge.SchemaInfo{
					"prop": {Name: "prop1"},
				},
			},
		},
	}

	providerInfo.Resources["testprovider_block_res"].Fields = map[string]*tfbridge.SchemaInfo{
		"collection": {
			Name: "collection1",
			Elem: &tfbridge.SchemaInfo{
				Fields: map[string]*tfbridge.SchemaInfo{
					"prop": {Name: "prop1"},
				},
			},
		},
	}

	res, err := GenerateSchema(context.Background(), GenerateSchemaOptions{
		ProviderInfo: providerInfo,
	})
	require.NoError(t, err)
	var prettyJSON bytes.Buffer
	if err := json.Indent(&prettyJSON, res.ProviderMetadata.PackageSchema, "", "    "); err != nil {
		t.Fatal(err)
	}

	autogold.ExpectFile(t, autogold.Raw(prettyJSON.String()))
}

func TestRegressDatabricksNestedCsharpOverrideBlocksSchema(t *testing.T) {
	t.Parallel()
	dsSchemaBlocks := dschema.Schema{
		Blocks: map[string]dschema.Block{
			"endpoints": dschema.ListNestedBlock{
				NestedObject: dschema.NestedBlockObject{
					Blocks: map[string]dschema.Block{
						"ai_gateway": dschema.ListNestedBlock{
							NestedObject: dschema.NestedBlockObject{
								Blocks: map[string]dschema.Block{
									"guardrails": dschema.ListNestedBlock{
										NestedObject: dschema.NestedBlockObject{
											Blocks: map[string]dschema.Block{
												"input": dschema.ListNestedBlock{
													NestedObject: dschema.NestedBlockObject{
														Attributes: map[string]dschema.Attribute{
															"max_batch_size": dschema.Int64Attribute{},
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
				},
			},
		},
	}

	schemaProvider := providerbuilder.NewProvider(
		providerbuilder.NewProviderArgs{
			AllDataSources: []providerbuilder.DataSource{
				{
					Name:             "serving_endpoints",
					DataSourceSchema: dsSchemaBlocks,
					ReadFunc: func(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
					},
				},
			},
		},
	)
	providerInfo := schemaProvider.ToProviderInfo()

	providerInfo.DataSources["testprovider_serving_endpoints"].Fields = map[string]*info.Schema{
		"endpoints": {
			Name: "endpoints1",
			Elem: &info.Schema{
				Fields: map[string]*info.Schema{
					"ai_gateway": {
						Name: "aiGateway1",
						Elem: &info.Schema{
							Fields: map[string]*info.Schema{
								"guardrails": {
									Name: "guardrails1",
									Elem: &info.Schema{
										Fields: map[string]*info.Schema{
											"input": {
												Name: "input1",
												Elem: &info.Schema{
													Fields: map[string]*info.Schema{
														"max_batch_size": {
															CSharpName: "MaxBatchSize1",
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
				},
			},
		},
	}

	res, err := GenerateSchema(context.Background(), GenerateSchemaOptions{
		ProviderInfo: providerInfo,
	})
	require.NoError(t, err)
	var prettyJSON bytes.Buffer
	if err := json.Indent(&prettyJSON, res.ProviderMetadata.PackageSchema, "", "    "); err != nil {
		t.Fatal(err)
	}

	autogold.ExpectFile(t, autogold.Raw(prettyJSON.String()))
}

func TestRegressDatabricksNestedCsharpOverrideAttributesSchema(t *testing.T) {
	t.Parallel()
	dsSchemaAttributes := dschema.Schema{
		Attributes: map[string]dschema.Attribute{
			"endpoints": dschema.ListNestedAttribute{
				NestedObject: dschema.NestedAttributeObject{
					Attributes: map[string]dschema.Attribute{
						"ai_gateway": dschema.ListNestedAttribute{
							NestedObject: dschema.NestedAttributeObject{
								Attributes: map[string]dschema.Attribute{
									"guardrails": dschema.ListNestedAttribute{
										NestedObject: dschema.NestedAttributeObject{
											Attributes: map[string]dschema.Attribute{
												"input": dschema.ListNestedAttribute{
													NestedObject: dschema.NestedAttributeObject{
														Attributes: map[string]dschema.Attribute{
															"max_batch_size": dschema.Int64Attribute{},
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
				},
			},
		},
	}

	schemaProvider := providerbuilder.NewProvider(
		providerbuilder.NewProviderArgs{
			AllDataSources: []providerbuilder.DataSource{
				{
					Name:             "serving_endpoints",
					DataSourceSchema: dsSchemaAttributes,
					ReadFunc: func(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
					},
				},
			},
		},
	)
	providerInfo := schemaProvider.ToProviderInfo()

	providerInfo.DataSources["testprovider_serving_endpoints"].Fields = map[string]*info.Schema{
		"endpoints": {
			Name: "endpoints1",
			Elem: &info.Schema{
				Fields: map[string]*info.Schema{
					"ai_gateway": {
						Name: "aiGateway1",
						Elem: &info.Schema{
							Fields: map[string]*info.Schema{
								"guardrails": {
									Name: "guardrails1",
									Elem: &info.Schema{
										Fields: map[string]*info.Schema{
											"input": {
												Name: "input1",
												Elem: &info.Schema{
													Fields: map[string]*info.Schema{
														"max_batch_size": {
															CSharpName: "MaxBatchSize1",
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
				},
			},
		},
	}

	res, err := GenerateSchema(context.Background(), GenerateSchemaOptions{
		ProviderInfo: providerInfo,
	})
	require.NoError(t, err)
	var prettyJSON bytes.Buffer
	if err := json.Indent(&prettyJSON, res.ProviderMetadata.PackageSchema, "", "    "); err != nil {
		t.Fatal(err)
	}

	autogold.ExpectFile(t, autogold.Raw(prettyJSON.String()))
}
