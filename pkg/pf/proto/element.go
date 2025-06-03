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
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/internal/internalinter"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

var (
	_ = shim.Schema(element{})
	_ = shim.Resource(elementObject{})
)

type element struct {
	typ      tftypes.Type
	optional bool
	internalinter.Internal
}

func newElement(typ tftypes.Type, optional bool) *element {
	return &element{typ, optional, internalinter.Internal{}}
}

type elementObject struct {
	pseudoResource
	typ tftypes.Object
	internalinter.Internal
}

type elementObjectMap tftypes.Object

func (e element) Type() shim.ValueType {
	t := e.typ
	switch {
	case t.Is(tftypes.Bool):
		return shim.TypeBool
	case t.Is(tftypes.Number):
		// TODO: It looks like this interface only exposes "number", not integer.
		//
		// We should see if there is a work-around here.
		return shim.TypeFloat
	case t.Is(tftypes.String):
		return shim.TypeString
	case t.Is(tftypes.List{}) || t.Is(tftypes.Set{}):
		return shim.TypeList
	case t.Is(tftypes.Map{}) || t.Is(tftypes.Object{}):
		return shim.TypeMap
	case t.Is(tftypes.DynamicPseudoType):
		return shim.TypeDynamic
	default:
		return shim.TypeInvalid
	}
}

func (e element) Elem() interface{} {
	switch t := e.typ.(type) {
	case tftypes.Object:
		return elementObject{typ: t}
	case tftypes.Set:
		return element{typ: t.ElementType}
	case tftypes.Map:
		return element{typ: t.ElementType}
	case tftypes.List:
		return element{typ: t.ElementType}
	}

	return nil
}

func (e element) Optional() bool { return e.optional }

// Methods that are not available to tftypes.Type

func (e element) Required() bool                              { return false }
func (e element) Default() interface{}                        { return nil }
func (e element) DefaultFunc() shim.SchemaDefaultFunc         { return nil }
func (e element) DefaultValue() (interface{}, error)          { return nil, nil }
func (e element) HasDefault() bool                            { return false }
func (e element) Description() string                         { return "" }
func (e element) Computed() bool                              { return false }
func (e element) ForceNew() bool                              { return false }
func (e element) StateFunc() shim.SchemaStateFunc             { return nil }
func (e element) MaxItems() int                               { return 0 }
func (e element) MinItems() int                               { return 0 }
func (e element) ConflictsWith() []string                     { return nil }
func (e element) ExactlyOneOf() []string                      { return nil }
func (e element) Deprecated() string                          { return "" }
func (e element) Removed() string                             { return "" }
func (e element) Sensitive() bool                             { return false }
func (e element) SetElement(interface{}) (interface{}, error) { return nil, nil }
func (e element) SetHash(interface{}) int                     { return 0 }
func (e element) SetElementHash(interface{}) (int, error)     { return 0, nil }
func (e element) NewSet([]interface{}) interface{}            { return nil }
func (e element) WriteOnly() bool                             { return false }

func (o elementObject) DeprecationMessage() string { return "" }
func (o elementObject) Schema() shim.SchemaMap {
	return elementObjectMap(o.typ)
}

func (m elementObjectMap) Len() int { return len(m.AttributeTypes) }

func (m elementObjectMap) Get(key string) shim.Schema { return getSchemaMap(m, key) }

func (m elementObjectMap) GetOk(key string) (shim.Schema, bool) {
	v, ok := m.AttributeTypes[key]
	if !ok {
		return nil, false
	}
	_, optional := m.OptionalAttributes[key]
	return newElement(v, optional), true
}

func (m elementObjectMap) Range(each func(key string, value shim.Schema) bool) {
	for k, v := range m.AttributeTypes {
		_, optional := m.OptionalAttributes[k]
		if !each(k, newElement(v, optional)) {
			return
		}
	}
}

func (m elementObjectMap) Validate() error { return nil }
