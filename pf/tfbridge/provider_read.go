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
	"fmt"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"

	"github.com/pulumi/pulumi-terraform-bridge/pf/internal/schemashim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
)

// Read the current live state associated with a resource. Enough state must be include in the inputs to uniquely
// identify the resource; this is typically just the resource ID, but may also include some properties. If the resource
// is missing (for instance, because it has been deleted), the resulting property map will be nil.
func (p *provider) Read(
	urn resource.URN,
	id resource.ID,
	oldInputs,
	currentStateMap resource.PropertyMap,
) (plugin.ReadResult, resource.Status, error) {
	var err error

	// Returning Status is required by the signature but ignored by the server implementation.
	var ignoredStatus resource.Status = resource.StatusOK

	// TODO[pulumi/pulumi-terraform-bridge#793] Add a test for Read handling a not-found resource
	ctx := context.TODO()

	rh, err := p.resourceHandle(ctx, urn)
	if err != nil {
		return plugin.ReadResult{}, 0, err
	}

	// Both "get" and "refresh" scenarios call Read. Detect and dispatch.
	isRefresh := len(currentStateMap) != 0

	var result plugin.ReadResult

	if isRefresh {
		result, err = p.readViaReadResource(ctx, &rh, id, oldInputs, currentStateMap)
	} else {
		result, err = p.readViaImportResourceState(ctx, &rh, id)
	}
	if err != nil {
		return result, ignoredStatus, err
	}

	if result.Outputs != nil {
		result.Inputs, err = tfbridge.ExtractInputsFromOutputs(
			oldInputs,
			result.Outputs,
			schemashim.NewSchemaMap(rh.schema),
			rh.pulumiResourceInfo.Fields,
			isRefresh)
		if err != nil {
			return result, ignoredStatus, err
		}

		// __defaults is not needed for Plugin Framework bridged providers
		if _, ok := result.Inputs["__defaults"]; ok {
			delete(result.Inputs, "__defaults")
		}
	}

	return result, ignoredStatus, err
}

func (p *provider) readViaReadResource(
	ctx context.Context,
	rh *resourceHandle,
	id resource.ID,
	unusedInputs,
	currentStateMap resource.PropertyMap,
) (plugin.ReadResult, error) {

	currentStateRaw, err := parseResourceState(rh, currentStateMap)
	if err != nil {
		return plugin.ReadResult{}, err
	}

	currentState, err := p.UpgradeResourceState(ctx, rh, currentStateRaw)
	if err != nil {
		return plugin.ReadResult{}, err
	}

	currentStateDV, err := makeDynamicValue(currentState.state.Value)
	if err != nil {
		return plugin.ReadResult{}, err
	}

	req := tfprotov6.ReadResourceRequest{
		TypeName:     rh.terraformResourceName,
		CurrentState: &currentStateDV,
	}

	// TODO[pulumi/pulumi-terraform-bridge#794] set ProviderMeta
	// TODO[pulumi/pulumi-terraform-bridge#747] set Private

	resp, err := p.tfServer.ReadResource(ctx, &req)
	if err != nil {
		return plugin.ReadResult{}, err
	}

	if err := p.processDiagnostics(resp.Diagnostics); err != nil {
		return plugin.ReadResult{}, err
	}

	// TODO[pulumi/pulumi-terraform-bridge#747] handle resp.Private
	if resp.NewState == nil {
		return plugin.ReadResult{}, nil
	}

	readState, err := parseResourceStateFromTF(ctx, rh, resp.NewState)
	if err != nil {
		return plugin.ReadResult{}, err
	}

	readID, err := readState.ExtractID(rh)
	if err != nil {
		readID = ""
	}

	readStateMap, err := readState.ToPropertyMap(rh)
	if err != nil {
		return plugin.ReadResult{}, err
	}

	return plugin.ReadResult{
		ID:      readID,
		Outputs: readStateMap,
	}, nil
}

func (p *provider) readViaImportResourceState(
	ctx context.Context,
	rh *resourceHandle,
	id resource.ID,
) (plugin.ReadResult, error) {
	// TODO[pulumi/pulumi-terraform-bridge#794] set ProviderMeta
	// TODO[pulumi/pulumi-terraform-bridge#747] set Private
	req := tfprotov6.ImportResourceStateRequest{
		TypeName: rh.terraformResourceName,
		ID:       string(id),
	}

	resp, err := p.tfServer.ImportResourceState(ctx, &req)
	if err != nil {
		return plugin.ReadResult{}, err
	}

	// TODO[pulumi/pulumi-terraform-bridge#747] handle resp.Private

	if err := p.processDiagnostics(resp.Diagnostics); err != nil {
		return plugin.ReadResult{}, err
	}

	if len(resp.ImportedResources) > 1 {
		return plugin.ReadResult{},
			fmt.Errorf("ImportResourceState returned more than one result, " +
				"but reading only one is supported by Pulumi")
	}

	if len(resp.ImportedResources) == 0 {
		return plugin.ReadResult{},
			fmt.Errorf("ImportResourceState failed to return a result")
	}

	r := resp.ImportedResources[0]

	readState, err := parseResourceStateFromTF(ctx, rh, r.State)
	if err != nil {
		return plugin.ReadResult{}, err
	}

	finalID, err := readState.ExtractID(rh)
	if err != nil {
		return plugin.ReadResult{}, err
	}

	readStateMap, err := readState.ToPropertyMap(rh)
	if err != nil {
		return plugin.ReadResult{}, err
	}

	return plugin.ReadResult{
		ID:      finalID,
		Outputs: readStateMap,
	}, nil
}
