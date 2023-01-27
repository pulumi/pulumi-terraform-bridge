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

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/pulumi/pulumi-terraform-bridge/pf/internal/pfutils"
)

// Simplifies calling PlanResourceChanges in Terraform.
//
// Pulumi providers use plan in a few places, for example:
//
//	Diff(priorState, checkedInputs):
//	    plannedState = plan(priorState, checkedInputs)
//	    priorState.Diff(plannedState)
//
// And create is a special case with priorState=nilState:
//
//	Create(checkedInputs):
//	    plannedState = plan(nilState, checkedInputs)
//	    ApplyResourceChange(plannedState)
func (p *provider) plan(
	ctx context.Context,
	typeName string,
	schema pfutils.Schema,
	priorState *resourceState,
	checkedInputs tftypes.Value,
) (*tfprotov6.PlanResourceChangeResponse, error) {
	proposedNewState, err := pfutils.ProposedNew(ctx, schema, priorState.Value, checkedInputs)
	if err != nil {
		return nil, err
	}

	priorStateV, configV, proposedNewStateV, err := makeDynamicValues3(
		priorState.Value, checkedInputs, proposedNewState)
	if err != nil {
		return nil, err
	}

	planReq := tfprotov6.PlanResourceChangeRequest{
		TypeName:         typeName,
		PriorState:       &priorStateV,
		ProposedNewState: &proposedNewStateV,
		Config:           &configV,

		// TODO PriorPrivate
		// TODO ProviderMeta
	}

	planResp, err := p.tfServer.PlanResourceChange(ctx, &planReq)
	if err != nil {
		return nil, err
	}

	return planResp, nil
}
