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

package providerbuilder

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
)

type DataSource struct {
	Name             string
	DataSourceSchema schema.Schema

	ReadFunc func(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse)
}

func (r *DataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, re *datasource.MetadataResponse) {
	re.TypeName = req.ProviderTypeName + "_" + r.Name
}

func (r *DataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, re *datasource.SchemaResponse) {
	re.Schema = r.DataSourceSchema
}

func (r *DataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	r.ReadFunc(ctx, req, resp)
}

var _ datasource.DataSource = &DataSource{}
