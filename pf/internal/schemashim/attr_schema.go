// Copyright 2016-2023, Pulumi Corporation.
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
	bridge "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"

	pfattr "github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"

	"github.com/pulumi/pulumi-terraform-bridge/pf/internal/pfutils"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

type attrSchema struct {
	key  string
	attr pfutils.Attr
}

var _ shim.Schema = (*attrSchema)(nil)

func (s *attrSchema) Type() shim.ValueType {
	ty := s.attr.GetType()
	vt, err := convertType(ty)
	if err != nil {
		panic(err)
	}
	return vt
}

func (s *attrSchema) Optional() bool {
	return s.attr.IsOptional()
}

func (s *attrSchema) Required() bool {
	return s.attr.IsRequired()
}

func (*attrSchema) Default() interface{} {
	panic("Default() should not be called during schema generation")
}
func (*attrSchema) DefaultFunc() shim.SchemaDefaultFunc {
	panic("DefaultFunc() should not be called during schema generation")
}
func (*attrSchema) DefaultValue() (interface{}, error) {
	// DefaultValue() should not be called by tfgen, but it currently may be called by ExtractInputsFromOutputs, so
	// returning nil is better than a panic.
	return nil, bridge.SchemaDefaultValueErr
}

func (s *attrSchema) Description() string {
	if desc := s.attr.GetMarkdownDescription(); desc != "" {
		return desc
	}
	return s.attr.GetDescription()
}

func (s *attrSchema) Computed() bool {
	return s.attr.IsComputed()
}

func (s *attrSchema) ForceNew() bool {
	// TODO[pulumi/pulumi-terraform-bridge#818] try harder to detect
	return false
}

func (*attrSchema) StateFunc() shim.SchemaStateFunc {
	// StateFunc() should not be called by tfgen, but it currently may be called by ExtractInputsFromOutputs, so
	// returning nil is better than a panic.
	return nil
}

// Needs to return a shim.Schema, a shim.Resource, or nil.
func (s *attrSchema) Elem() interface{} {
	t := s.attr.GetType()

	// The ObjectType can be triggered through tfsdk.SingleNestedAttributes. Logically it defines an attribute with
	// a type that is an Object type. To encode the schema of the Object type in a way the shim layer understands,
	// Elem() needes to return a Resource value.
	//
	// See also: documentation on shim.Schema.Elem().
	if tt, ok := t.(basetypes.ObjectTypable); ok {
		var res shim.Resource = newObjectPseudoResource(tt, s.attr.Nested(), nil)
		return res
	}
	if tt, ok := t.(pfattr.TypeWithElementTypes); ok {
		var res shim.Resource = newTuplePseudoResource(tt)
		return res
	}

	// Anything else that does not have an ElementType can be skipped.
	if _, ok := t.(pfattr.TypeWithElementType); !ok {
		return nil
	}

	var schema shim.Schema
	switch tt := t.(type) {
	case types.MapType:
		schema = newTypeSchema(tt.ElemType, s.attr.Nested())
	case types.ListType:
		schema = newTypeSchema(tt.ElemType, s.attr.Nested())
	case types.SetType:
		schema = newTypeSchema(tt.ElemType, s.attr.Nested())
	default:
		panic(fmt.Errorf("This Elem() case is not yet supported: %v", t))
	}
	return schema
}

func (*attrSchema) MaxItems() int {
	// There seems to be no way to find MaxItems/MinItems on attributes. There are workarounds employed to find them
	// for Blocks.
	return 0
}

func (*attrSchema) MinItems() int {
	// There seems to be no way to find MaxItems/MinItems on attributes. There are workarounds employed to find them
	// for Blocks.
	return 0
}

func (*attrSchema) ConflictsWith() []string {
	panic("ConflictsWith() should not be called during schema generation")

}
func (*attrSchema) ExactlyOneOf() []string {
	panic("ExactlyOneOf() should not be called during schema generation")
}

func (s *attrSchema) Deprecated() string {
	return s.attr.GetDeprecationMessage()
}

func (*attrSchema) Removed() string {
	// Following v2, returning empty string here. This does not seem to be supported.
	return ""
}

func (s *attrSchema) Sensitive() bool {
	return s.attr.IsSensitive()
}

func (*attrSchema) UnknownValue() interface{} {
	panic("UnknownValue() should not be called during schema generation")
}

func (*attrSchema) SetElement(config interface{}) (interface{}, error) {
	panic("SetElement() should not be called during schema generation")
}

func (*attrSchema) SetHash(v interface{}) int {
	panic("SetHash() should not be called during schema generation")
}
