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
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
)

func newSmacDataSource() datasource.DataSource {
	return &smacDataSource{}
}

type smacDataSource struct {
	smac *string
}

var _ datasource.DataSourceWithConfigure = &smacDataSource{}

func (*smacDataSource) Metadata(_ context.Context, rq datasource.MetadataRequest, re *datasource.MetadataResponse) {
	re.TypeName = rq.ProviderTypeName + "_smac_ds"
}

func (*smacDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"skip_metadata_api_check": schema.StringAttribute{Computed: true},
		},
	}
}

func (ds *smacDataSource) Configure(
	_ context.Context,
	rq datasource.ConfigureRequest,
	re *datasource.ConfigureResponse) {

	data, ok := rq.ProviderData.(resourceData)
	if ok {
		ds.smac = data.skipMetadataAPICheck
	}
}

func (ds *smacDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	diags := resp.State.SetAttribute(ctx, path.Root("skip_metadata_api_check"), ds.smac)
	resp.Diagnostics.Append(diags...)
}
