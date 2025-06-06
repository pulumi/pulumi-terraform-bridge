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
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/internal/internalinter"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/internal/pfutils"
	bridge "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

type typeSchema struct {
	t pfattr.Type

	// Object types record attr metadata for each field here, if available.
	nested map[string]pfutils.Attr
	internalinter.Internal
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
	case basetypes.ObjectTypable:
		var pseudoResource shim.Resource = newObjectPseudoResource(tt, s.nested, nil)
		return pseudoResource
	case basetypes.SetTypable, basetypes.ListTypable, basetypes.MapTypable:
		typeWithElementType, ok := s.t.(pfattr.TypeWithElementType)
		contract.Assertf(ok, "List, Set or Map type %T expect to implement TypeWithElementType", s.t)
		contract.Assertf(len(s.nested) == 0,
			"s.t==%T should not have any s.nested attrs", s.t)
		return newTypeSchema(typeWithElementType.ElementType(), nil)
	case pfattr.TypeWithElementTypes:
		pseudoResource := newTuplePseudoResource(tt)
		return pseudoResource
	default:
		return nil
	}
}
func (*typeSchema) MaxItems() int      { return 0 }
func (*typeSchema) MinItems() int      { return 0 }
func (*typeSchema) Deprecated() string { return "" }

func (*typeSchema) Default() interface{} {
	panic("Default() should not be called during schema generation")
}

func (*typeSchema) DefaultFunc() shim.SchemaDefaultFunc {
	panic("DefaultFunc() should not be called during schema generation")
}

func (*typeSchema) DefaultValue() (interface{}, error) {
	// DefaultValue() should not be called by tfgen, but it currently may be called by ExtractInputsFromOutputs, so
	// returning nil is better than a panic.
	return nil, bridge.ErrSchemaDefaultValue
}

func (*typeSchema) HasDefault() bool {
	return false
}

func (*typeSchema) Description() string {
	return ""
}

func (*typeSchema) StateFunc() shim.SchemaStateFunc {
	// StateFunc() should not be called during schema generation, but may be called by ExtractInputsFromOutputs.
	return nil
}

func (*typeSchema) ConflictsWith() []string {
	panic("ConflictsWith() should not be called during schema generation")
}

func (*typeSchema) ExactlyOneOf() []string {
	panic("ExactlyOneOf() should not be called during schema generation")
}

func (*typeSchema) Removed() string { panic("Removed() should not be called during schema generation") }

func (*typeSchema) SetElement(config interface{}) (interface{}, error) {
	panic("SetElement() should not be called during schema generation")
}

func (*typeSchema) SetHash(v interface{}) int {
	panic("SetHash() should not be called during schema generation")
}

func (*typeSchema) SetElementHash(v interface{}) (int, error) {
	panic("SetElementHash() should not be called during schema generation")
}

func (*typeSchema) NewSet(v []interface{}) interface{} {
	panic("NewSet() should not be called during schema generation")
}

func (*typeSchema) WriteOnly() bool {
	return false
}
