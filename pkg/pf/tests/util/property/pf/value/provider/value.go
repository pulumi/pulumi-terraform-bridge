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

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/zclconf/go-cty/cty"
	"pgregory.net/rapid"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	shimschema "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/schema"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

const maxIterableSize = 4

// Value represents a single conceptual value, represented in both a [cty.Value] and in a
// [resource.PropertyValue].
type Value struct {
	Tf cty.Value
	Pu resource.PropertyMap
}

type value struct {
	tf       cty.Value
	pu       resource.PropertyValue
	hasValue bool
}

type generator struct {
	// isInput means that the generator is generating input values, not output values.
	isInput bool

	seed maphash.Seed
}

func WithValue(ctx context.Context, s schema.Schema) *rapid.Generator[Value] {
	g := generator{
		isInput: true,
		seed:    maphash.MakeSeed(),
	}

	return rapid.Map(
		g.nestedBlockObject(ctx, schema.NestedBlockObject{
			Attributes: s.Attributes,
			Blocks:     s.Blocks,
		}),
		func(v value) Value {
			var pu resource.PropertyMap
			if !v.pu.IsNull() {
				pu = v.pu.ObjectValue()
			}
			return Value{Tf: v.tf, Pu: pu}
		},
	)
}

func (g generator) withAttr(ctx context.Context, attribute schema.Attribute) *rapid.Generator[value] {
	contract.Assertf(g.isInput, "only input values are implemented")

	// computed & !optional => the provider will set the value
	if attribute.IsComputed() && !attribute.IsOptional() {
		return rapid.Just(value{hasValue: false})
	}

	makeOptional := func(v *rapid.Generator[value]) *rapid.Generator[value] {
		if attribute.IsRequired() {
			return v
		}

		return rapid.Custom(func(t *rapid.T) value {
			// optional => we may set the value
			contract.Assertf(attribute.IsOptional(), "optional or required must be set")
			if rapid.Bool().Draw(t, "drop optional") {
				return value{hasValue: false, tf: cty.NullVal(ctyType(attribute.GetType().TerraformType(ctx)))}
			}

			return v.Draw(t, "v")
		})
	}

	var minIterableSize int
	if attribute.IsRequired() {
		minIterableSize = 1
	}

	getValue := func() *rapid.Generator[value] {
		switch attribute := attribute.(type) {

		// Primitive attributes

		case schema.StringAttribute:
			return rapid.Custom(stringVal)
		case schema.NumberAttribute:
			return rapid.Custom(float64Val)
		case schema.BoolAttribute:
			return rapid.Custom(boolVal)
		case schema.Int32Attribute:
			return rapid.Custom(int32Val)
		case schema.Int64Attribute:
			return rapid.Custom(int64Val)
		case schema.Float32Attribute:
			return rapid.Custom(float32Val)
		case schema.Float64Attribute:
			return rapid.Custom(float64Val)

		// Complex attributes

		case schema.MapAttribute:
			elemType := attribute.ElementType
			return rapid.Map(
				rapid.MapOfN(
					stringV(), g.attrType(ctx, elemType),
					minIterableSize, maxIterableSize,
				),
				makeConvertMap(ctyType(elemType.TerraformType(ctx))),
			)

		case schema.ListAttribute:
			elemType := attribute.ElementType
			return rapid.Map(
				rapid.SliceOfN(g.attrType(ctx, elemType), minIterableSize, maxIterableSize),
				makeConvertList(ctyType(elemType.TerraformType(ctx))),
			)
		case schema.SetAttribute:
			elemType := attribute.ElementType
			return rapid.Map(
				rapid.SliceOfNDistinct(g.attrType(ctx, elemType), minIterableSize, maxIterableSize, g.valueID),
				makeConvertSet(ctyType(elemType.TerraformType(ctx))),
			)

		case schema.ObjectAttribute:
			if len(attribute.AttributeTypes) == 0 {
				return rapid.Just(value{
					tf: cty.EmptyObjectVal,
					pu: resource.NewProperty(resource.PropertyMap{}),
				})
			}
			names := translateAttrNames(ctx, attribute.AttributeTypes)
			return rapid.Custom(func(t *rapid.T) value {
				m := make(map[string]value, len(attribute.AttributeTypes))
				stableIter(attribute.AttributeTypes, func(k string, a attr.Type) {
					m[k] = g.attrType(ctx, a).Draw(t, k)
				})
				return convertObject(m, names)
			})

		// Nested attributes
		case schema.SingleNestedAttribute:
			return g.nestedAttributeObject(ctx, schema.NestedAttributeObject{
				Attributes: attribute.Attributes,
			})
		case schema.MapNestedAttribute:
			return rapid.Map(
				rapid.MapOfN(stringV(), g.nestedAttributeObject(ctx, attribute.NestedObject), minIterableSize, maxIterableSize),
				makeConvertMap(ctyType(attribute.NestedObject.Type().TerraformType(ctx))),
			)
		case schema.ListNestedAttribute:
			return rapid.Map(
				rapid.SliceOfN(g.nestedAttributeObject(ctx, attribute.NestedObject), minIterableSize, maxIterableSize),
				makeConvertList(ctyType(attribute.NestedObject.Type().TerraformType(ctx))),
			)
		case schema.SetNestedAttribute:
			return rapid.Map(
				rapid.SliceOfNDistinct(g.nestedAttributeObject(ctx, attribute.NestedObject), minIterableSize, maxIterableSize, g.valueID),
				makeConvertSet(ctyType(attribute.NestedObject.Type().TerraformType(ctx))),
			)
		default:
			panic(fmt.Sprintf("Unknown schema.Attribute type %T", attribute))
		}
	}

	return makeOptional(getValue())
}

func (g generator) attrType(ctx context.Context, typ attr.Type) *rapid.Generator[value] {
	switch typ := typ.(type) {

	// Primitive types

	case basetypes.Int64Typable:
		return rapid.Custom(int64Val)
	case basetypes.Int32Typable:
		return rapid.Custom(int32Val)
	case basetypes.BoolTypable:
		return rapid.Custom(boolVal)
	case basetypes.StringTypable:
		return rapid.Custom(stringVal)
	case basetypes.NumberTypable:
		return rapid.Custom(float64Val)
	case basetypes.Float64Typable:
		return rapid.Custom(float64Val)
	case basetypes.Float32Typable:
		return rapid.Custom(float32Val)

	// Collection types

	case basetypes.MapType:
		return rapid.Map(
			rapid.MapOfN(stringV(), g.attrType(ctx, typ.ElemType), -1, maxIterableSize),
			makeConvertMap(ctyType(typ.ElemType.TerraformType(ctx))),
		)
	case basetypes.SetType:
		return rapid.Map(
			rapid.SliceOfNDistinct(g.attrType(ctx, typ.ElemType), -1, maxIterableSize, g.valueID),
			makeConvertSet(ctyType(typ.ElemType.TerraformType(ctx))),
		)
	case basetypes.ListType:
		return rapid.Map(
			rapid.SliceOfN(g.attrType(ctx, typ.ElemType), -1, maxIterableSize),
			makeConvertList(ctyType(typ.ElemType.TerraformType(ctx))),
		)

	// Product types

	case basetypes.ObjectType:
		if len(typ.AttrTypes) == 0 {
			m := make(map[string]value, len(typ.AttrTypes))
			return rapid.Just(convertObject(m, nil))
		}

		names := translateAttrNames(ctx, typ.AttrTypes)
		return rapid.Custom(func(t *rapid.T) value {
			m := make(map[string]value, len(typ.AttrTypes))
			stableIter(typ.AttrTypes, func(k string, v attr.Type) {
				m[k] = g.attrType(ctx, v).Draw(t, k)
			})

			return convertObject(m, names)
		})
	default:
		panic(fmt.Sprintf("Unknown attr.Type type %T", typ))
	}
}

func (g generator) nestedAttributeObject(ctx context.Context, obj schema.NestedAttributeObject) *rapid.Generator[value] {
	if len(obj.Attributes) == 0 {
		return rapid.Just(convertObject(map[string]value{}, nil))
	}

	names := translateNames(ctx, obj.Attributes, nil)
	return rapid.Custom(func(t *rapid.T) value {
		m := make(map[string]value, len(obj.Attributes))
		stableIter(obj.Attributes, func(k string, a schema.Attribute) {
			m[k] = g.withAttr(ctx, a).Draw(t, k)
		})
		return convertObject(m, names)
	})
}

func (g generator) withBlock(ctx context.Context, block schema.Block) *rapid.Generator[value] {
	contract.Assertf(g.isInput, "only input values are implemented")

	switch block := block.(type) {
	case schema.ListNestedBlock:
		return rapid.Map(
			rapid.SliceOfN(g.nestedBlockObject(ctx, block.NestedObject), -1, maxIterableSize),
			makeConvertList(ctyType(block.NestedObject.Type().TerraformType(ctx))),
		)
	case schema.SetNestedBlock:
		return rapid.Map(
			rapid.SliceOfNDistinct(g.nestedBlockObject(ctx, block.NestedObject), -1, maxIterableSize, g.valueID),
			makeConvertSet(ctyType(block.NestedObject.Type().TerraformType(ctx))),
		)
	case schema.SingleNestedBlock:
		return g.nestedBlockObject(ctx, schema.NestedBlockObject{
			Attributes: block.Attributes,
			Blocks:     block.Blocks,
		})
	default:
		panic(fmt.Sprintf("Unknown schema.Block type: %T", block))
	}
}

func (g generator) nestedBlockObject(ctx context.Context, obj schema.NestedBlockObject) *rapid.Generator[value] {

	// All rapid.Custom generators *must* consume entropy, so we special case empty
	// schemas.
	if len(obj.Attributes)+len(obj.Blocks) == 0 {
		return rapid.Just(value{
			tf: cty.EmptyObjectVal,
			pu: resource.NewNullProperty(),
		})
	}

	names := translateNames(ctx, obj.Attributes, obj.Blocks)

	return rapid.Custom(func(t *rapid.T) value {
		m := make(map[string]value, len(names))
		stableIter(obj.Attributes, func(k string, a schema.Attribute) {
			m[k] = g.withAttr(ctx, a).Draw(t, k)
		})

		stableIter(obj.Blocks, func(k string, b schema.Block) {
			m[k] = g.withBlock(ctx, b).Draw(t, k)
		})

		return convertObject(m, names)
	})
}

func translareTftypeNames(attrs map[string]tftypes.Type) map[string]resource.PropertyKey {
	sch := make(shimschema.SchemaMap, len(attrs))
	for k, v := range attrs {
		sch[k] = (&shimschema.Schema{
			Type: shimType(v),
		}).Shim()
	}

	names := make(map[string]resource.PropertyKey, len(sch))
	for k := range sch {
		names[k] = resource.PropertyKey(tfbridge.TerraformToPulumiNameV2(k, sch, nil))
	}
	return names
}

func translateAttrNames(ctx context.Context, attrs map[string]attr.Type) map[string]resource.PropertyKey {
	m := make(map[string]tftypes.Type, len(attrs))
	for k, v := range attrs {
		m[k] = v.TerraformType(ctx)
	}
	return translareTftypeNames(m)
}

func translateNames(ctx context.Context, attrs map[string]schema.Attribute, blocks map[string]schema.Block) map[string]resource.PropertyKey {
	sch := make(shimschema.SchemaMap, len(attrs)+len(blocks))
	for k, v := range attrs {
		sch[k] = (&shimschema.Schema{
			Type: shimType(v.GetType().TerraformType(ctx)),
		}).Shim()
	}
	for k, v := range blocks {
		sch[k] = (&shimschema.Schema{
			Type: shimType(v.Type().TerraformType(ctx)),
		}).Shim()
	}

	names := make(map[string]resource.PropertyKey, len(sch))
	for k := range sch {
		names[k] = resource.PropertyKey(tfbridge.TerraformToPulumiNameV2(k, sch, nil))
	}
	return names
}
