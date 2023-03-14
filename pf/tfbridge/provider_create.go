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

// Create allocates a new instance of the provided resource and returns its unique resource ID.
func (p *provider) CreateWithContext(
	ctx context.Context,
	urn resource.URN,
	checkedInputs resource.PropertyMap,
	timeout float64,
	preview bool,
) (resource.ID, resource.PropertyMap, resource.Status, error) {
	ctx = initLogging(ctx)

	rh, err := p.resourceHandle(ctx, urn)
	if err != nil {
		return "", nil, 0, err
	}

	tfType := rh.schema.Type().TerraformType(ctx).(tftypes.Object)

	priorState := newResourceState(ctx, &rh)

	checkedInputsValue, err := convert.EncodePropertyMap(rh.encoder, checkedInputs)
	if err != nil {
		return "", nil, 0, err
	}

	planResp, err := p.plan(ctx, rh.terraformResourceName, rh.schema, priorState, checkedInputsValue)
	if err != nil {
		return "", nil, 0, err
	}

	if err := p.processDiagnostics(planResp.Diagnostics); err != nil {
		return "", nil, 0, err
	}

	// TODO[pulumi/pulumi-terraform-bridge#747] handle planResp.PlannedPrivate

	// NOTE: it seems that planResp.RequiresReplace can be ignored in Create and must be false.

	if preview {
		plannedStatePropertyMap, err := convert.DecodePropertyMapFromDynamic(
			rh.decoder, tfType, planResp.PlannedState)
		if err != nil {
			return "", nil, 0, err
		}
		return "", plannedStatePropertyMap, resource.StatusOK, nil
	}

	priorStateValue, configValue, err := makeDynamicValues2(priorState.state.Value, checkedInputsValue)
	if err != nil {
		return "", nil, 0, err
	}

	req := tfprotov6.ApplyResourceChangeRequest{
		TypeName:     rh.terraformResourceName,
		PriorState:   &priorStateValue,
		PlannedState: planResp.PlannedState,
		Config:       &configValue,

		// TODO[pulumi/pulumi-terraform-bridge#747] PlannedPrivate []byte{},
		// TODO[pulumi/pulumi-terraform-bridge#794] set ProviderMeta
		//
		// See https://www.terraform.io/internals/provider-meta
	}

	resp, err := p.tfServer.ApplyResourceChange(ctx, &req)
	if err != nil {
		return "", nil, 0, err
	}

	if err := p.processDiagnostics(resp.Diagnostics); err != nil {
		return "", nil, 0, err
	}

	// TODO[pulumi/pulumi-terraform-bridge#747] handle resp.Private field to save that state inside Pulumi state.

	createdState, err := parseResourceStateFromTF(ctx, &rh, resp.NewState)
	if err != nil {
		return "", nil, 0, err
	}

	createdStateMap, err := createdState.ToPropertyMap(&rh)
	if err != nil {
		return "", nil, 0, err
	}

	createdID, err := createdState.ExtractID(&rh)
	if err != nil {
		return "", nil, 0, err
	}

	return createdID, createdStateMap, resource.StatusOK, nil
}
