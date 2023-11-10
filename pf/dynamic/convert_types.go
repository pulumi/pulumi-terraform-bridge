package dynamic

import (
	"fmt"
	"math/big"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/zclconf/go-cty/cty"
)

func marshalToDV(v *cty.Value) (*tfprotov6.DynamicValue, error) {
	tfVal, err := tfValueFromCtyValue(*v)
	if err != nil {
		return nil, err
	}

	dv, err := tfprotov6.NewDynamicValue(tfVal.Type(), *tfVal)
	if err != nil {
		return nil, err
	}
	return &dv, nil
}

func unmarshalFromDV(dv *tfprotov6.DynamicValue, ty cty.Type) (*cty.Value, error) {
	tfTy := tftypeFromCtyType(ty)
	tfVal, err := dv.Unmarshal(tfTy)
	if err != nil {
		return nil, err
	}
	val := ctyValueFromTfValue(tfVal)
	return val, nil
}

func tftypeFromCtyType(in cty.Type) tftypes.Type {
	switch {
	case in.Equals(cty.String):
		return tftypes.String

	case in.Equals(cty.Number):
		return tftypes.Number

	case in.Equals(cty.Bool):
		return tftypes.Bool

	case in.IsSetType():
		elemType := tftypeFromCtyType(in.ElementType())
		return tftypes.Set{ElementType: elemType}

	case in.IsListType():
		elemType := tftypeFromCtyType(in.ElementType())
		return tftypes.List{ElementType: elemType}

	case in.IsTupleType():
		elemTypes := make([]tftypes.Type, 0, in.Length())
		for _, typ := range in.TupleElementTypes() {
			elemType := tftypeFromCtyType(typ)
			elemTypes = append(elemTypes, elemType)
		}
		return tftypes.Tuple{ElementTypes: elemTypes}

	case in.IsMapType():
		elemType := tftypeFromCtyType(in.ElementType())
		return tftypes.Map{ElementType: elemType}

	case in.IsObjectType():
		attrTypes := make(map[string]tftypes.Type)
		for key, typ := range in.AttributeTypes() {
			attrType := tftypeFromCtyType(typ)
			attrTypes[key] = attrType
		}
		return tftypes.Object{
			AttributeTypes: attrTypes,
		}
	}
	return tftypes.DynamicPseudoType
}

func ctyTypeFromTFType(in tftypes.Type) (cty.Type, error) {
	switch {
	case in.Is(tftypes.String):
		return cty.String, nil
	case in.Is(tftypes.Bool):
		return cty.Bool, nil
	case in.Is(tftypes.Number):
		return cty.Number, nil
	case in.Is(tftypes.DynamicPseudoType):
		return cty.DynamicPseudoType, nil
	case in.Is(tftypes.List{}):
		elemType, err := ctyTypeFromTFType(in.(tftypes.List).ElementType)
		if err != nil {
			return cty.Type{}, err
		}
		return cty.List(elemType), nil
	case in.Is(tftypes.Set{}):
		elemType, err := ctyTypeFromTFType(in.(tftypes.Set).ElementType)
		if err != nil {
			return cty.Type{}, err
		}
		return cty.Set(elemType), nil
	case in.Is(tftypes.Map{}):
		elemType, err := ctyTypeFromTFType(in.(tftypes.Map).ElementType)
		if err != nil {
			return cty.Type{}, err
		}
		return cty.Map(elemType), nil
	case in.Is(tftypes.Tuple{}):
		elemTypes := make([]cty.Type, 0, len(in.(tftypes.Tuple).ElementTypes))
		for _, typ := range in.(tftypes.Tuple).ElementTypes {
			elemType, err := ctyTypeFromTFType(typ)
			if err != nil {
				return cty.Type{}, err
			}
			elemTypes = append(elemTypes, elemType)
		}
		return cty.Tuple(elemTypes), nil
	case in.Is(tftypes.Object{}):
		attrTypes := make(map[string]cty.Type, len(in.(tftypes.Object).AttributeTypes))
		for k, v := range in.(tftypes.Object).AttributeTypes {
			attrType, err := ctyTypeFromTFType(v)
			if err != nil {
				return cty.Type{}, err
			}
			attrTypes[k] = attrType
		}
		return cty.Object(attrTypes), nil
	}
	return cty.Type{}, fmt.Errorf("unknown tftypes.Type %s", in)
}

func ctyValueFromTfValue(val tftypes.Value) *cty.Value {
	in := val.Type()

	switch {
	case in.Is(tftypes.String):
		var s string
		panicIfErr(val.As(&s))
		ss := cty.StringVal(s)
		return &ss

	case in.Is(tftypes.Bool):
		var b bool
		panicIfErr(val.As(&b))
		bb := cty.BoolVal(b)
		return &bb

	case in.Is(tftypes.Number):
		n := big.NewFloat(0)
		panicIfErr(val.As(n))
		nn := cty.NumberVal(n)
		return &nn

	case in.Is(tftypes.List{}):
		var l []tftypes.Value
		panicIfErr(val.As(&l))
		ll := make([]cty.Value, 0, len(l))
		for _, v := range l {
			ll = append(ll, *ctyValueFromTfValue(v))
		}
		lll := cty.ListVal(ll)
		return &lll

	case in.Is(tftypes.Set{}):
		var l []tftypes.Value
		panicIfErr(val.As(&l))
		ll := make([]cty.Value, 0, len(l))
		for _, v := range l {
			ll = append(ll, *ctyValueFromTfValue(v))
		}
		lll := cty.SetVal(ll)
		return &lll

	case in.Is(tftypes.Map{}):
		m := map[string]tftypes.Value{}
		panicIfErr(val.As(&m))
		mm := make(map[string]cty.Value, len(m))
		for k, v := range m {
			mm[k] = *ctyValueFromTfValue(v)
		}
		var mmm cty.Value
		if len(mm) > 0 {
			mmm = cty.MapVal(mm)
		} else {
			ty, _ := ctyTypeFromTFType(in.(tftypes.Map).ElementType)
			mmm = cty.MapValEmpty(ty)
		}
		return &mmm

	case in.Is(tftypes.Tuple{}):
		var l []tftypes.Value
		panicIfErr(val.As(&l))
		ll := make([]cty.Value, 0, len(l))
		for _, v := range l {
			ll = append(ll, *ctyValueFromTfValue(v))
		}
		lll := cty.TupleVal(ll)
		return &lll

	case in.Is(tftypes.Object{}):
		m := map[string]tftypes.Value{}
		panicIfErr(val.As(&m))
		mm := make(map[string]cty.Value, len(m))
		for k, v := range m {
			mm[k] = *ctyValueFromTfValue(v)
		}
		mmm := cty.ObjectVal(mm)
		return &mmm
	}
	// Inconvertable type
	return &cty.DynamicVal
}

func tfValueFromCtyValue(val cty.Value) (*tftypes.Value, error) {
	typ := val.Type()
	switch {
	case typ.Equals(cty.String):
		v := tftypes.NewValue(tftypes.String, val.AsString())
		return &v, nil
	case typ.Equals(cty.Number):
		v := tftypes.NewValue(tftypes.Number, val.AsBigFloat())
		return &v, nil
	case typ.Equals(cty.Bool):
		v := tftypes.NewValue(tftypes.Bool, val.True())
		return &v, nil
	case typ.IsSetType():
		vals := make([]tftypes.Value, 0)
		for it := val.ElementIterator(); it.Next(); {
			_, ev := it.Element()
			v, err := tfValueFromCtyValue(ev)
			if err != nil {
				return nil, err
			}
			vals = append(vals, *v)
		}
		t, err := tftypes.TypeFromElements(vals)
		if err != nil {
			return nil, err
		}
		v := tftypes.NewValue(tftypes.Set{
			ElementType: t,
		}, vals)
		return &v, nil
	case typ.IsListType():
		vals := make([]tftypes.Value, 0)
		for it := val.ElementIterator(); it.Next(); {
			_, ev := it.Element()
			v, err := tfValueFromCtyValue(ev)
			if err != nil {
				return nil, err
			}
			vals = append(vals, *v)
		}
		t, err := tftypes.TypeFromElements(vals)
		if err != nil {
			return nil, err
		}
		v := tftypes.NewValue(tftypes.List{
			ElementType: t,
		}, vals)
		return &v, nil
	case typ.IsTupleType():
		typs := make([]tftypes.Type, 0)
		vals := make([]tftypes.Value, 0)
		for it := val.ElementIterator(); it.Next(); {
			_, ev := it.Element()
			v, err := tfValueFromCtyValue(ev)
			if err != nil {
				return nil, err
			}
			typs = append(typs, v.Type())
			vals = append(vals, *v)
		}
		t := tftypes.Tuple{
			ElementTypes: typs,
		}
		v := tftypes.NewValue(t, vals)
		return &v, nil
	case typ.IsMapType():
		vals := map[string]tftypes.Value{}
		for it := val.ElementIterator(); it.Next(); {
			k, ev := it.Element()
			rawK := k.AsString()
			v, err := tfValueFromCtyValue(ev)
			if err != nil {
				return nil, err
			}
			vals[rawK] = *v
		}
		t := tftypes.Map{
			ElementType: tftypes.String,
		}
		v := tftypes.NewValue(t, vals)
		return &v, nil
	case typ.IsObjectType():
		typs := make(map[string]tftypes.Type)
		vals := make(map[string]tftypes.Value)
		for it := val.ElementIterator(); it.Next(); {
			k, ev := it.Element()
			rawK := k.AsString()
			v, err := tfValueFromCtyValue(ev)
			if err != nil {
				return nil, err
			}
			typs[rawK] = v.Type()
			vals[rawK] = *v
		}
		t := tftypes.Object{
			AttributeTypes: typs,
		}
		v := tftypes.NewValue(t, vals)
		return &v, nil
	default:
		return nil, fmt.Errorf("unknown cty type %s", typ.GoString())
	}
}

func pathsToAttributePaths(paths []cty.Path) []*tftypes.AttributePath {
	res := make([]*tftypes.AttributePath, 0, len(paths))
	for _, p := range paths {
		res = append(res, pathToAttributePath(p))
	}
	return res
}

func pathToAttributePath(path cty.Path) *tftypes.AttributePath {
	var steps []tftypes.AttributePathStep

	for _, step := range path {
		switch s := step.(type) {
		case cty.GetAttrStep:
			steps = append(steps,
				tftypes.AttributeName(s.Name),
			)
		case cty.IndexStep:
			ty := s.Key.Type()
			switch ty {
			case cty.Number:
				i, _ := s.Key.AsBigFloat().Int64()
				steps = append(steps,
					tftypes.ElementKeyInt(i),
				)
			case cty.String:
				steps = append(steps,
					tftypes.ElementKeyString(s.Key.AsString()),
				)
			}
		}
	}

	if len(steps) < 1 {
		return nil
	}
	return tftypes.NewAttributePathWithSteps(steps)
}

func panicIfErr(err error) {
	if err != nil {
		panic(err)
	}
}
