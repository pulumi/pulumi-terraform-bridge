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

	"github.com/hashicorp/go-cty/cty"
	ctyjson "github.com/hashicorp/go-cty/cty/json"
)

// Wrap a cty.Value as Value.
func FromHCtyValue(v cty.Value) Value {
	return hctyValueShim(v)
}

// Wrap a cty.Type as Type.
func FromHCtyType(v cty.Type) Type {
	return hctyTypeShim(v)
}

type hctyValueShim cty.Value

var _ Value = (*hctyValueShim)(nil)

func (v hctyValueShim) val() cty.Value {
	return cty.Value(v)
}

func (v hctyValueShim) IsNull() bool {
	return v.val().IsNull()
}

func (v hctyValueShim) GoString() string {
	return v.val().GoString()
}

func (v hctyValueShim) Type() Type {
	return FromHCtyType(v.val().Type())
}

func (v hctyValueShim) StringValue() string {
	return v.val().AsString()
}

func (v hctyValueShim) BoolValue() bool {
	return v.val().True()
}

func (v hctyValueShim) NumberValue() float64 {
	f, _ := v.val().AsBigFloat().Float64()
	return f
}

func (v hctyValueShim) AsValueSlice() []Value {
	s := v.val().AsValueSlice()
	res := make([]Value, len(s))
	for i, v := range s {
		res[i] = hctyValueShim(v)
	}
	return res
}

func (v hctyValueShim) AsValueMap() map[string]Value {
	m := v.val().AsValueMap()
	res := make(map[string]Value, len(m))

	for k, v := range m {
		res[k] = hctyValueShim(v)
	}
	return res
}

func (v hctyValueShim) Remove(key string) Value {
	switch {
	case v.val().Type().IsObjectType():
		m := v.val().AsValueMap()
		delete(m, key)
		if len(m) == 0 {
			return hctyValueShim(cty.EmptyObjectVal)
		}
		return hctyValueShim(cty.ObjectVal(m))
	default:
		return v
	}
}

func (v hctyValueShim) Marshal(schemaType Type) (json.RawMessage, error) {
	vv := v.val()
	tt, ok := schemaType.(hctyTypeShim)
	if !ok {
		return nil, fmt.Errorf("Cannot marshal to RawState: "+
			"expected schemaType to be of type hctyTypeShim, got %#T",
			schemaType)
	}
	raw, err := ctyjson.Marshal(vv, tt.ty())
	if err != nil {
		return nil, fmt.Errorf("Cannot marshal to RawState: %w", err)
	}
	return json.RawMessage(raw), nil
}

type hctyTypeShim cty.Type

var _ Type = hctyTypeShim{}

func (t hctyTypeShim) ty() cty.Type {
	return cty.Type(t)
}

func (t hctyTypeShim) IsNumberType() bool {
	return t.ty().Equals(cty.Number)
}

func (t hctyTypeShim) IsBooleanType() bool {
	return t.ty().Equals(cty.Bool)
}

func (t hctyTypeShim) IsStringType() bool {
	return t.ty().Equals(cty.String)
}

func (t hctyTypeShim) IsListType() bool {
	return t.ty().IsListType()
}

func (t hctyTypeShim) IsMapType() bool {
	return t.ty().IsMapType()
}

func (t hctyTypeShim) IsSetType() bool {
	return t.ty().IsSetType()
}

func (t hctyTypeShim) IsObjectType() bool {
	return t.ty().IsObjectType()
}

func (t hctyTypeShim) AttributeType(name string) (Type, bool) {
	tt := t.ty()
	if !tt.IsObjectType() {
		return nil, false
	}
	if !tt.HasAttribute(name) {
		return nil, false
	}
	return FromHCtyType(tt.AttributeType(name)), true
}

func (t hctyTypeShim) ElementType() (Type, bool) {
	tt := t.ty()
	if !tt.IsCollectionType() {
		return nil, false
	}
	return FromHCtyType(tt.ElementType()), true
}

func (t hctyTypeShim) GoString() string {
	return t.ty().GoString()
}
