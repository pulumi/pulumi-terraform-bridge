// Copyright 2016-2025, Pulumi Corporation.
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

package testprovider

import (
	"context"
	_ "embed"
	"fmt"
	"reflect"

	"github.com/hashicorp/terraform-plugin-framework/action"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	prschema "github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	tfpf "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
)

//go:embed cmd/pulumi-resource-testbridge/bridge-metadata.json
var testBridgeMetadata []byte

// Synthetic provider is specifically constructed to test various
// features of tfbridge and is the core of pulumi-resource-testbridge.
func SyntheticTestBridgeProvider() tfbridge.ProviderInfo {
	info := tfbridge.ProviderInfo{
		Name:             "testbridge",
		P:                tfpf.ShimProvider(&syntheticProvider{}),
		Description:      "A Pulumi package to test pulumi-terraform-bridge Plugin Framework support.",
		Keywords:         []string{},
		License:          "Apache-2.0",
		Homepage:         "https://pulumi.io",
		Repository:       "https://github.com/pulumi/pulumi-terraform-bridge",
		Version:          "0.0.1",
		UpstreamRepoPath: ".", // Setting UpstreamRepoPath prevents the "could not find docs" warning.

		Config: map[string]*tfbridge.SchemaInfo{
			"string_defaultinfo_config_prop": {
				Default: &tfbridge.DefaultInfo{
					Value: "DEFAULT",
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
			"testbridge_testres": {Tok: "testbridge:index/testres:Testres"},
			"testbridge_testnest": {
				Tok: "testbridge:index/testnest:Testnest",
				Fields: map[string]*tfbridge.SchemaInfo{
					"rules": {
						Elem: &tfbridge.SchemaInfo{
							Fields: map[string]*tfbridge.SchemaInfo{
								"action_parameters": {
									MaxItemsOne: tfbridge.True(),
									Elem: &tfbridge.SchemaInfo{
										Fields: map[string]*tfbridge.SchemaInfo{
											"phases": {MaxItemsOne: tfbridge.True()},
										},
									},
								},
							},
						},
					},
				},
			},
			"testbridge_testnestattr": {
				Tok: "testbridge:index/testnestattr:Testnestattr",
			},
			"testbridge_testcompres":   {Tok: "testbridge:index/testres:Testcompres"},
			"testbridge_testconfigres": {Tok: "testbridge:index/testres:TestConfigRes"},

			"testbridge_test_default_info_res": {
				Tok: "testbridge:index/testres:TestDefaultInfoRes",
				Fields: map[string]*tfbridge.SchemaInfo{
					"str": {
						Default: &tfbridge.DefaultInfo{
							Value: "DEFAULT",
						},
					},
				},
			},

			"testbridge_privst":       {Tok: "testbridge:index/testres:Privst"},
			"testbridge_autoname_res": {Tok: "testbridge:index/testres:AutoNameRes"},
			"testbridge_int_id_res": {
				Tok: "testbridge:index/intID:IntID",
				Fields: map[string]*tfbridge.SchemaInfo{
					"id": {Type: "string"},
				},
			},
			"testbridge_vlan_names_res": {Tok: "testbridge:index/testres:VlanNamesRes"},
		},

		DataSources: map[string]*tfbridge.DataSourceInfo{
			"testbridge_echo": {Tok: "testbridge:index/echo:Echo"},

			"testbridge_test_defaultinfo": {
				Tok: "testbridge:index/testres:TestDefaultInfoDataSource",
				Fields: map[string]*tfbridge.SchemaInfo{
					"input": {
						Default: &tfbridge.DefaultInfo{
							Value: "DEFAULT",
						},
					},
				},
			},

			"testbridge_smac_ds": {Tok: "testbridge:index/smac:SMAC"},
		},

		Actions: map[string]*tfbridge.ActionInfo{
			"testbridge_print": {Tok: "testbridge:index/print:Print"},
		},

		MetadataInfo: tfbridge.NewProviderMetadata(testBridgeMetadata),
	}

	info.SetAutonaming(255, "-")

	return info
}

type syntheticProvider struct{}

type resourceData struct {
	stringConfigProp     *string
	skipMetadataAPICheck *string
}

var _ provider.Provider = (*syntheticProvider)(nil)
var _ provider.ProviderWithActions = (*syntheticProvider)(nil)

func (p *syntheticProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "testbridge"
	resp.Version = "0.0.1"
}

func (p *syntheticProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = prschema.Schema{
		Attributes: map[string]prschema.Attribute{
			"string_config_prop": prschema.StringAttribute{
				Optional: true,
			},
			"bool_config_prop": prschema.BoolAttribute{
				Optional: true,
			},
			"string_defaultinfo_config_prop": prschema.StringAttribute{
				Optional:    true,
				Description: "Used for testing DefaultInfo default application support",
			},
			"skip_metadata_api_check": prschema.StringAttribute{
				Optional: true,
				Description: "Example taken from pulumi-aws; used to validate string properties " +
					"remapped to bool type during briding",
			},

			"validate_nested": prschema.BoolAttribute{
				Optional:    true,
				Description: "Validate that nested values are received as expected.",
			},

			"map_nested_prop": prschema.MapAttribute{
				Optional:    true,
				ElementType: types.Int64Type,
			},
			"list_nested_prop": prschema.ListAttribute{
				Optional:    true,
				ElementType: types.BoolType,
			},
		},
		Blocks: map[string]prschema.Block{
			"single_nested": prschema.SingleNestedBlock{
				Attributes: map[string]prschema.Attribute{
					"string_prop": prschema.StringAttribute{
						Optional: true,
					},
					"bool_prop": prschema.BoolAttribute{
						Optional: true,
					},
					"map_nested_prop": prschema.MapAttribute{
						Optional:    true,
						ElementType: types.Int64Type,
					},
					"list_nested_prop": prschema.ListAttribute{
						Optional:    true,
						ElementType: types.BoolType,
					},
				},
			},
			"list_nested": prschema.ListNestedBlock{
				NestedObject: prschema.NestedBlockObject{
					Attributes: map[string]prschema.Attribute{
						"string_prop": prschema.StringAttribute{
							Optional: true,
						},
						"bool_prop": prschema.BoolAttribute{
							Optional: true,
						},
						"map_nested_prop": prschema.MapAttribute{
							Optional:    true,
							ElementType: types.Int64Type,
						},
						"list_nested_prop": prschema.ListAttribute{
							Optional:    true,
							ElementType: types.BoolType,
						},
					},
				},
			},
		},
	}
}

func validateNested(
	ctx context.Context,
	req provider.ConfigureRequest,
	resp *provider.ConfigureResponse,
) {
	check := func(path path.Path, expected, actual any) {
		if !reflect.DeepEqual(expected, actual) {
			resp.Diagnostics.AddAttributeError(path, "mismatched expectations",
				fmt.Sprintf("\nExpected %#v\nFound %#v", expected, actual))
		}
	}

	// Validate top level fields are received as expected
	var mapNestedProp map[string]int64
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("map_nested_prop"), &mapNestedProp)...)
	check(path.Root("map_nested_prop"), map[string]int64{"k1": 1, "k2": 2}, mapNestedProp)

	var listNestedProp []bool
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("list_nested_prop"), &listNestedProp)...)
	check(path.Root("list_nested_prop"), []bool{true, false}, listNestedProp)

	if resp.Diagnostics.HasError() {
		return
	}

	validate := func(name path.Path) {
		type nested struct {
			String types.String `tfsdk:"string_prop"`
			Bool   types.Bool   `tfsdk:"bool_prop"`
			Map    types.Map    `tfsdk:"map_nested_prop"`
			List   types.List   `tfsdk:"list_nested_prop"`
		}

		var actual nested
		resp.Diagnostics.Append(req.Config.GetAttribute(ctx, name, &actual)...)
		check(name, nested{
			String: types.StringValue("foo"),
			Bool:   types.BoolValue(true),
			Map: types.MapValueMust(types.Int64Type, map[string]attr.Value{
				"v1": types.Int64Value(1234),
			}),
			List: types.ListValueMust(types.BoolType, []attr.Value{
				types.BoolValue(true),
				types.BoolValue(false),
			}),
		}, actual)
	}

	validate(path.Root("single_nested"))
	validate(path.Root("list_nested").AtListIndex(0))
}

func (p *syntheticProvider) Configure(
	ctx context.Context,
	req provider.ConfigureRequest,
	resp *provider.ConfigureResponse,
) {
	rd := resourceData{}

	var stringConfigProp *string
	diags := req.Config.GetAttribute(ctx, path.Root("string_config_prop"), &stringConfigProp)
	resp.Diagnostics.Append(diags...)
	if stringConfigProp != nil {
		rd.stringConfigProp = stringConfigProp
	}

	var smac *string
	diags2 := req.Config.GetAttribute(ctx, path.Root("skip_metadata_api_check"), &smac)
	resp.Diagnostics.Append(diags2...)
	if smac != nil {
		switch *smac {
		case "true", "false", "":
			rd.skipMetadataAPICheck = smac
		default:
			resp.Diagnostics.AddError("cannot parse skip_metadata_api_check", *smac)
		}
	}

	var doValidateNested *bool
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("validate_nested"), &doValidateNested)...)
	if doValidateNested != nil && *doValidateNested {
		validateNested(ctx, req, resp)
	}

	resp.ResourceData = rd
	resp.DataSourceData = rd
}

func (p *syntheticProvider) DataSources(context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		newEchoDataSource,
		newTestDefaultInfoDataSource,
		newSmacDataSource,
	}
}

func (p *syntheticProvider) Resources(context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		newTestres,
		newTestnest,
		newTestnestattr,
		newTestCompRes,
		newTestConfigRes,
		newTestDefaultInfoRes,
		newPrivst,
		newAutoNameRes,
		newIntIDRes,
		newVlanNamesRes,
	}
}

func (p *syntheticProvider) Actions(context.Context) []func() action.Action {
	return []func() action.Action{
		newPrintAction,
	}
}
