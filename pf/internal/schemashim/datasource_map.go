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
	"github.com/pulumi/pulumi-terraform-bridge/pf/internal/pfutils"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

type schemaOnlyDataSourceMap struct {
	dataSources pfutils.DataSources
}

var _ shim.ResourceMap = (*schemaOnlyDataSourceMap)(nil)

func (m *schemaOnlyDataSourceMap) Len() int {
	return len(m.dataSources.All())
}

func (m *schemaOnlyDataSourceMap) Get(key string) shim.Resource {
	s := m.dataSources.Schema(pfutils.TypeName(key))
	return &schemaOnlyDataSource{s}
}

func (m *schemaOnlyDataSourceMap) GetOk(key string) (shim.Resource, bool) {
	if !m.dataSources.Has(pfutils.TypeName(key)) {
		return nil, false
	}
	return m.Get(key), true
}

func (m *schemaOnlyDataSourceMap) Range(each func(key string, value shim.Resource) bool) {
	for _, typeName := range m.dataSources.All() {
		key := string(typeName)
		if !each(key, m.Get(key)) {
			return
		}
	}
}

func (*schemaOnlyDataSourceMap) Set(key string, value shim.Resource) {
	panic("Set not supported - is it possible to treat this as immutable?")
}

func (*schemaOnlyDataSourceMap) AddAlias(alias, target string) {
	panic("AddAlias not supported")
}
