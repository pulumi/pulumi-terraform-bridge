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

	pfresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	pulumiresource "github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"

	"github.com/pulumi/pulumi-terraform-bridge/pkg/tfpfbridge/info"
	"github.com/pulumi/pulumi-terraform-bridge/pkg/tfpfbridge/internal/convert"
	"github.com/pulumi/pulumi-terraform-bridge/pkg/tfpfbridge/pfutils"
)

type resourceHandle struct {
	token                 tokens.Type
	makeResource          func() pfresource.Resource
	terraformResourceName string
	schema                pfutils.Schema
	pulumiResourceInfo    *info.ResourceInfo // optional
	idExtractor           idExtractor
	encoder               convert.Encoder
	decoder               convert.Decoder
}

func (p *Provider) resourceHandle(ctx context.Context, urn pulumiresource.URN) (resourceHandle, error) {
	resources := p.resources

	typeName, err := p.terraformResourceName(urn.Type())
	if err != nil {
		return resourceHandle{}, err
	}

	n := pfutils.TypeName(typeName)
	schema := resources.Schema(n)

	idExtractor, err := newIdExtractor(ctx, typeName, schema)
	if err != nil {
		return resourceHandle{}, err
	}

	result := resourceHandle{
		makeResource: func() pfresource.Resource {
			return resources.Resource(n)
		},
		terraformResourceName: typeName,
		schema:                schema,
		idExtractor:           idExtractor,
	}

	if info, ok := p.info.Resources[typeName]; ok {
		result.pulumiResourceInfo = info
	}

	token := result.pulumiResourceInfo.Tok
	if token == "" {
		return resourceHandle{}, fmt.Errorf("Tok cannot be empty: %s", token)
	}

	objectType := result.schema.Type().TerraformType(ctx).(tftypes.Object)

	encoder, err := p.encoding.NewResourceEncoder(token, objectType)
	if err != nil {
		return resourceHandle{}, fmt.Errorf("Failed to prepare a resource encoder: %s", err)
	}

	outputsDecoder, err := p.encoding.NewResourceDecoder(token, objectType)
	if err != nil {
		return resourceHandle{}, fmt.Errorf("Failed to prepare an resoure decoder: %s", err)
	}

	result.encoder = encoder
	result.decoder = outputsDecoder
	result.token = token
	return result, nil
}
