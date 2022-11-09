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
	"github.com/pulumi/pulumi-terraform-bridge/pkg/tfpfbridge/pfutils"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

type schemaOnlyResourceMap struct {
	resources pfutils.Resources
}

var _ shim.ResourceMap = (*schemaOnlyResourceMap)(nil)

func (m *schemaOnlyResourceMap) Len() int {
	return len(m.resources.All())
}

func (m *schemaOnlyResourceMap) Get(key string) shim.Resource {
	n := pfutils.TypeName(key)
	s := m.resources.Schema(n)
	return &schemaOnlyResource{&s}
}

func (m *schemaOnlyResourceMap) GetOk(key string) (shim.Resource, bool) {
	n := pfutils.TypeName(key)
	if !m.resources.Has(n) {
		return nil, false
	}
	return m.Get(key), true
}

func (m *schemaOnlyResourceMap) Range(each func(key string, value shim.Resource) bool) {
	for _, name := range m.resources.All() {
		key := string(name)
		if !each(key, m.Get(key)) {
			return
		}
	}
}

func (*schemaOnlyResourceMap) Set(key string, value shim.Resource) {
	panic("Set not supported - is it possible to treat this as immutable?")
}
