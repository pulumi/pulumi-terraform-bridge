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
	"fmt"
	"os"
)

// Configures how the provider performs Diff. Since this is a sensitive method that can result in unexpected breaking
// changes, using a configurable DiffStrategy as a feature flag assists gradual rollout.
type DiffStrategy int

const (
	// Uses the strategy from pulumi-terraform-bridge v3.41.0 and before.
	ClassicDiff DiffStrategy = 0

	// An experimental strategy that mimics the behavior of Terraform CLI to compute PlannedState as part of the
	// diff, and performs better than ClassicDiff in complicated cases of set-nested blocks.
	PlanState DiffStrategy = 1

	// This strategy would run both PlanState and ClassicDiff strategies and compre their result, generating a
	// warning if they mismatch.
	TryPlanState DiffStrategy = 2
)

func (s DiffStrategy) String() string {
	switch s {
	case ClassicDiff:
		return "ClassicDiff"
	case PlanState:
		return "PlanState"
	case TryPlanState:
		return "TryPlanState"
	default:
		return "<unknown>"
	}
}

func ParseDiffStrategy(text string) (DiffStrategy, error) {
	switch text {
	case "ClassicDiff":
		return ClassicDiff, nil
	case "PlanState":
		return PlanState, nil
	case "TryPlanState":
		return TryPlanState, nil
	default:
		return 0, fmt.Errorf("Unknown DiffStrategy: %q", text)
	}
}

func ParseDiffStrategyFromEnv() (DiffStrategy, bool, error) {
	s := os.Getenv("PULUMI_DIFF_STRATEGY")
	if s == "" {
		return 0, false, nil
	}
	p, err := ParseDiffStrategy(s)
	return p, err == nil, err
}
