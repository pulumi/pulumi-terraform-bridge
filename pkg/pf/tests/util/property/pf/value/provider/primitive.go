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

package provider

import (
	"github.com/zclconf/go-cty/cty"
	"pgregory.net/rapid"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

func boolVal(t *rapid.T) value {
	f := rapid.Bool().Draw(t, "v")
	return value{
		hasValue: true,
		tf:       cty.BoolVal(f),
		pu:       resource.NewProperty(f),
	}
}

func stringV() *rapid.Generator[string] { return rapid.StringMatching("[a-zA-Z0-9-]*") }

func stringVal(t *rapid.T) value {
	s := stringV().Draw(t, "v")
	return value{
		hasValue: true,
		tf:       cty.StringVal(s),
		pu:       resource.NewProperty(s),
	}

}

func float64Val(t *rapid.T) value {
	f := rapid.Float64().Draw(t, "v")
	return value{
		hasValue: true,
		tf:       cty.NumberFloatVal(f),
		pu:       resource.NewProperty(f),
	}
}

func float32Val(t *rapid.T) value {
	f := rapid.Float32().Draw(t, "v")
	return value{
		hasValue: true,
		tf:       cty.NumberFloatVal(float64(f)),
		pu:       resource.NewProperty(float64(f)),
	}
}

func int32Val(t *rapid.T) value {
	f := rapid.Int32().Draw(t, "v")
	return value{
		hasValue: true,
		tf:       cty.NumberIntVal(int64(f)),
		pu:       resource.NewProperty(float64(f)),
	}
}

func int64Val(t *rapid.T) value {
	// TODO: We know that we cannot round trip int64 values.
	return int32Val(t)
}
