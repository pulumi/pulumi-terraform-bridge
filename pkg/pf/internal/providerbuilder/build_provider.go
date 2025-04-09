// Copyright 2016-2023, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package providerbuilder

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tfbridge"
	tfbridge0 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/tokens"
)

// Provider is a test provider that can be used in tests.
// Instantiate it with NewProvider.
type Provider struct {
	TypeName       string
	Version        string
	ProviderSchema schema.Schema
	AllResources   []Resource
	AllDataSources []DataSource

	configureFunc func(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse)
}

var _ provider.Provider = (*Provider)(nil)

func (impl *Provider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = impl.TypeName
	resp.Version = impl.Version
}

func (impl *Provider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = impl.ProviderSchema
}

func (impl *Provider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	if impl.configureFunc != nil {
		impl.configureFunc(ctx, req, resp)
	}
}

func (impl *Provider) DataSources(ctx context.Context) []func() datasource.DataSource {
	d := make([]func() datasource.DataSource, len(impl.AllDataSources))
	for i := 0; i < len(impl.AllDataSources); i++ {
		i := i
		d[i] = func() datasource.DataSource {
			return &impl.AllDataSources[i]
		}
	}
	return d
}

func (impl *Provider) Resources(ctx context.Context) []func() resource.Resource {
	r := make([]func() resource.Resource, len(impl.AllResources))
	for i := 0; i < len(impl.AllResources); i++ {
		i := i
		r[i] = func() resource.Resource {
			return &impl.AllResources[i]
		}
	}
	return r
}

func (impl *Provider) GRPCProvider() tfprotov6.ProviderServer {
	return providerserver.NewProtocol6(impl)()
}

func (impl *Provider) ToProviderInfo() tfbridge0.ProviderInfo {
	shimProvider := tfbridge.ShimProvider(impl)

	provider := tfbridge0.ProviderInfo{
		P:            shimProvider,
		Name:         impl.TypeName,
		Version:      "0.0.1",
		MetadataInfo: &tfbridge0.MetadataInfo{},
	}

	provider.MustComputeTokens(tokens.SingleModule(impl.TypeName, "index", tokens.MakeStandard(impl.TypeName)))

	return provider
}

type NewProviderArgs struct {
	TypeName       string
	Version        string
	ProviderSchema schema.Schema
	AllResources   []Resource
	AllDataSources []DataSource

	ConfigureFunc func(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse)
}

// NewProvider creates a new provider with the given resources, filling reasonable defaults.
func NewProvider(params NewProviderArgs) *Provider {
	prov := &Provider{
		TypeName:       params.TypeName,
		Version:        params.Version,
		ProviderSchema: params.ProviderSchema,
		AllResources:   params.AllResources,
		AllDataSources: params.AllDataSources,

		configureFunc: params.ConfigureFunc,
	}

	if prov.TypeName == "" {
		prov.TypeName = "testprovider"
	}
	if prov.Version == "" {
		prov.Version = "0.0.1"
	}

	for i := range prov.AllDataSources {
		d := &prov.AllDataSources[i]
		if d.DataSourceSchema.Attributes == nil {
			d.DataSourceSchema.Attributes = map[string]dschema.Attribute{}
		}

		if d.ReadFunc == nil {
			d.ReadFunc = func(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
				resp.State = tfsdk.State(req.Config)
			}
		}
	}

	return prov
}
