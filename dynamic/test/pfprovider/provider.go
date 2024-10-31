package main

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/function"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure ScaffoldingProvider satisfies various provider interfaces.
var (
	_ provider.Provider              = &PFProvider{}
	_ provider.ProviderWithFunctions = &PFProvider{}
)

// PFProvider defines the provider implementation.
type PFProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" when running acceptance
	// testing.
	version string
}

// ScaffoldingProviderModel describes the provider data model.
type ScaffoldingProviderModel struct {
	Endpoint types.String `tfsdk:"endpoint"`
	Nested   types.Object `tfsdk:"nested"`
}

func (p *PFProvider) Metadata(
	ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse,
) {
	resp.TypeName = "pfprovider"
	resp.Version = p.version
}

func (p *PFProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"endpoint": schema.StringAttribute{
				MarkdownDescription: "Example provider attribute",
				Optional:            true,
			},
		},
		Blocks: map[string]schema.Block{
			"nested": schema.SingleNestedBlock{
				Attributes: map[string]schema.Attribute{
					"field1": schema.StringAttribute{Optional: true},
					"field2": schema.BoolAttribute{Optional: true},
				},
			},
		},
	}
}

func (p *PFProvider) Configure(
	ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse,
) {
	var data ScaffoldingProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if !data.Nested.IsNull() {
		var nested struct {
			F1 types.String `tfsdk:"field1"`
			F2 types.Bool   `tfsdk:"field2"`
		}
		resp.Diagnostics.Append(req.Config.Get(ctx, &nested)...)
		if resp.Diagnostics.HasError() {
			return
		}

		if nested.F1.ValueString() != "true" {
			resp.Diagnostics.AddError("unexpected string", "")
		}
		if nested.F2.ValueBool() {
			resp.Diagnostics.AddError("unexpected bool", "")
		}
	}

	resp.DataSourceData = data.Endpoint.ValueString()
}

func (p *PFProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewExampleResource,
		NewPanicResource,
	}
}

func (p *PFProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewExampleDataSource,
	}
}

func (p *PFProvider) Functions(ctx context.Context) []func() function.Function {
	return []func() function.Function{}
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &PFProvider{
			version: version,
		}
	}
}
