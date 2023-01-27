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
func (p *provider) Create(
	urn resource.URN,
	checkedInputs resource.PropertyMap,
	timeout float64,
	preview bool,
) (resource.ID, resource.PropertyMap, resource.Status, error) {
	ctx := context.TODO()

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

	// TODO handle planResp.Diagnostics
	// TODO handle planResp.PlannedPrivate
	// TODO handle planResp.RequiresReplace - probably can be ignored in Create

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

		// TODO PlannedPrivate []byte{},
		// TODO Set ProviderMeta
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

	// TODO handle resp.Private field to save that state inside Pulumi state.

	stateValue, err := resp.NewState.Unmarshal(tfType)
	if err != nil {
		return "", nil, 0, err
	}

	idString, err := rh.idExtractor.extractID(stateValue)
	if err != nil {
		return "", nil, 0, err
	}

	createdState, err := convert.DecodePropertyMap(rh.decoder, stateValue)
	if err != nil {
		return "", nil, 0, err
	}

	return resource.ID(idString), createdState, resource.StatusOK, nil
}
