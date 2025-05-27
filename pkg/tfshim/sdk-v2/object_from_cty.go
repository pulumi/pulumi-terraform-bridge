package sdkv2

import (
	"github.com/hashicorp/go-cty/cty"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

func objectFromCtyValue(val cty.Value) map[string]any {
	res := objectFromCtyValueInner(val)
	if res == nil {
		return nil
	}
	return res.(map[string]any)
}

func objectFromCtyValueInner(val cty.Value) any {
	contract.Assertf(!val.IsMarked(), "value has marks, so it cannot be serialized")
	if val.IsNull() {
		return nil
	}

	if !val.IsKnown() {
		return terraformUnknownVariableValue
	}

	switch {
	case val.Type().IsPrimitiveType():
		switch val.Type() {
		case cty.String:
			return val.AsString()
		case cty.Number:
			return val.AsBigFloat().Text('f', -1)
		case cty.Bool:
			return val.True()
		default:
			contract.Failf("unsupported primitive type: %s", val.Type().FriendlyName())
		}
	case val.Type().IsListType(), val.Type().IsSetType(), val.Type().IsTupleType():
		l := make([]interface{}, 0, val.LengthInt())
		it := val.ElementIterator()
		for it.Next() {
			_, ev := it.Element()
			elem := objectFromCtyValueInner(ev)
			l = append(l, elem)
		}
		return l
	case val.Type().IsObjectType(), val.Type().IsMapType():
		l := make(map[string]interface{})
		it := val.ElementIterator()
		for it.Next() {
			ek, ev := it.Element()
			cv := objectFromCtyValueInner(ev)
			l[ek.AsString()] = cv
		}
		return l
	}

	contract.Failf("unsupported type: %s", val.Type().FriendlyName())
	return nil
}
