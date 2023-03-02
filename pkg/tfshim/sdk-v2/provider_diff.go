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

	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

func (p v2Provider) Diff(t string, s shim.InstanceState, c shim.ResourceConfig) (shim.InstanceDiff, error) {
	opts, err := getProviderOptions(p.opts)
	if err != nil {
		return nil, err
	}
	switch opts.diffStrategy {
	case ClassicDiff:
		return p.classicDiff(t, s, c)
	case PlanState:
		return p.planStateDiff(t, s, c)
	case TryPlanState:
		return nil, fmt.Errorf("TODO")
	default:
		return p.classicDiff(t, s, c)
	}
}

func (p v2Provider) classicDiff(t string, s shim.InstanceState, c shim.ResourceConfig) (shim.InstanceDiff, error) {
	if c == nil {
		return diffToShim(&terraform.InstanceDiff{Destroy: true}), nil
	}
	r, ok := p.tf.ResourcesMap[t]
	if !ok {
		return nil, fmt.Errorf("unknown resource %v", t)
	}

	config, state := configFromShim(c), stateFromShim(s)
	rawConfig := makeResourceRawConfig(config, r)
	if state != nil {
		state.RawConfig = rawConfig
	}

	state, err := upgradeResourceState(p.tf, r, state)
	if err != nil {
		return nil, fmt.Errorf("failed to upgrade resource state: %w", err)
	}
	diff, err := r.SimpleDiff(context.TODO(), state, config, p.tf.Meta())
	if diff != nil {
		diff.RawConfig = rawConfig
	}
	return diffToShim(diff), err
}

func (p v2Provider) planStateDiff(t string, s shim.InstanceState, c shim.ResourceConfig) (shim.InstanceDiff, error) {
	if c == nil {
		return diffToShim(&terraform.InstanceDiff{Destroy: true}), nil
	}
	r, ok := p.tf.ResourcesMap[t]
	if !ok {
		return nil, fmt.Errorf("unknown resource %v", t)
	}

	config, state := configFromShim(c), stateFromShim(s)
	rawConfig := makeResourceRawConfig(config, r)
	if state != nil {
		state.RawConfig = rawConfig
	}

	state, err := upgradeResourceState(p.tf, r, state)
	if err != nil {
		return nil, fmt.Errorf("failed to upgrade resource state: %w", err)
	}

	diff, err := simpleDiff(context.TODO(), r, state, config, rawConfig, p.tf.Meta())
	if diff != nil {
		diff.RawConfig = rawConfig
	}

	return diffToShim(diff), err
}
