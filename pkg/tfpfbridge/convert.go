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

	return func(m resource.PropertyMap) (tftypes.Value, error) {
		fields := map[string]tftypes.Value{}
		for p, conv := range props {
			v, gotV := m[resource.PropertyKey(p)]
			if gotV {
				convertedV, err := conv(v)
				if err != nil {
					return tftypes.NewValue(objectType, nil), err
				}
				fields[p] = convertedV
			} else if _, optional := objectType.OptionalAttributes[p]; !optional {
				fields[p] = tftypes.NewValue(objectType.AttributeTypes[p], nil)
			}
		}
		return tftypes.NewValue(objectType, fields), nil
	}
}

func ConvertTFValueToPropertyMap(
	objectType tftypes.Object,
) func(v tftypes.Value) (resource.PropertyMap, error) {
	props := map[string]func(tftypes.Value) (resource.PropertyValue, error){}

	for attr, attrType := range objectType.AttributeTypes {
		props[attr] = ConvertTFValueToProperty(attrType)
	}

	return func(obj tftypes.Value) (resource.PropertyMap, error) {
		result := make(resource.PropertyMap)
		var contents map[string]tftypes.Value
		if err := obj.As(&contents); err != nil {
			return result, err
		}
		for p, conv := range props {
			v, gotV := contents[p]
			if gotV && !v.IsNull() {
				convertedV, err := conv(v)
				if err != nil {
					return result, err
				}
				result[resource.PropertyKey(p)] = convertedV
			}
		}
		return result, nil
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
	default:
		return func(v tftypes.Value) (resource.PropertyValue, error) {
			return resource.PropertyValue{},
				fmt.Errorf("ConvertTFValueToProperty does not support type %s: %s", ty.String(), v.String())
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
	default:
		return func(p resource.PropertyValue) (tftypes.Value, error) {
			return tftypes.NewValue(ty, nil),
				fmt.Errorf("ConvertPropertyToTFValue does not supported: %s", ty.String())
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
			return tfprotov6.DynamicValue{}, err
		}
		return tfprotov6.NewDynamicValue(objectType, v)
	}
}

func ConvertDynamicValueToPropertyMap(
	objectType tftypes.Object,
) func(dv tfprotov6.DynamicValue) (resource.PropertyMap, error) {
	f := ConvertTFValueToPropertyMap(objectType)
	return func(dv tfprotov6.DynamicValue) (resource.PropertyMap, error) {
		v, err := dv.Unmarshal(objectType)
		if err != nil {
			return resource.PropertyMap{}, err
		}
		return f(v)
	}
}

func encString(p resource.PropertyValue) (tftypes.Value, error) {
	if p.IsNull() {
		return tftypes.NewValue(tftypes.String, nil), nil
	}
	if !p.IsString() {
		return tftypes.NewValue(tftypes.String, nil), fmt.Errorf("Expected a string")
	}
	// TODO handle unknowns
	return tftypes.NewValue(tftypes.String, p.StringValue()), nil
}

func decString(v tftypes.Value) (resource.PropertyValue, error) {
	if v.IsNull() {
		return resource.NewPropertyValue(nil), nil
	}
	// TODO handle unknowns
	var s string
	if err := v.As(&s); err != nil {
		return resource.PropertyValue{}, err
	}
	return resource.NewStringProperty(s), nil
}

func encNumber(p resource.PropertyValue) (tftypes.Value, error) {
	if p.IsNull() {
		return tftypes.NewValue(tftypes.Number, nil), nil
	}
	if !p.IsNumber() {
		return tftypes.NewValue(tftypes.Number, nil), fmt.Errorf("Expected a Number")
	}
	// TODO handle unknowns
	return tftypes.NewValue(tftypes.Number, p.NumberValue()), nil
}

func decNumber(v tftypes.Value) (resource.PropertyValue, error) {
	if v.IsNull() {
		return resource.NewPropertyValue(nil), nil
	}
	// TODO handle unknowns

	var n big.Float
	if err := v.As(&n); err != nil {
		return resource.PropertyValue{}, fmt.Errorf("decNumber fails with %s: %w", v.String(), err)
	}
	f64, _ := n.Float64()
	return resource.NewNumberProperty(f64), nil
}
