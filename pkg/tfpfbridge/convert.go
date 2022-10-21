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

func ConvertPropertyToTFValue(
	ty tftypes.Type,
) func(p resource.PropertyValue) (tftypes.Value, error) {
	switch {
	case ty.Is(tftypes.String):
		return func(p resource.PropertyValue) (tftypes.Value, error) {
			if p.IsNull() {
				return tftypes.NewValue(tftypes.String, nil), nil
			}
			if !p.IsString() {
				return tftypes.NewValue(ty, nil),
					fmt.Errorf("Expected a string")
			}
			// handle unknowns?
			return tftypes.NewValue(tftypes.String, p.String()), nil
		}
	case ty.Is(tftypes.Number):
		return func(p resource.PropertyValue) (tftypes.Value, error) {
			if p.IsNull() {
				return tftypes.NewValue(tftypes.Number, nil), nil
			}
			if !p.IsNumber() {
				return tftypes.NewValue(ty, nil),
					fmt.Errorf("Expected a Number")
			}
			// handle unknowns?
			return tftypes.NewValue(tftypes.Number, p.NumberValue()), nil
		}
	default:
		return func(p resource.PropertyValue) (tftypes.Value, error) {
			return tftypes.NewValue(ty, nil),
				fmt.Errorf("Not supported: %s", ty.String())
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
