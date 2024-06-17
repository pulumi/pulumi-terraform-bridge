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
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/convert"
)

// Create allocates a new instance of the provided resource and returns its unique resource ID.
func (p *provider) Create(
	ctx context.Context,
	req plugin.CreateRequest,
) (plugin.CreateResponse, error) {
	ctx = p.initLogging(ctx, p.logSink, req.URN)

	rh, err := p.resourceHandle(ctx, req.URN)
	if err != nil {
		return plugin.CreateResponse{}, err
	}

	tfType := rh.schema.Type(ctx).(tftypes.Object)

	priorState := newResourceState(ctx, &rh, nil /*private state*/)

	checkedInputsValue, err := convert.EncodePropertyMap(rh.encoder, req.Properties)
	if err != nil {
		return plugin.CreateResponse{}, err
	}

	planResp, err := p.plan(ctx, rh.terraformResourceName, rh.schema, priorState, checkedInputsValue)
	if err != nil {
		return plugin.CreateResponse{}, err
	}

	if err := p.processDiagnostics(planResp.Diagnostics); err != nil {
		return plugin.CreateResponse{}, err
	}

	// NOTE: it seems that planResp.RequiresReplace can be ignored in Create and must be false.

	if req.Preview {
		plannedStatePropertyMap, err := convert.DecodePropertyMapFromDynamic(ctx,
			rh.decoder, tfType, planResp.PlannedState)
		if err != nil {
			return plugin.CreateResponse{}, err
		}

		if rh.pulumiResourceInfo.TransformOutputs != nil {
			var err error
			plannedStatePropertyMap, err = rh.pulumiResourceInfo.TransformOutputs(ctx,
				plannedStatePropertyMap)
			if err != nil {
				return plugin.CreateResponse{}, err
			}
		}

		return plugin.CreateResponse{
			ID:         "",
			Properties: plannedStatePropertyMap,
			Status:     resource.StatusOK,
		}, nil
	}

	priorStateValue, configValue, err := makeDynamicValues2(priorState.state.Value, checkedInputsValue)
	if err != nil {
		return plugin.CreateResponse{}, err
	}

	tfReq := tfprotov6.ApplyResourceChangeRequest{
		TypeName:       rh.terraformResourceName,
		PriorState:     &priorStateValue,
		PlannedState:   planResp.PlannedState,
		Config:         &configValue,
		PlannedPrivate: planResp.PlannedPrivate,
		// TODO[pulumi/pulumi-terraform-bridge#794] set ProviderMeta
		//
		// See https://www.terraform.io/internals/provider-meta
	}

	resp, err := p.tfServer.ApplyResourceChange(ctx, &tfReq)
	if err != nil {
		return plugin.CreateResponse{}, err
	}

	if err := p.processDiagnostics(resp.Diagnostics); err != nil {
		return plugin.CreateResponse{}, err
	}

	createdState, err := parseResourceStateFromTF(ctx, &rh, resp.NewState, resp.Private)
	if err != nil {
		return plugin.CreateResponse{}, err
	}

	createdStateMap, err := createdState.ToPropertyMap(ctx, &rh)
	if err != nil {
		return plugin.CreateResponse{}, err
	}

	if rh.pulumiResourceInfo.TransformOutputs != nil {
		var err error
		createdStateMap, err = rh.pulumiResourceInfo.TransformOutputs(ctx, createdStateMap)
		if err != nil {
			return plugin.CreateResponse{}, err
		}
	}

	rn := rh.terraformResourceName
	createdID, err := extractID(ctx, rn, rh.pulumiResourceInfo, createdStateMap)
	if err != nil {
		return plugin.CreateResponse{}, err
	}

	return plugin.CreateResponse{
		ID:         createdID,
		Properties: createdStateMap,
		Status:     resource.StatusOK,
	}, nil
}
