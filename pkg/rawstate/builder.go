// Copyright 2016-2025, Pulumi Corporation.
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

// A typed model for Terraform Raw State and utilities for working with it.
package rawstate

import (
	"encoding/json"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// A quick builder API for creating RawState.
//
// The exposed constructor functions help avoid mistakes such as forgetting number precision, such as using float64
// instead of the higher precision json.Number representation.
type Builder struct {
	v any
}

func (x Builder) Build() RawState {
	j, err := json.Marshal(x.v)
	contract.AssertNoErrorf(err, "Build() failed")
	return j
}

func Null() Builder                { return Builder{v: nil} }
func String(s string) Builder      { return Builder{v: s} }
func Bool(x bool) Builder          { return Builder{v: x} }
func Number(n json.Number) Builder { return Builder{v: n} }
func State(msg RawState) Builder   { return Builder{v: json.RawMessage(msg)} }

func Array(elements ...Builder) Builder {
	a := make([]any, len(elements))
	for i, e := range elements {
		a[i] = e.v
	}
	return Builder{v: a}
}

func Object(elements map[string]Builder) Builder {
	a := make(map[string]any, len(elements))
	for k, e := range elements {
		a[k] = e.v
	}
	return Builder{v: a}
}
