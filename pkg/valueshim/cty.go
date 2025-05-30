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
	"math/big"

	"github.com/hashicorp/go-cty/cty"
	ctyjson "github.com/hashicorp/go-cty/cty/json"
)

// Wrap a cty.Value as Value.
func FromHCtyValue(v cty.Value) Value {
	return ctyValueShim(v)
}

// Wrap a cty.Type as Type.
func FromHCtyType(v cty.Type) Type {
	return ctyTypeShim(v)
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
	return FromHCtyType(v.val().Type())
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

func (v ctyValueShim) Marshal() (json.RawMessage, error) {
	vv := v.val()
	raw, err := ctyjson.Marshal(vv, vv.Type())
	if err != nil {
		return nil, err
	}
	return json.RawMessage(raw), nil
}

type ctyTypeShim cty.Type

var _ Type = (*ctyTypeShim)(nil)

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

func (t ctyTypeShim) GoString() string {
	return t.ty().GoString()
}
