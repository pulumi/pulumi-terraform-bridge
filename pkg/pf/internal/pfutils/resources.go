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

	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/resource"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/internal/runtypes"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

// Collects all resources from prov and indexes them by TypeName.
func GatherResources[F func(Schema) shim.SchemaMap](
	ctx context.Context, prov provider.Provider, f F,
) (runtypes.Resources, error) {
	provMetadata := queryProviderMetadata(ctx, prov)
	rs := make(collection[func() resource.Resource])

	for _, makeResource := range prov.Resources(ctx) {
		res := makeResource()

		meta := resource.MetadataResponse{}
		res.Metadata(ctx, resource.MetadataRequest{
			ProviderTypeName: provMetadata.TypeName,
		}, &meta)

		schemaResponse := &resource.SchemaResponse{}
		res.Schema(ctx, resource.SchemaRequest{}, schemaResponse)

		resSchema, diag := schemaResponse.Schema, schemaResponse.Diagnostics
		if err := checkDiagsForErrors(diag); err != nil {
			return nil, fmt.Errorf("Resource %s GetSchema() error: %w", meta.TypeName, err)
		}

		rs[runtypes.TypeOrRenamedEntityName(meta.TypeName)] = entry[func() resource.Resource]{
			t:      makeResource,
			schema: FromResourceSchema(resSchema),
			tfName: runtypes.TypeName(meta.TypeName),
		}
	}

	return &resources{collection: rs, convert: f}, nil
}

var _ runtypes.Resources = resources{}

type resources struct {
	collection[func() resource.Resource]
	convert func(Schema) shim.SchemaMap
}

type runtypesSchemaAdapter struct {
	Schema
	converter func(Schema) shim.SchemaMap
	tfName    runtypes.TypeName
}

func (r runtypesSchemaAdapter) Shim() shim.SchemaMap {
	return r.converter(r.Schema)
}

func (r runtypesSchemaAdapter) TFName() runtypes.TypeName {
	return r.tfName
}

func (r resources) Schema(t runtypes.TypeOrRenamedEntityName) runtypes.Schema {
	entry := r.collection[t]
	return runtypesSchemaAdapter{entry.schema, r.convert, entry.tfName}
}

func (resources) IsResources() {}
