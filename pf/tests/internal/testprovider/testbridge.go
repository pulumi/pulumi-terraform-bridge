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

package testprovider

import (
	"context"
	_ "embed"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	pschema "github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"

	tfpf "github.com/pulumi/pulumi-terraform-bridge/pf/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
)

//go:embed cmd/pulumi-resource-testbridge/bridge-metadata.json
var testBridgeMetadata []byte

// Synthetic provider is specifically constructed to test various
// features of tfbridge and is the core of pulumi-resource-testbridge.
func SyntheticTestBridgeProvider() tfpf.ProviderInfo {
	defineProvider := func() provider.Provider {
		return &syntheticProvider{}
	}
	info := tfbridge.ProviderInfo{
		Name:        "testbridge",
		Description: "A Pulumi package to test pulumi-terraform-bridge Plugin Framework support.",
		Keywords:    []string{},
		License:     "Apache-2.0",
		Homepage:    "https://pulumi.io",
		Repository:  "https://github.com/pulumi/pulumi-terraform-bridge",
		Version:     "0.0.1",

		Config: map[string]*tfbridge.SchemaInfo{
			"string_defaultinfo_config_prop": {
				Default: &tfbridge.DefaultInfo{
					Value: "DEFAULT",
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
		},

		MetadataInfo: tfbridge.NewProviderMetadata(testBridgeMetadata),
	}
	return tfpf.ProviderInfo{
		ProviderInfo: info,
		NewProvider:  defineProvider,
	}
}

type syntheticProvider struct {
}

var _ provider.Provider = (*syntheticProvider)(nil)

func (p *syntheticProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "testbridge"
	resp.Version = "0.0.1"
}

func (p *syntheticProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = pschema.Schema{
		Attributes: map[string]pschema.Attribute{
			"string_config_prop": pschema.StringAttribute{},
			"bool_config_prop": pschema.BoolAttribute{
				Optional: true,
			},
			"string_defaultinfo_config_prop": pschema.StringAttribute{
				Optional:    true,
				Description: "Used for testing DefaultInfo default application support",
			},
		},
	}
}

func (p *syntheticProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var stringConfigProp *string
	diags := req.Config.GetAttribute(ctx, path.Root("string_config_prop"), &stringConfigProp)
	resp.Diagnostics.Append(diags...)
	if stringConfigProp != nil {
		resp.ResourceData = stringConfigProp
	}
}

func (p *syntheticProvider) DataSources(context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		newEchoDataSource,
		newTestDefaultInfoDataSource,
	}
}

func (p *syntheticProvider) Resources(context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		newTestres,
		newTestnest,
		newTestCompRes,
		newTestConfigRes,
		newTestDefaultInfoRes,
	}
}
