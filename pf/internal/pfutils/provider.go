package pfutils

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

// Creates a pf provider that's backed by a protov6.ProviderServer
func SchemaOnlyProvider(server *tfprotov6.ProviderServer) provider.Provider {
	return schemaOnlyProvider{server: server}
}

type schemaOnlyProvider struct {
	server *tfprotov6.ProviderServer
}

var _ provider.Provider = schemaOnlyProvider{}

func (schemaOnlyProvider) Metadata(context.Context, provider.MetadataRequest, *provider.MetadataResponse) {
	panic("Not Implemented")
}

func (schemaOnlyProvider) Schema(context.Context, provider.SchemaRequest, *provider.SchemaResponse) {
	panic("Not Implemented")
}

func (schemaOnlyProvider) Configure(context.Context, provider.ConfigureRequest, *provider.ConfigureResponse) {
	panic("Not Implemented")
}

func (schemaOnlyProvider) DataSources(context.Context) []func() datasource.DataSource {
	panic("Not Implemented")
}

func (schemaOnlyProvider) Resources(context.Context) []func() resource.Resource {
	panic("Not Implemented")
}
