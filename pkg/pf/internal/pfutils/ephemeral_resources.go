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

	"github.com/hashicorp/terraform-plugin-framework/ephemeral"
	"github.com/hashicorp/terraform-plugin-framework/provider"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/internal/runtypes"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

func GatherEphemeralResources[F func(Schema) shim.SchemaMap](
	ctx context.Context, prov provider.Provider, f F,
) (runtypes.EphemeralResources, error) {
	provMetadata := queryProviderMetadata(ctx, prov)
	es := make(collection[func() ephemeral.EphemeralResource])

	eprov, ok := prov.(provider.ProviderWithEphemeralResources)
	if ok {
		for _, makeEphemeralResource := range eprov.EphemeralResources(ctx) {
			ephemeralResource := makeEphemeralResource()

			meta := ephemeral.MetadataResponse{}
			ephemeralResource.Metadata(ctx, ephemeral.MetadataRequest{
				ProviderTypeName: provMetadata.TypeName,
			}, &meta)

			schemaResponse := &ephemeral.SchemaResponse{}
			ephemeralResource.Schema(ctx, ephemeral.SchemaRequest{}, schemaResponse)

			ephemeralResourceSchema := schemaResponse.Schema
			diag := schemaResponse.Diagnostics
			if err := checkDiagsForErrors(diag); err != nil {
				return nil, fmt.Errorf("EphemeralResource %s GetSchema() error: %w", meta.TypeName, err)
			}

			es[runtypes.TypeOrRenamedEntityName(meta.TypeName)] = entry[func() ephemeral.EphemeralResource]{
				t:      makeEphemeralResource,
				schema: FromEphemeralResourceSchema(ephemeralResourceSchema),
				tfName: runtypes.TypeName(meta.TypeName),
			}
		}
	}

	return &ephemeralResources{collection: es, convert: f}, nil
}

type ephemeralResources struct {
	collection[func() ephemeral.EphemeralResource]
	convert func(Schema) shim.SchemaMap
}

func (r ephemeralResources) Schema(t runtypes.TypeOrRenamedEntityName) runtypes.Schema {
	entry := r.collection[t]
	return runtypesSchemaAdapter{entry.schema, r.convert, entry.tfName}
}

func (ephemeralResources) IsEphemeralResources() {}
