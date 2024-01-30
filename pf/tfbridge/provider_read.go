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
func (p *provider) ReadWithContext(
	ctx context.Context,
	urn resource.URN,
	id resource.ID,
	oldInputs,
	currentStateMap resource.PropertyMap,
) (plugin.ReadResult, resource.Status, error) {
	ctx = p.initLogging(ctx, p.logSink, urn)

	var err error

	// Returning Status is required by the signature but ignored by the server implementation.
	var ignoredStatus resource.Status = resource.StatusOK

	// TODO[pulumi/pulumi-terraform-bridge#793] Add a test for Read handling a not-found resource

	rh, err := p.resourceHandle(ctx, urn)
	if err != nil {
		return plugin.ReadResult{}, 0, err
	}

	// Both "get" and "refresh" scenarios call Read. Detect and dispatch.
	isRefresh := len(currentStateMap) != 0

	var result plugin.ReadResult

	if isRefresh {
		// If we are in a refresh, then currentStateMap was read from the state
		// and should be transformed.
		currentStateMap, err = transformFromState(ctx, rh, currentStateMap)
		if err != nil {
			return plugin.ReadResult{}, 0, err
		}

		result, err = p.readResource(ctx, &rh, currentStateMap)
	} else {
		result, err = p.importResource(ctx, &rh, id)
	}
	if err != nil {
		return result, ignoredStatus, err
	}

	if result.Outputs != nil && rh.pulumiResourceInfo.TransformOutputs != nil {
		var err error
		result.Outputs, err = rh.pulumiResourceInfo.TransformOutputs(ctx, result.Outputs)
		if err != nil {
			return result, ignoredStatus, err
		}
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
		delete(result.Inputs, "__defaults")
	}

	return result, ignoredStatus, err
}

// readResource calls the PF's ReadResource method on the given resource.
func (p *provider) readResource(
	ctx context.Context,
	rh *resourceHandle,
	currentStateMap resource.PropertyMap,
) (plugin.ReadResult, error) {

	currentStateRaw, err := parseResourceState(rh, currentStateMap)
	if err != nil {
		return plugin.ReadResult{}, fmt.Errorf("failed to get current raw state: %w", err)
	}

	currentState, err := p.UpgradeResourceState(ctx, rh, currentStateRaw)
	if err != nil {
		return plugin.ReadResult{}, fmt.Errorf("failed to get current state: %w", err)
	}

	currentStateDV, err := makeDynamicValue(currentState.state.Value)
	if err != nil {
		return plugin.ReadResult{}, fmt.Errorf("failed to get dynamic value: %w", err)
	}

	req := tfprotov6.ReadResourceRequest{
		TypeName:     rh.terraformResourceName,
		CurrentState: &currentStateDV,
		Private:      currentState.PrivateState(),
	}

	// TODO[pulumi/pulumi-terraform-bridge#794] set ProviderMeta

	resp, err := p.tfServer.ReadResource(ctx, &req)
	if err != nil {
		return plugin.ReadResult{}, err
	}

	if err := p.processDiagnostics(resp.Diagnostics); err != nil {
		return plugin.ReadResult{}, err
	}

	if resp.NewState == nil {
		return plugin.ReadResult{}, nil
	}

	// TF interpretes a null new state as an indication that the resource does not
	// exist in the cloud provider.
	newStateNull, err := resp.NewState.IsNull()
	if err != nil {
		return plugin.ReadResult{}, fmt.Errorf("checking null state: %w", err)
	}
	if newStateNull {
		return plugin.ReadResult{}, nil
	}

	readState, err := parseResourceStateFromTF(ctx, rh, resp.NewState, resp.Private)
	if err != nil {
		return plugin.ReadResult{}, fmt.Errorf("parsing resource state: %w", err)
	}

	readStateMap, err := readState.ToPropertyMap(rh)
	if err != nil {
		return plugin.ReadResult{}, fmt.Errorf("converting to property map: %w", err)
	}

	readID, err := extractID(ctx, rh.terraformResourceName, rh.pulumiResourceInfo, readStateMap)
	if err != nil {
		readID = ""
	}

	return plugin.ReadResult{
		ID:      readID,
		Outputs: readStateMap,
	}, nil
}

// Execute a Pulumi import against a PF resource.
//
// PF models an import with 2 steps:
//
// 1. ImportState the resource into TF state.
// 2. Read against the recently imported resource.
//
// According to PF's documentation:
//
//	Resources can implement the ImportState method, which must either specify enough
//	Terraform state for the Read method to refresh resource.Resource or return an
//	error.
//
// source: https://developer.hashicorp.com/terraform/plugin/framework/resources/import
//
// This model is commonly implemented with ImportState simply translating from the import
// string to resource state, without reaching the cloud.
//
// The Read method is generally responsible for checking if a resource exists, returning a
// nil output map is no resource is found.
func (p *provider) importResource(
	ctx context.Context,
	rh *resourceHandle,
	id resource.ID,
) (plugin.ReadResult, error) {
	// TODO[pulumi/pulumi-terraform-bridge#794] set ProviderMeta
	req := tfprotov6.ImportResourceStateRequest{
		TypeName: rh.terraformResourceName,
		ID:       string(id),
	}

	resp, err := p.tfServer.ImportResourceState(ctx, &req)
	if err != nil {
		return plugin.ReadResult{}, err
	}

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

	readState, err := parseResourceStateFromTF(ctx, rh, r.State, r.Private)
	if err != nil {
		return plugin.ReadResult{}, err
	}

	readStateMap, err := readState.ToPropertyMap(rh)
	if err != nil {
		return plugin.ReadResult{}, err
	}

	isNull, err := r.State.IsNull()
	if err != nil {
		return plugin.ReadResult{}, err
	}

	// If the resulting map is null
	if isNull {
		// Returning a result where plugin.ReadResult.Outputs is nil indicates
		// that the found resource does not exist.
		return plugin.ReadResult{}, nil
	}

	// Now that the resource has been translated to TF state, read it.
	return p.readResource(ctx, rh, readStateMap)
}
