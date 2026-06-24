// Copyright 2016-2024, Pulumi Corporation.
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

package sdkv2

import (
	"context"
	"unsafe"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"

	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

// planResourceChange reproduces the planning half of the SDK's
// GRPCProviderServer.PlanResourceChange, but additionally returns the
// *terraform.InstanceDiff that drives the bridge's diff and lets the bridge
// transform that diff (to apply ignoreChanges) before it is turned into the
// planned state.
//
// Upstream's PlanResourceChange computes the InstanceDiff in a local variable
// and never exposes it; the github.com/pulumi/terraform-plugin-sdk fork added a
// PlanResourceChangeExtra method to surface it. Targeting upstream directly, we
// recompute the diff here via the public Resource.SimpleDiff and reuse the SDK's
// (otherwise unexported) state-shaping helpers through //go:linkname; see
// linkname.go.
//
// Branches that the bridge does not exercise — provider deferral, resource
// identity, and RequiresReplace (the bridge encodes replacement in the
// InstanceDiff itself) — are intentionally omitted.
func (s *grpcServer) planResourceChange(
	ctx context.Context,
	res *schema.Resource,
	config, priorStateVal, proposedNewStateVal cty.Value,
	priorMeta map[string]interface{},
	providerMeta *cty.Value,
	ignores shim.IgnoreChanges,
) (cty.Value, map[string]interface{}, *terraform.InstanceDiff, error) {
	schemaBlock := res.CoreConfigSchema()
	create := priorStateVal.IsNull()

	// We do not plan destroys here; mirror the SDK by returning the prior state.
	if proposedNewStateVal.IsNull() {
		return priorStateVal, priorMeta, nil, nil
	}

	// Ensure there are no nulls that will cause helper/schema to panic.
	if err := handleDiagnostics(ctx, validateConfigNulls(ctx, proposedNewStateVal, nil), nil); err != nil {
		return cty.NilVal, nil, nil, err
	}

	priorState, err := res.ShimInstanceStateFromValue(priorStateVal)
	if err != nil {
		return cty.NilVal, nil, nil, err
	}
	priorState.RawState = priorStateVal
	priorState.RawPlan = proposedNewStateVal
	priorState.RawConfig = config
	priorState.Meta = priorMeta
	if priorState.Meta == nil {
		priorState.Meta = map[string]interface{}{}
	}
	if providerMeta != nil {
		priorState.ProviderMeta = *providerMeta
	}

	// Turn the proposed state into a legacy configuration.
	cfg := terraform.NewResourceConfigShimmed(proposedNewStateVal, schemaBlock)

	diff, err := res.SimpleDiff(ctx, priorState, cfg, s.provider.Meta())
	if err != nil {
		return cty.NilVal, nil, nil, err
	}

	// If this is a new instance, we need to make sure ID is going to be computed.
	if create {
		if diff == nil {
			diff = terraform.NewInstanceDiff()
		}
		diff.Attributes["id"] = &terraform.ResourceAttrDiff{NewComputed: true}
	}

	// Apply Pulumi ignoreChanges by stripping the ignored attributes from the
	// diff before it is applied to produce the planned state. This is the work
	// the fork's PlanResourceChangeExtra.TransformInstanceDiff hook performed.
	if ignores != nil && diff != nil {
		dd := v2InstanceDiff{tf: diff}
		dd.processIgnoreChanges(ignores)
		diff = dd.tf
	}

	if diff == nil || (len(diff.Attributes) == 0 && len(diff.Identity) == 0) {
		// SimpleDiff produced no changes: the planned state is the prior state.
		return priorStateVal, priorMeta, diff, nil
	}

	// Apply the diff to the prior state to obtain the planned state.
	plannedAttrs, err := diff.Apply(priorState.Attributes, schemaBlock)
	if err != nil {
		return cty.NilVal, nil, nil, err
	}

	plannedStateVal, err := hcl2ValueFromFlatmap(plannedAttrs, schemaBlock.ImpliedType())
	if err != nil {
		return cty.NilVal, nil, nil, err
	}

	plannedStateVal, err = schemaBlock.CoerceValue(plannedStateVal)
	if err != nil {
		return cty.NilVal, nil, nil, err
	}

	plannedStateVal = normalizeNullValues(plannedStateVal, proposedNewStateVal, false)
	plannedStateVal = copyTimeoutValues(plannedStateVal, proposedNewStateVal)

	// The old SDK code has some imprecisions that cause it to sometimes generate
	// differences that the SDK itself does not consider significant. When prior
	// and planned are SDK-equivalent, treat the plan as a no-op.
	if valuesSDKEquivalent(priorStateVal, plannedStateVal) {
		plannedStateVal = priorStateVal
	}

	// If this was creating the resource, set any remaining computed fields.
	if create {
		plannedStateVal = schema.SetUnknowns(plannedStateVal, schemaBlock)
	}

	// Set any write-only attribute values to null.
	plannedStateVal = setWriteOnlyNullValues(plannedStateVal, unsafe.Pointer(schemaBlock))

	// Encode timeouts into the diff Meta, which becomes the planned private state.
	timeout := &schema.ResourceTimeout{}
	if err := timeout.ConfigDecode(res, cfg); err != nil {
		return cty.NilVal, nil, nil, err
	}
	if err := timeout.DiffEncode(diff); err != nil {
		return cty.NilVal, nil, nil, err
	}

	// Store any NewExtra values, which are where StateFunc-modified config fields
	// are hidden.
	privateMap := diff.Meta
	if privateMap == nil {
		privateMap = map[string]interface{}{}
	}
	newExtra := map[string]interface{}{}
	for k, v := range diff.Attributes {
		if v.NewExtra != nil {
			newExtra[k] = v.NewExtra
		}
	}
	privateMap[newExtraKey] = newExtra

	return plannedStateVal, privateMap, diff, nil
}
