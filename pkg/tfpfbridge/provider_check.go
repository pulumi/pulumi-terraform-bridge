// Copyright 2016-2022, Pulumi Corporation.
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

package tfbridge

import (
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
)

// Check validates that the given property bag is valid for a resource of the given type and returns the inputs that
// should be passed to successive calls to Diff, Create, or Update for this resource.
func (p *Provider) Check(urn resource.URN, olds, news resource.PropertyMap,
	allowUnknowns bool, randomSeed []byte) (resource.PropertyMap, []plugin.CheckFailure, error) {

	// TODO Properly implement CHECK to allow provider to fill out
	// default values.

	result := olds.Copy()
	for k, v := range news {
		result[k] = v
	}
	return result, []plugin.CheckFailure{}, nil
}
