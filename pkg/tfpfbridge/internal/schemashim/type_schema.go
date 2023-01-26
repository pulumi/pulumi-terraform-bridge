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
	pfattr "github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/pulumi/pulumi-terraform-bridge/pkg/tfpfbridge/internal/pfutils"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

type typeSchema struct {
	t pfattr.Type

	// Object types record attr metadata for each field here, if available.
	nested map[string]pfutils.Attr
}

var _ shim.Schema = (*typeSchema)(nil)

func newTypeSchema(t pfattr.Type, nested map[string]pfutils.Attr) *typeSchema {
	return &typeSchema{
		t:      t,
		nested: nested,
	}
}

func (s *typeSchema) Type() shim.ValueType {
	vt, err := convertType(s.t)
	if err != nil {
		panic(err)
	}
	return vt
}

// Return zero values the following methods. GenerateSchema calls them, although they make no sense
// at the level of typeSchema. While attrSchema may be Optional or Required, typeSchema is cannot.
func (*typeSchema) Computed() bool  { return false }
func (*typeSchema) ForceNew() bool  { return false }
func (*typeSchema) Optional() bool  { return false }
func (*typeSchema) Required() bool  { return false }
func (*typeSchema) Sensitive() bool { return false }

func (s *typeSchema) Elem() interface{} {
	switch tt := s.t.(type) {
	case types.ObjectType:
		var pseudoResource shim.Resource = newObjectPseudoResource(tt, s.nested)
		return pseudoResource
	case types.ListType:
		contract.Assert(s.nested == nil || len(s.nested) == 0)
		return newTypeSchema(tt.ElemType, nil)
	case types.MapType:
		contract.Assert(s.nested == nil || len(s.nested) == 0)
		return newTypeSchema(tt.ElemType, nil)
	default:
		// TODO Set case
		return nil
	}
}

func (*typeSchema) MaxItems() int      { return 0 }
func (*typeSchema) MinItems() int      { return 0 }
func (*typeSchema) Deprecated() string { return "" }

func (*typeSchema) Default() interface{}                { panic("TODO") }
func (*typeSchema) DefaultFunc() shim.SchemaDefaultFunc { panic("TODO") }
func (*typeSchema) DefaultValue() (interface{}, error)  { panic("TODO") }
func (*typeSchema) Description() string                 { panic("TODO") }
func (*typeSchema) StateFunc() shim.SchemaStateFunc     { panic("TODO") }
func (*typeSchema) ConflictsWith() []string             { panic("TODO") }
func (*typeSchema) ExactlyOneOf() []string              { panic("TODO") }
func (*typeSchema) Removed() string                     { panic("TODO") }

func (*typeSchema) UnknownValue() interface{}                          { panic("TODO") }
func (*typeSchema) SetElement(config interface{}) (interface{}, error) { panic("TODO") }
func (*typeSchema) SetHash(v interface{}) int                          { panic("TODO") }
