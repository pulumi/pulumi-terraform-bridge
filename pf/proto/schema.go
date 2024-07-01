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

package proto

import (
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"

	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

// pseudoResource represents a type that must pretent to be a [shim.Resource], but does not represent a resource.
type pseudoResource struct{}

func (pseudoResource) SchemaVersion() int              { return 0 }
func (pseudoResource) Importer() shim.ImportFunc       { return nil }
func (pseudoResource) Timeouts() *shim.ResourceTimeout { return nil }
func (pseudoResource) InstanceState(id string, object, meta map[string]interface{}) (shim.InstanceState, error) {
	panic("Cannot invoke InstanceState on a pseudoResource")
}
func (pseudoResource) DecodeTimeouts(config shim.ResourceConfig) (*shim.ResourceTimeout, error) {
	panic("Cannot invoke DecodeTimeouts on a pseudoResource")
}

func getSchemaMap[T any](m interface {
	GetOk(string) (T, bool)
}, key string) T {
	v, ok := m.GetOk(key)
	contract.Assertf(ok, "Could not find key %q", key)
	return v
}

func deprecated(isDeprecated bool) string {
	if isDeprecated {
		return "Deprecated"
	}
	return ""
}
