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

	"github.com/golang/glog"
	hcty "github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"

	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

func (p v2Provider) Diff(t string, s shim.InstanceState, c shim.ResourceConfig) (shim.InstanceDiff, error) {
	if c == nil {
		return diffToShim(&terraform.InstanceDiff{Destroy: true}), nil
	}

	opts, err := getProviderOptions(p.opts)
	if err != nil {
		return nil, err
	}

	r, ok := p.tf.ResourcesMap[t]
	if !ok {
		return nil, fmt.Errorf("unknown resource %v", t)
	}

	config, state := configFromShim(c), stateFromShim(s)
	rawConfig := makeResourceRawConfig(opts.diffStrategy, config, r)
	if state != nil {
		state.RawConfig = rawConfig
	}

	state, err = upgradeResourceState(p.tf, r, state)
	if err != nil {
		return nil, fmt.Errorf("failed to upgrade resource state: %w", err)
	}
	diff, err := p.simpleDiff(opts.diffStrategy, r, state, config, rawConfig, p.tf.Meta())
	if diff != nil {
		diff.RawConfig = rawConfig
	}
	return diffToShim(diff), err
}

func (p v2Provider) simpleDiff(
	diffStrat DiffStrategy,
	res *schema.Resource,
	s *terraform.InstanceState,
	c *terraform.ResourceConfig,
	rawConfigVal hcty.Value,
	meta interface{},
) (*terraform.InstanceDiff, error) {
	ctx := context.TODO()

	switch diffStrat {
	case ClassicDiff:
		return res.SimpleDiff(ctx, s, c, meta)
	case PlanState:
		return simpleDiffViaPlanState(ctx, res, s, rawConfigVal, meta)
	case TryPlanState:
		classicResult, err := res.SimpleDiff(ctx, s, c, meta)
		if err != nil {
			return nil, err
		}
		planStateResult, err := simpleDiffViaPlanState(ctx, res, s, rawConfigVal, meta)
		if err != nil {
			glog.Errorf("Ignoring PlanState DiffStrategy that failed with an unexpected error. "+
				"You can set the environment variable %s to %q to avoid this message. "+
				"Please report the error details to github.com/pulumi/pulumi-terraform-bridge: %v",
				diffStrategyEnvVar, ClassicDiff.String(), err)
			return classicResult, nil
		}
		if planStateResult.ChangeType() != classicResult.ChangeType() {
			glog.Warningf("Ignoring PlanState DiffStrategy that returns %q disagreeing "+
				" with ClassicDiff result %q. "+
				"You can set the environment variable %s to %q to avoid this message. "+
				"Please report this warning to github.com/pulumi/pulumi-terraform-bridge",
				showDiffChangeType(byte(planStateResult.ChangeType())),
				showDiffChangeType(byte(classicResult.ChangeType())),
				diffStrategyEnvVar, ClassicDiff.String())
			return classicResult, nil
		}
		if planStateResult.RequiresNew() != classicResult.RequiresNew() {
			glog.Warningf("Ignoring PlanState DiffStrategy that decided RequiresNew()=%v disagreeing "+
				" with ClassicDiff result RequiresNew()=%v. "+
				"You can set the environment variable %s to %q to avoid this message. "+
				"Please report this warning to github.com/pulumi/pulumi-terraform-bridge",
				planStateResult.RequiresNew(),
				classicResult.RequiresNew(),
				diffStrategyEnvVar, ClassicDiff.String())
			return classicResult, nil
		}
		return planStateResult, nil
	default:
		return res.SimpleDiff(ctx, s, c, meta)
	}
}

func simpleDiffViaPlanState(
	ctx context.Context,
	res *schema.Resource,
	s *terraform.InstanceState,
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
	return res.SimpleDiff(ctx, s, planned, meta)
}

func showDiffChangeType(b byte) string {
	// based on diffChangeType enumeration from terraform.InstanceDiff ChangeType() result
	switch b {
	case 1:
		return "diffNone"
	case 2:
		return "diffCreate"
	case 3:
		return "diffCreate"
	case 4:
		return "diffUpdate"
	case 5:
		return "diffDestroy"
	case 6:
		return "diffDestroyCreate"
	default:
		return "diffInvalid"
	}
}
