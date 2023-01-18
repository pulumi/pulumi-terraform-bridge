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
	"fmt"
	"sort"

	"github.com/pulumi/pulumi-terraform-bridge/pkg/tfpfbridge/pfutils"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

type schemaMap struct {
	attrs  map[string]pfutils.Attr
	blocks map[string]pfutils.Block
}

func newSchemaMap(tf pfutils.Schema) *schemaMap {
	return &schemaMap{
		attrs:  tf.Attrs(),
		blocks: tf.Blocks(),
	}
}

var _ shim.SchemaMap = (*schemaMap)(nil)

func (m *schemaMap) Len() int {
	n := 0
	m.Range(func(string, shim.Schema) bool {
		n++
		return true
	})
	return n
}

func (m *schemaMap) Get(key string) shim.Schema {
	s, ok := m.GetOk(key)
	if !ok {
		panic(fmt.Sprintf("Missing key: %v", key))
	}
	return s
}

func (m *schemaMap) GetOk(key string) (shim.Schema, bool) {
	attr, ok := m.attrs[key]
	if !ok {
		block, ok := m.blocks[key]
		if !ok {
			return nil, false
		}
		return &blockSchema{key: key, block: block}, true
	}
	return &attrSchema{key: key, attr: attr}, true
}

func (m *schemaMap) Range(each func(key string, value shim.Schema) bool) {
	sortedKeys := []string{}
	seenKeys := map[string]struct{}{}

	for k := range m.attrs {
		sortedKeys = append(sortedKeys, k)
		seenKeys[k] = struct{}{}
	}

	for k := range m.blocks {
		if _, seen := seenKeys[k]; !seen {
			sortedKeys = append(sortedKeys, k)
			seenKeys[k] = struct{}{}
		}
	}

	sort.Strings(sortedKeys)

	for _, key := range sortedKeys {
		if !each(key, m.Get(key)) {
			return
		}
	}
}

func (m *schemaMap) Set(key string, value shim.Schema) {
	panic("Set not supported - is it possible to treat this as immutable?")
}

func (m *schemaMap) Delete(key string) {
	panic("Delete not supported - is it possible to treat this as immutable?")
}
