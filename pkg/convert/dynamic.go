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

package convert

import (
	"fmt"
	"math/big"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// To support DynamicPseudoType, perform a best-effort conversion at the data level without any type information.
func newDynamicEncoder() Encoder {
	return &dynamicEncoder{}
}

type dynamicEncoder struct{}

// Convert a PropertyValue to a tftypes.Value. If the optional ty parameter is specified, ensure that the resulting
// tftypes.Value is constructed with the given type.
func (enc *dynamicEncoder) fromPropertyValue(p resource.PropertyValue) (tftypes.Value, error) {
	switch {
	case propertyValueIsUnknown(p):
		return tftypes.NewValue(tftypes.Object{}, tftypes.UnknownValue), nil
	case p.IsNull():
		return tftypes.NewValue(tftypes.Object{}, nil), nil
	case p.IsBool():
		return tftypes.NewValue(tftypes.Bool, p.BoolValue()), nil
	case p.IsNumber():
		return tftypes.NewValue(tftypes.Number, p.NumberValue()), nil
	case p.IsString():
		return tftypes.NewValue(tftypes.String, p.StringValue()), nil
	case p.IsArray():
		var result []tftypes.Value
		for _, e := range p.ArrayValue() {
			te, err := enc.fromPropertyValue(e)
			if err != nil {
				return tftypes.Value{}, err
			}
			result = append(result, te)
		}
		// tftypes.NewValue is pretty strict in that array elements must have the same type.
		// Try to encode this as an array and fallback on a tuple if the types do not match.
		if commonTy, err := tftypes.TypeFromElements(result); err == nil {
			return tftypes.NewValue(tftypes.List{ElementType: commonTy}, result), nil
		} else {
			types := []tftypes.Type{}
			for _, r := range result {
				types = append(types, r.Type())
			}
			return tftypes.NewValue(tftypes.Tuple{ElementTypes: types}, result), nil
		}
	case p.IsAsset():
		return tftypes.Value{}, fmt.Errorf("Assets inside dynamically typed blocks are not yet supported")
	case p.IsArchive():
		return tftypes.Value{}, fmt.Errorf("Archives inside dynamically typed blocks are not yet supported")
	case p.IsSecret() || p.IsOutput() && p.OutputValue().Secret:
		return tftypes.Value{}, fmt.Errorf("Secrets inside dynamically typed blocks are not yet supported")
	case p.IsOutput():
		// Cannot be unknown at this point, or secret. Strip dependencies and send the value.
		return enc.fromPropertyValue(p.OutputValue().Element)
	case p.IsObject():
		// Maps and objects are confused in this Pulumi representation, we cannot reliably tell them apart.
		// Assume this is an object, but do not inflect property names in any way.
		objT := tftypes.Object{AttributeTypes: map[string]tftypes.Type{}}
		result := map[string]tftypes.Value{}
		for k, v := range p.ObjectValue() {
			te, err := enc.fromPropertyValue(v)
			if err != nil {
				return tftypes.Value{}, err
			}
			result[string(k)] = te
			objT.AttributeTypes[string(k)] = te.Type()
		}
		return tftypes.NewValue(objT, result), nil
	case p.IsResourceReference():
		return tftypes.Value{}, fmt.Errorf("Resource references inside dynamically typed blocks are not yet supported")
	default:
		contract.Failf("Unexpected PropertyValue case: %v", p)
		panic("Unreachable")
	}
}

type dynamicDecoder struct{}

func newDynamicDecoder() Decoder {
	return &dynamicDecoder{}
}

func (dec *dynamicDecoder) toPropertyValue(v tftypes.Value) (resource.PropertyValue, error) {
	switch {
	case !v.IsKnown():
		return unknownProperty(), nil
	case v.IsNull():
		return resource.NewPropertyValue(nil), nil
	case v.Type().Is(tftypes.Bool):
		var x bool
		err := v.As(&x)
		if err != nil {
			return resource.PropertyValue{}, err
		}
		return resource.NewBoolProperty(x), nil
	case v.Type().Is(tftypes.Number):
		var x big.Float
		err := v.As(&x)
		if err != nil {
			return resource.PropertyValue{}, err
		}
		f, _ := x.Float64()
		return resource.NewNumberProperty(f), nil
	case v.Type().Is(tftypes.String):
		var s string
		err := v.As(&s)
		if err != nil {
			return resource.PropertyValue{}, err
		}
		return resource.NewStringProperty(s), nil
	case v.Type().Is(tftypes.List{}):
		var elements []tftypes.Value
		err := v.As(&elements)
		if err != nil {
			return resource.PropertyValue{}, err
		}
		var translated []resource.PropertyValue
		for _, e := range elements {
			te, err := dec.toPropertyValue(e)
			if err != nil {
				return resource.PropertyValue{}, err
			}
			translated = append(translated, te)
		}
		return resource.NewArrayProperty(translated), nil
	case v.Type().Is(tftypes.Map{}), v.Type().Is(tftypes.Object{}):
		var elements map[string]tftypes.Value
		err := v.As(&elements)
		if err != nil {
			return resource.PropertyValue{}, err
		}
		translated := make(resource.PropertyMap)
		for k, v := range elements {
			tv, err := dec.toPropertyValue(v)
			if err != nil {
				return resource.PropertyValue{}, err
			}
			translated[resource.PropertyKey(k)] = tv
		}
		return resource.NewObjectProperty(translated), nil
	case v.Type().Is(tftypes.Tuple{}):
		// Unlike the normal encoding for tuples, assume here that this is an array where dynamic encoding
		// failed to compute a uniform type for every value, and therefore decode it back to an array.
		var elements []tftypes.Value
		err := v.As(&elements)
		if err != nil {
			return resource.PropertyValue{}, err
		}
		var result []resource.PropertyValue
		for _, e := range elements {
			te, err := dec.toPropertyValue(e)
			if err != nil {
				return resource.PropertyValue{}, err
			}
			result = append(result, te)
		}
		return resource.NewArrayProperty(result), nil
	default:
		contract.Failf("Unexpected tftypes.Value case: %v", v.String())
		panic("Unreachable")
	}
}
