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

	// TODO[pulumi/pulumi-terraform-bridge#793] Add a test for Read handling a not-found resource
	ctx := context.TODO()

	rh, err := p.resourceHandle(ctx, urn)
	if err != nil {
		return plugin.ReadResult{}, 0, err
	}

	// Both "get" and "refresh" scenarios call Read. Detect and dispatch.
	isRefresh := len(currentStateMap) != 0

	if isRefresh {
		return p.readViaReadResource(ctx, &rh, id, inputs, currentStateMap)
	}

	return p.readViaImportResourceState(ctx, &rh, id)

	// panic(fmt.Sprintf("READ urn=%v id=%v inputs=%v currentStateMap=%v", urn, id, len(inputs), len(currentStateMap)))
	// panic: READ urn=urn:pulumi:dev::re::random:index/randomPassword:RandomPassword::newPassword id=supersecret inputs=0 currentStateMap=0
}

func (p *provider) readViaReadResource(
	ctx context.Context,
	rh *resourceHandle,
	id resource.ID,
	unusedInputs,
	currentStateMap resource.PropertyMap,
) (plugin.ReadResult, resource.Status, error) {

	currentStateRaw, err := parseResourceState(rh, currentStateMap)
	if err != nil {
		return plugin.ReadResult{}, 0, err
	}

	currentState, err := p.UpgradeResourceState(ctx, rh, currentStateRaw)
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

	// TODO[pulumi/pulumi-terraform-bridge#794] set ProviderMeta
	// TODO[pulumi/pulumi-terraform-bridge#747] set Private

	resp, err := p.tfServer.ReadResource(ctx, &req)
	if err != nil {
		return plugin.ReadResult{}, 0, err
	}

	if err := p.processDiagnostics(resp.Diagnostics); err != nil {
		return plugin.ReadResult{}, 0, err
	}

	// TODO[pulumi/pulumi-terraform-bridge#747] handle resp.Private
	if resp.NewState == nil {
		return plugin.ReadResult{}, resource.StatusOK, nil
	}

	readState, err := parseResourceStateFromTF(ctx, rh, resp.NewState)
	if err != nil {
		return plugin.ReadResult{}, 0, err
	}

	readID, err := readState.ExtractID(rh)
	if err != nil {
		readID = ""
	}

	readStateMap, err := readState.ToPropertyMap(rh)
	if err != nil {
		return plugin.ReadResult{}, 0, err
	}

	return plugin.ReadResult{
		ID: readID,
		// TODO[pulumi/pulumi-terraform-bridge#795] populate Inputs
		Inputs:  nil,
		Outputs: readStateMap,
	}, resource.StatusOK, nil
}

func (p *provider) readViaImportResourceState(
	ctx context.Context,
	rh *resourceHandle,
	id resource.ID,
) (plugin.ReadResult, resource.Status, error) {
	// TODO[pulumi/pulumi-terraform-bridge#794] set ProviderMeta
	// TODO[pulumi/pulumi-terraform-bridge#747] set Private
	req := tfprotov6.ImportResourceStateRequest{
		TypeName: rh.terraformResourceName,
		ID:       string(id),
	}

	resp, err := p.tfServer.ImportResourceState(ctx, &req)
	if err != nil {
		return plugin.ReadResult{}, 0, err
	}

	// TODO[pulumi/pulumi-terraform-bridge#747] handle resp.Private

	if err := p.processDiagnostics(resp.Diagnostics); err != nil {
		return plugin.ReadResult{}, 0, err
	}

	if len(resp.ImportedResources) > 1 {
		return plugin.ReadResult{}, 0,
			fmt.Errorf("ImportResourceState returned more than one result, " +
				"but reading only one is supported by Pulumi")
	}

	if len(resp.ImportedResources) == 0 {
		return plugin.ReadResult{}, 0,
			fmt.Errorf("ImportResourceState failed to return a result")
	}

	r := resp.ImportedResources[0]

	readState, err := parseResourceStateFromTF(ctx, rh, r.State)
	if err != nil {
		return plugin.ReadResult{}, 0, err
	}

	finalID, err := readState.ExtractID(rh)
	if err != nil {
		return plugin.ReadResult{}, 0, err
	}

	readStateMap, err := readState.ToPropertyMap(rh)
	if err != nil {
		return plugin.ReadResult{}, 0, err
	}

	return plugin.ReadResult{
		ID: finalID,
		// TODO[pulumi/pulumi-terraform-bridge#795] populate Inputs
		Inputs:  nil,
		Outputs: readStateMap,
	}, resource.StatusOK, nil
}
