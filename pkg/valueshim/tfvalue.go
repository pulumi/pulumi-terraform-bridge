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

	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// Wrap a tftypes.Value as Value.
func FromTValue(v tftypes.Value) Value {
	return tValueShim(v)
}

// Wrap a tftypes.Type as Type.
func FromTType(t tftypes.Type) Type {
	return tTypeShim{t: t}
}

type tValueShim tftypes.Value

var _ Value = (*tValueShim)(nil)

func (v tValueShim) val() tftypes.Value {
	return tftypes.Value(v)
}

func (v tValueShim) IsNull() bool {
	return v.val().IsNull()
}

func (v tValueShim) GoString() string {
	return v.val().String()
}

func (v tValueShim) Type() Type {
	return FromTType(v.val().Type())
}

func (v tValueShim) AsValueSlice() []Value {
	var s []tftypes.Value
	err := v.val().As(&s)
	contract.AssertNoErrorf(err, "AsValueSlice failed")
	res := make([]Value, len(s))
	for i, v := range s {
		res[i] = tValueShim(v)
	}
	return res
}

func (v tValueShim) AsValueMap() map[string]Value {
	var m map[string]tftypes.Value
	err := v.val().As(&m)
	contract.AssertNoErrorf(err, "AsValueMap failed")
	res := make(map[string]Value, len(m))
	for k, v := range m {
		res[k] = tValueShim(v)
	}
	return res
}

func (v tValueShim) Marshal() (json.RawMessage, error) {
	inmem, err := jsonMarshal(v.val(), tftypes.NewAttributePath())
	if err != nil {
		return nil, err
	}
	return json.Marshal(inmem)
}

func (v tValueShim) Remove(prop string) Value {
	if !v.Type().IsObjectType() {
		return v
	}

	var m map[string]tftypes.Value
	err := v.val().As(&m)
	contract.AssertNoErrorf(err, "AsValueMap failed")

	delete(m, prop)

	t1 := tftypes.Object{}
	t0 := v.val().Type().(tftypes.Object)

	for k, v := range t0.AttributeTypes {
		if k == prop {
			continue
		}
		if t1.AttributeTypes == nil {
			t1.AttributeTypes = make(map[string]tftypes.Type)
		}
		t1.AttributeTypes[k] = v
	}
	for k, v := range t0.OptionalAttributes {
		if k == prop {
			continue
		}
		if t1.OptionalAttributes == nil {
			t1.OptionalAttributes = make(map[string]struct{})
		}
		t1.OptionalAttributes[k] = v
	}
	return tValueShim(tftypes.NewValue(t1, m))
}

func (v tValueShim) BoolValue() bool {
	var result bool
	err := v.val().As(&result)
	contract.AssertNoErrorf(err, "Cannot cast value as BoolValue")
	return result
}

func (v tValueShim) NumberValue() float64 {
	result := v.BigFloatValue()
	f, _ := result.Float64()
	return f
}

func (v tValueShim) BigFloatValue() *big.Float {
	var result big.Float
	err := v.val().As(&result)
	contract.AssertNoErrorf(err, "Cannot cast value as NumberValue")
	return &result
}

func (v tValueShim) StringValue() string {
	var result string
	err := v.val().As(&result)
	contract.AssertNoErrorf(err, "Cannot cast value as StringValue")
	return result
}

func jsonMarshal(v tftypes.Value, p *tftypes.AttributePath) (interface{}, error) {
	if v.IsNull() {
		return nil, nil
	}
	if !v.IsKnown() {
		return nil, p.NewErrorf("unknown values cannot be serialized to JSON")
	}
	typ := v.Type()
	switch {
	case typ.Is(tftypes.String):
		return jsonMarshalString(v, p)
	case typ.Is(tftypes.Number):
		return jsonMarshalNumber(v, p)
	case typ.Is(tftypes.Bool):
		return jsonMarshalBool(v, p)
	case typ.Is(tftypes.List{}):
		return jsonMarshalList(v, p)
	case typ.Is(tftypes.Set{}):
		return jsonMarshalSet(v, p)
	case typ.Is(tftypes.Map{}):
		return jsonMarshalMap(v, p)
	case typ.Is(tftypes.Tuple{}):
		return jsonMarshalTuple(v, p)
	case typ.Is(tftypes.Object{}):
		return jsonMarshalObject(v, p)
	}

	return nil, p.NewErrorf("unknown type %s", typ)
}

func jsonMarshalString(v tftypes.Value, p *tftypes.AttributePath) (interface{}, error) {
	var stringValue string
	err := v.As(&stringValue)
	if err != nil {
		return nil, p.NewError(err)
	}
	return stringValue, nil
}

func jsonMarshalNumber(v tftypes.Value, p *tftypes.AttributePath) (interface{}, error) {
	var n big.Float
	err := v.As(&n)
	if err != nil {
		return nil, p.NewError(err)
	}
	return json.Number(n.Text('f', -1)), nil
}

func jsonMarshalBool(v tftypes.Value, p *tftypes.AttributePath) (interface{}, error) {
	var b bool
	err := v.As(&b)
	if err != nil {
		return nil, p.NewError(err)
	}
	return b, nil
}

func jsonMarshalList(v tftypes.Value, p *tftypes.AttributePath) (interface{}, error) {
	var vs []tftypes.Value
	err := v.As(&vs)
	if err != nil {
		return nil, p.NewError(err)
	}
	res := make([]any, len(vs))
	for i, v := range vs {
		ep := p.WithElementKeyInt(i)
		e, err := jsonMarshal(v, ep)
		if err != nil {
			return nil, ep.NewError(err)
		}
		res[i] = e
	}
	return res, nil
}

// Important to preserve original order of tftypes.Value here.
func jsonMarshalSet(v tftypes.Value, p *tftypes.AttributePath) (interface{}, error) {
	var vs []tftypes.Value
	err := v.As(&vs)
	if err != nil {
		return nil, p.NewError(err)
	}
	res := make([]any, len(vs))
	for i, v := range vs {
		ep := p.WithElementKeyValue(v)
		e, err := jsonMarshal(v, ep)
		if err != nil {
			return nil, ep.NewError(err)
		}
		res[i] = e
	}
	return res, nil
}

func jsonMarshalMap(v tftypes.Value, p *tftypes.AttributePath) (interface{}, error) {
	var vs map[string]tftypes.Value
	err := v.As(&vs)
	if err != nil {
		return nil, p.NewError(err)
	}
	res := make(map[string]any, len(vs))
	for k, v := range vs {
		ep := p.WithElementKeyValue(v)
		e, err := jsonMarshal(v, ep)
		if err != nil {
			return nil, ep.NewError(err)
		}
		res[k] = e
	}
	return res, nil
}

func jsonMarshalTuple(v tftypes.Value, p *tftypes.AttributePath) (interface{}, error) {
	var vs []tftypes.Value
	err := v.As(&vs)
	if err != nil {
		return nil, p.NewError(err)
	}
	res := make([]any, len(vs))
	for i, v := range vs {
		ep := p.WithElementKeyInt(i)
		e, err := jsonMarshal(v, ep)
		if err != nil {
			return nil, ep.NewError(err)
		}
		res[i] = e
	}
	return res, nil
}

func jsonMarshalObject(v tftypes.Value, p *tftypes.AttributePath) (interface{}, error) {
	var vs map[string]tftypes.Value
	err := v.As(&vs)
	if err != nil {
		return nil, p.NewError(err)
	}
	res := make(map[string]any, len(vs))
	for k, v := range vs {
		ep := p.WithAttributeName(k)
		e, err := jsonMarshal(v, ep)
		if err != nil {
			return nil, ep.NewError(err)
		}
		res[k] = e
	}
	return res, nil
}

type tTypeShim struct {
	t tftypes.Type
}

var _ Type = (*tTypeShim)(nil)

func (t tTypeShim) ty() tftypes.Type {
	return t.t
}

func (t tTypeShim) IsNumberType() bool {
	return t.ty().Is(tftypes.Number)
}

func (t tTypeShim) IsBooleanType() bool {
	return t.ty().Is(tftypes.Bool)
}

func (t tTypeShim) IsStringType() bool {
	return t.ty().Is(tftypes.String)
}

func (t tTypeShim) IsListType() bool {
	return t.ty().Is(tftypes.List{})
}

func (t tTypeShim) IsMapType() bool {
	return t.ty().Is(tftypes.Map{})
}

func (t tTypeShim) IsSetType() bool {
	return t.ty().Is(tftypes.Set{})
}

func (t tTypeShim) IsObjectType() bool {
	return t.ty().Is(tftypes.Object{})
}

func (t tTypeShim) GoString() string {
	return t.ty().String()
}
