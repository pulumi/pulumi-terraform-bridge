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

package tfbridge

/*

   	"github.com/hashicorp/terraform-plugin-go/tftypes"
        type system


        primitive types:
            DynamicPseudoType   String|Bool|Number|Tuple|Object
            String              string, *string
            Number
            Bool

        composite type:
            Object {AttributeTypes, OptionalAttributes}
            Set
            Tuple
            Map
            List


Values:
     type Value

     UnknownValue

     v.IsNull() bool
     v.IsKnown() bool
     v.As(dst)

So presumably every type may have null and unknown values.

      NewValue(t Type, val interface{})
      ValidateValue(t, val)

*/

import (
	"fmt"
	"math/big"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

func ConvertPropertyMapToTFValue(
	objectType tftypes.Object,
) func(m resource.PropertyMap) (tftypes.Value, error) {
	props := map[string]func(p resource.PropertyValue) (tftypes.Value, error){}

	for attr, attrType := range objectType.AttributeTypes {
		props[attr] = ConvertPropertyToTFValue(attrType)
	}

	convert := func(m resource.PropertyMap) (tftypes.Value, error) {
		fields := map[string]tftypes.Value{}
		for p, conv := range props {
			v, gotV := m[resource.PropertyKey(p)]
			if gotV {
				convertedV, err := conv(v)
				if err != nil {
					return tftypes.NewValue(objectType, nil),
						fmt.Errorf("Property %v failed to convert: %w",
							p, err)
				}
				fields[p] = convertedV
			} else if _, optional := objectType.OptionalAttributes[p]; !optional {
				fields[p] = tftypes.NewValue(objectType.AttributeTypes[p], nil)
			}
		}
		return tftypes.NewValue(objectType, fields), nil
	}
	return func(m resource.PropertyMap) (tftypes.Value, error) {
		v, err := convert(m)
		if err != nil {
			return v, fmt.Errorf("ConvertPropertyMapToTFValue failed: %w", err)
		}
		return v, nil
	}
}

func ConvertTFValueToPropertyMap(
	objectType tftypes.Object,
) func(v tftypes.Value) (resource.PropertyMap, error) {
	props := map[string]func(tftypes.Value) (resource.PropertyValue, error){}

	for attr, attrType := range objectType.AttributeTypes {
		props[attr] = ConvertTFValueToProperty(attrType)
	}

	convert := func(obj tftypes.Value) (resource.PropertyMap, error) {
		result := make(resource.PropertyMap)
		var contents map[string]tftypes.Value
		if err := obj.As(&contents); err != nil {
			return result, fmt.Errorf(
				"tftypes.Value.As(map[string]tftypes.Value) failed: %w", err)
		}
		for p, conv := range props {
			v, gotV := contents[p]
			if gotV && !v.IsNull() {
				convertedV, err := conv(v)
				if err != nil {
					return result,
						fmt.Errorf("Property %s failed to convert: %w",
							p, err)
				}
				result[resource.PropertyKey(p)] = convertedV
			}
		}
		return result, nil
	}
	return func(obj tftypes.Value) (resource.PropertyMap, error) {
		pm, err := convert(obj)
		if err != nil {
			return pm, fmt.Errorf("ConvertTFValueToPropertyMap failed: %w", err)
		}
		return pm, nil
	}
}

func ConvertTFValueToProperty(
	ty tftypes.Type,
) func(p tftypes.Value) (resource.PropertyValue, error) {
	switch {
	case ty.Is(tftypes.String):
		return decString
	case ty.Is(tftypes.Number):
		return decNumber
	case ty.Is(tftypes.Bool):
		return decBool
	case ty.Is(tftypes.List{}):
		listTy := ty.(tftypes.List)
		decElem := ConvertTFValueToProperty(listTy.ElementType)
		return decList(decElem)
	case ty.Is(tftypes.Map{}):
		mapTy := ty.(tftypes.Map)
		decElem := ConvertTFValueToProperty(mapTy.ElementType)
		return decMap(decElem)
	default:
		return func(v tftypes.Value) (resource.PropertyValue, error) {
			return resource.PropertyValue{},
				fmt.Errorf("ConvertTFValueToProperty does not support type %s: %s",
					ty.String(), v.String())
		}
	}
}

func ConvertPropertyToTFValue(
	ty tftypes.Type,
) func(p resource.PropertyValue) (tftypes.Value, error) {
	switch {
	case ty.Is(tftypes.String):
		return encString
	case ty.Is(tftypes.Number):
		return encNumber
	case ty.Is(tftypes.Bool):
		return encBool
	case ty.Is(tftypes.List{}):
		listTy := ty.(tftypes.List)
		encElem := ConvertPropertyToTFValue(listTy.ElementType)
		return encList(listTy.ElementType, encElem)
	case ty.Is(tftypes.Map{}):
		mapTy := ty.(tftypes.Map)
		encElem := ConvertPropertyToTFValue(mapTy.ElementType)
		return encMap(mapTy.ElementType, encElem)
	default:
		return func(p resource.PropertyValue) (tftypes.Value, error) {
			return tftypes.NewValue(ty, nil),
				fmt.Errorf("ConvertPropertyToTFValue does not support type %s: %s",
					ty.String(), p.String())
		}
	}
}

func ConvertPropertyMapToDynamicValue(
	objectType tftypes.Object,
) func(m resource.PropertyMap) (tfprotov6.DynamicValue, error) {
	f := ConvertPropertyMapToTFValue(objectType)
	return func(m resource.PropertyMap) (tfprotov6.DynamicValue, error) {
		v, err := f(m)
		if err != nil {
			return tfprotov6.DynamicValue{},
				fmt.Errorf("ConvertPropertyMapToDynamicValue failed: %w", err)
		}
		return tfprotov6.NewDynamicValue(objectType, v)
	}
}

func ConvertDynamicValueToPropertyMap(
	objectType tftypes.Object,
) func(dv tfprotov6.DynamicValue) (resource.PropertyMap, error) {
	f := ConvertTFValueToPropertyMap(objectType)
	convert := func(dv tfprotov6.DynamicValue) (resource.PropertyMap, error) {
		v, err := dv.Unmarshal(objectType)
		if err != nil {
			return resource.PropertyMap{},
				fmt.Errorf("DynamicValue.Unmarshal failed: %w", err)
		}
		return f(v)
	}
	return func(dv tfprotov6.DynamicValue) (resource.PropertyMap, error) {
		pm, err := convert(dv)
		if err != nil {
			return pm, fmt.Errorf("ConvertDynamicValueToPropertyMap failed: %w", err)
		}
		return pm, err
	}
}

func encString(p resource.PropertyValue) (tftypes.Value, error) {
	if propertyValueIsUnkonwn(p) {
		return tftypes.NewValue(tftypes.String, tftypes.UnknownValue), nil
	}
	if p.IsNull() {
		return tftypes.NewValue(tftypes.String, nil), nil
	}
	if !p.IsString() {
		return tftypes.NewValue(tftypes.String, nil),
			fmt.Errorf("Expected a string, got: %v", p)
	}
	return tftypes.NewValue(tftypes.String, p.StringValue()), nil
}

// This is how p.ContainsUnknowns checks if the value itself is unknown before recursing.
func propertyValueIsUnkonwn(p resource.PropertyValue) bool {
	return p.IsComputed() || (p.IsOutput() && !p.OutputValue().Known)
}

var unknownStringPropertyValue resource.PropertyValue = resource.NewComputedProperty(
	resource.Computed{Element: resource.NewStringProperty("")})

func decString(v tftypes.Value) (resource.PropertyValue, error) {
	if !v.IsKnown() {
		return unknownStringPropertyValue, nil
	}
	if v.IsNull() {
		return resource.NewPropertyValue(nil), nil
	}
	var s string
	if err := v.As(&s); err != nil {
		return resource.PropertyValue{},
			fmt.Errorf("tftypes.Value.As(string) failed: %w", err)
	}
	return resource.NewStringProperty(s), nil
}

func encNumber(p resource.PropertyValue) (tftypes.Value, error) {
	if propertyValueIsUnkonwn(p) {
		return tftypes.NewValue(tftypes.Number, tftypes.UnknownValue), nil
	}
	if p.IsNull() {
		return tftypes.NewValue(tftypes.Number, nil), nil
	}
	if !p.IsNumber() {
		return tftypes.NewValue(tftypes.Number, nil),
			fmt.Errorf("Expected a Number")
	}
	return tftypes.NewValue(tftypes.Number, p.NumberValue()), nil
}

var unknownNumberPropertyValue resource.PropertyValue = resource.NewComputedProperty(
	resource.Computed{Element: resource.NewNumberProperty(0)})

func decNumber(v tftypes.Value) (resource.PropertyValue, error) {
	if !v.IsKnown() {
		return unknownNumberPropertyValue, nil
	}
	if v.IsNull() {
		return resource.NewPropertyValue(nil), nil
	}
	var n big.Float
	if err := v.As(&n); err != nil {
		return resource.PropertyValue{},
			fmt.Errorf("decNumber fails with %s: %w", v.String(), err)
	}
	f64, _ := n.Float64()
	return resource.NewNumberProperty(f64), nil
}

func encBool(p resource.PropertyValue) (tftypes.Value, error) {
	if propertyValueIsUnkonwn(p) {
		return tftypes.NewValue(tftypes.Bool, tftypes.UnknownValue), nil
	}
	if p.IsNull() {
		return tftypes.NewValue(tftypes.Bool, nil), nil
	}
	if !p.IsBool() {
		return tftypes.NewValue(tftypes.Bool, nil),
			fmt.Errorf("Expected a Boolean")
	}
	return tftypes.NewValue(tftypes.Bool, p.BoolValue()), nil
}

var unknownBoolPropertyValue resource.PropertyValue = resource.NewComputedProperty(
	resource.Computed{Element: resource.NewBoolProperty(false)})

func decBool(v tftypes.Value) (resource.PropertyValue, error) {
	if !v.IsKnown() {
		return unknownBoolPropertyValue, nil
	}
	if v.IsNull() {
		return resource.NewPropertyValue(nil), nil
	}
	var b bool
	if err := v.As(&b); err != nil {
		return resource.PropertyValue{},
			fmt.Errorf("decBool fails with %s: %w", v.String(), err)
	}
	return resource.NewBoolProperty(b), nil
}

type decoder = func(tftypes.Value) (resource.PropertyValue, error)
type encoder = func(p resource.PropertyValue) (tftypes.Value, error)

func decList(decElem decoder) decoder {
	zero := resource.NewArrayProperty([]resource.PropertyValue{})
	return func(v tftypes.Value) (resource.PropertyValue, error) {
		if !v.IsKnown() {
			return resource.NewComputedProperty(resource.Computed{Element: zero}), nil
		}
		if v.IsNull() {
			return resource.NewPropertyValue(nil), nil
		}
		var elements []tftypes.Value
		if err := v.As(&elements); err != nil {
			return resource.PropertyValue{},
				fmt.Errorf("decList fails with %s: %w", v.String(), err)
		}

		values := []resource.PropertyValue{}
		for _, e := range elements {
			ev, err := decElem(e)
			if err != nil {
				return resource.PropertyValue{},
					fmt.Errorf("decList fails with %s: %w", e.String(), err)
			}
			values = append(values, ev)
		}
		return resource.NewArrayProperty(values), nil
	}
}

func encList(elemTy tftypes.Type, encElem encoder) encoder {
	listTy := tftypes.List{ElementType: elemTy}
	return func(p resource.PropertyValue) (tftypes.Value, error) {
		if propertyValueIsUnkonwn(p) {
			return tftypes.NewValue(listTy, tftypes.UnknownValue), nil
		}
		if p.IsNull() {
			return tftypes.NewValue(listTy, nil), nil
		}
		if !p.IsArray() {
			return tftypes.NewValue(listTy, nil),
				fmt.Errorf("Expected an Array PropertyValue")
		}
		var values []tftypes.Value
		for _, pv := range p.ArrayValue() {
			v, err := encElem(pv)
			if err != nil {
				return tftypes.NewValue(listTy, nil),
					fmt.Errorf("encList failed on %v", pv)
			}
			values = append(values, v)
		}
		return tftypes.NewValue(listTy, values), nil
	}
}

func decMap(decElem decoder) decoder {
	zero := resource.NewObjectProperty(make(resource.PropertyMap))
	return func(v tftypes.Value) (resource.PropertyValue, error) {
		if !v.IsKnown() {
			return resource.NewComputedProperty(resource.Computed{Element: zero}), nil
		}
		if v.IsNull() {
			return resource.NewPropertyValue(nil), nil
		}
		elements := map[string]tftypes.Value{}
		if err := v.As(&elements); err != nil {
			return resource.PropertyValue{},
				fmt.Errorf("decMap fails with %s: %w", v.String(), err)
		}

		values := make(resource.PropertyMap)
		for k, e := range elements {
			ev, err := decElem(e)
			if err != nil {
				return resource.PropertyValue{},
					fmt.Errorf("decMap fails with %s: %w", e.String(), err)
			}
			values[resource.PropertyKey(k)] = ev
		}
		return resource.NewObjectProperty(values), nil
	}
}

func encMap(elemTy tftypes.Type, encElem encoder) encoder {
	mapTy := tftypes.Map{ElementType: elemTy}
	return func(p resource.PropertyValue) (tftypes.Value, error) {
		if propertyValueIsUnkonwn(p) {
			return tftypes.NewValue(mapTy, tftypes.UnknownValue), nil
		}
		if p.IsNull() {
			return tftypes.NewValue(mapTy, nil), nil
		}
		if !p.IsObject() {
			return tftypes.NewValue(mapTy, nil),
				fmt.Errorf("Expected an Array PropertyValue")
		}
		values := map[string]tftypes.Value{}
		for key, pv := range p.ObjectValue() {
			v, err := encElem(pv)
			if err != nil {
				return tftypes.NewValue(mapTy, nil),
					fmt.Errorf("encMap failed on %v", pv)
			}
			values[string(key)] = v
		}
		return tftypes.NewValue(mapTy, values), nil
	}
}
