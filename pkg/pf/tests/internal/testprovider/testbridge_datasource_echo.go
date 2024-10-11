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

func newEchoDataSource() datasource.DataSource {
	return &echoDataSource{}
}

type echoDataSource struct{}

func (*echoDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest,
	resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_echo"
}

func (*echoDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"input":     schema.StringAttribute{Required: true},
			"output":    schema.StringAttribute{Computed: true},
			"sensitive": schema.StringAttribute{Computed: true, Sensitive: true},
		},
	}
}

func (*echoDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var input string
	diags := req.Config.GetAttribute(ctx, path.Root("input"), &input)
	resp.Diagnostics.Append(diags...)

	output := input
	diags2 := resp.State.SetAttribute(ctx, path.Root("output"), output)
	resp.Diagnostics.Append(diags2...)

	sensitive := input
	diags3 := resp.State.SetAttribute(ctx, path.Root("sensitive"), sensitive)
	resp.Diagnostics.Append(diags3...)
}
