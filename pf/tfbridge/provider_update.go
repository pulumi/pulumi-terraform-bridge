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

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"

	"github.com/pulumi/pulumi-terraform-bridge/pf/internal/convert"
)

// Update updates an existing resource with new values.
func (p *provider) UpdateWithContext(
	ctx context.Context,
	urn resource.URN,
	id resource.ID,
	priorStateMap resource.PropertyMap,
	checkedInputs resource.PropertyMap,
	timeout float64,
	ignoreChanges []string,
	preview bool,
) (resource.PropertyMap, resource.Status, error) {
	ctx = p.initLogging(ctx, p.logSink, urn)

	rh, err := p.resourceHandle(ctx, urn)
	if err != nil {
		return nil, 0, err
	}

	tfType := rh.schema.Type().TerraformType(ctx).(tftypes.Object)

	rawPriorState, err := parseResourceState(&rh, priorStateMap)
	if err != nil {
		return nil, 0, err
	}

	priorState, err := p.UpgradeResourceState(ctx, &rh, rawPriorState)
	if err != nil {
		return nil, 0, err
	}

	checkedInputsValue, err := convert.EncodePropertyMap(rh.encoder, checkedInputs)
	if err != nil {
		return nil, 0, err
	}

	planResp, err := p.plan(ctx, rh.terraformResourceName, rh.schema, priorState, checkedInputsValue)
	if err != nil {
		return nil, 0, err
	}

	if preview {
		plannedStatePropertyMap, err := convert.DecodePropertyMapFromDynamic(
			rh.decoder, tfType, planResp.PlannedState)
		if err != nil {
			return nil, 0, err
		}
		return plannedStatePropertyMap, resource.StatusOK, nil
	}

	priorStateDV, checkedInputsDV, err := makeDynamicValues2(priorState.state.Value, checkedInputsValue)
	if err != nil {
		return nil, 0, err
	}

	req := tfprotov6.ApplyResourceChangeRequest{
		TypeName:     rh.terraformResourceName,
		Config:       &checkedInputsDV,
		PriorState:   &priorStateDV,
		PlannedState: planResp.PlannedState,
	}

	resp, err := p.tfServer.ApplyResourceChange(ctx, &req)
	if err != nil {
		return nil, 0, err
	}

	if err := p.processDiagnostics(resp.Diagnostics); err != nil {
		return nil, 0, err
	}

	// TODO[pulumi/pulumi-terraform-bridge#747] handle resp.Private
	updatedState, err := parseResourceStateFromTF(ctx, &rh, resp.NewState)
	if err != nil {
		return nil, 0, err
	}

	updatedStateMap, err := updatedState.ToPropertyMap(&rh)
	if err != nil {
		return nil, 0, err
	}

	return updatedStateMap, resource.StatusOK, nil
}
