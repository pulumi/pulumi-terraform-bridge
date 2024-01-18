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
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	pschema "github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"

	tfpf "github.com/pulumi/pulumi-terraform-bridge/pf/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
)

func AssertProvider(f AssertFn) tfbridge.ProviderInfo {
	info := tfbridge.ProviderInfo{
		Name:             "assert",
		P:                tfpf.ShimProvider(&assertProvider{f}),
		Description:      "A Pulumi package to test pulumi-terraform-bridge Plugin Framework support.",
		Keywords:         []string{},
		License:          "Apache-2.0",
		Homepage:         "https://pulumi.io",
		Repository:       "https://github.com/pulumi/pulumi-terraform-bridge",
		Version:          "0.0.1",
		UpstreamRepoPath: ".",

		Config: map[string]*tfbridge.SchemaInfo{},

		Resources: map[string]*tfbridge.ResourceInfo{
			"assert_echo": {Tok: "assert:index/echo:Echo"},
		},

		MetadataInfo: tfbridge.NewProviderMetadata(testBridgeMetadata),
	}

	info.SetAutonaming(255, "-")

	return info
}

type AssertFn = func(config tfsdk.Config, old *tfsdk.State, new *tfsdk.State)

type assertProvider struct{ f AssertFn }

var _ provider.Provider = (*assertProvider)(nil)

func (p *assertProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "assert"
	resp.Version = "0.0.1"
}

func (p *assertProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = pschema.Schema{Attributes: map[string]pschema.Attribute{}}
}

func (p *assertProvider) Configure(
	ctx context.Context,
	req provider.ConfigureRequest,
	resp *provider.ConfigureResponse,
) {
}

func (p *assertProvider) DataSources(context.Context) []func() datasource.DataSource { return nil }

func (p *assertProvider) Resources(context.Context) []func() resource.Resource {
	return []func() resource.Resource{func() resource.Resource { return &assertRes{p} }}
}

type assertRes struct{ p *assertProvider }

func (e *assertRes) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = rschema.Schema{
		Attributes: map[string]rschema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"string_property_value": schema.StringAttribute{
				Optional: true,
			},
		},
	}
}
func (e *assertRes) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_echo"
}

func (e *assertRes) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	e.p.f(req.Config, &tfsdk.State{}, &resp.State)
	var s string
	diag := req.Plan.GetAttribute(ctx, path.Root("string_property_value"), &s)
	resp.Diagnostics.Append(diag...)
	if resp.Diagnostics.HasError() {
		return
	}

	diag = resp.State.SetAttribute(ctx, path.Root("id"), "0")
	resp.Diagnostics.Append(diag...)
	if resp.Diagnostics.HasError() {
		return
	}

	diag = resp.State.SetAttribute(ctx, path.Root("string_property_value"), s)
	resp.Diagnostics.Append(diag...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (e *assertRes) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	e.p.f(tfsdk.Config{}, &req.State, &resp.State)
}

func (e *assertRes) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	e.p.f(tfsdk.Config{}, &req.State, &resp.State)

	diag := resp.State.SetAttribute(ctx, path.Root("id"), "0")
	resp.Diagnostics.Append(diag...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (e *assertRes) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	e.p.f(req.Config, &req.State, &resp.State)
}
