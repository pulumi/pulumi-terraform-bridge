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

// Package checkconfig adapts CheckConfig implementations to the CheckConfig2 RPC.
package checkconfig

import (
	"context"

	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

// CheckConfig2 answers a CheckConfig2 call by delegating to a CheckConfig implementation, so that both RPCs give the
// same answer.
func CheckConfig2(
	ctx context.Context,
	req *pulumirpc.CheckConfigRequest,
	checkConfig func(context.Context, *pulumirpc.CheckRequest) (*pulumirpc.CheckResponse, error),
) (*pulumirpc.CheckConfigResponse, error) {
	resp, err := checkConfig(ctx, &pulumirpc.CheckRequest{
		Urn:        req.GetUrn(),
		Olds:       req.GetOlds(),
		News:       req.GetNews(),
		RandomSeed: req.GetRandomSeed(),
		Name:       req.GetName(),
		Type:       req.GetType(),
		OldOutputs: req.GetOldOutputs(),
	})
	if resp == nil {
		return nil, err
	}

	var failures []*pulumirpc.CheckConfigFailure
	if fs := resp.GetFailures(); fs != nil {
		failures = make([]*pulumirpc.CheckConfigFailure, len(fs))
		for i, f := range fs {
			failures[i] = &pulumirpc.CheckConfigFailure{Property: f.GetProperty(), Reason: f.GetReason()}
		}
	}
	return &pulumirpc.CheckConfigResponse{Inputs: resp.GetInputs(), Failures: failures}, err
}
