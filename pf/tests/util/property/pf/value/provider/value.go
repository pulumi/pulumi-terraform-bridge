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
	"context"
	"fmt"
	"hash/maphash"

	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/zclconf/go-cty/cty"
	"pgregory.net/rapid"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// Value represents a single conceptual value, represented in both a [cty.Value] and in a
// [resource.PropertyValue].
type Value struct {
	Tf cty.Value
	Pu resource.PropertyMap
}

type value struct {
	Tf       cty.Value
	Pu       resource.PropertyValue
	hasValue bool
}

type generator struct {
	// isInput means that the generator is generating input values, not output values.
	isInput bool
}

func WithValue(schema schema.Schema) *rapid.Generator[Value] {

	// All rapid.Custom generators *must* consume entropy, so we special case empty
	// schemas.
	if len(schema.Attributes)+len(schema.Blocks) == 0 {
		return rapid.Just(Value{
			Tf: cty.EmptyObjectVal,
			Pu: resource.PropertyMap{},
		})
	}
	return rapid.Custom(func(t *rapid.T) Value {
		g := generator{isInput: true}
		ctyM, puM := map[string]cty.Value{}, resource.PropertyMap{}

		for k, s := range schema.Attributes {
			v := g.withAttr(s).Draw(t, k)
			ctyM[k] = v.Tf
			if v.hasValue {
				puM[resource.PropertyKey(k)] = v.Pu
			}
		}

		for k, b := range schema.Blocks {
			v := g.withBlock(b).Draw(t, k)
			ctyM[k] = v.Tf
			if v.hasValue {
				puM[resource.PropertyKey(k)] = v.Pu
			}
		}

		return Value{
			Tf: cty.ObjectVal(ctyM),
			Pu: puM,
		}
	})
}

func (g generator) withAttr(attr schema.Attribute) *rapid.Generator[value] {
	contract.Assertf(g.isInput, "only input values are implemented")

	ctx := context.Background()

	// computed & !optional => the provider will set the value
	if attr.IsComputed() && !attr.IsOptional() {
		return rapid.Just(value{hasValue: false})
	}

	makeOptional := func(v *rapid.Generator[value]) *rapid.Generator[value] {
		if attr.IsRequired() {
			return v
		}

		return rapid.Custom(func(t *rapid.T) value {
			// optional => we may set the value
			v := v.Draw(t, "v")
			if attr.IsOptional() && rapid.Bool().Draw(t, "drop optional") {
				return value{hasValue: false, Tf: cty.NullVal(v.Tf.Type())}
			}

			return v
		})
	}

	getValue := func() *rapid.Generator[value] {
		switch attr := attr.(type) {

		// Primitive attributes

		case schema.StringAttribute:
			return rapid.Custom(stringVal)
		case schema.NumberAttribute:
			return rapid.Custom(numberVal)
		case schema.BoolAttribute:
			return rapid.Custom(boolVal)

		case schema.Int32Attribute:
			return rapid.Custom(func(t *rapid.T) value {
				i := rapid.Int32().Draw(t, "v")
				return value{
					hasValue: true,
					Tf:       cty.NumberIntVal(int64(i)),
					Pu:       resource.NewProperty(float64(i)),
				}
			})
		case schema.Int64Attribute:
			return rapid.Custom(func(t *rapid.T) value {
				i := rapid.Int64().Draw(t, "v")
				return value{
					hasValue: true,
					Tf:       cty.NumberIntVal(i),
					Pu:       resource.NewProperty(float64(i)),
				}
			})
		case schema.Float32Attribute:
			return rapid.Custom(func(t *rapid.T) value {
				f := rapid.Float32().Draw(t, "v")
				return value{
					hasValue: true,
					Tf:       cty.NumberFloatVal(float64(f)),
					Pu:       resource.NewProperty(float64(f)),
				}
			})
		case schema.Float64Attribute:
			return rapid.Custom(func(t *rapid.T) value {
				f := rapid.Float64().Draw(t, "v")
				return value{
					hasValue: true,
					Tf:       cty.NumberFloatVal(f),
					Pu:       resource.NewProperty(f),
				}
			})

		// Complex attributes

		case schema.MapAttribute:
			elemType := attr.ElementType.TerraformType(ctx)
			return rapid.Map(
				rapid.MapOf(rapid.String(), baseAttr(elemType)),
				makeConvertMap(ctyType(elemType)),
			)

		case schema.ListAttribute:
			elemType := attr.ElementType.TerraformType(ctx)
			return rapid.Map(
				rapid.SliceOf(baseAttr(elemType)),
				makeConvertList(ctyType(elemType)),
			)
		case schema.SetAttribute:
			elemType := attr.ElementType.TerraformType(ctx)
			return rapid.Map(
				rapid.SliceOfDistinct(baseAttr(elemType), valueID),
				makeConvertList(ctyType(elemType)),
			)

		case schema.ObjectAttribute:
			m := make(map[string]value, len(attr.AttributeTypes))
			if len(attr.AttributeTypes) == 0 {
				return rapid.Just(convertObject(m))
			}
			return rapid.Custom(func(t *rapid.T) value {
				for k, a := range attr.AttributeTypes {
					m[k] = baseAttr(a.TerraformType(ctx)).Draw(t, k)
				}
				return convertObject(m)
			})

		// Nested attributes
		case schema.SingleNestedAttribute:
			m := make(map[string]value, len(attr.Attributes))
			if len(attr.Attributes) == 0 {
				return rapid.Just(convertObject(m))
			}
			return rapid.Custom(func(t *rapid.T) value {
				for k, a := range attr.Attributes {
					m[k] = g.withAttr(a).Draw(t, k)
				}
				return convertObject(m)
			})
		case schema.MapNestedAttribute:
			return rapid.Map(
				rapid.MapOf(rapid.String(), g.nestedObject(attr.NestedObject)),
				makeConvertMap(ctyType(attr.NestedObject.Type().TerraformType(ctx))),
			)
		case schema.ListNestedAttribute:
			return rapid.Map(
				rapid.SliceOf(g.nestedObject(attr.NestedObject)),
				makeConvertList(ctyType(attr.NestedObject.Type().TerraformType(ctx))),
			)
		case schema.SetNestedAttribute:
			return rapid.Map(
				rapid.SliceOfDistinct(g.nestedObject(attr.NestedObject), valueID),
				makeConvertList(ctyType(attr.NestedObject.Type().TerraformType(ctx))),
			)
		default:
			panic(fmt.Sprintf("Unknown schema.Attribute type %T", attr))
		}
	}

	return makeOptional(getValue())
}

func baseAttr(typ tftypes.Type) *rapid.Generator[value] {
	switch {
	case typ.Is(tftypes.Object{}):
		o := typ.(tftypes.Object)
		if len(o.AttributeTypes) == 0 {
			return rapid.Just(value{
				Tf:       cty.EmptyObjectVal,
				hasValue: false,
			})
		}
		return rapid.Custom(func(t *rapid.T) value {
			m := make(map[string]value, len(o.AttributeTypes))
			for k, a := range o.AttributeTypes {
				if _, isOptional := o.OptionalAttributes[k]; isOptional &&
					rapid.Bool().Draw(t, "ignore") {
					m[k] = value{Tf: cty.NullVal(ctyType(a))}
				} else {
					m[k] = baseAttr(a).Draw(t, k)
				}
			}
			return convertObject(m)
		})
	case typ.Is(tftypes.Bool):
		return rapid.Custom(boolVal)
	case typ.Is(tftypes.Number):
		return rapid.Custom(numberVal)
	case typ.Is(tftypes.String):
		return rapid.Custom(stringVal)
	case typ.Is(tftypes.Map{}):
		elemType := typ.(tftypes.Map).ElementType
		return rapid.Map(
			rapid.MapOf(rapid.String(), baseAttr(elemType)),
			makeConvertMap(ctyType(elemType)),
		)
	case typ.Is(tftypes.List{}):
		elemType := typ.(tftypes.List).ElementType
		return rapid.Map(
			rapid.SliceOf(baseAttr(elemType)),
			makeConvertList(ctyType(elemType)),
		)
	case typ.Is(tftypes.Set{}):
		elemType := typ.(tftypes.Set).ElementType
		return rapid.Map(
			rapid.SliceOfDistinct(baseAttr(elemType), valueID),
			makeConvertList(ctyType(elemType)),
		)
	default:
		panic(fmt.Sprintf("Unknown tftypes.Type: %v", typ))
	}
}

func (g generator) nestedObject(obj schema.NestedAttributeObject) *rapid.Generator[value] {
	if len(obj.Attributes) == 0 {
		m := make(map[string]value, len(obj.Attributes))
		return rapid.Just(convertObject(m))
	}
	return rapid.Custom(func(t *rapid.T) value {
		m := make(map[string]value, len(obj.Attributes))
		for k, a := range obj.Attributes {
			if v := g.withAttr(a).Draw(t, k); v.hasValue {
				m[k] = v
			}
		}
		return convertObject(m)
	})
}

func (g generator) withBlock(block schema.Block) *rapid.Generator[value] {
	contract.Assertf(g.isInput, "only input values are implemented")
	return baseAttr(block.Type().TerraformType(context.Background()))
}

func valueID(v value) uint64 {
	h := maphash.Hash{}
	h.WriteString(v.Tf.GoString())
	h.WriteString(v.Pu.String())
	return h.Sum64()
}
