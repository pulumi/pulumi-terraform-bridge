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
		return cty.List(FromType(t.(tftypes.List).ElementType).ToCty())
	case t.Is(tftypes.Set{}):
		return cty.Set(FromType(t.(tftypes.Set).ElementType).ToCty())
	case t.Is(tftypes.Map{}):
		return cty.Map(FromType(t.(tftypes.Map).ElementType).ToCty())
	case t.Is(tftypes.Object{}):
		fields := map[string]cty.Type{}
		for k, v := range t.(tftypes.Object).AttributeTypes {
			fields[k] = FromType(v).ToCty()
		}
		return cty.Object(fields)
	default:
		contract.Failf("unexpected type %v", t)
		return cty.NilType
	}
}

func FromType(t tftypes.Type) *typeAdapter {
	return &typeAdapter{t}
}

type valueAdapter struct {
	value *tftypes.Value
}

func (va *valueAdapter) ToCty() cty.Value {
	v := va.value
	t := v.Type()
	switch {
	case v.IsNull():
		return cty.NullVal(FromType(t).ToCty())
	case !v.IsKnown():
		return cty.UnknownVal(FromType(t).ToCty())
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
		var vals []*tftypes.Value
		err := v.As(&vals)
		contract.AssertNoErrorf(err, "unexpected error converting list")
		var outVals []cty.Value
		for i, el := range vals {
			outVals[i] = FromValue(el).ToCty()
		}
		return cty.ListVal(outVals)
	case t.Is(tftypes.Set{}):
		var vals []*tftypes.Value
		err := v.As(&vals)
		contract.AssertNoErrorf(err, "unexpected error converting set")
		var outVals []cty.Value
		for i, el := range vals {
			outVals[i] = FromValue(el).ToCty()
		}
		return cty.SetVal(outVals)
	case t.Is(tftypes.Map{}):
		var vals map[string]*tftypes.Value
		err := v.As(&vals)
		contract.AssertNoErrorf(err, "unexpected error converting map")
		outVals := make(map[string]cty.Value, len(vals))
		for k, el := range vals {
			outVals[k] = FromValue(el).ToCty()
		}
		return cty.MapVal(outVals)
	case t.Is(tftypes.Object{}):
		var vals map[string]*tftypes.Value
		err := v.As(&vals)
		contract.AssertNoErrorf(err, "unexpected error converting object")
		outVals := make(map[string]cty.Value, len(vals))
		for k, el := range vals {
			outVals[k] = FromValue(el).ToCty()
		}
		return cty.ObjectVal(outVals)

	default:
		contract.Failf("unexpected type %v", t)
		return cty.NilVal
	}
}

func FromValue(v *tftypes.Value) *valueAdapter {
	return &valueAdapter{v}
}
