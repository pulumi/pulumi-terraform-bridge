// Copyright 2016-2023, Pulumi Corporation.
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

	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/convert"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/internal/runtypes"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

type actionHandle struct {
	token               tokens.ModuleMember
	terraformActionName string
	schema              runtypes.Schema
	encoder             convert.Encoder
	schemaOnlyShim      shim.Action
	pulumiActionInfo    *tfbridge.ActionInfo // optional
}

func (p *provider) actionHandle(ctx context.Context, token tokens.ModuleMember) (actionHandle, bool, error) {
	actName, has := p.terraformActionNameOrRenamedEntity(token)
	if !has {
		return actionHandle{}, false, nil
	}

	schema := p.actions.Schema(runtypes.TypeOrRenamedEntityName(actName))

	typ := schema.Type(ctx).(tftypes.Object)

	encoder, err := p.encoding.NewActionEncoder(actName, typ)
	if err != nil {
		return actionHandle{}, true, err
	}

	shim, _ := p.schemaOnlyProvider.ActionsMap().GetOk(actName)

	result := actionHandle{
		token:               token,
		terraformActionName: actName,
		schema:              schema,
		encoder:             encoder,
		schemaOnlyShim:      shim,
	}

	if info, ok := p.info.Actions[actName]; ok {
		result.pulumiActionInfo = info
	}

	return result, true, nil
}
