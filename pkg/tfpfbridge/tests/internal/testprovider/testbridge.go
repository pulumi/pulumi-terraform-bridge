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

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/pulumi/pulumi-terraform-bridge/pkg/tfpfbridge/info"
)

// Synthetic provider is specifically constructed to test various
// features of tfbridge and is the core of pulumi-resource-testbridge.
func SyntheticTestBridgeProvider() info.ProviderInfo {
	defineProvider := func() provider.Provider {
		return &syntheticProvider{}
	}
	return info.ProviderInfo{
		P:           defineProvider,
		Name:        "testbridge",
		Description: "A Pulumi package to test pulumi-terraform-bridge Plugin Framework support.",
		Keywords:    []string{},
		License:     "Apache-2.0",
		Homepage:    "https://pulumi.io",
		Repository:  "https://github.com/pulumi/pulumi-terraform-bridge",
		Version:     "0.0.1",
		Resources: map[string]*info.ResourceInfo{
			"testbridge_testres":       {Tok: "testbridge:index/testres:Testres"},
			"testbridge_testcompres":   {Tok: "testbridge:index/testres:Testcompres"},
			"testbridge_testconfigres": {Tok: "testbridge:index/testres:TestConfigRes"},
		},
	}
}

type syntheticProvider struct {
}

var _ provider.ProviderWithMetadata = (*syntheticProvider)(nil)

func (p *syntheticProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "testbridge"
	resp.Version = "0.0.1"
}

func (p *syntheticProvider) GetSchema(context.Context) (tfsdk.Schema, diag.Diagnostics) {
	return tfsdk.Schema{
		Attributes: map[string]tfsdk.Attribute{
			"stringConfigProp": {
				Type: types.StringType,
			},
		},
	}, nil
}

func (p *syntheticProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var stringConfigProp string
	diags := req.Config.GetAttribute(ctx, path.Root("stringConfigProp"), &stringConfigProp)
	resp.Diagnostics.Append(diags...)
	resp.ResourceData = stringConfigProp
	return
}

func (p *syntheticProvider) DataSources(context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{}
}

func (p *syntheticProvider) Resources(context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		newTestres,
		newTestCompRes,
		newTestConfigRes,
	}
}
