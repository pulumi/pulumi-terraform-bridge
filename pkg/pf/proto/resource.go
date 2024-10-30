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
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"

	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

var (
	_ = shim.ResourceMap(resourceMap{})
	_ = shim.Resource(resource{})
)

type resourceMap map[string]*tfprotov6.Schema

func (m resourceMap) Len() int {
	return len(m)
}

func (m resourceMap) Get(key string) shim.Resource {
	v, ok := m.GetOk(key)
	contract.Assertf(ok, "unknown key %q", key)
	return v
}

func (m resourceMap) GetOk(key string) (shim.Resource, bool) {
	v, ok := m[key]
	if !ok {
		return nil, false
	}
	return resource{v}, true
}

func (m resourceMap) Range(each func(key string, value shim.Resource) bool) {
	for k, v := range m {
		if !each(k, resource{v}) {
			return
		}
	}
}

func (m resourceMap) Set(key string, value shim.Resource) {
	v, ok := value.(resource)
	contract.Assertf(ok, "Set must be a %T, found a %T", v, value)
	m[key] = v.r
}

type resource struct{ r *tfprotov6.Schema }

func (r resource) Schema() shim.SchemaMap {
	return blockMap{r.r.Block}
}

func (r resource) SchemaVersion() int { return int(r.r.Version) }

func (r resource) Importer() shim.ImportFunc {
	panic("Cannot call Importer for a schema only resource")
}

func (r resource) DeprecationMessage() string { return deprecated(r.r.Block.Deprecated) }

func (r resource) Timeouts() *shim.ResourceTimeout {
	panic("Cannot call Timeouts for a schema only resource")
}

func (r resource) InstanceState(id string, object, meta map[string]interface{}) (shim.InstanceState, error) {
	panic("Cannot call InstanceState for a schema only resource")
}
func (r resource) DecodeTimeouts(config shim.ResourceConfig) (*shim.ResourceTimeout, error) {
	panic("Cannot call DecodeTimeouts for a schema only resource")
}
