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
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
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
