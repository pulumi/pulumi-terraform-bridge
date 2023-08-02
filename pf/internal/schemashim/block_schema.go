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

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"

	"github.com/pulumi/pulumi-terraform-bridge/pf/internal/pfutils"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

type blockSchema struct {
	key   string
	block pfutils.Block
}

func newBlockSchema(key string, block pfutils.Block) *blockSchema {
	return &blockSchema{key, block}
}

var _ shim.Schema = (*blockSchema)(nil)

func (s *blockSchema) Type() shim.ValueType {
	ty := s.block.Type()
	vt, err := convertType(ty)
	if err != nil {
		panic(err)
	}
	return vt
}

func (s *blockSchema) Description() string {
	if desc := s.block.GetMarkdownDescription(); desc != "" {
		return desc
	}
	return s.block.GetDescription()
}

// Needs to return a shim.Schema, a shim.Resource, or nil. See docstring on shim.Schema.Elem().
func (s *blockSchema) Elem() interface{} {

	asObjectType := func(typ any) (shim.Resource, bool) {
		if tt, ok := typ.(basetypes.ObjectTypable); ok {
			var res shim.Resource = newObjectPseudoResource(tt,
				s.block.NestedAttrs(),
				s.block.NestedBlocks())
			return res, true
		}
		return nil, false
	}

	t := s.block.Type()

	// Single-nested blocks have a block.Type() that is an ObjectTypeable directly.
	if r, ok := asObjectType(t); ok {
		return r
	}

	switch tt := s.block.Type().(type) {
	case types.ListType:
		r, ok := asObjectType(tt.ElemType)
		if !ok {
			panic(fmt.Errorf("List-nested block expect an ObjectTypeable "+
				"block.Type().ElemType, but got %v", tt.ElemType))
		}
		return r
	case types.SetType:
		r, ok := asObjectType(tt.ElemType)
		if !ok {
			panic(fmt.Errorf("Set-nested block expect an ObjectTypeable "+
				"block.Type().ElemType, but got %v", tt.ElemType))
		}
		return r
	default:
		panic(fmt.Errorf("block.Type()==%v is not supported for blocks", t))
	}
}

func (s *blockSchema) Optional() bool {
	return !s.Required()
}

func (s *blockSchema) Required() bool {
	return s.block.GetMinItems() > 0
}

func (*blockSchema) Computed() bool       { return false }
func (*blockSchema) ForceNew() bool       { return false }
func (*blockSchema) Removed() string      { return "" }
func (*blockSchema) Sensitive() bool      { return false }
func (s *blockSchema) Deprecated() string { return s.block.GetDeprecationMessage() }
func (s *blockSchema) MaxItems() int      { return int(s.block.GetMaxItems()) }
func (s *blockSchema) MinItems() int      { return int(s.block.GetMinItems()) }

func (*blockSchema) ConflictsWith() []string {
	panic("ConflictsWith() should not be called during schema generation")
}

func (*blockSchema) Default() interface{} {
	panic("Default() should not be called during schema generation")
}

func (*blockSchema) DefaultFunc() shim.SchemaDefaultFunc {
	panic("DefaultFunc() should not be called during schema generation")
}

func (*blockSchema) DefaultValue() (interface{}, error) {
	// DefaultValue() should not be called by tfgen, but it currently may be called by ExtractInputsFromOutputs, so
	// returning nil is better than a panic.
	return nil, nil
}

func (*blockSchema) ExactlyOneOf() []string {
	panic("ExactlyOneOf() should not be called during schema generation")
}

func (*blockSchema) SetElement(config interface{}) (interface{}, error) {
	panic("SetElement() should not be called during schema generation")
}

func (*blockSchema) SetHash(v interface{}) int {
	panic("SetHash() should not be called during schema generation")
}

func (*blockSchema) StateFunc() shim.SchemaStateFunc {
	panic("StateFunc() should not be called during schema generation")
}

func (*blockSchema) UnknownValue() interface{} {
	panic("UnknownValue() should not be called during schema generation")
}
