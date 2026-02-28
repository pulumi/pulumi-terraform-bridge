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

	"github.com/hashicorp/terraform-plugin-framework/action"
	"github.com/hashicorp/terraform-plugin-framework/provider"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/internal/runtypes"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

func GatherActions[F func(Schema) shim.SchemaMap](
	ctx context.Context, prov provider.Provider, f F,
) (runtypes.Actions, error) {
	provMetadata := queryProviderMetadata(ctx, prov)
	ds := make(collection[func() action.Action])

	aprov, is := prov.(provider.ProviderWithActions)
	if is {
		for _, makeAction := range aprov.Actions(ctx) {
			act := makeAction()

			meta := action.MetadataResponse{}
			act.Metadata(ctx, action.MetadataRequest{
				ProviderTypeName: provMetadata.TypeName,
			}, &meta)

			schemaResponse := &action.SchemaResponse{}
			act.Schema(ctx, action.SchemaRequest{}, schemaResponse)

			actionSchema := schemaResponse.Schema
			diag := schemaResponse.Diagnostics
			if err := checkDiagsForErrors(diag); err != nil {
				return nil, fmt.Errorf("Action %s GetSchema() error: %w", meta.TypeName, err)
			}

			ds[runtypes.TypeOrRenamedEntityName(meta.TypeName)] = entry[func() action.Action]{
				t:      makeAction,
				schema: FromActionSchema(actionSchema),
				tfName: runtypes.TypeName(meta.TypeName),
			}
		}
	}

	return &actions{collection: ds, convert: f}, nil
}

type actions struct {
	collection[func() action.Action]
	convert func(Schema) shim.SchemaMap
}

func (r actions) Schema(t runtypes.TypeOrRenamedEntityName) runtypes.Schema {
	entry := r.collection[t]
	return runtypesSchemaAdapter{entry.schema, r.convert, entry.tfName}
}

func (actions) IsActions() {}
