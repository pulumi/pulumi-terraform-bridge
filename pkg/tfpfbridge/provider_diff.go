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
	"sort"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

// Diff checks what impacts a hypothetical update will have on the resource's properties. Receives checkedInputs from
// Check and the old state. The implementation here calls PlanResourceChange Terraform method. Essentially:
//
//     Diff(oldState, checkedInputs) = oldState.Diff(PlanResourceChange(oldState, checkedInputs))
func (p *Provider) Diff(
	urn resource.URN,
	id resource.ID,
	oldState resource.PropertyMap,
	checkedInputs resource.PropertyMap,
	allowUnknowns bool,
	ignoreChanges []string,
) (plugin.DiffResult, error) {

	ctx := context.TODO()

	rh, err := p.resourceHandle(ctx, urn)
	if err != nil {
		return plugin.DiffResult{}, err
	}

	tfType := rh.schema.Type().TerraformType(ctx)

	oldStateValue, err := ConvertPropertyMapToTFValue(tfType.(tftypes.Object))(oldState)
	if err != nil {
		return plugin.DiffResult{}, err
	}

	checkedInputsValue, err := ConvertPropertyMapToTFValue(tfType.(tftypes.Object))(checkedInputs)
	if err != nil {
		return plugin.DiffResult{}, err
	}

	proposedNewStateValue, err := applyChanges(oldStateValue, checkedInputsValue)
	if err != nil {
		return plugin.DiffResult{}, err
	}

	priorState, config, proposedNewState, err := makeDynamicValues3(
		oldStateValue, checkedInputsValue, proposedNewStateValue)
	if err != nil {
		return plugin.DiffResult{}, err
	}

	planReq := tfprotov6.PlanResourceChangeRequest{
		TypeName:         rh.terraformResourceName,
		PriorState:       &priorState,
		ProposedNewState: &proposedNewState,
		Config:           &config,

		// TODO PriorPrivate
		// TODO ProviderMeta
	}

	planResp, err := p.tfServer.PlanResourceChange(ctx, &planReq)
	if err != nil {
		return plugin.DiffResult{}, err
	}

	// TODO handle planResp.PlannedPrivate

	// TODO detect errors in planResp.Diagnostics

	// TODO process ignoreChanges

	allTfDiffs, err := diffDynamicValues(tfType, &priorState, planResp.PlannedState)
	if err != nil {
		return plugin.DiffResult{}, err
	}

	// Filter out diffs that resolve an unknown value into a known value. This happens for example with previewing
	// computed attributes.
	var tfDiff []tftypes.ValueDiff
	for _, diff := range allTfDiffs {
		// Skip diffs that replace a fully unknown with a known.
		//
		// TODO consider diffs are resolving unknows deeper in the value tree, are those possible to observe or
		// will they be reported as separate ValueDiff entries under a deeper path? If possible to observe,
		// should they be excluded from Pulumi diff also? A condition would look like this:
		//
		//    v, ok := unify(diff.Value1, diff.Value2); ok && v.Equal(diff.Value2)
		//
		// where unify(x, y) is like recursive Equal but resolving unknowns in x from y.
		notChanging := !diff.Value1.IsKnown() && diff.Value2.IsKnown()
		if !notChanging {
			tfDiff = append(tfDiff, diff)
		}
	}

	replaceKeys, err := diffPathsToPropertyKeySet(planResp.RequiresReplace)
	if err != nil {
		return plugin.DiffResult{}, err
	}

	changedKeys, err := diffChangedKeys(tfDiff)
	if err != nil {
		return plugin.DiffResult{}, err
	}

	// Compute deleteBeforeReplace. TODO there are some intricacies in the old bridge regarding
	// nameRequiresDeleteBeforeReplace that are not handled yet.
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

	// TODO how to compute StableKeys

	// TODO currently not yet computing DetailedDiff, which is intricate in the old bridge due to Set encoding as
	// lists in Pulumi.
	return diffResult, nil
}

// Every entry in tfDiff has an AttributePath; extract the set of paths and find their roots.
func diffChangedKeys(tfDiff []tftypes.ValueDiff) ([]resource.PropertyKey, error) {
	paths := []*tftypes.AttributePath{}
	for _, diff := range tfDiff {
		paths = append(paths, diff.Path)
	}
	return diffPathsToPropertyKeySet(paths)
}

// Convert AttributeName to PropertyKey. Currently assume property names are identical in Pulumi and TF worlds.
func diffAttributeNameToPropertyKey(name tftypes.AttributeName) resource.PropertyKey {
	return resource.PropertyKey(tokens.Name(string(name)))
}

// For AttributePath that drills down from a property key, return that top-level propery key.
func diffPathToPropertyKey(path *tftypes.AttributePath) (resource.PropertyKey, error) {
	steps := path.Steps()
	if len(steps) == 0 {
		return "", fmt.Errorf("Unexpected empty AttributePath")
	}

	firstStep := steps[0]
	name, ok := firstStep.(tftypes.AttributeName)
	if !ok {
		return "", fmt.Errorf("AttributePath did not start with AttributeName: %v", path.String())
	}

	return diffAttributeNameToPropertyKey(name), nil
}

// Computes diffPathToPropertyKey for every path and gathers root property keys into a set.
func diffPathsToPropertyKeySet(paths []*tftypes.AttributePath) ([]resource.PropertyKey, error) {
	keySet := map[resource.PropertyKey]struct{}{}
	for _, path := range paths {
		key, err := diffPathToPropertyKey(path)
		if err != nil {
			return nil, err
		}
		keySet[key] = struct{}{}
	}
	keySlice := []resource.PropertyKey{}
	for k := range keySet {
		keySlice = append(keySlice, k)
	}
	sort.SliceStable(keySlice, func(i, j int) bool {
		return keySlice[i] < keySlice[j]
	})
	return keySlice, nil
}

func diffDynamicValues(typ tftypes.Type, before, after *tfprotov6.DynamicValue) ([]tftypes.ValueDiff, error) {
	b, err := before.Unmarshal(typ)
	if err != nil {
		return nil, err
	}
	a, err := after.Unmarshal(typ)
	if err != nil {
		return nil, err
	}
	return a.Diff(b)
}

func makeDynamicValues3(a, b, c tftypes.Value) (tfprotov6.DynamicValue, tfprotov6.DynamicValue, tfprotov6.DynamicValue, error) {
	var n tfprotov6.DynamicValue
	av, err := tfprotov6.NewDynamicValue(a.Type(), a)
	if err != nil {
		return n, n, n, err
	}
	bv, err := tfprotov6.NewDynamicValue(b.Type(), b)
	if err != nil {
		return n, n, n, err
	}
	cv, err := tfprotov6.NewDynamicValue(c.Type(), c)
	if err != nil {
		return n, n, n, err
	}
	return av, bv, cv, nil
}
