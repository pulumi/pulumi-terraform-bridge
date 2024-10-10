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
	"fmt"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/zclconf/go-cty/cty"
	"pgregory.net/rapid"
)

func boolVal(t *rapid.T) value {
	f := rapid.Bool().Draw(t, "v")
	return value{
		hasValue: true,
		Tf:       cty.BoolVal(f),
		Pu:       resource.NewProperty(f),
	}
}

func stringVal(t *rapid.T) value {
	s := rapid.String().Draw(t, "v")

	return value{
		hasValue: true,
		Tf:       cty.StringVal(s),
		Pu:       resource.NewProperty(s),
	}

}

func numberVal(t *rapid.T) value {
	f := rapid.Float64().Draw(t, "v")
	return value{
		hasValue: true,
		Tf:       cty.NumberFloatVal(f),
		Pu:       resource.NewProperty(f),
	}
}

func makeConvertMap(elem cty.Type) func(map[string]value) value {
	return func(m map[string]value) value { return convertMap(m, elem) }
}

func convertMap(m map[string]value, elem cty.Type) value {
	tfMap, puMap := make(map[string]cty.Value, len(m)), make(resource.PropertyMap, len(m))
	for k, v := range m {
		tfMap[k] = v.Tf
		if v.hasValue {
			puMap[resource.PropertyKey(k)] = v.Pu
		}
	}

	ctyMap := cty.MapValEmpty(elem)
	if len(tfMap) > 0 {
		ctyMap = cty.MapVal(tfMap)
	}

	return value{
		hasValue: true,
		Tf:       ctyMap,
		Pu:       resource.NewProperty(puMap),
	}
}

func convertObject(m map[string]value) value {
	tfMap, puMap := make(map[string]cty.Value, len(m)), make(resource.PropertyMap, len(m))
	for k, v := range m {
		tfMap[k] = v.Tf
		if v.hasValue {
			// TODO: Correctly handle name conversion
			puMap[resource.PropertyKey(tfbridge.TerraformToPulumiNameV2(k, nil, nil))] = v.Pu
		}
	}
	return value{
		hasValue: true,
		Tf:       cty.ObjectVal(tfMap),
		Pu:       resource.NewProperty(puMap),
	}
}

func makeConvertList(elem cty.Type) func([]value) value {
	return func(a []value) value { return convertList(a, elem) }
}

func convertList(a []value, elem cty.Type) value {
	tfArr, puArr := make([]cty.Value, len(a)), make([]resource.PropertyValue, len(a))
	for i, v := range a {
		tfArr[i] = v.Tf
		puArr[i] = v.Pu
	}

	ctyList := cty.ListValEmpty(elem)
	if len(tfArr) > 0 {
		ctyList = cty.ListVal(tfArr)
	}

	return value{
		hasValue: true,
		Tf:       ctyList,
		Pu:       resource.NewProperty(puArr),
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

		return cty.ObjectWithOptionalAttrs(attrs, optionals)
	default:
		panic(fmt.Sprintf("Unknown tftypes.Type: %v", typ))
	}

}
