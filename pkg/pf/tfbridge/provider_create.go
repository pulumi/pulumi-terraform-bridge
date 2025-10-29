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

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/convert"
)

// Create allocates a new instance of the provided resource and returns its unique resource ID.
func (p *provider) CreateWithContext(
	ctx context.Context,
	urn resource.URN,
	checkedInputs resource.PropertyMap,
	timeout float64,
	preview bool,
) (resource.ID, resource.PropertyMap, resource.Status, error) {
	ctx = p.initLogging(ctx, p.logSink, urn)

	rh, has, err := p.resourceHandle(ctx, urn)
	if err != nil {
		return "", nil, 0, err
	}

	if has {
		tfType := rh.schema.Type(ctx).(tftypes.Object)

		priorState := newResourceState(ctx, rh.schema, nil /*private state*/)

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

		// NOTE: it seems that planResp.RequiresReplace can be ignored in Create and must be false.

		if preview {
			plannedStatePropertyMap, err := convert.DecodePropertyMapFromDynamic(ctx,
				rh.decoder, tfType, planResp.PlannedState)
			if err != nil {
				return "", nil, 0, err
			}

			if rh.pulumiResourceInfo.TransformOutputs != nil {
				var err error
				plannedStatePropertyMap, err = rh.pulumiResourceInfo.TransformOutputs(ctx,
					plannedStatePropertyMap)
				if err != nil {
					return "", nil, 0, err
				}
			}

			return "", plannedStatePropertyMap, resource.StatusOK, nil
		}

		priorStateValue, configValue, err := makeDynamicValues2(priorState.Value, checkedInputsValue)
		if err != nil {
			return "", nil, 0, err
		}

		req := tfprotov6.ApplyResourceChangeRequest{
			TypeName:       rh.terraformResourceName,
			PriorState:     &priorStateValue,
			PlannedState:   planResp.PlannedState,
			Config:         &configValue,
			PlannedPrivate: planResp.PlannedPrivate,
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

		createdState, err := parseResourceStateFromTF(ctx, rh.schema, resp.NewState, resp.Private)
		if err != nil {
			return "", nil, 0, err
		}

		createdStateMap, err := createdState.ToPropertyMap(ctx, rh.decoder)
		if err != nil {
			return "", nil, 0, err
		}

		if rh.pulumiResourceInfo.TransformOutputs != nil {
			var err error
			createdStateMap, err = rh.pulumiResourceInfo.TransformOutputs(ctx, createdStateMap)
			if err != nil {
				return "", nil, 0, err
			}
		}

		rn := rh.terraformResourceName
		createdID, err := extractID(ctx, rn, rh.pulumiResourceInfo, createdStateMap)
		if err != nil {
			return "", nil, 0, err
		}

		if err := insertRawStateDelta(ctx, &rh, createdStateMap, createdState.Value); err != nil {
			return "", nil, 0, err
		}

		return createdID, createdStateMap, resource.StatusOK, nil
	}

	eh, has, err := p.ephemeralResourceHandle(ctx, urn)
	if err != nil {
		return "", nil, 0, err
	}
	if has {
		checkedInputsValue, err := convert.EncodePropertyMap(eh.encoder, checkedInputs)
		if err != nil {
			return "", nil, 0, err
		}

		configValue, err := makeDynamicValue(checkedInputsValue)
		if err != nil {
			return "", nil, 0, err
		}

		req := tfprotov6.OpenEphemeralResourceRequest{
			TypeName: eh.terraformResourceName,
			Config:   &configValue,
		}

		resp, err := p.tfServer.OpenEphemeralResource(ctx, &req)
		if err != nil {
			return "", nil, 0, err
		}

		if err := p.processDiagnostics(resp.Diagnostics); err != nil {
			return "", nil, 0, err
		}

		createdState, err := parseResourceStateFromTF(ctx, eh.schema, resp.Result, resp.Private)
		if err != nil {
			return "", nil, 0, err
		}

		createdStateMap, err := createdState.ToPropertyMap(ctx, eh.decoder)
		if err != nil {
			return "", nil, 0, err
		}

		if eh.pulumiResourceInfo.TransformOutputs != nil {
			var err error
			createdStateMap, err = eh.pulumiResourceInfo.TransformOutputs(ctx, createdStateMap)
			if err != nil {
				return "", nil, 0, err
			}
		}

		rn := eh.terraformResourceName
		createdID, err := extractID(ctx, rn, eh.pulumiResourceInfo, createdStateMap)
		if err != nil {
			return "", nil, 0, err
		}

		// Save any provider private state for this ephemeral resource.
		p.privateState[string(urn)] = resp.Private

		// TODO: Setup a timer to call Renew on this resource if needed.

		return createdID, createdStateMap, resource.StatusOK, nil
	}

	return "", nil, 0, fmt.Errorf("[pf/tfbridge] unknown resource token: %v", urn.Type())
}
