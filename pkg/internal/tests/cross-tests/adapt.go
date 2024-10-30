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

// Adapters for converting morally equivalent typed representations of TF values for integrating with all the libraries
// cross-testing is using.
package crosstests

import (
	"context"
	"fmt"
	"math/big"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/convert"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/unstable/logging"
)

// inferPulumiValue generates a Pulumi value that is semantically equivalent to v.
//
// inferPulumiValue takes into account schema information.
func inferPulumiValue(t T, schema shim.SchemaMap, infos map[string]*info.Schema, v cty.Value) resource.PropertyMap {
	if v.IsNull() {
		return nil
	}
	decoder, err := convert.NewObjectDecoder(convert.ObjectSchema{
		SchemaMap:   schema,
		SchemaInfos: infos,
	})
	require.NoError(t, err)

	ctx := logging.InitLogging(context.Background(), logging.LogOptions{})
	// There is not yet a way to opt out of marking schema secrets, so the resulting map might have secrets marked.
	pm, err := convert.DecodePropertyMap(ctx, decoder, ctyToTftypes(v))
	require.NoError(t, err)
	return pm
}

func ctyToTftypes(v cty.Value) tftypes.Value {
	typ := v.Type()
	if !v.IsKnown() {
		return tftypes.NewValue(ctyTypeToTfType(typ), tftypes.UnknownValue)
	}
	if v.IsNull() {
		return tftypes.NewValue(ctyTypeToTfType(typ), nil)
	}
	switch {
	case typ.Equals(cty.String):
		return tftypes.NewValue(ctyTypeToTfType(typ), v.AsString())
	case typ.Equals(cty.Bool):
		return tftypes.NewValue(ctyTypeToTfType(typ), v.True())
	case typ.Equals(cty.Number):
		return tftypes.NewValue(ctyTypeToTfType(typ), v.AsBigFloat())

	case typ.IsListType():
		src := v.AsValueSlice()
		dst := make([]tftypes.Value, len(src))
		for i, v := range src {
			dst[i] = ctyToTftypes(v)
		}
		return tftypes.NewValue(ctyTypeToTfType(typ), dst)
	case typ.IsSetType():
		src := v.AsValueSet().Values()
		dst := make([]tftypes.Value, len(src))
		for i, v := range src {
			dst[i] = ctyToTftypes(v)
		}
		return tftypes.NewValue(ctyTypeToTfType(typ), dst)
	case typ.IsMapType():
		src := v.AsValueMap()
		dst := make(map[string]tftypes.Value, len(src))
		for k, v := range src {
			dst[k] = ctyToTftypes(v)
		}
		return tftypes.NewValue(ctyTypeToTfType(typ), dst)
	case typ.IsObjectType():
		src := v.AsValueMap()
		dst := make(map[string]tftypes.Value, len(src))
		for k, v := range src {
			dst[k] = ctyToTftypes(v)
		}
		return tftypes.NewValue(ctyTypeToTfType(typ), dst)
	default:
		panic(fmt.Sprintf("unknown type %s", typ.GoString()))
	}
}

func ctyTypeToTfType(typ cty.Type) tftypes.Type {
	switch {
	case typ.Equals(cty.String):
		return tftypes.String
	case typ.Equals(cty.Bool):
		return tftypes.Bool
	case typ.Equals(cty.Number):
		return tftypes.Number
	case typ == cty.DynamicPseudoType:
		return tftypes.DynamicPseudoType

	case typ.IsListType():
		return tftypes.List{ElementType: ctyTypeToTfType(typ.ElementType())}
	case typ.IsSetType():
		return tftypes.Set{ElementType: ctyTypeToTfType(typ.ElementType())}
	case typ.IsMapType():
		return tftypes.Map{ElementType: ctyTypeToTfType(typ.ElementType())}
	case typ.IsObjectType():
		src := typ.AttributeTypes()
		dst := make(map[string]tftypes.Type, len(src))
		for k, v := range src {
			dst[k] = ctyTypeToTfType(v)
		}
		return tftypes.Object{AttributeTypes: dst}
	default:
		panic(fmt.Sprintf("unknown type %s", typ.GoString()))
	}
}

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
				_, ok := aT[k]
				contract.Assertf(ok, "no type for attribute %q", k)
				values[k] = fromType(aT[k]).NewValue(el)
			}
			return tftypes.NewValue(t, values)
		}
	}
	return tftypes.NewValue(t, value)
}

func fromType(t tftypes.Type) *typeAdapter {
	contract.Assertf(t != nil, "type cannot be nil")
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
