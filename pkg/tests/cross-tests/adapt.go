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

// Adapters for converting morally several morally equivalent typed representations of TF values for integrating with
// all the libraries cross-testing is using.
package crosstests

import (
	"math/big"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/zclconf/go-cty/cty"
)

type typeAdapter struct {
	typ tftypes.Type
}

func (ta *typeAdapter) ToCty() cty.Type {
	t := ta.typ
	switch {
	case t.Is(tftypes.String):
		return cty.String
	case t.Is(tftypes.Number):
		return cty.Number
	case t.Is(tftypes.Bool):
		return cty.Bool
	case t.Is(tftypes.List{}):
		return cty.List(fromType(t.(tftypes.List).ElementType).ToCty())
	case t.Is(tftypes.Set{}):
		return cty.Set(fromType(t.(tftypes.Set).ElementType).ToCty())
	case t.Is(tftypes.Map{}):
		return cty.Map(fromType(t.(tftypes.Map).ElementType).ToCty())
	case t.Is(tftypes.Object{}):
		fields := map[string]cty.Type{}
		for k, v := range t.(tftypes.Object).AttributeTypes {
			fields[k] = fromType(v).ToCty()
		}
		return cty.Object(fields)
	default:
		contract.Failf("unexpected type %v", t)
		return cty.NilType
	}
}

func (ta *typeAdapter) NewValue(value any) tftypes.Value {
	t := ta.typ
	if value == nil {
		return tftypes.NewValue(t, nil)
	}
	switch t := value.(type) {
	case tftypes.Value:
		return t
	case *tftypes.Value:
		return *t
	}
	switch {
	case t.Is(tftypes.List{}):
		elT := t.(tftypes.List).ElementType
		switch v := value.(type) {
		case []any:
			values := []tftypes.Value{}
			for _, el := range v {
				values = append(values, fromType(elT).NewValue(el))
			}
			return tftypes.NewValue(t, values)
		}
	case t.Is(tftypes.Set{}):
		elT := t.(tftypes.Set).ElementType
		switch v := value.(type) {
		case []any:
			values := []tftypes.Value{}
			for _, el := range v {
				values = append(values, fromType(elT).NewValue(el))
			}
			return tftypes.NewValue(t, values)
		}
	case t.Is(tftypes.Map{}):
		elT := t.(tftypes.Map).ElementType
		switch v := value.(type) {
		case map[string]any:
			values := map[string]tftypes.Value{}
			for k, el := range v {
				values[k] = fromType(elT).NewValue(el)
			}
			return tftypes.NewValue(t, values)
		}
	case t.Is(tftypes.Object{}):
		aT := t.(tftypes.Object).AttributeTypes
		switch v := value.(type) {
		case map[string]any:
			values := map[string]tftypes.Value{}
			for k, el := range v {
				values[k] = fromType(aT[k]).NewValue(el)
			}
			return tftypes.NewValue(t, values)
		}
	}
	return tftypes.NewValue(t, value)
}

func fromType(t tftypes.Type) *typeAdapter {
	return &typeAdapter{t}
}

type valueAdapter struct {
	value tftypes.Value
}

func (va *valueAdapter) ToCty() cty.Value {
	v := va.value
	t := v.Type()
	switch {
	case v.IsNull():
		return cty.NullVal(fromType(t).ToCty())
	case !v.IsKnown():
		return cty.UnknownVal(fromType(t).ToCty())
	case t.Is(tftypes.String):
		var s string
		err := v.As(&s)
		contract.AssertNoErrorf(err, "unexpected error converting string")
		return cty.StringVal(s)
	case t.Is(tftypes.Number):
		var n *big.Float
		err := v.As(&n)
		contract.AssertNoErrorf(err, "unexpected error converting number")
		return cty.NumberVal(n)
	case t.Is(tftypes.Bool):
		var b bool
		err := v.As(&b)
		contract.AssertNoErrorf(err, "unexpected error converting bool")
		return cty.BoolVal(b)
	case t.Is(tftypes.List{}):
		var vals []tftypes.Value
		err := v.As(&vals)
		contract.AssertNoErrorf(err, "unexpected error converting list")
		if len(vals) == 0 {
			return cty.ListValEmpty(fromType(t).ToCty())
		}
		outVals := make([]cty.Value, len(vals))
		for i, el := range vals {
			outVals[i] = fromValue(el).ToCty()
		}
		return cty.ListVal(outVals)
	case t.Is(tftypes.Set{}):
		var vals []tftypes.Value
		err := v.As(&vals)
		if len(vals) == 0 {
			return cty.SetValEmpty(fromType(t).ToCty())
		}
		contract.AssertNoErrorf(err, "unexpected error converting set")
		outVals := make([]cty.Value, len(vals))
		for i, el := range vals {
			outVals[i] = fromValue(el).ToCty()
		}
		return cty.SetVal(outVals)
	case t.Is(tftypes.Map{}):
		var vals map[string]tftypes.Value
		err := v.As(&vals)
		if len(vals) == 0 {
			return cty.MapValEmpty(fromType(t).ToCty())
		}
		contract.AssertNoErrorf(err, "unexpected error converting map")
		outVals := make(map[string]cty.Value, len(vals))
		for k, el := range vals {
			outVals[k] = fromValue(el).ToCty()
		}
		return cty.MapVal(outVals)
	case t.Is(tftypes.Object{}):
		var vals map[string]tftypes.Value
		err := v.As(&vals)
		if len(vals) == 0 {
			return cty.EmptyObjectVal
		}
		contract.AssertNoErrorf(err, "unexpected error converting object")
		outVals := make(map[string]cty.Value, len(vals))
		for k, el := range vals {
			outVals[k] = fromValue(el).ToCty()
		}
		return cty.ObjectVal(outVals)
	default:
		contract.Failf("unexpected type %v", t)
		return cty.NilVal
	}
}

func fromValue(v tftypes.Value) *valueAdapter {
	return &valueAdapter{v}
}
