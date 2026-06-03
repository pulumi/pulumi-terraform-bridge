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
	tflist "github.com/hashicorp/terraform-plugin-framework/list"
	lschema "github.com/hashicorp/terraform-plugin-framework/list/schema"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	prschema "github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	sdkschema "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hexops/autogold/v2"
	puschema "github.com/pulumi/pulumi/pkg/v3/codegen/schema" //nolint:importas
	"github.com/stretchr/testify/require"

	pfmuxer "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/internal/muxer"
	pftfbridge "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	sdkv2shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v2"
)

// Regressing an issue with AWS provider not recognizing that assume_role config setting is singular via
// listvalidator.SizeAtMost(1).
func TestMaxItemsOne(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := prschema.Schema{
		Blocks: map[string]prschema.Block{
			"assume_role": prschema.ListNestedBlock{
				Validators: []validator.List{
					listvalidator.SizeAtMost(1),
				},
				NestedObject: prschema.NestedBlockObject{
					Attributes: map[string]prschema.Attribute{
						"external_id": prschema.StringAttribute{
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

	var schema puschema.PackageSpec
	if err := json.Unmarshal(res.ProviderMetadata.PackageSchema, &schema); err != nil {
		t.Fatal(err)
	}

	require.Contains(t, schema.Config.Variables, "assumeRole")
	require.NotContains(t, schema.Config.Variables, "assumeRoles")
}

type schemaTestProvider struct {
	schema      prschema.Schema
	resources   map[string]rschema.Schema
	dataSources map[string]dschema.Schema
	lists       map[string]lschema.Schema
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

func (p *schemaTestProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	r := make([]func() datasource.DataSource, 0, len(p.dataSources))
	for k, v := range p.dataSources {
		r = append(r, makeTestDataSource(k, v))
	}
	return r
}

func (p *schemaTestProvider) Resources(context.Context) []func() resource.Resource {
	r := make([]func() resource.Resource, 0, len(p.resources))
	for k, v := range p.resources {
		r = append(r, makeTestResource(k, v))
	}
	return r
}

func (p *schemaTestProvider) ListResources(context.Context) []func() tflist.ListResource {
	r := make([]func() tflist.ListResource, 0, len(p.lists))
	for k, v := range p.lists {
		r = append(r, makeTestListResource(k, v))
	}
	return r
}

func makeTestListResource(name string, schema lschema.Schema) func() tflist.ListResource {
	return func() tflist.ListResource { return schemaTestListResource{name, schema} }
}

type schemaTestListResource struct {
	name   string
	schema lschema.Schema
}

func (r schemaTestListResource) Metadata(
	_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse,
) {
	resp.TypeName = req.ProviderTypeName + r.name
}

func (r schemaTestListResource) ListResourceConfigSchema(
	_ context.Context, _ tflist.ListResourceSchemaRequest, resp *tflist.ListResourceSchemaResponse,
) {
	resp.Schema = r.schema
}

func (r schemaTestListResource) List(
	_ context.Context, _ tflist.ListRequest, _ *tflist.ListResultsStream,
) {
	panic(r.name)
}

func TestListInputs(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	p := &schemaTestProvider{
		resources: map[string]rschema.Schema{
			"thing": {
				Attributes: map[string]rschema.Attribute{
					"name": rschema.StringAttribute{
						Required: true,
					},
				},
			},
		},
		lists: map[string]lschema.Schema{
			"thing": {
				Attributes: map[string]lschema.Attribute{
					"parent_id": lschema.StringAttribute{
						Required:    true,
						Description: "The parent identifier to list things under.",
					},
					"prefix": lschema.StringAttribute{
						Optional:    true,
						Description: "An optional name prefix filter.",
					},
				},
			},
		},
	}

	res, err := GenerateSchema(ctx, GenerateSchemaOptions{
		ProviderInfo: tfbridge.ProviderInfo{
			Name: "testprovider",
			P:    pftfbridge.ShimProvider(p),
			Resources: map[string]*tfbridge.ResourceInfo{
				"test_thing": {
					Tok: "testprovider:index:Thing",
				},
			},
		},
		XInMemoryDocs: true,
	})
	require.NoError(t, err)

	var schema puschema.PackageSpec
	require.NoError(t, json.Unmarshal(res.ProviderMetadata.PackageSchema, &schema))

	resource := schema.Resources["testprovider:index:Thing"]
	require.NotNil(t, resource.ListInputs)
	require.Equal(t, "object", resource.ListInputs.Type)
	require.Equal(t, []string{"parentId"}, resource.ListInputs.Required)
	require.Contains(t, resource.ListInputs.Properties, "parentId")
	require.Contains(t, resource.ListInputs.Properties, "prefix")
}

func TestListInputsForSDKv2ResourceFromPFListResource(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	sdkProvider := &sdkschema.Provider{
		Schema: map[string]*sdkschema.Schema{},
		ResourcesMap: map[string]*sdkschema.Resource{
			"test_thing": {
				Schema: map[string]*sdkschema.Schema{
					"id": {
						Type:     sdkschema.TypeString,
						Computed: true,
					},
				},
			},
		},
	}
	pfListProvider := &schemaTestProvider{
		lists: map[string]lschema.Schema{
			"thing": {
				Attributes: map[string]lschema.Attribute{
					"parent_id": lschema.StringAttribute{Required: true},
				},
			},
		},
	}
	providerInfo := tfbridge.ProviderInfo{
		Name: "testprovider",
		P: pftfbridge.MuxShimWithPF(ctx,
			sdkv2shim.NewProvider(sdkProvider),
			pfListProvider),
		Resources: map[string]*tfbridge.ResourceInfo{
			"test_thing": {
				Tok: "testprovider:index:Thing",
			},
		},
	}

	res, err := GenerateSchema(ctx, GenerateSchemaOptions{
		ProviderInfo:  providerInfo,
		XInMemoryDocs: true,
	})
	require.NoError(t, err)

	var schema puschema.PackageSpec
	require.NoError(t, json.Unmarshal(res.ProviderMetadata.PackageSchema, &schema))
	require.NotNil(t, schema.Resources["testprovider:index:Thing"].ListInputs)
	require.Contains(t, schema.Resources["testprovider:index:Thing"].ListInputs.Properties, "parentId")

	dispatch, err := providerInfo.P.(*pfmuxer.ProviderShim).ResolveDispatch(&providerInfo)
	require.NoError(t, err)
	require.Equal(t, 0, dispatch.Resources["testprovider:index:Thing"])
	require.Equal(t, 1, dispatch.ListResources["testprovider:index:Thing"])
}

func TestListInputsEmptySchema(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	p := &schemaTestProvider{
		resources: map[string]rschema.Schema{
			"thing": {
				Attributes: map[string]rschema.Attribute{
					"name": rschema.StringAttribute{Required: true},
				},
			},
		},
		lists: map[string]lschema.Schema{
			"thing": {},
		},
	}

	res, err := GenerateSchema(ctx, GenerateSchemaOptions{
		ProviderInfo: tfbridge.ProviderInfo{
			Name: "testprovider",
			P:    pftfbridge.ShimProvider(p),
			Resources: map[string]*tfbridge.ResourceInfo{
				"test_thing": {
					Tok: "testprovider:index:Thing",
				},
			},
		},
		XInMemoryDocs: true,
	})
	require.NoError(t, err)

	var schema puschema.PackageSpec
	require.NoError(t, json.Unmarshal(res.ProviderMetadata.PackageSchema, &schema))

	resource := schema.Resources["testprovider:index:Thing"]
	require.NotNil(t, resource.ListInputs)
	require.Equal(t, "object", resource.ListInputs.Type)
	require.Empty(t, resource.ListInputs.Properties)
	require.Empty(t, resource.ListInputs.Required)
}

func TestListInputsOmittedWithoutListResource(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	p := &schemaTestProvider{
		resources: map[string]rschema.Schema{
			"thing": {
				Attributes: map[string]rschema.Attribute{
					"name": rschema.StringAttribute{Required: true},
				},
			},
		},
	}

	res, err := GenerateSchema(ctx, GenerateSchemaOptions{
		ProviderInfo: tfbridge.ProviderInfo{
			Name: "testprovider",
			P:    pftfbridge.ShimProvider(p),
			Resources: map[string]*tfbridge.ResourceInfo{
				"test_thing": {
					Tok: "testprovider:index:Thing",
				},
			},
		},
		XInMemoryDocs: true,
	})
	require.NoError(t, err)

	var schema puschema.PackageSpec
	require.NoError(t, json.Unmarshal(res.ProviderMetadata.PackageSchema, &schema))

	resource := schema.Resources["testprovider:index:Thing"]
	require.Nil(t, resource.ListInputs)
}

func makeTestResource(name string, schema rschema.Schema) func() resource.Resource {
	return func() resource.Resource { return schemaTestResource{name, schema} }
}

func makeTestDataSource(name string, schema dschema.Schema) func() datasource.DataSource {
	return func() datasource.DataSource { return schemaTestDataSource{name, schema} }
}

type schemaTestDataSource struct {
	name   string
	schema dschema.Schema
}

func (d schemaTestDataSource) Metadata(
	_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse,
) {
	resp.TypeName = req.ProviderTypeName + d.name
}

func (d schemaTestDataSource) Schema(
	_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse,
) {
	resp.Schema = d.schema
}

func (d schemaTestDataSource) Read(context.Context, datasource.ReadRequest, *datasource.ReadResponse) {
	panic(d.name)
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
					SchemaPostProcessor: func(p *puschema.PackageSpec) {
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

func TestWriteOnlyAttributesGenerateToSchema(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skipf("Skipping on windows - tests cases need to be made robust to newline handling")
	}

	schema := rschema.Schema{
		Attributes: map[string]rschema.Attribute{
			"a1": rschema.StringAttribute{
				Optional:  true,
				WriteOnly: true,
			},
		},
	}

	info := &tfbridge.ResourceInfo{
		Tok:  "testprovider:index:Res",
		Docs: &tfbridge.DocInfo{Markdown: []byte{' '}},
	}

	if _, ok := schema.Attributes["id"]; !ok {
		schema.Attributes["id"] = rschema.StringAttribute{Computed: true}
	}

	res, err := GenerateSchema(context.Background(), GenerateSchemaOptions{
		ProviderInfo: tfbridge.ProviderInfo{
			Name:             "testprovider",
			UpstreamRepoPath: ".", // no invalid mappings warnings
			P: pftfbridge.ShimProvider(&schemaTestProvider{
				resources: map[string]rschema.Schema{
					"res": schema,
				},
			}),
			Resources: map[string]*tfbridge.ResourceInfo{
				"test_res": info,
			},
			// Trim the schema for easier comparison
			SchemaPostProcessor: func(p *puschema.PackageSpec) {
				p.Language = nil
				p.Provider.Description = ""
			},
		},
	})
	require.NoError(t, err)
	var b bytes.Buffer
	require.NoError(t, json.Indent(&b, res.ProviderMetadata.PackageSchema, "", "    "))
	autogold.ExpectFile(t, autogold.Raw(b.String()))
}

func TestGenerateSchemaFailsOnInvalidPFResourceSchemaImplementation(t *testing.T) {
	t.Parallel()

	_, err := GenerateSchema(context.Background(), GenerateSchemaOptions{
		ProviderInfo: tfbridge.ProviderInfo{
			Name:             "testprovider",
			UpstreamRepoPath: ".", // no invalid mappings warnings
			P: pftfbridge.ShimProvider(&schemaTestProvider{
				resources: map[string]rschema.Schema{
					"res": invalidResourceSchemaImplementation(),
				},
			}),
			Resources: map[string]*tfbridge.ResourceInfo{
				"test_res": {
					Tok:  "testprovider:index:Res",
					Docs: &tfbridge.DocInfo{Markdown: []byte{' '}},
				},
			},
		},
	})
	require.ErrorContains(t, err, "Plugin Framework resource test_res ValidateImplementation failed")
	require.ErrorContains(t, err, `Attribute "a1" must be computed when using default`)
}

func TestGenerateSchemaFailsOnTokenlessPFResourceSchemaImplementation(t *testing.T) {
	t.Parallel()

	_, err := GenerateSchema(context.Background(), GenerateSchemaOptions{
		ProviderInfo: tfbridge.ProviderInfo{
			Name:             "testprovider",
			UpstreamRepoPath: ".", // no invalid mappings warnings
			P: pftfbridge.ShimProvider(&schemaTestProvider{
				resources: map[string]rschema.Schema{
					"res": invalidResourceSchemaImplementation(),
				},
			}),
			Resources: map[string]*tfbridge.ResourceInfo{
				"test_res": {Docs: &tfbridge.DocInfo{Markdown: []byte{' '}}},
			},
		},
	})
	require.ErrorContains(t, err, "Plugin Framework resource test_res ValidateImplementation failed")
	require.ErrorContains(t, err, `Attribute "a1" must be computed when using default`)
}

func TestGenerateSchemaFailsOnPFProviderSchemaImplementation(t *testing.T) {
	t.Parallel()

	_, err := GenerateSchema(context.Background(), GenerateSchemaOptions{
		ProviderInfo: tfbridge.ProviderInfo{
			Name:             "testprovider",
			UpstreamRepoPath: ".", // no invalid mappings warnings
			P: pftfbridge.ShimProvider(&schemaTestProvider{
				schema: invalidProviderSchemaImplementation(),
			}),
		},
	})
	require.ErrorContains(t, err, "Plugin Framework provider test_ ValidateImplementation failed")
	require.ErrorContains(t, err, "missing the CustomType or ElementType field")
}

func TestGenerateSchemaFailsOnPFDataSourceSchemaImplementation(t *testing.T) {
	t.Parallel()

	_, err := GenerateSchema(context.Background(), GenerateSchemaOptions{
		ProviderInfo: tfbridge.ProviderInfo{
			Name:             "testprovider",
			UpstreamRepoPath: ".", // no invalid mappings warnings
			P: pftfbridge.ShimProvider(&schemaTestProvider{
				dataSources: map[string]dschema.Schema{
					"lookup": invalidDataSourceSchemaImplementation(),
				},
			}),
			DataSources: map[string]*tfbridge.DataSourceInfo{
				"test_lookup": {
					Tok:  "testprovider:index:getLookup",
					Docs: &tfbridge.DocInfo{Markdown: []byte{' '}},
				},
			},
		},
	})
	require.ErrorContains(t, err, "Plugin Framework data source test_lookup ValidateImplementation failed")
	require.ErrorContains(t, err, "missing the CustomType or ElementType field")
}

func TestGenerateSchemaFailsOnTokenlessPFDataSourceSchemaImplementation(t *testing.T) {
	t.Parallel()

	_, err := GenerateSchema(context.Background(), GenerateSchemaOptions{
		ProviderInfo: tfbridge.ProviderInfo{
			Name:             "testprovider",
			UpstreamRepoPath: ".", // no invalid mappings warnings
			P: pftfbridge.ShimProvider(&schemaTestProvider{
				dataSources: map[string]dschema.Schema{
					"lookup": invalidDataSourceSchemaImplementation(),
				},
			}),
			DataSources: map[string]*tfbridge.DataSourceInfo{
				"test_lookup": {Docs: &tfbridge.DocInfo{Markdown: []byte{' '}}},
			},
		},
	})
	require.ErrorContains(t, err, "Plugin Framework data source test_lookup ValidateImplementation failed")
	require.ErrorContains(t, err, "missing the CustomType or ElementType field")
}

func TestGenerateSchemaFailsOnPFListResourceSchemaImplementation(t *testing.T) {
	t.Parallel()

	_, err := GenerateSchema(context.Background(), GenerateSchemaOptions{
		ProviderInfo: tfbridge.ProviderInfo{
			Name:             "testprovider",
			UpstreamRepoPath: ".", // no invalid mappings warnings
			P: pftfbridge.ShimProvider(&schemaTestProvider{
				lists: map[string]lschema.Schema{
					"thing": invalidListResourceSchemaImplementation(),
				},
			}),
			Resources: map[string]*tfbridge.ResourceInfo{
				"test_thing": {
					Tok:  "testprovider:index:Thing",
					Docs: &tfbridge.DocInfo{Markdown: []byte{' '}},
				},
			},
		},
	})
	require.ErrorContains(t, err, "Plugin Framework list resource test_thing ValidateImplementation failed")
	require.ErrorContains(t, err, "missing the CustomType or ElementType field")
}

func TestGenerateSchemaFailsOnTokenlessPFListResourceSchemaImplementation(t *testing.T) {
	t.Parallel()

	_, err := GenerateSchema(context.Background(), GenerateSchemaOptions{
		ProviderInfo: tfbridge.ProviderInfo{
			Name:             "testprovider",
			UpstreamRepoPath: ".", // no invalid mappings warnings
			P: pftfbridge.ShimProvider(&schemaTestProvider{
				lists: map[string]lschema.Schema{
					"thing": invalidListResourceSchemaImplementation(),
				},
			}),
			Resources: map[string]*tfbridge.ResourceInfo{
				"test_thing": {Docs: &tfbridge.DocInfo{Markdown: []byte{' '}}},
			},
		},
	})
	require.ErrorContains(t, err, "Plugin Framework list resource test_thing ValidateImplementation failed")
	require.ErrorContains(t, err, "missing the CustomType or ElementType field")
}

func TestGenerateSchemaThreadsContextIntoFrameworkValidation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := GenerateSchema(ctx, GenerateSchemaOptions{
		ProviderInfo: tfbridge.ProviderInfo{
			Name:             "testprovider",
			UpstreamRepoPath: ".", // no invalid mappings warnings
			P:                pftfbridge.ShimProvider(&contextCheckingProvider{}),
			Resources: map[string]*tfbridge.ResourceInfo{
				"test_res": {
					Tok:  "testprovider:index:Res",
					Docs: &tfbridge.DocInfo{Markdown: []byte{' '}},
				},
			},
		},
	})
	require.ErrorContains(t, err, "Plugin Framework resource test_res Schema failed")
	require.ErrorContains(t, err, context.Canceled.Error())
}

func TestGenerateSchemaIgnoresInvalidUnmappedPFResourceSchemaImplementation(t *testing.T) {
	t.Parallel()

	_, err := GenerateSchema(context.Background(), GenerateSchemaOptions{
		ProviderInfo: tfbridge.ProviderInfo{
			Name:             "testprovider",
			UpstreamRepoPath: ".", // no invalid mappings warnings
			P: pftfbridge.ShimProvider(&schemaTestProvider{
				resources: map[string]rschema.Schema{
					"res": {
						Attributes: map[string]rschema.Attribute{
							"id": rschema.StringAttribute{Computed: true},
							"a1": rschema.StringAttribute{
								Optional: true,
								Default:  stringdefault.StaticString("default"),
							},
						},
					},
				},
			}),
			IgnoreMappings: []string{"test_res"},
		},
	})
	require.NoError(t, err)
}

func invalidResourceSchemaImplementation() rschema.Schema {
	return rschema.Schema{
		Attributes: map[string]rschema.Attribute{
			"id": rschema.StringAttribute{Computed: true},
			"a1": rschema.StringAttribute{
				Optional: true,
				Default:  stringdefault.StaticString("default"),
			},
		},
	}
}

func invalidProviderSchemaImplementation() prschema.Schema {
	return prschema.Schema{
		Attributes: map[string]prschema.Attribute{
			"bad": prschema.ListAttribute{Optional: true},
		},
	}
}

func invalidDataSourceSchemaImplementation() dschema.Schema {
	return dschema.Schema{
		Attributes: map[string]dschema.Attribute{
			"bad": dschema.ListAttribute{Computed: true},
		},
	}
}

func invalidListResourceSchemaImplementation() lschema.Schema {
	return lschema.Schema{
		Attributes: map[string]lschema.Attribute{
			"bad": lschema.ListAttribute{},
		},
	}
}

type contextCheckingProvider struct{}

func (*contextCheckingProvider) Metadata(
	_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse,
) {
	resp.TypeName = "test_"
}

func (*contextCheckingProvider) Schema(context.Context, provider.SchemaRequest, *provider.SchemaResponse) {
}

func (*contextCheckingProvider) Configure(context.Context, provider.ConfigureRequest, *provider.ConfigureResponse) {
	panic("NOT IMPLEMENTED")
}

func (*contextCheckingProvider) Resources(context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		func() resource.Resource { return contextCheckingResource{} },
	}
}

func (*contextCheckingProvider) DataSources(context.Context) []func() datasource.DataSource {
	return nil
}

type contextCheckingResource struct{}

func (contextCheckingResource) Metadata(
	_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse,
) {
	resp.TypeName = req.ProviderTypeName + "res"
}

func (contextCheckingResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	if ctx.Err() != nil {
		resp.Diagnostics.AddError("validation context error", ctx.Err().Error())
	}
	resp.Schema = rschema.Schema{
		Attributes: map[string]rschema.Attribute{
			"id": rschema.StringAttribute{Computed: true},
		},
	}
}

func (contextCheckingResource) Create(context.Context, resource.CreateRequest, *resource.CreateResponse) {
}

func (contextCheckingResource) Read(context.Context, resource.ReadRequest, *resource.ReadResponse) {}

func (contextCheckingResource) Update(context.Context, resource.UpdateRequest, *resource.UpdateResponse) {
}

func (contextCheckingResource) Delete(context.Context, resource.DeleteRequest, *resource.DeleteResponse) {
}
