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

	//"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

// Represents all provider's resources pre-indexed by TypeName.
type Resources interface {
	All() []TypeName
	Has(TypeName) bool
	Schema(TypeName) Schema
}

// Collects all resources from prov and indexes them by TypeName.
func GatherResources(ctx context.Context, prov provider.Provider) (Resources, error) {
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

		rs[TypeName(meta.TypeName)] = entry[func() resource.Resource]{
			t:           makeResource,
			schema:      FromResourceSchema(resSchema),
			diagnostics: diag,
		}
	}

	return &resources{collection: rs}, nil
}

type resources struct {
	collection[func() resource.Resource]
}
