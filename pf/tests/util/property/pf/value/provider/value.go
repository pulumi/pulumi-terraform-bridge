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
			if v := g.withAttr(s).Draw(t, k); v.hasValue {
				ctyM[k] = v.Tf
				puM[resource.PropertyKey(k)] = v.Pu
			}
		}

		for k, b := range schema.Blocks {
			if v := g.withBlock(b).Draw(t, k); v.hasValue {
				ctyM[k] = v.Tf
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

	return rapid.Custom(func(t *rapid.T) value {
		// optional => we may set the value
		if attr.IsOptional() && rapid.Bool().Draw(t, "drop optional") {
			return value{hasValue: false}
		}

		// At this point, we have decided to return a value for attr.
		switch attr := attr.(type) {

		// Primitive attributes

		case schema.StringAttribute:
			return stringVal(t)
		case schema.Int32Attribute:
			i := rapid.Int32().Draw(t, "v")
			return value{
				hasValue: true,
				Tf:       cty.NumberIntVal(int64(i)),
				Pu:       resource.NewProperty(float64(i)),
			}
		case schema.Int64Attribute:
			i := rapid.Int64().Draw(t, "v")
			return value{
				hasValue: true,
				Tf:       cty.NumberIntVal(i),
				Pu:       resource.NewProperty(float64(i)),
			}
		case schema.Float32Attribute:
			f := rapid.Float32().Draw(t, "v")
			return value{
				hasValue: true,
				Tf:       cty.NumberFloatVal(float64(f)),
				Pu:       resource.NewProperty(float64(f)),
			}
		case schema.Float64Attribute:
			f := rapid.Float64().Draw(t, "v")
			return value{
				hasValue: true,
				Tf:       cty.NumberFloatVal(f),
				Pu:       resource.NewProperty(f),
			}

		case schema.NumberAttribute:
			return numberVal(t)

		case schema.BoolAttribute:
			return boolVal(t)

		// Complex attributes

		case schema.MapAttribute:
			elemType := attr.ElementType.TerraformType(ctx)
			return rapid.Map(
				rapid.MapOf(rapid.String(), baseAttr(elemType)),
				makeConvertMap(ctyType(elemType)),
			).Draw(t, "v")

		case schema.ListAttribute:
			elemType := attr.ElementType.TerraformType(ctx)
			return rapid.Map(
				rapid.SliceOf(baseAttr(elemType)),
				makeConvertList(ctyType(elemType)),
			).Draw(t, "v")
		case schema.SetAttribute:
			elemType := attr.ElementType.TerraformType(ctx)
			return rapid.Map(
				rapid.SliceOfDistinct(baseAttr(elemType), valueID),
				makeConvertList(ctyType(elemType)),
			).Draw(t, "v")
		case schema.ObjectAttribute:
			m := make(map[string]value, len(attr.AttributeTypes))
			for k, a := range attr.AttributeTypes {
				m[k] = baseAttr(a.TerraformType(ctx)).Draw(t, k)
			}
			return convertObject(m)

		// Nested attributes

		case schema.MapNestedAttribute:
			return rapid.Map(
				rapid.MapOf(rapid.String(), g.nestedObject(attr.NestedObject)),
				makeConvertMap(ctyType(attr.NestedObject.Type().TerraformType(ctx))),
			).Draw(t, "v")
		case schema.ListNestedAttribute:
			return rapid.Map(
				rapid.SliceOf(g.nestedObject(attr.NestedObject)),
				makeConvertList(ctyType(attr.NestedObject.Type().TerraformType(ctx))),
			).Draw(t, "v")
		case schema.SetNestedAttribute:
			return rapid.Map(
				rapid.SliceOfDistinct(g.nestedObject(attr.NestedObject), valueID),
				makeConvertList(ctyType(attr.NestedObject.Type().TerraformType(ctx))),
			).Draw(t, "v")
		default:
			panic(fmt.Sprintf("Unknown schema.Attribute type %T", attr))
		}
	})
}

func baseAttr(typ tftypes.Type) *rapid.Generator[value] {

	// Objects might be empty, which would draw no entropy.
	//
	// We need to handle this outside of the rapid.Custom so all calls to rapid.Custom
	// draw entropy.
	if typ.Is(tftypes.Object{}) {
		o := typ.(tftypes.Object)
		m := make(map[string]value, len(o.AttributeTypes))
		if len(m) > 0 {
			return rapid.Just(convertObject(m))
		}
		return rapid.Custom(func(t *rapid.T) value {
			for k, a := range o.AttributeTypes {
				if _, isOptional := o.OptionalAttributes[k]; isOptional &&
					rapid.Bool().Draw(t, "ignore") {
					continue
				}
				m[k] = baseAttr(a).Draw(t, k)
			}
			return convertObject(m)
		})
	}

	return rapid.Custom(func(t *rapid.T) value {
		switch {
		case typ.Is(tftypes.Bool):
			return boolVal(t)
		case typ.Is(tftypes.Number):
			return numberVal(t)
		case typ.Is(tftypes.String):
			return stringVal(t)
		case typ.Is(tftypes.Map{}):
			elemType := typ.(tftypes.Map).ElementType
			return rapid.Map(
				rapid.MapOf(rapid.String(), baseAttr(elemType)),
				makeConvertMap(ctyType(elemType)),
			).Draw(t, "v")
		case typ.Is(tftypes.List{}):
			elemType := typ.(tftypes.List).ElementType
			return rapid.Map(
				rapid.SliceOf(baseAttr(elemType)),
				makeConvertList(ctyType(elemType)),
			).Draw(t, "v")
		case typ.Is(tftypes.Set{}):
			elemType := typ.(tftypes.Set).ElementType
			return rapid.Map(
				rapid.SliceOfDistinct(baseAttr(elemType), valueID),
				makeConvertList(ctyType(elemType)),
			).Draw(t, "v")
		default:
			panic(fmt.Sprintf("Unknown tftypes.Type: %v", typ))
		}
	})
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
