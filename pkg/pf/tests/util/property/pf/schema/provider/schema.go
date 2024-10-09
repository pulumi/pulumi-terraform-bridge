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

// Package pf provides [rapid] based property testing to Hashicorp's Plugin Framework for
// TF SDKs.
package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"pgregory.net/rapid"
)

const (
	tfIdentifierRegexp = "[a-z]|([a-z_][a-z_0-9]+)"
	defaultDepth       = 3
	maxObjectSize      = 4
)

func Schema(ctx context.Context) *rapid.Generator[schema.Schema] {
	return rapid.Custom(func(t *rapid.T) schema.Schema {
		// We need to generate a set of attributes and blocks, all with non-overlapping names.
		names := attrAndBlockNames(allowEmpty).Draw(t, "names")
		s := schema.Schema{
			Attributes: attributes(names.attr, defaultDepth).Draw(t, "attributes"),
			Blocks:     blocks(names.block, defaultDepth).Draw(t, "blocks"),
		}

		if diags := s.ValidateImplementation(ctx); diags.HasError() {
			t.Fatalf("Invalid schema generated: %#v", diags)
		}

		return s
	})
}

type canBeEmpty uint8

const (
	allowEmpty canBeEmpty = iota
	notEmpty   canBeEmpty = iota
)

func attrAndBlockNames(canBeEmpty canBeEmpty) *rapid.Generator[struct{ attr, block []string }] {
	minSize := 1
	if canBeEmpty == allowEmpty {
		minSize = 0
	}
	return rapid.Custom(func(t *rapid.T) struct{ attr, block []string } {
		attrAndBlockNames := rapid.SliceOfNDistinct(
			rapid.StringMatching(tfIdentifierRegexp),
			minSize,
			maxObjectSize,
			rapid.ID,
		).Draw(t, "names")
		split := rapid.IntRange(0, len(attrAndBlockNames)).Draw(t, "attr block split")
		attrNames, blockNames := attrAndBlockNames[:split], attrAndBlockNames[split:]
		return struct{ attr, block []string }{attrNames, blockNames}
	})
}

func attributes(names []string, depth int) *rapid.Generator[map[string]schema.Attribute] {
	if len(names) == 0 {
		return rapid.Just[map[string]schema.Attribute](nil)
	}
	return rapid.Custom(func(t *rapid.T) map[string]schema.Attribute {
		m := make(map[string]schema.Attribute, len(names))
		for _, v := range names {
			m[v] = attribute(depth).Draw(t, v)
		}
		return m
	})
}

func attribute(depth int) *rapid.Generator[schema.Attribute] {
	return rapid.Custom(func(t *rapid.T) schema.Attribute {
		if depth <= 1 {
			return rapid.OneOf(primitiveAttributes()...).Draw(t, "attr")
		}
		return rapid.OneOf(
			append(primitiveAttributes(), complexAttributes(depth)...)...,
		).Draw(t, "attr")
	})
}

func primitiveAttributes() []*rapid.Generator[schema.Attribute] {
	return []*rapid.Generator[schema.Attribute]{
		rapid.Map(stringAttr(), castToAttribute),
		rapid.Map(boolAttr(), castToAttribute),
		rapid.Map(float64Attr(), castToAttribute),
		rapid.Map(int64Attr(), castToAttribute),
		rapid.Map(numberAttr(), castToAttribute),
	}
}

func castToAttribute[T schema.Attribute](v T) schema.Attribute { return v }

func boolAttr() *rapid.Generator[schema.BoolAttribute] {
	return rapid.Custom(func(t *rapid.T) schema.BoolAttribute {

		required := rapid.Bool().Draw(t, "required")
		optional := !required
		return schema.BoolAttribute{
			Required:  required,
			Optional:  optional,
			Sensitive: rapid.Bool().Draw(t, "sensitive"),
		}
	})
}

func float64Attr() *rapid.Generator[schema.Float64Attribute] {
	return rapid.Custom(func(t *rapid.T) schema.Float64Attribute {

		required := rapid.Bool().Draw(t, "required")
		optional := !required
		return schema.Float64Attribute{
			Required:  required,
			Optional:  optional,
			Sensitive: rapid.Bool().Draw(t, "sensitive"),
		}
	})
}

func int64Attr() *rapid.Generator[schema.Int64Attribute] {
	return rapid.Custom(func(t *rapid.T) schema.Int64Attribute {

		required := rapid.Bool().Draw(t, "required")
		optional := !required
		return schema.Int64Attribute{
			Required:  required,
			Optional:  optional,
			Sensitive: rapid.Bool().Draw(t, "sensitive"),
		}
	})
}

func numberAttr() *rapid.Generator[schema.NumberAttribute] {
	return rapid.Custom(func(t *rapid.T) schema.NumberAttribute {

		required := rapid.Bool().Draw(t, "required")
		optional := !required
		return schema.NumberAttribute{
			Required:  required,
			Optional:  optional,
			Sensitive: rapid.Bool().Draw(t, "sensitive"),
		}
	})
}

func stringAttr() *rapid.Generator[schema.StringAttribute] {
	return rapid.Custom(func(t *rapid.T) schema.StringAttribute {

		required := rapid.Bool().Draw(t, "required")
		optional := !required
		return schema.StringAttribute{
			Required:  required,
			Optional:  optional,
			Sensitive: rapid.Bool().Draw(t, "sensitive"),
		}
	})
}

func complexAttributes(depth int) []*rapid.Generator[schema.Attribute] {
	return []*rapid.Generator[schema.Attribute]{
		rapid.Map(listAttr(depth), castToAttribute),
		rapid.Map(listNestedAttr(depth), castToAttribute),
		rapid.Map(mapAttr(depth), castToAttribute),
		rapid.Map(mapNestedAttr(depth), castToAttribute),
		rapid.Map(setAttr(depth), castToAttribute),
		rapid.Map(setNestedAttr(depth), castToAttribute),
		rapid.Map(objectAttr(depth), castToAttribute),
		rapid.Map(singleNestedAttr(depth), castToAttribute),
	}
}

func listAttr(depth int) *rapid.Generator[schema.ListAttribute] {
	return rapid.Custom(func(t *rapid.T) schema.ListAttribute {

		required := rapid.Bool().Draw(t, "required")
		optional := !required
		return schema.ListAttribute{
			ElementType: attrType(depth-1).Draw(t, "ElementType"),
			Required:    required,
			Optional:    optional,
			Sensitive:   rapid.Bool().Draw(t, "sensitive"),
		}
	})
}

func listNestedAttr(depth int) *rapid.Generator[schema.ListNestedAttribute] {
	return rapid.Custom(func(t *rapid.T) schema.ListNestedAttribute {

		required := rapid.Bool().Draw(t, "required")
		optional := !required
		return schema.ListNestedAttribute{
			NestedObject: nestedAttributeObject(depth-1).Draw(t, "NestedObject"),
			Required:     required,
			Optional:     optional,
			Sensitive:    rapid.Bool().Draw(t, "sensitive"),
		}
	})
}

func mapAttr(depth int) *rapid.Generator[schema.MapAttribute] {
	return rapid.Custom(func(t *rapid.T) schema.MapAttribute {

		required := rapid.Bool().Draw(t, "required")
		optional := !required
		return schema.MapAttribute{
			ElementType: attrType(depth-1).Draw(t, "ElementType"),
			Required:    required,
			Optional:    optional,
			Sensitive:   rapid.Bool().Draw(t, "sensitive"),
		}
	})
}

func mapNestedAttr(depth int) *rapid.Generator[schema.MapNestedAttribute] {
	return rapid.Custom(func(t *rapid.T) schema.MapNestedAttribute {

		required := rapid.Bool().Draw(t, "required")
		optional := !required
		return schema.MapNestedAttribute{
			NestedObject: nestedAttributeObject(depth-1).Draw(t, "NestedObject"),
			Required:     required,
			Optional:     optional,
			Sensitive:    rapid.Bool().Draw(t, "sensitive"),
		}
	})
}

func setAttr(depth int) *rapid.Generator[schema.SetAttribute] {
	return rapid.Custom(func(t *rapid.T) schema.SetAttribute {

		required := rapid.Bool().Draw(t, "required")
		optional := !required
		return schema.SetAttribute{
			ElementType: attrType(depth-1).Draw(t, "ElementType"),
			Required:    required,
			Optional:    optional,
			Sensitive:   rapid.Bool().Draw(t, "sensitive"),
		}
	})
}

func setNestedAttr(depth int) *rapid.Generator[schema.SetNestedAttribute] {
	return rapid.Custom(func(t *rapid.T) schema.SetNestedAttribute {

		required := rapid.Bool().Draw(t, "required")
		optional := !required
		return schema.SetNestedAttribute{
			NestedObject: nestedAttributeObject(depth-1).Draw(t, "NestedObject"),
			Required:     required,
			Optional:     optional,
			Sensitive:    rapid.Bool().Draw(t, "sensitive"),
		}
	})
}

func objectAttr(depth int) *rapid.Generator[schema.ObjectAttribute] {
	return rapid.Custom(func(t *rapid.T) schema.ObjectAttribute {

		required := rapid.Bool().Draw(t, "required")
		optional := !required
		return schema.ObjectAttribute{
			AttributeTypes: attrObjectType(depth-1).Draw(t, "AttributeTypes").AttrTypes,
			Required:       required,
			Optional:       optional,
			Sensitive:      rapid.Bool().Draw(t, "sensitive"),
		}
	})
}

func singleNestedAttr(depth int) *rapid.Generator[schema.SingleNestedAttribute] {
	return rapid.Custom(func(t *rapid.T) schema.SingleNestedAttribute {

		required := rapid.Bool().Draw(t, "required")
		optional := !required
		return schema.SingleNestedAttribute{
			Attributes: nestedAttributeObject(depth-1).Draw(t, "Attributes").Attributes,
			Required:   required,
			Optional:   optional,
			Sensitive:  rapid.Bool().Draw(t, "sensitive"),
		}
	})
}

func nestedAttributeObject(depth int) *rapid.Generator[schema.NestedAttributeObject] {
	return rapid.Custom(func(t *rapid.T) schema.NestedAttributeObject {
		return schema.NestedAttributeObject{
			Attributes: rapid.MapOfN(
				rapid.StringMatching(tfIdentifierRegexp),
				attribute(depth-1),
				1, maxObjectSize,
			).Draw(t, "Attributes"),
		}
	})
}

func blocks(names []string, depth int) *rapid.Generator[map[string]schema.Block] {
	if len(names) == 0 || depth < 1 {
		return rapid.Just[map[string]schema.Block](nil)
	}
	return rapid.Custom(func(t *rapid.T) map[string]schema.Block {
		m := make(map[string]schema.Block, len(names))
		for _, k := range names {
			m[k] = block(depth).Draw(t, k)
		}
		return m
	})
}

func castToBlock[T schema.Block](v T) schema.Block { return v }

func block(depth int) *rapid.Generator[schema.Block] {
	return rapid.OneOf(
		rapid.Map(listNestedBlock(depth), castToBlock),
		rapid.Map(setNestedBlock(depth), castToBlock),
		rapid.Map(singleNestedBlock(depth), castToBlock),
	)
}

func listNestedBlock(depth int) *rapid.Generator[schema.ListNestedBlock] {
	return rapid.Custom(func(t *rapid.T) schema.ListNestedBlock {
		return schema.ListNestedBlock{
			NestedObject: nestedBlockObject(depth-1).Draw(t, "NestedObject"),
		}
	})
}

func setNestedBlock(depth int) *rapid.Generator[schema.SetNestedBlock] {
	return rapid.Custom(func(t *rapid.T) schema.SetNestedBlock {
		return schema.SetNestedBlock{
			NestedObject: nestedBlockObject(depth-1).Draw(t, "NestedObject"),
		}
	})
}

func singleNestedBlock(depth int) *rapid.Generator[schema.SingleNestedBlock] {
	return rapid.Custom(func(t *rapid.T) schema.SingleNestedBlock {
		names := attrAndBlockNames(notEmpty).Draw(t, "names")
		return schema.SingleNestedBlock{
			Attributes: attributes(names.attr, depth-1).Draw(t, "Attributes"),
			Blocks:     blocks(names.block, depth-1).Draw(t, "Blocks"),
		}
	})
}

func nestedBlockObject(depth int) *rapid.Generator[schema.NestedBlockObject] {
	return rapid.Custom(func(t *rapid.T) schema.NestedBlockObject {
		names := attrAndBlockNames(notEmpty).Draw(t, "names")
		return schema.NestedBlockObject{
			Attributes: attributes(names.attr, depth-1).Draw(t, "Attributes"),
			Blocks:     blocks(names.block, depth-1).Draw(t, "Blocks"),
		}
	})
}
