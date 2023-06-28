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
	"reflect"
	"sync"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

var (
	instanceStateStrategyByResource sync.Map
	defaultInstanceStateStrategy    InstanceStateStrategy
)

func init() {
	s, ok, err := ParseInstanceStateStrategyFromEnv()
	contract.AssertNoErrorf(err, "incorrect %s", instanteStateStrategyEnvVar)
	if ok {
		defaultInstanceStateStrategy = s
	} else {
		defaultInstanceStateStrategy = ClassicInstanceState
	}
}

func SetInstanceStateStrategy(resource shim.Resource, strategy InstanceStateStrategy) {
	v2Res, ok := resource.(v2Resource)
	contract.Assertf(ok, "expecting resource to be shimmed with sdkv2 shim, got %v", reflect.TypeOf(resource))
	instanceStateStrategyByResource.Store(v2Res, strategy)
}

func GetInstanceStateStrategy(resource shim.Resource) InstanceStateStrategy {
	if resource == nil {
		return defaultInstanceStateStrategy
	}
	v2Res, ok := resource.(v2Resource)
	contract.Assertf(ok, "expecting resource to be shimmed with sdkv2 shim, got %v", reflect.TypeOf(resource))
	v, ok := instanceStateStrategyByResource.Load(v2Res)
	if !ok {
		return defaultInstanceStateStrategy
	}
	r, ok := v.(InstanceStateStrategy)
	if !ok {
		return defaultInstanceStateStrategy
	}
	return r
}

// Feature flag to configures how the provider reads instance state.
type InstanceStateStrategy int

const (
	// The legacy strategy.
	ClassicInstanceState InstanceStateStrategy = 0

	// The strategy based on transcoding to cty.Value.
	CtyInstanceState InstanceStateStrategy = 1
)

func (s InstanceStateStrategy) String() string {
	switch s {
	case ClassicInstanceState:
		return "ClassicInstanceState"
	case CtyInstanceState:
		return "CtyInstanceState"
	default:
		return "<unknown>"
	}
}

func ParseInstanceStateStrategy(text string) (InstanceStateStrategy, error) {
	switch text {
	case "ClassicInstanceState":
		return ClassicInstanceState, nil
	case "CtyInstanceState":
		return CtyInstanceState, nil
	default:
		return 0, fmt.Errorf("Unknown InstanceStateRead: %q", text)
	}
}

const instanteStateStrategyEnvVar = "PULUMI_INSTANCE_STATE_STRATEGY"

func ParseInstanceStateStrategyFromEnv() (InstanceStateStrategy, bool, error) {
	s := os.Getenv(instanteStateStrategyEnvVar)
	if s == "" {
		return ClassicInstanceState, false, nil
	}
	p, err := ParseInstanceStateStrategy(s)
	return p, err == nil, err
}
