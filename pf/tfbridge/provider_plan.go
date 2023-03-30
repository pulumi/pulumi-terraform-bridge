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

	"github.com/hashicorp/go-multierror"
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
	priorState *upgradedResourceState,
	checkedInputs tftypes.Value,
) (*tfprotov6.PlanResourceChangeResponse, error) {

	resSchema, ok := p.schemaResponse.ResourceSchemas[typeName]
	if !ok {
		return nil, fmt.Errorf("Unknown typeName: %q", typeName)
	}

	schemaBlock := resSchema.Block

	proposedNewState, err := pfutils.ProposedNew(ctx, schemaBlock, priorState.state.Value, checkedInputs)
	if err != nil {
		return nil, err
	}

	priorStateV, configV, proposedNewStateV, err := makeDynamicValues3(
		priorState.state.Value, checkedInputs, proposedNewState)
	if err != nil {
		return nil, err
	}

	planReq := tfprotov6.PlanResourceChangeRequest{
		TypeName:         typeName,
		PriorState:       &priorStateV,
		ProposedNewState: &proposedNewStateV,
		Config:           &configV,

		// TODO[pulumi/pulumi-terraform-bridge#747] PriorPrivate
		// TODO[pulumi/pulumi-terraform-bridge#794] set ProviderMeta
	}

	planResp, err := p.tfServer.PlanResourceChange(ctx, &planReq)
	if err != nil {
		return nil, err
	}

	var diags multierror.Error
	for _, diag := range planResp.Diagnostics {
		if diag.Severity == tfprotov6.DiagnosticSeverityError {
			diags.Errors = append(diags.Errors, fmt.Errorf("%s: %s - %s", diag.Attribute, diag.Summary, diag.Detail))
		}
	}

	return planResp, diags.ErrorOrNil()
}
