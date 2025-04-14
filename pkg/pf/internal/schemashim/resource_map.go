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

package schemashim

import (
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/internal/runtypes"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

// Resource map needs to support Set (mutability) for RenameResourceWithAlias.
func newSchemaOnlyResourceMap(resources runtypes.Resources) schemaOnlyResourceMap {
	m := schemaOnlyResourceMap{}
	for _, name := range resources.All() {
		key := string(name)
		v := resources.Schema(name)
		m[key] = &schemaOnlyResource{v}
	}
	return m
}

type schemaOnlyResourceMap map[string]*schemaOnlyResource

var (
	_ shim.ResourceMap   = schemaOnlyResourceMap{}
	_ runtypes.Resources = schemaOnlyResourceMap{}
)

func (m schemaOnlyResourceMap) Len() int {
	return len(m)
}

func (m schemaOnlyResourceMap) Get(key string) shim.Resource {
	return m[key]
}

func (m schemaOnlyResourceMap) GetOk(key string) (shim.Resource, bool) {
	v, ok := m[key]
	return v, ok
}

func (m schemaOnlyResourceMap) Range(each func(key string, value shim.Resource) bool) {
	for k, v := range m {
		if !each(k, v) {
			return
		}
	}
}

func (m schemaOnlyResourceMap) Set(key string, value shim.Resource) {
	v, ok := value.(*schemaOnlyResource)
	contract.Assertf(ok, "Set must be a %T, found a %T", v, value)
	m[key] = v
}

func (m schemaOnlyResourceMap) All() []runtypes.TypeOrRenamedEntityName {
	arr := make([]runtypes.TypeOrRenamedEntityName, 0, len(m))
	for k := range m {
		arr = append(arr, runtypes.TypeOrRenamedEntityName(k))
	}
	return arr
}

func (m schemaOnlyResourceMap) Has(key runtypes.TypeOrRenamedEntityName) bool {
	_, ok := m[string(key)]
	return ok
}

func (m schemaOnlyResourceMap) Schema(key runtypes.TypeOrRenamedEntityName) runtypes.Schema {
	return m[string(key)].tf
}

func (m schemaOnlyResourceMap) IsResources() {}
