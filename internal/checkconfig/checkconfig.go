// Copyright 2016-2026, Pulumi Corporation.
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

// Package checkconfig converts between the CheckConfig and CheckConfig2 RPCs.
package checkconfig

import (
	"context"

	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
)

// CheckConfig2ViaCheckConfig answers a CheckConfig2 call by delegating to a CheckConfig implementation, for servers
// that only implement the older method.
func CheckConfig2ViaCheckConfig(
	ctx context.Context,
	req *pulumirpc.CheckConfigRequest,
	checkConfig func(context.Context, *pulumirpc.CheckRequest) (*pulumirpc.CheckResponse, error),
) (*pulumirpc.CheckConfigResponse, error) {
	resp, err := checkConfig(ctx, &pulumirpc.CheckRequest{
		Urn:        req.GetUrn(),
		Olds:       req.GetOldInputs(),
		News:       req.GetNewInputs(),
		RandomSeed: req.GetRandomSeed(),
		Name:       req.GetName(),
		Type:       req.GetType(),
		OldOutputs: req.GetOldOutputs(),
	})
	if resp == nil {
		return nil, err
	}
	return &pulumirpc.CheckConfigResponse{Inputs: resp.GetInputs(), Failures: Failures(resp.GetFailures())}, err
}

// CheckConfigViaCheckConfig2 answers a CheckConfig call by delegating to a CheckConfig2 implementation, so that both
// RPCs give the same answer. Anything that only CheckConfig2 can carry is dropped.
//
//nolint:revive // The name mirrors CheckConfig2ViaCheckConfig; the stutter with the package name is intended.
func CheckConfigViaCheckConfig2(
	ctx context.Context,
	req *pulumirpc.CheckRequest,
	checkConfig2 func(context.Context, *pulumirpc.CheckConfigRequest) (*pulumirpc.CheckConfigResponse, error),
) (*pulumirpc.CheckResponse, error) {
	resp, err := checkConfig2(ctx, &pulumirpc.CheckConfigRequest{
		Urn:        req.GetUrn(),
		Name:       req.GetName(),
		Type:       req.GetType(),
		OldInputs:  req.GetOlds(),
		NewInputs:  req.GetNews(),
		RandomSeed: req.GetRandomSeed(),
		OldOutputs: req.GetOldOutputs(),
	})
	if resp == nil {
		return nil, err
	}

	var failures []*pulumirpc.CheckFailure
	if fs := resp.GetFailures(); fs != nil {
		failures = make([]*pulumirpc.CheckFailure, len(fs))
		for i, f := range fs {
			failures[i] = &pulumirpc.CheckFailure{Property: f.GetProperty(), Reason: f.GetReason()}
		}
	}
	return &pulumirpc.CheckResponse{Inputs: resp.GetInputs(), Failures: failures}, err
}

// Failures converts check failures to their CheckConfig2 representation.
func Failures(failures []*pulumirpc.CheckFailure) []*pulumirpc.CheckConfigFailure {
	if failures == nil {
		return nil
	}
	result := make([]*pulumirpc.CheckConfigFailure, len(failures))
	for i, f := range failures {
		result[i] = &pulumirpc.CheckConfigFailure{Property: f.GetProperty(), Reason: f.GetReason()}
	}
	return result
}

// Remediations converts command remediations to their CheckConfig2 representation.
func Remediations(remediations []info.CommandRemediation) []*pulumirpc.CommandRemediation {
	rpcRemediations := make([]*pulumirpc.CommandRemediation, 0, len(remediations))
	for _, r := range remediations {
		rpcRemediations = append(rpcRemediations, &pulumirpc.CommandRemediation{
			Description: r.Description,
			Command:     r.Command,
			Env:         r.Env,
		})
	}
	return rpcRemediations
}
