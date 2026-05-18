// Copyright 2026, Pulumi Corporation.
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

	tflist "github.com/hashicorp/terraform-plugin-framework/list"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/resource"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/internal/runtypes"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

func GatherListResources[F func(Schema) shim.SchemaMap](
	ctx context.Context, prov provider.Provider, f F,
) (runtypes.ListResources, error) {
	provMetadata := queryProviderMetadata(ctx, prov)
	ls := make(collection[func() tflist.ListResource])

	lprov, ok := prov.(provider.ProviderWithListResources)
	if !ok {
		return &listResources{collection: ls, convert: f}, nil
	}

	for _, makeListResource := range lprov.ListResources(ctx) {
		listResource := makeListResource()

		meta := resource.MetadataResponse{}
		listResource.Metadata(ctx, resource.MetadataRequest{
			ProviderTypeName: provMetadata.TypeName,
		}, &meta)

		schemaResponse := &tflist.ListResourceSchemaResponse{}
		listResource.ListResourceConfigSchema(ctx, tflist.ListResourceSchemaRequest{}, schemaResponse)

		listResourceSchema := schemaResponse.Schema
		diag := schemaResponse.Diagnostics
		if err := checkDiagsForErrors(diag); err != nil {
			return nil, fmt.Errorf("Resource %s GetSchema() error: %w", meta.TypeName, err)
		}

		ls[runtypes.TypeOrRenamedEntityName(meta.TypeName)] = entry[func() tflist.ListResource]{
			t:      makeListResource,
			schema: FromListSchema(listResourceSchema),
			tfName: runtypes.TypeName(meta.TypeName),
		}
	}

	return &listResources{collection: ls, convert: f}, nil
}

type listResources struct {
	collection[func() tflist.ListResource]
	convert func(Schema) shim.SchemaMap
}

func (r listResources) Schema(t runtypes.TypeOrRenamedEntityName) runtypes.Schema {
	entry := r.collection[t]
	return runtypesSchemaAdapter{entry.schema, r.convert, entry.tfName}
}

func (listResources) IsListResources() {}
