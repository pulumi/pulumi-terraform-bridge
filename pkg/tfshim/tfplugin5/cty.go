package tfplugin5

import (
	"fmt"
	"reflect"

	"github.com/hashicorp/go-cty/cty"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// ctyToGo converts a cty.Value to a plain Go value with the notable exception of sets, which are left as-is. Sets can
// be converted to plain values by calling provider.IsSet ala tfbridge. Capsule types are not supported.
func ctyToGo(val cty.Value) (interface{}, error) {
	switch {
	case val.IsNull():
		// Convert null values to nil.
		return nil, nil
	case !val.IsKnown():
		// Convert unknown values to the unknown variable value.
		return UnknownVariableValue, nil
	case val.Type().IsPrimitiveType():
		switch val.Type() {
		case cty.Bool:
			return val.True(), nil
		case cty.Number:
			// Convert number values to floats.
			float, _ := val.AsBigFloat().Float64()
			return float, nil
		case cty.String:
			return val.AsString(), nil
		}
	case val.Type().IsListType(), val.Type().IsTupleType():
		// Recursively convert lists and tuples into a slice of Go values.
		result := make([]interface{}, val.LengthInt())
		iter := val.ElementIterator()
		for iter.Next() {
			k, v := iter.Element()
			i, _ := k.AsBigFloat().Int64()

			gv, err := ctyToGo(v)
			if err != nil {
				return nil, err
			}
			result[int(i)] = gv
		}
		return result, nil
	case val.Type().IsSetType():
		// Leave sets as-is. We'll convert their element to Go values when they need to be accessed.
		return val, nil
	case val.Type().IsMapType(), val.Type().IsObjectType():
		// Recursively convert maps and objects to maps of Go values.
		result := map[string]interface{}{}
		iter := val.ElementIterator()
		for iter.Next() {
			k, v := iter.Element()

			contract.Assert(k.Type() == cty.String)
			contract.Assert(!k.IsNull())
			if !k.IsKnown() {
				return UnknownVariableValue, nil
			}

			gv, err := ctyToGo(v)
			if err != nil {
				return nil, err
			}
			result[k.AsString()] = gv
		}
		return result, nil
	}
	return nil, fmt.Errorf("unsupported cty type %v", val.Type().FriendlyName())
}

// goToCty converts a reflect.Value to a cty.Value of the given type. Capsule types are not supported.
// Only a limited set of Go values are supported: bools, ints/uints/floats, strings, arrays/slices, and maps with
// string-typed keys. Structs are not supported.
func goToCty(v interface{}, ty cty.Type) (cty.Value, error) {
	return reflectToCty(reflect.ValueOf(v), ty)
}

var ctyValueType = reflect.TypeOf((*cty.Value)(nil)).Elem()

// reflectToCty converts a reflect.Value to a cty.Value of the given type. Capsule types are not supported.
// Only a limited set of Go values are supported: bools, ints/uints/floats, strings, arrays/slices, and maps with
// string-typed keys. Structs are not supported.
func reflectToCty(v reflect.Value, ty cty.Type) (cty.Value, error) {
	if v.Type() == ctyValueType {
		if !v.CanInterface() {
			return cty.NullVal(ty), nil
		}
		return v.Interface().(cty.Value), nil
	}

	if !v.IsValid() {
		return cty.NullVal(ty), nil
	}

	switch v.Type().Kind() {
	case reflect.Ptr, reflect.Interface:
		if v.IsNil() {
			return cty.NullVal(ty), nil
		}
		return reflectToCty(v.Elem(), ty)
	case reflect.Bool:
		if ty != cty.Bool {
			return cty.NilVal, fmt.Errorf("can't convert Go bool to %v", ty.FriendlyName())
		}
		return cty.BoolVal(v.Bool()), nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if ty != cty.Number {
			return cty.NilVal, fmt.Errorf("can't convert Go int to %v", ty.FriendlyName())
		}
		return cty.NumberIntVal(v.Int()), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		if ty != cty.Number {
			return cty.NilVal, fmt.Errorf("can't convert Go uint to %v", ty.FriendlyName())
		}
		return cty.NumberUIntVal(v.Uint()), nil
	case reflect.Float32, reflect.Float64:
		if ty != cty.Number {
			return cty.NilVal, fmt.Errorf("can't convert Go float to %v", ty.FriendlyName())
		}
		return cty.NumberFloatVal(v.Float()), nil
	case reflect.String:
		s := v.String()
		if s == UnknownVariableValue {
			return cty.UnknownVal(ty), nil
		}
		if ty != cty.String {
			return cty.NilVal, fmt.Errorf("can't convert Go string to %v", ty.FriendlyName())
		}
		return cty.StringVal(s), nil
	case reflect.Slice, reflect.Array:
		switch {
		case ty.IsListType():
			if v.Len() == 0 {
				return cty.ListValEmpty(ty.ElementType()), nil
			}

			values := make([]cty.Value, v.Len())
			for i := 0; i < len(values); i++ {
				val, err := reflectToCty(v.Index(i), ty.ElementType())
				if err != nil {
					return cty.NilVal, err
				}
				values[i] = val
			}
			return cty.ListVal(values), nil
		case ty.IsTupleType():
			if v.Len() != ty.Length() {
				return cty.NilVal, fmt.Errorf("can't convert Go slice to %v", ty.FriendlyName())
			}

			values := make([]cty.Value, v.Len())
			for i := 0; i < len(values); i++ {
				val, err := reflectToCty(v.Index(i), ty.TupleElementType(i))
				if err != nil {
					return cty.NilVal, err
				}
				values[i] = val
			}
			return cty.TupleVal(values), nil
		case ty.IsSetType():
			if v.Len() == 0 {
				return cty.SetValEmpty(ty.ElementType()), nil
			}

			values := make([]cty.Value, v.Len())
			for i := 0; i < len(values); i++ {
				val, err := reflectToCty(v.Index(i), ty.ElementType())
				if err != nil {
					return cty.NilVal, err
				}

				// Sets cannot be partially-known: if any element is unknown, the entire set is unknown.
				if !val.IsKnown() {
					return cty.UnknownVal(ty), nil
				}
				values[i] = val
			}
			return cty.SetVal(values), nil
		default:
			return cty.NilVal, fmt.Errorf("can't convert Go slice to %v", ty.FriendlyName())
		}
	case reflect.Map:
		if v.Type().Key().Kind() != reflect.String {
			return cty.NilVal, fmt.Errorf("can't convert Go map with keys that are not strings")
		}

		switch {
		case ty.IsMapType():
			if v.Len() == 0 {
				return cty.MapValEmpty(ty.ElementType()), nil
			}

			values := map[string]cty.Value{}
			iter := v.MapRange()
			for iter.Next() {
				k, v := iter.Key().String(), iter.Value()
				if k == UnknownVariableValue {
					return cty.NilVal, fmt.Errorf("can't convert Go map with unknown keys")
				}

				val, err := reflectToCty(v, ty.ElementType())
				if err != nil {
					return cty.NilVal, err
				}
				values[k] = val
			}
			return cty.MapVal(values), nil
		case ty.IsObjectType():
			values := map[string]cty.Value{}

			iter := v.MapRange()
			for iter.Next() {
				k, v := iter.Key().String(), iter.Value()
				if k == UnknownVariableValue {
					return cty.NilVal, fmt.Errorf("can't convert Go map with unknown keys")
				}

				if ty.HasAttribute(k) {
					val, err := reflectToCty(v, ty.AttributeType(k))
					if err != nil {
						return cty.NilVal, err
					}
					values[k] = val
				}
			}

			for k, ty := range ty.AttributeTypes() {
				if _, ok := values[k]; !ok {
					values[k] = cty.NullVal(ty)
				}
			}

			return cty.ObjectVal(values), nil
		default:
			return cty.NilVal, fmt.Errorf("can't convert Go map to %v", ty.FriendlyName())
		}
	default:
		return cty.NilVal, fmt.Errorf("unsupported Go value of type %v", v.Type())
	}
}
