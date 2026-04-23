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

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/internal/internalinter"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/internal/runtypes"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

// Data Source map needs to support Set (mutability) for RenameAction.
func newSchemaOnlyActionMap(actions runtypes.Actions) schemaOnlyActionMap {
	m := schemaOnlyActionMap{}
	for _, name := range actions.All() {
		key := string(name)
		v := actions.Schema(name)
		m[key] = &schemaOnlyAction{v, internalinter.Internal{}}
	}
	return m
}

type schemaOnlyActionMap map[string]*schemaOnlyAction

var (
	_ shim.ActionMap   = schemaOnlyActionMap{}
	_ runtypes.Actions = schemaOnlyActionMap{}
)

func (m schemaOnlyActionMap) Len() int {
	return len(m)
}

func (m schemaOnlyActionMap) Get(key string) shim.Action {
	return m[key]
}

func (m schemaOnlyActionMap) GetOk(key string) (shim.Action, bool) {
	v, ok := m[key]
	return v, ok
}

func (m schemaOnlyActionMap) Range(each func(key string, value shim.Action) bool) {
	for k, v := range m {
		if !each(k, v) {
			return
		}
	}
}

func (m schemaOnlyActionMap) Set(key string, value shim.Action) {
	v, ok := value.(*schemaOnlyAction)
	contract.Assertf(ok, "Set must be a %T, found a %T", v, value)
	m[key] = v
}

func (m schemaOnlyActionMap) All() []runtypes.TypeOrRenamedEntityName {
	arr := make([]runtypes.TypeOrRenamedEntityName, 0, len(m))
	for k := range m {
		arr = append(arr, runtypes.TypeOrRenamedEntityName(k))
	}
	return arr
}

func (m schemaOnlyActionMap) Has(key runtypes.TypeOrRenamedEntityName) bool {
	_, ok := m[string(key)]
	return ok
}

func (m schemaOnlyActionMap) Schema(key runtypes.TypeOrRenamedEntityName) runtypes.Schema {
	return m[string(key)].tf
}

func (m schemaOnlyActionMap) IsActions() {}
