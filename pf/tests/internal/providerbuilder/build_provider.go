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
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

type Provider struct {
	TypeName       string
	Version        string
	ProviderSchema schema.Schema
	AllResources   []Resource
}

var _ provider.Provider = (*Provider)(nil)

func (impl *Provider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = impl.TypeName
	resp.Version = impl.Version
}

func (impl *Provider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = impl.ProviderSchema
}

func (*Provider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
}

func (*Provider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{}
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

type NewProviderArgs struct {
	TypeName       string
	Version        string
	ProviderSchema schema.Schema
	AllResources   []Resource
}

// NewProvider creates a new provider with the given resources, filling reasonable defaults.
func NewProvider(params NewProviderArgs) *Provider {
	prov := &Provider{
		TypeName:       params.TypeName,
		Version:        params.Version,
		ProviderSchema: params.ProviderSchema,
		AllResources:   params.AllResources,
	}

	if prov.TypeName == "" {
		prov.TypeName = "testprovider"
	}
	if prov.Version == "" {
		prov.Version = "0.0.1"
	}

	for i := range prov.AllResources {
		r := &prov.AllResources[i]
		if r.ResourceSchema.Attributes == nil {
			r.ResourceSchema.Attributes = map[string]rschema.Attribute{}
		}

		if r.ResourceSchema.Attributes["id"] == nil {
			r.ResourceSchema.Attributes["id"] = rschema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			}
		}
		if r.CreateFunc == nil {
			r.CreateFunc = func(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
				resp.State = tfsdk.State(req.Plan)
				resp.State.SetAttribute(ctx, path.Root("id"), "test-id")
			}
		}
		if r.UpdateFunc == nil {
			r.UpdateFunc = func(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
				resp.State = tfsdk.State(req.Plan)
			}
		}
	}

	return prov
}
