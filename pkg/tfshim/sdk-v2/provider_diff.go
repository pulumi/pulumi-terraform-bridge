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

package sdkv2

import (
	"context"
	"fmt"

	hcty "github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"

	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

func (p v2Provider) Diff(
	ctx context.Context,
	t string,
	s shim.InstanceState,
	c shim.ResourceConfig,
	opts shim.DiffOptions,
) (theDiff shim.InstanceDiff, theError error) {
	defer func() {
		switch theDiff := theDiff.(type) {
		case v2InstanceDiff:
			theDiff.applyTimeoutOptions(opts.TimeoutOptions)
		}
	}()
	if c == nil {
		return diffToShim(&terraform.InstanceDiff{Destroy: true}), nil
	}

	r, ok := p.tf.ResourcesMap[t]
	if !ok {
		return nil, fmt.Errorf("unknown resource %v", t)
	}

	config, state := configFromShim(c), stateFromShim(s)
	rawConfig := makeResourceRawConfig(config, r)

	if state == nil {
		// When handling Create Pulumi passes nil for state, but this diverges from how Terraform does things,
		// see: https://github.com/pulumi/pulumi-terraform-bridge/issues/911 and can lead to panics. Compensate
		// by constructing an InstanceState.

		state = &terraform.InstanceState{
			RawConfig: rawConfig,
		}
	} else {
		// Upgrades are needed only if we have non-empty prior state.
		var err error
		state, err = upgradeResourceState(ctx, t, p.tf, r, state)
		if err != nil {
			return nil, fmt.Errorf("failed to upgrade resource state: %w", err)
		}
	}

	diff, err := p.simpleDiff(ctx, r, state, config, rawConfig, p.tf.Meta())
	if err != nil {
		return nil, err
	}
	if diff != nil {
		diff.RawConfig = rawConfig
	}

	resultingDiff := diffToShim(diff)

	if dd, ok := resultingDiff.(v2InstanceDiff); ok && opts.IgnoreChanges != nil {
		dd.processIgnoreChanges(opts.IgnoreChanges)
	}

	return resultingDiff, err
}

func (p v2Provider) simpleDiff(
	ctx context.Context,
	res *schema.Resource,
	s *terraform.InstanceState,
	c *terraform.ResourceConfig,
	rawConfigVal hcty.Value,
	meta interface{},
) (*terraform.InstanceDiff, error) {
	priorStateVal, err := s.AttrsAsObjectValue(res.CoreConfigSchema().ImpliedType())
	if err != nil {
		return nil, err
	}
	proposedNewStateVal, err := proposedNew(res, priorStateVal, rawConfigVal)
	if err != nil {
		return nil, err
	}

	planned := terraform.NewResourceConfigShimmed(proposedNewStateVal, res.CoreConfigSchema())
	state := s.DeepCopy()
	state.RawPlan = proposedNewStateVal
	if state.RawState.IsNull() {
		state.RawState = priorStateVal
	}
	if state.RawConfig.IsNull() {
		state.RawConfig = rawConfigVal
	}
	diff, err := res.SimpleDiff(ctx, state, planned, meta)
	if err != nil {
		return nil, err
	}

	// TF gRPC servers compensate for the fact that SimpleDiff may return
	// terraform.NewInstanceDiff(), nil dropping any available information on RawPlan.
	//
	// See for example this code in PlanResourceChange:
	//
	// See https://github.com/hashicorp/terraform-plugin-sdk/blob/
	//         28e631776d97f0a5a5942b3524814addbef90875/helper/schema/grpc_provider.go#L797
	//
	// In TF this is communicated from PlanResourceChange to ApplyResourceChange; unlike TF, in
	// the current codebase InstanceDiff is passed directly to Apply. If RawPlan is not set on
	// the diff it may cause nil panics in the provider.
	if diff != nil && len(diff.Attributes) == 0 {
		diff.RawPlan = priorStateVal
		// TODO[pulumi/pulumi-terraform-bridge#1505] handle private state similar to upstream
	}

	return diff, nil
}
