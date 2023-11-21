package testprovider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

var _ provider.Provider = (*ConfigTestProvider)(nil)

type ConfigTestProvider struct {
	ProviderSchema schema.Schema
	ConfigErr      diag.ErrorDiagnostic
}

func (*ConfigTestProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
}

func (p *ConfigTestProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = p.ProviderSchema
}

func (p *ConfigTestProvider) Configure(_ context.Context, _ provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	resp.Diagnostics = append(resp.Diagnostics, p.ConfigErr)
}

func (*ConfigTestProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return nil
}

func (*ConfigTestProvider) Resources(context.Context) []func() resource.Resource {
	return nil
}
