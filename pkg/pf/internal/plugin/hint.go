// Copyright 2026, Pulumi Corporation.
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

package plugin

import (
	pl "github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

// HintToRPC converts the product hint of a check failure to its wire form.
func HintToRPC(h pl.Hint) pulumirpc.Hint {
	switch h {
	case pl.HintDynamicCredentials:
		return pulumirpc.Hint_HINT_DYNAMIC_CREDENTIALS
	case pl.HintUnspecified:
		return pulumirpc.Hint_HINT_UNSPECIFIED
	default:
		return pulumirpc.Hint_HINT_UNSPECIFIED
	}
}

// HintFromRPC converts the wire form of a check failure product hint. Hints this bridge does not know are
// unspecified.
func HintFromRPC(h pulumirpc.Hint) pl.Hint {
	switch h {
	case pulumirpc.Hint_HINT_DYNAMIC_CREDENTIALS:
		return pl.HintDynamicCredentials
	case pulumirpc.Hint_HINT_UNSPECIFIED:
		return pl.HintUnspecified
	default:
		return pl.HintUnspecified
	}
}
