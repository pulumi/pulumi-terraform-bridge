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

	pfattr "github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/pulumi/pulumi-terraform-bridge/pkg/tfpfbridge/internal/pfutils"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

type blockSchema struct {
	key   string
	block pfutils.Block
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

// Needs to return a shim.Schema, a shim.Resource, or nil.
func (s *blockSchema) Elem() interface{} {
	t := s.block.Type()

	// The ObjectType can be triggered through tfsdk.SingleNestedAttributes. Logically it defines an attribute with
	// a type that is an Object type. To encode the schema of the Object type in a way the shim layer understands,
	// Elem() needes to return a Resource value.
	//
	// See also: documentation on shim.Schema.Elem().
	if tt, ok := t.(types.ObjectType); ok {
		var res shim.Resource = newObjectPseudoResource(tt, s.block.NestedAttrs())
		return res
	}

	// Anything else that does not have an ElementType can be skipped.
	if _, ok := t.(pfattr.TypeWithElementType); !ok {
		return nil
	}

	var schema shim.Schema
	switch tt := t.(type) {
	case types.MapType:
		schema = newTypeSchema(tt.ElemType, s.block.NestedAttrs())
	case types.ListType:
		schema = newTypeSchema(tt.ElemType, s.block.NestedAttrs())
	default:
		// TODO SetType
		panic(fmt.Errorf("TODO: unhandled elem case: %v", t))
	}
	return schema
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

func (*blockSchema) ConflictsWith() []string                            { panic("TODO") }
func (*blockSchema) Default() interface{}                               { panic("TODO") }
func (*blockSchema) DefaultFunc() shim.SchemaDefaultFunc                { panic("TODO") }
func (*blockSchema) DefaultValue() (interface{}, error)                 { panic("TODO") }
func (*blockSchema) ExactlyOneOf() []string                             { panic("TODO") }
func (*blockSchema) SetElement(config interface{}) (interface{}, error) { panic("TODO") }
func (*blockSchema) SetHash(v interface{}) int                          { panic("TODO") }
func (*blockSchema) StateFunc() shim.SchemaStateFunc                    { panic("TODO") }
func (*blockSchema) UnknownValue() interface{}                          { panic("TODO") }
