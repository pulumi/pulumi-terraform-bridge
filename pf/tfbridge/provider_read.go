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

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
)

// Read the current live state associated with a resource. Enough state must be include in the inputs to uniquely
// identify the resource; this is typically just the resource ID, but may also include some properties. If the resource
// is missing (for instance, because it has been deleted), the resulting property map will be nil.
func (p *provider) Read(
	urn resource.URN,
	id resource.ID,
	inputs,
	currentStateMap resource.PropertyMap,
) (plugin.ReadResult, resource.Status, error) {
	// TODO test for a resource that is not found

	ctx := context.TODO()

	rh, err := p.resourceHandle(ctx, urn)
	if err != nil {
		return plugin.ReadResult{}, 0, err
	}

	currentStateRaw, err := parseResourceState(&rh, currentStateMap)
	if err != nil {
		return plugin.ReadResult{}, 0, err
	}

	currentState, err := p.UpgradeResourceState(ctx, &rh, currentStateRaw)
	if err != nil {
		return plugin.ReadResult{}, 0, err
	}

	currentStateDV, err := makeDynamicValue(currentState.state.Value)
	if err != nil {
		return plugin.ReadResult{}, 0, err
	}

	req := tfprotov6.ReadResourceRequest{
		TypeName:     rh.terraformResourceName,
		CurrentState: &currentStateDV,
	}

	// TODO Set ProviderMeta
	// TODO Set Private

	resp, err := p.tfServer.ReadResource(ctx, &req)
	if err != nil {
		return plugin.ReadResult{}, 0, err
	}

	if err := p.processDiagnostics(resp.Diagnostics); err != nil {
		return plugin.ReadResult{}, 0, err
	}

	// TODO handle resp.Private

	if resp.NewState == nil {
		return plugin.ReadResult{}, resource.StatusUnknown, nil
	}

	readState, err := parseResourceStateFromTF(ctx, &rh, resp.NewState)
	if err != nil {
		return plugin.ReadResult{}, 0, err
	}

	readID, err := readState.ExtractID(&rh)
	if err != nil {
		return plugin.ReadResult{}, 0, err
	}

	readStateMap, err := readState.ToPropertyMap(&rh)
	if err != nil {
		return plugin.ReadResult{}, 0, err
	}

	return plugin.ReadResult{
		ID: readID,
		// TODO support populating inputs, see extractInputsFromOutputs in the prod bridge.
		Inputs:  nil,
		Outputs: readStateMap,
	}, resource.StatusOK, nil
}
