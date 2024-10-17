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

package provider

import (
	"cmp"
	"fmt"
	"hash/maphash"
	"slices"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/zclconf/go-cty/cty"

	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

func (g generator) valueID(v value) uint64 {
	v = coelesceNulls(v)
	h := maphash.Hash{}
	h.SetSeed(g.seed)
	_, err := h.WriteString(v.tf.GoString())
	contract.AssertNoErrorf(err, "maphash.Hash.WriteString does not error")
	_, err = h.WriteString(v.pu.String())
	contract.AssertNoErrorf(err, "maphash.Hash.WriteString does not error")
	return h.Sum64()
}

func coelesceNulls(v value) value {
	var coelescePu func(v resource.PropertyValue) resource.PropertyValue
	coelescePu = func(v resource.PropertyValue) resource.PropertyValue {
		switch {
		case v.IsObject():
			src := v.ObjectValue()
			dst := make(resource.PropertyMap, len(src))
			elementsAreNull := true
			for i, e := range src {
				e = coelescePu(e)
				elementsAreNull = elementsAreNull && e.IsNull()
				dst[i] = e
			}
			if elementsAreNull {
				return resource.NewNullProperty()
			}
			return resource.NewProperty(dst)
		case v.IsArray():
			src := v.ArrayValue()
			dst := make([]resource.PropertyValue, len(src))
			elementsAreNull := true
			for i, e := range src {
				e = coelescePu(e)
				elementsAreNull = elementsAreNull && e.IsNull()
				dst[i] = e
			}
			if elementsAreNull {
				return resource.NewNullProperty()
			}
			return resource.NewProperty(dst)
		case v.IsSecret():
			return resource.MakeSecret(coelescePu(v.SecretValue().Element))
		default:
			return v
		}
	}

	var coelesceTf func(v cty.Value) cty.Value
	coelesceTf = func(v cty.Value) cty.Value {
		if v.IsNull() || !v.IsKnown() {
			return v
		}
		typ := v.Type()

		switch {
		case typ.IsListType():
			arr := v.AsValueSlice()
			elementsAreNull := true
			for i, e := range arr {
				arr[i] = coelesceTf(e)
				elementsAreNull = elementsAreNull && arr[i].IsNull()
			}
			if elementsAreNull {
				return cty.NullVal(typ)
			}
			return cty.ListVal(arr)
		case typ.IsSetType():
			arr := v.AsValueSlice()
			elementsAreNull := true
			for i, e := range arr {
				arr[i] = coelesceTf(e)
				elementsAreNull = elementsAreNull && arr[i].IsNull()
			}
			if elementsAreNull {
				return cty.NullVal(typ)
			}
			return cty.SetVal(arr)
		case typ.IsMapType():
			m := v.AsValueMap()
			elementsAreNull := true
			for k, e := range m {
				m[k] = coelesceTf(e)
				elementsAreNull = elementsAreNull && m[k].IsNull()
			}
			if elementsAreNull {
				return cty.NullVal(typ)
			}
			return cty.MapVal(m)
		case typ.IsObjectType():
			m := v.AsValueMap()
			elementsAreNull := true
			for k, e := range m {
				m[k] = coelesceTf(e)
				elementsAreNull = elementsAreNull && m[k].IsNull()
			}
			if elementsAreNull {
				return cty.NullVal(typ)
			}
			return cty.ObjectVal(m)
		default:
			return v
		}
	}

	return value{
		tf:       coelesceTf(v.tf),
		pu:       coelescePu(v.pu),
		hasValue: v.hasValue,
	}
}

func shimType(t tftypes.Type) shim.ValueType {
	switch {
	case t.Is(tftypes.Set{}):
		return shim.TypeSet
	case t.Is(tftypes.Map{}):
		return shim.TypeMap
	case t.Is(tftypes.List{}):
		return shim.TypeList
	default:
		// shimType is only used when interacting with
		// [tfbridge.TerraformToPulumiNameV2], so other scalar types don't matter
		// here.
		return shim.TypeBool
	}
}

func makeConvertMap(elem cty.Type) func(map[string]value) value {
	return func(m map[string]value) value { return convertMap(m, elem) }
}

func convertMap(m map[string]value, elem cty.Type) value {
	tfMap, puMap := make(map[string]cty.Value, len(m)), make(resource.PropertyMap, len(m))
	for k, v := range m {
		tfMap[k] = v.tf
		if v.hasValue {
			puMap[resource.PropertyKey(k)] = v.pu
		}
	}

	ctyMap := cty.MapValEmpty(elem)
	if len(tfMap) > 0 {
		ctyMap = cty.MapVal(tfMap)
	}

	return value{
		hasValue: true,
		tf:       ctyMap,
		pu:       resource.NewProperty(puMap),
	}
}

func convertObject(m map[string]value, names map[string]resource.PropertyKey) value {
	tfMap, puMap := make(map[string]cty.Value, len(m)), make(resource.PropertyMap, len(m))
	for k, v := range m {
		tfMap[k] = v.tf
		if v.hasValue {
			puMap[names[k]] = v.pu
		}
	}
	return value{
		hasValue: true,
		tf:       cty.ObjectVal(tfMap),
		pu:       resource.NewProperty(puMap),
	}
}

func makeConvertSet(elem cty.Type) func([]value) value {
	return func(a []value) value { return convertSet(a, elem) }
}

func convertSet(a []value, elem cty.Type) value {
	tfArr, puArr := make([]cty.Value, len(a)), make([]resource.PropertyValue, len(a))
	for i, v := range a {
		tfArr[i] = v.tf
		puArr[i] = v.pu
	}

	tfSet := cty.NullVal(cty.Set(elem))
	if len(tfArr) > 0 {
		tfSet = cty.SetVal(tfArr)
		contract.Assertf(tfSet.LengthInt() == len(puArr),
			"Set values have different lengths: (%d != %d), of %#v", tfSet.LengthInt(), len(puArr), a)
	}
	puSet := resource.NewNullProperty()
	if len(puArr) > 0 {
		puSet = resource.NewProperty(puArr)
	}

	return value{
		hasValue: true,
		tf:       tfSet,
		pu:       puSet,
	}
}

func makeConvertList(elem cty.Type) func([]value) value {
	return func(a []value) value { return convertList(a, elem) }
}

func convertList(a []value, elem cty.Type) value {
	tfArr, puArr := make([]cty.Value, len(a)), make([]resource.PropertyValue, len(a))
	for i, v := range a {
		tfArr[i] = v.tf
		puArr[i] = v.pu
	}

	tfList := cty.NullVal(cty.List(elem))
	if len(tfArr) > 0 {
		tfList = cty.ListVal(tfArr)
	}
	puList := resource.NewNullProperty()
	if len(puArr) > 0 {
		puList = resource.NewProperty(puArr)
	}

	return value{
		hasValue: true,
		tf:       tfList,
		pu:       puList,
	}
}

func ctyType(typ tftypes.Type) cty.Type {
	switch {
	case typ.Is(tftypes.Bool):
		return cty.Bool
	case typ.Is(tftypes.Number):
		return cty.Number
	case typ.Is(tftypes.String):
		return cty.String
	case typ.Is(tftypes.Map{}):
		return cty.Map(ctyType(typ.(tftypes.Map).ElementType))
	case typ.Is(tftypes.List{}):
		return cty.List(ctyType(typ.(tftypes.List).ElementType))
	case typ.Is(tftypes.Set{}):
		return cty.Set(ctyType(typ.(tftypes.Set).ElementType))
	case typ.Is(tftypes.Object{}):
		o := typ.(tftypes.Object)
		attrs := make(map[string]cty.Type, len(o.AttributeTypes))
		optionals := make([]string, 0, len(o.OptionalAttributes))

		for k, v := range o.AttributeTypes {
			attrs[k] = ctyType(v)
		}
		for k := range o.OptionalAttributes {
			optionals = append(optionals, k)
		}

		slices.Sort(optionals)
		return cty.ObjectWithOptionalAttrs(attrs, optionals)
	default:
		panic(fmt.Sprintf("Unknown tftypes.Type: %v", typ))
	}

}

func stableIter[K cmp.Ordered, V any](m map[K]V, f func(k K, v V)) {
	order := make([]K, 0, len(m))
	for k := range m {
		order = append(order, k)
	}
	slices.Sort(order)

	for _, k := range order {
		f(k, m[k])
	}
}
