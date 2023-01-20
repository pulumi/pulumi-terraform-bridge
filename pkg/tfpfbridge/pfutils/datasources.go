// Copyright 2016-2022, Pulumi Corporation.
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

package pfutils

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
)

// Represents all provider's datasources pre-indexed by TypeName.
type DataSources interface {
	All() []TypeName
	Has(TypeName) bool
	Schema(TypeName) tfsdk.Schema
	Diagnostics(TypeName) diag.Diagnostics
	AllDiagnostics() diag.Diagnostics
	DataSource(TypeName) datasource.DataSource
}

func GatherDatasources(ctx context.Context, prov provider.Provider) (DataSources, error) {
	provMetadata := queryProviderMetadata(ctx, prov)
	ds := make(collection[func() datasource.DataSource])

	for _, makeDataSource := range prov.DataSources(ctx) {
		dataSource := makeDataSource()

		meta := datasource.MetadataResponse{}
		dataSource.Metadata(ctx, datasource.MetadataRequest{
			ProviderTypeName: provMetadata.TypeName,
		}, &meta)

		dataSourceSchema, diag := dataSource.GetSchema(ctx)
		if err := checkDiagsForErrors(diag); err != nil {
			return nil, fmt.Errorf("Resource %s GetSchema() error: %w", meta.TypeName, err)
		}

		ds[TypeName(meta.TypeName)] = entry[func() datasource.DataSource]{
			t:           makeDataSource,
			schema:      dataSourceSchema,
			diagnostics: diag,
		}
	}

	return &dataSources{collection: ds}, nil
}

type dataSources struct {
	collection[func() datasource.DataSource]
}

func (d *dataSources) DataSource(name TypeName) datasource.DataSource {
	return d.collection[name].t()
}
