// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

func New() provider.Provider {
	return &randomProvider{}
}

var _ provider.Provider = (*randomProvider)(nil)

type randomProvider struct{}

func (p *randomProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "random"
}

func (p *randomProvider) Schema(context.Context, provider.SchemaRequest, *provider.SchemaResponse) {
}

func (p *randomProvider) Configure(context.Context, provider.ConfigureRequest, *provider.ConfigureResponse) {
}

func (p *randomProvider) Resources(context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewIdResource,
		NewBytesResource,
		NewIntegerResource,
		NewPasswordResource,
		NewPetResource,
		NewShuffleResource,
		NewStringResource,
		NewUuidResource,
	}
}

func (p *randomProvider) DataSources(context.Context) []func() datasource.DataSource {
	return nil
}
