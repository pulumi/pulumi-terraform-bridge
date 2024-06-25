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
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/pulumi/pulumi-terraform-bridge/pf/internal/runtypes"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

func GatherDatasources[F func(Schema) shim.SchemaMap](
	ctx context.Context, prov provider.Provider, f F,
) (runtypes.DataSources, error) {
	provMetadata := queryProviderMetadata(ctx, prov)
	ds := make(collection[func() datasource.DataSource])

	for _, makeDataSource := range prov.DataSources(ctx) {
		dataSource := makeDataSource()

		meta := datasource.MetadataResponse{}
		dataSource.Metadata(ctx, datasource.MetadataRequest{
			ProviderTypeName: provMetadata.TypeName,
		}, &meta)

		schemaResponse := &datasource.SchemaResponse{}
		dataSource.Schema(ctx, datasource.SchemaRequest{}, schemaResponse)

		dataSourceSchema := schemaResponse.Schema
		diag := schemaResponse.Diagnostics
		if err := checkDiagsForErrors(diag); err != nil {
			return nil, fmt.Errorf("Resource %s GetSchema() error: %w", meta.TypeName, err)
		}

		ds[runtypes.TypeName(meta.TypeName)] = entry[func() datasource.DataSource]{
			t:      makeDataSource,
			schema: FromDataSourceSchema(dataSourceSchema),
		}
	}

	return &dataSources{collection: ds, convert: f}, nil
}

type dataSources struct {
	collection[func() datasource.DataSource]
	convert func(Schema) shim.SchemaMap
}

func (r dataSources) Schema(t runtypes.TypeName) runtypes.Schema {
	return runtypesSchemaAdapter{r.collection.Schema(t), r.convert}
}

func (dataSources) IsDataSources() {}
