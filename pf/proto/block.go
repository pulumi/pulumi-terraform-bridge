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
	"fmt"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"

	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
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
	block *tfprotov6.SchemaBlock
}

func (b blockResource) Implementation() string     { return "pf" }
func (b blockResource) UseJSONNumber() bool        { return false }
func (b blockResource) Schema() shim.SchemaMap     { return blockMap{b.block} }
func (b blockResource) DeprecationMessage() string { return deprecated(b.block.Deprecated) }

type blockMap struct{ block *tfprotov6.SchemaBlock }

func (m blockMap) Len() int { return len(m.block.Attributes) + len(m.block.BlockTypes) }

func (m blockMap) Get(key string) shim.Schema { return getSchemaMap(m, key) }

func (m blockMap) GetOk(key string) (shim.Schema, bool) {
	for _, a := range m.block.Attributes {
		if a.Name == key {
			return attribute{*a}, true
		}
	}

	for _, b := range m.block.BlockTypes {
		if b.TypeName == key {
			return blockSchema{*b}, true
		}
	}

	return nil, false
}

func (m blockMap) Range(each func(key string, value shim.Schema) bool) {
	for _, a := range m.block.Attributes {
		if !each(a.Name, attribute{*a}) {
			return
		}
	}

	for _, b := range m.block.BlockTypes {
		if !each(b.TypeName, blockSchema{*b}) {
			return
		}
	}
}

func (m blockMap) Validate() error { return nil }

type blockSchema struct{ block tfprotov6.SchemaNestedBlock }

func (m blockSchema) Type() shim.ValueType {
	switch m.block.Nesting {
	case tfprotov6.SchemaNestedBlockNestingModeGroup,
		tfprotov6.SchemaNestedBlockNestingModeSingle,
		tfprotov6.SchemaNestedBlockNestingModeMap:
		return shim.TypeMap
	case tfprotov6.SchemaNestedBlockNestingModeList,
		tfprotov6.SchemaNestedBlockNestingModeSet:
		return shim.TypeList
	default:
		panic(fmt.Sprintf("Invalid nesting kind %v", m.block.Nesting))
	}
}

func (m blockSchema) Elem() interface{} {
	// for a detailed case analysis of the logic here, see [attribute.Elem].
	switch m.block.Nesting {
	// m represents a Map<Object>, so return a [shim.Schema].
	case tfprotov6.SchemaNestedBlockNestingModeMap:
		contract.Assertf(m.Type() == shim.TypeMap, "this must be a map")
		m.block.Nesting = tfprotov6.SchemaNestedBlockNestingModeSingle
		return blockSchema{m.block}
	default:
		return blockResource{block: m.block.Block}
	}
}

func (m blockSchema) Implementation() string              { return "pf" }
func (m blockSchema) Optional() bool                      { return false }
func (m blockSchema) Required() bool                      { return false }
func (m blockSchema) Default() interface{}                { return nil }
func (m blockSchema) DefaultFunc() shim.SchemaDefaultFunc { return nil }
func (m blockSchema) DefaultValue() (interface{}, error)  { return nil, nil }
func (m blockSchema) Description() string                 { return m.block.Block.Description }
func (m blockSchema) Computed() bool                      { return false }
func (m blockSchema) ForceNew() bool                      { return false }
func (m blockSchema) StateFunc() shim.SchemaStateFunc     { return nil }
func (m blockSchema) MaxItems() int                       { return int(m.block.MaxItems) }
func (m blockSchema) MinItems() int                       { return int(m.block.MinItems) }
func (m blockSchema) ConflictsWith() []string             { return nil }
func (m blockSchema) ExactlyOneOf() []string              { return nil }
func (a blockSchema) AtLeastOneOf() []string              { return nil }
func (a blockSchema) RequiredWith() []string              { return nil }
func (a blockSchema) ConfigMode() shim.ConfigModeType     { return 0 }
func (m blockSchema) Removed() string                     { return "" }
func (m blockSchema) Sensitive() bool                     { return false }
func (m blockSchema) Deprecated() string                  { return deprecated(m.block.Block.Deprecated) }

func (m blockSchema) SetElement(config interface{}) (interface{}, error) {
	panic("Cannot set a an element for a map type")
}
func (m blockSchema) SetHash(v interface{}) int { panic("Cannot set an hash for an object type") }
