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

package pfutils

import (
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// Inverse of tftypes.ValueFromJson.
func ValueToJSON(typ tftypes.Type, v tftypes.Value) ([]byte, error) {
	raw, err := jsonMarshal(v, typ, tftypes.NewAttributePath())
	if err != nil {
		return nil, err
	}
	return json.Marshal(raw)
}

func jsonMarshal(v tftypes.Value, typ tftypes.Type, p *tftypes.AttributePath) (interface{}, error) {
	if v.IsNull() {
		return nil, nil
	}
	if !v.IsKnown() {
		return nil, p.NewErrorf("unknown values cannot be serialized to JSON")
	}
	switch {
	case typ.Is(tftypes.String):
		return jsonMarshalString(v, typ, p)
	case typ.Is(tftypes.Number):
		return jsonMarshalNumber(v, typ, p)
	case typ.Is(tftypes.Bool):
		return jsonMarshalBool(v, typ, p)
	case typ.Is(tftypes.DynamicPseudoType):
		return jsonMarshalDynamicPseudoType(v, typ, p)
	case typ.Is(tftypes.List{}):
		return jsonMarshalList(v, typ.(tftypes.List).ElementType, p)
	case typ.Is(tftypes.Set{}):
		return jsonMarshalSet(v, typ.(tftypes.Set).ElementType, p)
	case typ.Is(tftypes.Map{}):
		return jsonMarshalMap(v, typ.(tftypes.Map).ElementType, p)
	case typ.Is(tftypes.Tuple{}):
		return jsonMarshalTuple(v, typ.(tftypes.Tuple).ElementTypes, p)
	case typ.Is(tftypes.Object{}):
		return jsonMarshalObject(v, typ.(tftypes.Object).AttributeTypes, p)
	}

	return nil, p.NewErrorf("unknown type %s", typ)
}

func jsonMarshalString(v tftypes.Value, typ tftypes.Type, p *tftypes.AttributePath) (interface{}, error) {
	var stringValue string
	err := v.As(&stringValue)
	if err != nil {
		return nil, p.NewError(err)
	}
	return stringValue, nil
}

func jsonMarshalNumber(v tftypes.Value, typ tftypes.Type, p *tftypes.AttributePath) (interface{}, error) {
	var n big.Float
	err := v.As(&n)
	if err != nil {
		return nil, p.NewError(err)
	}
	f64, _ := n.Float64()
	return f64, nil
}

func jsonMarshalBool(v tftypes.Value, typ tftypes.Type, p *tftypes.AttributePath) (interface{}, error) {
	var b bool
	err := v.As(&b)
	if err != nil {
		return nil, p.NewError(err)
	}
	return b, nil
}

func jsonMarshalDynamicPseudoType(v tftypes.Value, typ tftypes.Type, p *tftypes.AttributePath) ([]byte, error) {
	return nil, fmt.Errorf("DynamicPseudoType is not yet supported")
}

func jsonMarshalList(v tftypes.Value, elementType tftypes.Type, p *tftypes.AttributePath) (interface{}, error) {
	var vs []tftypes.Value
	err := v.As(&vs)
	if err != nil {
		return nil, p.NewError(err)
	}
	var res []interface{}
	for i, v := range vs {
		ep := p.WithElementKeyInt(i)
		e, err := jsonMarshal(v, elementType, ep)
		if err != nil {
			return nil, ep.NewError(err)
		}
		res = append(res, e)
	}
	return res, nil
}

func jsonMarshalSet(v tftypes.Value, elementType tftypes.Type, p *tftypes.AttributePath) (interface{}, error) {
	var vs []tftypes.Value
	err := v.As(&vs)
	if err != nil {
		return nil, p.NewError(err)
	}
	var res []interface{}
	for _, v := range vs {
		ep := p.WithElementKeyValue(v)
		e, err := jsonMarshal(v, elementType, ep)
		if err != nil {
			return nil, ep.NewError(err)
		}
		res = append(res, e)
	}
	return res, nil
}

func jsonMarshalMap(v tftypes.Value, elementType tftypes.Type, p *tftypes.AttributePath) (interface{}, error) {
	var vs map[string]tftypes.Value
	err := v.As(&vs)
	if err != nil {
		return nil, p.NewError(err)
	}
	var res map[string]interface{}
	for k, v := range vs {
		ep := p.WithElementKeyValue(v)
		e, err := jsonMarshal(v, elementType, ep)
		if err != nil {
			return nil, ep.NewError(err)
		}
		res[k] = e
	}
	return res, nil
}

func jsonMarshalTuple(v tftypes.Value, elementTypes []tftypes.Type, p *tftypes.AttributePath) (interface{}, error) {
	var vs []tftypes.Value
	err := v.As(&vs)
	if err != nil {
		return nil, p.NewError(err)
	}
	var res []interface{}
	for i, v := range vs {
		ep := p.WithElementKeyInt(i)
		e, err := jsonMarshal(v, elementTypes[i], ep)
		if err != nil {
			return nil, ep.NewError(err)
		}
		res = append(res, e)
	}
	return res, nil
}

func jsonMarshalObject(
	v tftypes.Value,
	elementTypes map[string]tftypes.Type,
	p *tftypes.AttributePath,
) (interface{}, error) {
	var vs map[string]tftypes.Value
	err := v.As(&vs)
	if err != nil {
		return nil, p.NewError(err)
	}
	res := map[string]interface{}{}
	for k, v := range vs {
		ep := p.WithAttributeName(k)
		e, err := jsonMarshal(v, elementTypes[k], ep)
		if err != nil {
			return nil, ep.NewError(err)
		}
		res[k] = e
	}
	return res, nil
}
