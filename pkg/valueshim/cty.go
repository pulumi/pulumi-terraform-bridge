// Copyright 2016-2025, Pulumi Corporation.
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

package valueshim

import (
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/zclconf/go-cty/cty"
	ctyjson "github.com/zclconf/go-cty/cty/json"
)

// Wrap a cty.Value as Value.
func FromCtyValue(v cty.Value) Value {
	return ctyValueShim(v)
}

// Wrap a cty.Type as Type.
func FromCtyType(v cty.Type) Type {
	return ctyTypeShim(v)
}

// ToCtyType extracts the underlying cty.Type from a Type.
func ToCtyType(t Type) (cty.Type, error) {
	if ct, ok := t.(ctyTypeShim); ok {
		return ct.ty(), nil
	}
	return cty.Type{}, fmt.Errorf("Cannot convert Type to cty.Type: %#T", t)
}

type ctyValueShim cty.Value

var _ Value = (*ctyValueShim)(nil)

func (v ctyValueShim) val() cty.Value {
	return cty.Value(v)
}

func (v ctyValueShim) IsNull() bool {
	return v.val().IsNull()
}

func (v ctyValueShim) GoString() string {
	return v.val().GoString()
}

func (v ctyValueShim) Type() Type {
	return FromCtyType(v.val().Type())
}

func (v ctyValueShim) StringValue() string {
	return v.val().AsString()
}

func (v ctyValueShim) BoolValue() bool {
	return v.val().True()
}

func (v ctyValueShim) NumberValue() float64 {
	bf := v.BigFloatValue()
	f, _ := bf.Float64()
	return f
}

func (v ctyValueShim) BigFloatValue() *big.Float {
	return v.val().AsBigFloat()
}

func (v ctyValueShim) AsValueSlice() []Value {
	s := v.val().AsValueSlice()
	res := make([]Value, len(s))
	for i, v := range s {
		res[i] = ctyValueShim(v)
	}
	return res
}

func (v ctyValueShim) AsValueMap() map[string]Value {
	m := v.val().AsValueMap()
	res := make(map[string]Value, len(m))

	for k, v := range m {
		res[k] = ctyValueShim(v)
	}
	return res
}

func (v ctyValueShim) Remove(key string) Value {
	switch {
	case v.val().Type().IsObjectType():
		m := v.val().AsValueMap()
		delete(m, key)
		if len(m) == 0 {
			return ctyValueShim(cty.EmptyObjectVal)
		}
		return ctyValueShim(cty.ObjectVal(m))
	default:
		return v
	}
}

func (v ctyValueShim) Marshal(schemaType Type) (json.RawMessage, error) {
	vv := v.val()
	tt, ok := schemaType.(ctyTypeShim)
	if !ok {
		return nil, fmt.Errorf("Cannot marshal to RawState: "+
			"expected schemaType to be of type ctyTypeShim, got %#T",
			schemaType)
	}
	raw, err := ctyjson.Marshal(vv, tt.ty())
	if err != nil {
		return nil, fmt.Errorf("Cannot marshal to RawState: %w", err)
	}
	return json.RawMessage(raw), nil
}

type ctyTypeShim cty.Type

var _ Type = ctyTypeShim{}

func (t ctyTypeShim) ty() cty.Type {
	return cty.Type(t)
}

func (t ctyTypeShim) IsNumberType() bool {
	return t.ty().Equals(cty.Number)
}

func (t ctyTypeShim) IsBooleanType() bool {
	return t.ty().Equals(cty.Bool)
}

func (t ctyTypeShim) IsStringType() bool {
	return t.ty().Equals(cty.String)
}

func (t ctyTypeShim) IsListType() bool {
	return t.ty().IsListType()
}

func (t ctyTypeShim) IsMapType() bool {
	return t.ty().IsMapType()
}

func (t ctyTypeShim) IsSetType() bool {
	return t.ty().IsSetType()
}

func (t ctyTypeShim) IsObjectType() bool {
	return t.ty().IsObjectType()
}

func (t ctyTypeShim) IsDynamicType() bool {
	return t.ty().Equals(cty.DynamicPseudoType)
}

func (t ctyTypeShim) AttributeType(name string) (Type, bool) {
	tt := t.ty()
	if !tt.IsObjectType() {
		return nil, false
	}
	if !tt.HasAttribute(name) {
		return nil, false
	}
	return FromCtyType(tt.AttributeType(name)), true
}

func (t ctyTypeShim) ElementType() (Type, bool) {
	tt := t.ty()
	if !tt.IsCollectionType() {
		return nil, false
	}
	return FromCtyType(tt.ElementType()), true
}

func (t ctyTypeShim) GoString() string {
	return t.ty().GoString()
}
