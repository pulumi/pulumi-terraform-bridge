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

// Data Source map needs to support Set (mutability) for RenameDataSource.
func newSchemaOnlyDataSourceMap(dataSources runtypes.DataSources) schemaOnlyDataSourceMap {
	m := schemaOnlyDataSourceMap{}
	for _, name := range dataSources.All() {
		key := string(name)
		v := dataSources.Schema(name)
		m[key] = &schemaOnlyDataSource{v}
	}
	return m
}

type schemaOnlyDataSourceMap map[string]*schemaOnlyDataSource

var (
	_ shim.ResourceMap     = schemaOnlyDataSourceMap{}
	_ runtypes.DataSources = schemaOnlyDataSourceMap{}
)

func (m schemaOnlyDataSourceMap) Len() int {
	return len(m)
}

func (m schemaOnlyDataSourceMap) Get(key string) shim.Resource {
	return m[key]
}

func (m schemaOnlyDataSourceMap) GetOk(key string) (shim.Resource, bool) {
	v, ok := m[key]
	return v, ok
}

func (m schemaOnlyDataSourceMap) Range(each func(key string, value shim.Resource) bool) {
	for k, v := range m {
		if !each(k, v) {
			return
		}
	}
}

func (m schemaOnlyDataSourceMap) Set(key string, value shim.Resource) {
	v, ok := value.(*schemaOnlyDataSource)
	contract.Assertf(ok, "Set must be a %T, found a %T", v, value)
	m[key] = v
}

func (m schemaOnlyDataSourceMap) All() []runtypes.TypeOrRenamedEntityName {
	arr := make([]runtypes.TypeOrRenamedEntityName, 0, len(m))
	for k := range m {
		arr = append(arr, runtypes.TypeOrRenamedEntityName(k))
	}
	return arr
}

func (m schemaOnlyDataSourceMap) Has(key runtypes.TypeOrRenamedEntityName) bool {
	_, ok := m[string(key)]
	return ok
}

func (m schemaOnlyDataSourceMap) Schema(key runtypes.TypeOrRenamedEntityName) runtypes.Schema {
	return m[string(key)].tf
}

func (m schemaOnlyDataSourceMap) IsDataSources() {}
