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

package tfbridge

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
	pulumiresource "github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/convert"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/internal/runtypes"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

type resourceHandle struct {
	token                  tokens.Type
	terraformResourceName  string
	schema                 runtypes.Schema
	pulumiResourceInfo     *info.Resource // optional
	encoder                convert.Encoder
	decoder                convert.Decoder
	schemaOnlyShimResource shim.Resource
}

func (p *provider) resourceHandle(ctx context.Context, urn pulumiresource.URN) (resourceHandle, error) {
	typeOrRenamedEntityName, err := p.terraformResourceNameOrRenamedEntity(urn.Type())
	if err != nil {
		return resourceHandle{}, err
	}

	schema := p.resources.Schema(runtypes.TypeOrRenamedEntityName(typeOrRenamedEntityName))

	result := resourceHandle{
		terraformResourceName: string(schema.TFName()),
		schema:                schema,
	}

	if info, ok := p.info.Resources[typeOrRenamedEntityName]; ok {
		result.pulumiResourceInfo = info
	}

	token := result.pulumiResourceInfo.Tok
	if token == "" {
		return resourceHandle{}, fmt.Errorf("Tok cannot be empty: %s", token)
	}

	objectType := result.schema.Type(ctx).(tftypes.Object)

	encoder, err := p.encoding.NewResourceEncoder(typeOrRenamedEntityName, objectType)
	if err != nil {
		return resourceHandle{}, fmt.Errorf("Failed to prepare a resource encoder: %s", err)
	}

	outputsDecoder, err := p.encoding.NewResourceDecoder(typeOrRenamedEntityName, objectType)
	if err != nil {
		return resourceHandle{}, fmt.Errorf("Failed to prepare an resoure decoder: %s", err)
	}

	result.encoder = encoder
	result.decoder = outputsDecoder
	result.token = token

	result.schemaOnlyShimResource, _ = p.schemaOnlyProvider.ResourcesMap().GetOk(typeOrRenamedEntityName)
	return result, nil
}

func transformFromState(
	ctx context.Context, rh resourceHandle, state pulumiresource.PropertyMap,
) (pulumiresource.PropertyMap, error) {
	if rh.pulumiResourceInfo == nil {
		return state, nil
	}
	f := rh.pulumiResourceInfo.TransformFromState
	if f == nil {
		return state, nil
	}
	o, err := f(ctx, state)
	if err != nil {
		return nil, fmt.Errorf("transforming from state: %w", err)
	}
	return o, err
}
