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
// "fmt"
// "math/big"

// "github.com/hashicorp/terraform-plugin-go/tfprotov6"
// "github.com/hashicorp/terraform-plugin-go/tftypes"

// "github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

/*
func ConvertPropertyMapToTFValue(
	objectType tftypes.Object,
) func(m resource.PropertyMap) (tftypes.Value, error) {
	props := map[string]func(p resource.PropertyValue) (tftypes.Value, error){}
	keys := map[string]resource.PropertyKey{}

	for attr, attrType := range objectType.AttributeTypes {
		props[attr] = ConvertPropertyToTFValue(attrType)
		keys[attr] = toPropertyKey(attr, attrType)
	}

	convert := func(m resource.PropertyMap) (tftypes.Value, error) {
		fields := map[string]tftypes.Value{}
		for p, conv := range props {
			v, gotV := m[keys[p]]
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
	keys := map[string]resource.PropertyKey{}

	for attr, attrType := range objectType.AttributeTypes {
		props[attr] = ConvertTFValueToProperty(attrType)
		keys[attr] = toPropertyKey(attr, attrType)
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
				result[keys[p]] = convertedV
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
	case ty.Is(tftypes.Object{}):
		objTy := ty.(tftypes.Object)
		return decObj(objTy)
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
	case ty.Is(tftypes.Object{}):
		objTy := ty.(tftypes.Object)
		return encObj(objTy)
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

func decObj(objType tftypes.Object) decoder {
}

func encObj(objType tftypes.Object) encoder {
}
*/
