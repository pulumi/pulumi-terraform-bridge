// Copyright 2016-2025, Pulumi Corporation.
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
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

type listResourceHandle struct {
	token                     tokens.ModuleMember
	terraformListResourceName string
	schema                    runtypes.Schema
	encoder                   convert.Encoder
	decoder                   convert.Decoder
	schemaOnlyShim            shim.Resource      // optional
	pulumiListResourceInfo    *info.ListResource // optional
}

func (p *provider) isListResource(pulumiType tokens.ModuleMember) (string, *info.ListResource, bool) {
	for tfResourceType, res := range p.info.ListResources {
		if pulumiType == tokens.ModuleMember(res.Tok) {
			return tfResourceType, res, true
		}
	}

	return "", nil, false
}

func (p *provider) listResourceHandle(ctx context.Context, token tokens.ModuleMember) (listResourceHandle, error) {
	listResourceName, info, ok := p.isListResource(token)
	if !ok {
		return listResourceHandle{}, fmt.Errorf("unknown list resource %q", token)
	}

	schema := p.listResources.Schema(runtypes.TypeOrRenamedEntityName(listResourceName))
	typ := schema.Type(ctx).(tftypes.Object)

	encoder, err := p.encoding.NewDataSourceEncoder(listResourceName, typ)
	if err != nil {
		return listResourceHandle{}, err
	}

	decoder, err := p.encoding.NewDataSourceDecoder(listResourceName, typ)
	if err != nil {
		return listResourceHandle{}, err
	}

	shim, _ := p.schemaOnlyProvider.ListResourcesMap().GetOk(listResourceName)

	result := listResourceHandle{
		token:                     token,
		terraformListResourceName: listResourceName,
		schema:                    schema,
		encoder:                   encoder,
		decoder:                   decoder,
		schemaOnlyShim:            shim,
		pulumiListResourceInfo:    info,
	}

	return result, nil
}
