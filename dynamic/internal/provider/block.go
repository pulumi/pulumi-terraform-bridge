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

package provider

import (
	otshim "github.com/opentofu/opentofu/shim"

	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

var (
	_ = shim.Resource(block{})
	_ = shim.SchemaMap(blockSchema{})
	_ = shim.Schema(nestedBlockAttr{})
)

type block struct {
	pseudoResource
	block otshim.SchemaBlock
}

func (b block) Schema() shim.SchemaMap {
	return blockSchema{b.block}
}

func (b block) DeprecationMessage() string { return b.block.Description }

type blockSchema struct{ block otshim.SchemaBlock }

func (m blockSchema) Len() int { return len(m.block.Attributes) + len(m.block.BlockTypes) }

func (m blockSchema) Get(key string) shim.Schema { return getSchemaMap(m, key) }

func (m blockSchema) GetOk(key string) (shim.Schema, bool) {
	if a, ok := m.block.Attributes[key]; ok {
		return attribute{*a}, true
	}

	if b, ok := m.block.BlockTypes[key]; ok {
		return nestedBlockAttr{*b}, true
	}

	return nil, false
}

func (m blockSchema) Range(each func(key string, value shim.Schema) bool) {
	for k, a := range m.block.Attributes {
		if !each(k, attribute{*a}) {
			return
		}
	}

	for k, b := range m.block.BlockTypes {
		if !each(k, nestedBlockAttr{*b}) {
			return
		}
	}
}

func (m blockSchema) Set(key string, value shim.Schema) {
	switch v := value.(type) {
	case attribute:
		m.block.Attributes[key] = &v.attr
	case nestedBlockAttr:
		m.block.BlockTypes[key] = &v.block
	default:
		contract.Failf("Must set an %T, found %T", v, value)
	}
}

func (m blockSchema) Delete(key string) {
	// Because blocks are attributes are disjoint, we can just attempt to delete from
	// both.
	delete(m.block.Attributes, key)
	delete(m.block.BlockTypes, key)
}

func (m blockSchema) Validate() error { return m.block.InternalValidate() }

type nestedBlockAttr struct{ block otshim.SchemaNestedBlock }

func (m nestedBlockAttr) Type() shim.ValueType                { return shim.TypeMap }
func (m nestedBlockAttr) Optional() bool                      { return false }
func (m nestedBlockAttr) Required() bool                      { return false }
func (m nestedBlockAttr) Default() interface{}                { return nil }
func (m nestedBlockAttr) DefaultFunc() shim.SchemaDefaultFunc { return nil }
func (m nestedBlockAttr) DefaultValue() (interface{}, error)  { return nil, nil }
func (m nestedBlockAttr) Description() string                 { return m.block.Description }
func (m nestedBlockAttr) Computed() bool                      { return false }
func (m nestedBlockAttr) ForceNew() bool                      { return false }
func (m nestedBlockAttr) StateFunc() shim.SchemaStateFunc     { return nil }
func (m nestedBlockAttr) Elem() interface{}                   { return block{block: m.block.Block} }
func (m nestedBlockAttr) MaxItems() int                       { return m.block.MaxItems }
func (m nestedBlockAttr) MinItems() int                       { return m.block.MinItems }
func (m nestedBlockAttr) ConflictsWith() []string             { return nil }
func (m nestedBlockAttr) ExactlyOneOf() []string              { return nil }
func (m nestedBlockAttr) Removed() string                     { return "" }
func (m nestedBlockAttr) Sensitive() bool                     { return false }

func (m nestedBlockAttr) Deprecated() string {
	if m.block.Deprecated {
		return "Deprecated"
	}
	return ""
}

func (m nestedBlockAttr) SetElement(config interface{}) (interface{}, error) {
	panic("Cannot set a an element for a map type")
}
func (m nestedBlockAttr) SetHash(v interface{}) int { panic("Cannot set an hash for an object type") }
