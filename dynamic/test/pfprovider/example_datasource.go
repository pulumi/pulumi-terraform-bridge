package main

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &ExampleDataSource{}

func NewExampleDataSource() datasource.DataSource {
	return &ExampleDataSource{}
}

// ExampleDataSource defines the data source implementation.
type ExampleDataSource struct {
	data string
}

// ExampleDataSourceModel describes the data source data model.
type ExampleDataSourceModel struct {
	Endpoint types.String `tfsdk:"endpoint"`
}

func (d *ExampleDataSource) Metadata(
	ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse,
) {
	resp.TypeName = req.ProviderTypeName + "_config_endpoint"
}

func (d *ExampleDataSource) Schema(
	ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse,
) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Example data source",

		Attributes: map[string]schema.Attribute{
			"endpoint": schema.StringAttribute{
				MarkdownDescription: "Example configurable attribute",
				Computed:            true,
			},
		},
	}
}

func (d *ExampleDataSource) Configure(
	ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse,
) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	var ok bool
	d.data, ok = req.ProviderData.(string)
	if !ok {
		resp.Diagnostics.AddError("missing provider configuration", "datasource")
	}
}

func (d *ExampleDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ExampleDataSourceModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	data.Endpoint = types.StringValue(d.data)

	// resp.Diagnostics.Append(req.ProviderMeta.GetAttribute(ctx, path.Root("endpoint"), &data.Endpoint)...)

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
