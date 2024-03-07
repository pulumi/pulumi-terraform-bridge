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
	"sort"

	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"

	"github.com/pulumi/pulumi-terraform-bridge/pf/internal/convert"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/unstable/propertyvalue"
)

// Diff checks what impacts a hypothetical update will have on the resource's properties. Receives checkedInputs from
// Check and the prior state. The implementation here calls PlanResourceChange Terraform method. Essentially:
//
//	Diff(priorState, checkedInputs):
//	    proposedNewState = priorState.applyChanges(checkedInputs)
//	    plannedState = PlanResourceChange(priorState, proposedNewState)
//	    priorState.Diff(plannedState)
func (p *provider) DiffWithContext(
	ctx context.Context,
	urn resource.URN,
	id resource.ID,
	priorStateMap resource.PropertyMap,
	checkedInputs resource.PropertyMap,
	allowUnknowns bool,
	ignoreChanges []string,
) (plugin.DiffResult, error) {
	ctx = p.initLogging(ctx, p.logSink, urn)
	rh, err := p.resourceHandle(ctx, urn)
	if err != nil {
		return plugin.DiffResult{}, err
	}

	priorStateMap, err = transformFromState(ctx, rh, priorStateMap)
	if err != nil {
		return plugin.DiffResult{}, err
	}

	checkedInputs, err = propertyvalue.ApplyIgnoreChanges(priorStateMap, checkedInputs, ignoreChanges)
	if err != nil {
		return plugin.DiffResult{}, fmt.Errorf("failed to apply ignore changes: %w", err)
	}

	rawPriorState, err := parseResourceState(&rh, priorStateMap)
	if err != nil {
		return plugin.DiffResult{}, err
	}

	priorState, err := p.UpgradeResourceState(ctx, &rh, rawPriorState)
	if err != nil {
		return plugin.DiffResult{}, err
	}

	tfType := rh.schema.Type().TerraformType(ctx).(tftypes.Object)

	checkedInputsValue, err := convert.EncodePropertyMap(rh.encoder, checkedInputs)
	if err != nil {
		return plugin.DiffResult{}, err
	}

	planResp, err := p.plan(ctx, rh.terraformResourceName, rh.schema, priorState, checkedInputsValue)
	if err != nil {
		return plugin.DiffResult{}, err
	}

	// NOTE: this currently ignores planRep.PlanedPrivate but it is unclear if it should signal differences between
	// planResp.PlannedPrivate and priorState.PrivateState() when the only thing that is changing is the private
	// state. Currently assume that planResp will signal RequiresReplace if needed anyway and there is no useful way
	// to surface private state differences to the user from the Diff method.

	if err := p.processDiagnostics(planResp.Diagnostics); err != nil {
		return plugin.DiffResult{}, err
	}

	// TODO[pulumi/pulumi-terraform-bridge#751] ignoreChanges support
	plannedStateValue, err := planResp.PlannedState.Unmarshal(tfType)
	if err != nil {
		return plugin.DiffResult{}, err
	}

	tfDiff, err := priorState.state.Value.Diff(plannedStateValue)
	if err != nil {
		return plugin.DiffResult{}, err
	}

	resSchemaMap := rh.schemaOnlyShimResource.Schema()
	resFields := rh.pulumiResourceInfo.GetFields()
	replaceKeys := topLevelPropertyKeySet(resSchemaMap, resFields, planResp.RequiresReplace)
	changedKeys := topLevelPropertyKeySet(resSchemaMap, resFields, diffAttributePaths(tfDiff))

	// TODO[pulumi/pulumi-terraform-bridge#823] nameRequiresDeleteBeforeReplace intricacies
	deleteBeforeReplace := false
	if len(replaceKeys) > 0 {
		info := rh.pulumiResourceInfo
		if info != nil && info.DeleteBeforeReplace {
			deleteBeforeReplace = true
		}
	}

	changes := plugin.DiffNone
	if len(changedKeys) > 0 {
		changes = plugin.DiffSome
	}

	diffResult := plugin.DiffResult{
		Changes:             changes,
		ReplaceKeys:         replaceKeys,
		ChangedKeys:         changedKeys,
		DeleteBeforeReplace: deleteBeforeReplace,
	}

	// TODO[pulumi/pulumi-terraform-bridge#824] StableKeys
	return diffResult, nil
}

// For each path x.y.z extracts the next step x and converts it to a matching Pulumi key. Removes
// duplicates and orders the result.
func topLevelPropertyKeySet(
	sch shim.SchemaMap, ps map[string]*tfbridge.SchemaInfo, paths []*tftypes.AttributePath,
) []resource.PropertyKey {
	found := map[resource.PropertyKey]struct{}{}
	for _, path := range paths {
		switch nextStep := path.NextStep().(type) {
		// All the paths are likely starting with an AttributeName, could assert that but
		// for now just filtering out those that do not.
		case tftypes.AttributeName:
			pn := tfbridge.TerraformToPulumiNameV2(string(nextStep), sch, ps)
			pk := resource.PropertyKey(pn)
			found[pk] = struct{}{}
		}
	}
	var keys []resource.PropertyKey
	for k := range found {
		keys = append(keys, k)
	}
	sort.SliceStable(keys, func(i, j int) bool {
		return keys[i] < keys[j]
	})
	return keys
}

func diffAttributePaths(tfDiff []tftypes.ValueDiff) []*tftypes.AttributePath {
	paths := []*tftypes.AttributePath{}
	for _, diff := range tfDiff {
		paths = append(paths, diff.Path)
	}
	return paths
}
