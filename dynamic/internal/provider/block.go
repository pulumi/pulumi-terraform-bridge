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

// Each conceptual block has three components:
//
// - blockSchema can be directly returned from a [shim.SchemaMap], as it implements
// [shim.Schema].
//
// - blockResource is the pseudoResource that represents a partial access of the fields in
// a map. It is only created as the result of [blockSchema.Elem()].
//
// - blockMap is the attribute map returned from calling [blockResource.Schema()].

var (
	_ = shim.Schema(blockSchema{})
	_ = shim.Resource(blockResource{})
	_ = shim.SchemaMap(blockMap{})
)

type blockResource struct {
	pseudoResource
	block otshim.SchemaBlock
}

func (b blockResource) Schema() shim.SchemaMap     { return blockMap{b.block} }
func (b blockResource) DeprecationMessage() string { return deprecated(b.block.Deprecated) }

type blockMap struct{ block otshim.SchemaBlock }

func (m blockMap) Len() int { return len(m.block.Attributes) + len(m.block.BlockTypes) }

func (m blockMap) Get(key string) shim.Schema { return getSchemaMap(m, key) }

func (m blockMap) GetOk(key string) (shim.Schema, bool) {
	if a, ok := m.block.Attributes[key]; ok {
		return attribute{*a}, true
	}

	if b, ok := m.block.BlockTypes[key]; ok {
		return blockSchema{*b}, true
	}

	return nil, false
}

func (m blockMap) Range(each func(key string, value shim.Schema) bool) {
	for k, a := range m.block.Attributes {
		if !each(k, attribute{*a}) {
			return
		}
	}

	for k, b := range m.block.BlockTypes {
		if !each(k, blockSchema{*b}) {
			return
		}
	}
}

func (m blockMap) Set(key string, value shim.Schema) {
	switch v := value.(type) {
	case attribute:
		m.block.Attributes[key] = &v.attr
	case blockSchema:
		m.block.BlockTypes[key] = &v.block
	default:
		contract.Failf("Must set an %T, found %T", v, value)
	}
}

func (m blockMap) Delete(key string) {
	// Because blocks are attributes are disjoint, we can just attempt to delete from
	// both.
	delete(m.block.Attributes, key)
	delete(m.block.BlockTypes, key)
}

func (m blockMap) Validate() error { return m.block.InternalValidate() }

type blockSchema struct{ block otshim.SchemaNestedBlock }

func (m blockSchema) Type() shim.ValueType                { return shim.TypeMap }
func (m blockSchema) Optional() bool                      { return false }
func (m blockSchema) Required() bool                      { return false }
func (m blockSchema) Default() interface{}                { return nil }
func (m blockSchema) DefaultFunc() shim.SchemaDefaultFunc { return nil }
func (m blockSchema) DefaultValue() (interface{}, error)  { return nil, nil }
func (m blockSchema) Description() string                 { return m.block.Description }
func (m blockSchema) Computed() bool                      { return false }
func (m blockSchema) ForceNew() bool                      { return false }
func (m blockSchema) StateFunc() shim.SchemaStateFunc     { return nil }
func (m blockSchema) Elem() interface{}                   { return blockResource{block: m.block.Block} }
func (m blockSchema) MaxItems() int                       { return m.block.MaxItems }
func (m blockSchema) MinItems() int                       { return m.block.MinItems }
func (m blockSchema) ConflictsWith() []string             { return nil }
func (m blockSchema) ExactlyOneOf() []string              { return nil }
func (m blockSchema) Removed() string                     { return "" }
func (m blockSchema) Sensitive() bool                     { return false }
func (m blockSchema) Deprecated() string                  { return deprecated(m.block.Deprecated) }

func (m blockSchema) SetElement(config interface{}) (interface{}, error) {
	panic("Cannot set a an element for a map type")
}
func (m blockSchema) SetHash(v interface{}) int { panic("Cannot set an hash for an object type") }
