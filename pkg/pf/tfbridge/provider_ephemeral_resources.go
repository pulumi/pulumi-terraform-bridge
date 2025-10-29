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
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/convert"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/internal/runtypes"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

type ephemeralResourceHandle struct {
	token                          tokens.ModuleMember
	terraformEphemeralResourceName string
	schema                         runtypes.Schema
	pulumiEphemeralResourceInfo    *tfbridge.EphemeralResourceInfo // optional
	encoder                        convert.Encoder
	decoder                        convert.Decoder
	schemaOnlyShim                 shim.Resource
}

func (p *provider) ephemeralResourceHandle(
	ctx context.Context, tok tokens.ModuleMember,
) (ephemeralResourceHandle, bool, error) {
	typeOrRenamedEntityName, has := p.terraformEphemeralResourceNameOrRenamedEntity(tok)
	if !has {
		return ephemeralResourceHandle{}, false, nil
	}

	schema := p.ephemeralResources.Schema(runtypes.TypeOrRenamedEntityName(typeOrRenamedEntityName))

	result := ephemeralResourceHandle{
		terraformEphemeralResourceName: string(schema.TFName()),
		schema:                         schema,
	}

	if info, ok := p.info.EphemeralResources[typeOrRenamedEntityName]; ok {
		result.pulumiEphemeralResourceInfo = info
	}

	token := result.pulumiEphemeralResourceInfo.Tok
	if token == "" {
		return ephemeralResourceHandle{}, true, fmt.Errorf("Tok cannot be empty: %s", token)
	}

	objectType := result.schema.Type(ctx).(tftypes.Object)

	encoder, err := p.encoding.NewEphemeralResourceEncoder(typeOrRenamedEntityName, objectType)
	if err != nil {
		return ephemeralResourceHandle{}, true, fmt.Errorf("Failed to prepare an ephemeral resource encoder: %s", err)
	}

	outputsDecoder, err := p.encoding.NewEphemeralResourceDecoder(typeOrRenamedEntityName, objectType)
	if err != nil {
		return ephemeralResourceHandle{}, true, fmt.Errorf("Failed to prepare an ephemeral resource decoder: %s", err)
	}

	result.encoder = encoder
	result.decoder = outputsDecoder
	result.token = token

	result.schemaOnlyShim, _ = p.schemaOnlyProvider.EphemeralResourcesMap().GetOk(typeOrRenamedEntityName)
	return result, true, nil
}
