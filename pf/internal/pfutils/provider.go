package pfutils

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	pschema "github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

// Creates a pf provider that's backed by a protov6.ProviderServer
func SchemaOnlyProvider(name, version string, server tfprotov6.ProviderServer) provider.Provider {
	return &schemaOnlyProvider{name: name, version: version, server: server}
}

type schemaOnlyProvider struct {
	name    string // The TF name of the provider, as used for TF name prefixes.
	version string // The version of the provider (not currently hooked up upstream)
	server  tfprotov6.ProviderServer

	// A cached schema from the wire, may be nil
	schema *tfprotov6.GetProviderSchemaResponse
}

var _ provider.Provider = &schemaOnlyProvider{}

func (p schemaOnlyProvider) Metadata(ctx context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = p.name
	resp.Version = p.version
}

func (p *schemaOnlyProvider) Schema(ctx context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	schema, err := p.getSchema(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Schema", err.Error())
		return
	}
	resp.Schema, resp.Diagnostics = convertProviderSchema(schema)
}

func (p *schemaOnlyProvider) getSchema(ctx context.Context) (*tfprotov6.GetProviderSchemaResponse, error) {
	if p.schema != nil {
		return p.schema, nil
	}
	var err error
	p.schema, err = p.server.GetProviderSchema(ctx, &tfprotov6.GetProviderSchemaRequest{})
	return p.schema, err
}

func (schemaOnlyProvider) Configure(context.Context, provider.ConfigureRequest, *provider.ConfigureResponse) {
	panic("Not Implemented")
}

func (p *schemaOnlyProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	schema, err := p.getSchema(ctx)
	if err != nil {
		// Since there is no method to return an error here, we panic.
		panic(err.Error())
	}
	resp := make([]func() datasource.DataSource, 0, len(schema.DataSourceSchemas))
	for k, v := range schema.DataSourceSchemas {
		resp = append(resp, schemaOnlyDataSource{name: k, schema: v}.identity)
	}
	return resp
}

func (p *schemaOnlyProvider) Resources(ctx context.Context) []func() resource.Resource {
	schema, err := p.getSchema(ctx)
	if err != nil {
		// Since there is no method to return an error here, we panic.
		panic(err.Error())
	}
	resp := make([]func() resource.Resource, 0, len(schema.ResourceSchemas))
	for k, v := range schema.ResourceSchemas {
		resp = append(resp, schemaOnlyResource{name: k, schema: v}.identity)
	}
	return resp
}

func convertProviderSchema(resp *tfprotov6.GetProviderSchemaResponse) (pschema.Schema, diag.Diagnostics) {
	panic("Not Implemented")
}

func convertResourceSchema(*tfprotov6.Schema) (rschema.Schema, diag.Diagnostics) {
	panic("Not Implemented")
}

func convertDataSourceSchema(req *tfprotov6.Schema) (dschema.Schema, diag.Diagnostics) {
	panic("Not Implemented")
}

type schemaOnlyDataSource struct {
	name   string
	schema *tfprotov6.Schema
}

func (s schemaOnlyDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_" + s.name
}

// Schema should return the schema for this data source.
func (s schemaOnlyDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema, resp.Diagnostics = convertDataSourceSchema(s.schema)
}

// Read is called when the provider must read data source values in
// order to update state. Config values should be read from the
// ReadRequest and new state values set on the ReadResponse.
func (s schemaOnlyDataSource) Read(context.Context, datasource.ReadRequest, *datasource.ReadResponse) {
	panic("schemaOnlyDataSource does not implement Read")
}

func (s schemaOnlyDataSource) identity() datasource.DataSource { return s }

type schemaOnlyResource struct {
	name   string
	schema *tfprotov6.Schema
}

func (s schemaOnlyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_" + s.name
}

// Schema should return the schema for this resource.
func (s schemaOnlyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema, resp.Diagnostics = convertResourceSchema(s.schema)
}

// Create is called when the provider must create a new resource. Config
// and planned state values should be read from the
// CreateRequest and new state values set on the CreateResponse.
func (s schemaOnlyResource) Create(context.Context, resource.CreateRequest, *resource.CreateResponse) {
	panic("schemaOnlyResource does not implement Create")
}

// Read is called when the provider must read resource values in order
// to update state. Planned state values should be read from the
// ReadRequest and new state values set on the ReadResponse.
func (s schemaOnlyResource) Read(context.Context, resource.ReadRequest, *resource.ReadResponse) {
	panic("schemaOnlyResource does not implement Read")
}

// Update is called to update the state of the resource. Config, planned
// state, and prior state values should be read from the
// UpdateRequest and new state values set on the UpdateResponse.
func (s schemaOnlyResource) Update(context.Context, resource.UpdateRequest, *resource.UpdateResponse) {
	panic("schemaOnlyResource does not implement Update")
}

// Delete is called when the provider must delete the resource. Config
// values may be read from the DeleteRequest.
//
// If execution completes without error, the framework will automatically
// call DeleteResponse.State.RemoveResource(), so it can be omitted
// from provider logic.
func (s schemaOnlyResource) Delete(context.Context, resource.DeleteRequest, *resource.DeleteResponse) {
	panic("schemaOnlyResource does not implement Delete")
}

func (s schemaOnlyResource) identity() resource.Resource { return s }
