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

// Rapid generators for schema and value spaces.
package crosstests

import (
	"fmt"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"pgregory.net/rapid"
)

// Combines a TF value representing resource inputs with its schema. The value has to conform to the schema. The name
// "tv" stands for a typed value.
type tv struct {
	schema   schema.Schema
	typ      tftypes.Type
	valueGen *rapid.Generator[tftypes.Value]
}

// Here "tb" stands for a typed block. Like [tv] but for non-nil/unknown blocks.
type tb struct {
	schemaMap map[string]*schema.Schema
	typ       tftypes.Object
	valueGen  *rapid.Generator[tftypes.Value]
}

type schemaT func(schema.Schema) schema.Schema

type attrKind int

const (
	invalidAttr attrKind = iota
	optionalAttr
	requiredAttr
	computedAttr
	computedOptionalAttr
)

type tvGen struct {
	generateUnknowns       bool
	generateConfigModeAttr bool
}

func (tvg *tvGen) GenBlock(parentName string) *rapid.Generator[tv] {
	return rapid.Custom[tv](func(t *rapid.T) tv {
		depth := rapid.IntRange(0, 3).Draw(t, "depth")
		tv := tvg.GenBlockOrAttrWithDepth(depth, parentName).Draw(t, "tv")
		return tv
	})
}

func (tvg *tvGen) GenAttr() *rapid.Generator[tv] {
	return rapid.Custom[tv](func(t *rapid.T) tv {
		depth := rapid.IntRange(0, 3).Draw(t, "depth")
		tv := tvg.GenAttrWithDepth(depth).Draw(t, "tv")
		return tv
	})
}

func (tvg *tvGen) GenBlockOrAttrWithDepth(depth int, parentName string) *rapid.Generator[tv] {
	opts := []*rapid.Generator[tv]{
		tvg.GenAttrWithDepth(depth),
	}
	if depth > 1 {
		opts = append(opts,
			tvg.GenSingleNestedBlock(depth-1, parentName),
			tvg.GenListNestedBlock(depth-1, parentName),
			tvg.GenSetNestedBlock(depth-1, parentName),
		)
	}
	return rapid.OneOf(opts...)
}

func (tvg *tvGen) GenAttrWithDepth(depth int) *rapid.Generator[tv] {
	opts := []*rapid.Generator[tv]{
		tvg.GenString(),
		tvg.GenBool(),
		tvg.GenInt(),
		tvg.GenFloat(),
	}
	if depth > 1 {
		opts = append(opts,
			tvg.GenMapAttr(depth-1),
			tvg.GenListAttr(depth-1),
			tvg.GenSetAttr(depth-1),
		)
	}
	return rapid.OneOf(opts...)
}

func (tvg *tvGen) GenMapAttr(depth int) *rapid.Generator[tv] {
	ge := rapid.Custom[tv](func(t *rapid.T) tv {
		// Only generating primitive types for the value of a Map. This is due to this limitation:
		// https://github.com/hashicorp/terraform-plugin-sdk/issues/62
		//
		// Ideally per pulumi/pulumi-terraform-bridge#1873 bridged providers should reject these at build time
		// so the runtime tests should not concern themselves with these unsupported combinations.
		zeroDepth := 0
		inner := tvg.GenAttrWithDepth(zeroDepth).Draw(t, "attrGen")
		mapWrapType := tftypes.Map{ElementType: inner.typ}
		mapWrap := func(vs map[string]tftypes.Value) tftypes.Value {
			return tftypes.NewValue(mapWrapType, vs)
		}
		keyGen := rapid.SampledFrom([]string{"a", "b"})
		vg := rapid.Map(rapid.MapOf(keyGen, inner.valueGen), mapWrap)
		return tv{
			schema: schema.Schema{
				Type: schema.TypeMap,
				Elem: &inner.schema,
			},
			typ:      mapWrapType,
			valueGen: vg,
		}
	})
	ge = tvg.WithSchemaTransform(ge)
	ge = tvg.WithNullAndUnknown(ge)
	return ge
}

func (tvg *tvGen) GenListAttr(depth int) *rapid.Generator[tv] {
	ge := rapid.Custom[tv](func(t *rapid.T) tv {
		inner := tvg.GenAttrWithDepth(depth).Draw(t, "attrGen")
		listWrapType := tftypes.List{ElementType: inner.typ}
		listWrap := func(vs []tftypes.Value) tftypes.Value {
			return tftypes.NewValue(listWrapType, vs)
		}
		vg := rapid.Map(rapid.SliceOfN(inner.valueGen, 0, 3), listWrap)
		return tv{
			schema: schema.Schema{
				// TODO get creative with the hash function
				Type: schema.TypeList,
				Elem: &inner.schema,
			},
			typ:      listWrapType,
			valueGen: vg,
		}
	})
	ge = tvg.WithSchemaTransform(ge)
	ge = tvg.WithNullAndUnknown(ge)
	return ge
}

func (tvg *tvGen) GenSetAttr(depth int) *rapid.Generator[tv] {
	ge := rapid.Custom[tv](func(t *rapid.T) tv {
		inner := tvg.GenAttrWithDepth(depth).Draw(t, "attrGen")
		setWrapType := tftypes.Set{ElementType: inner.typ}
		setWrap := func(vs []tftypes.Value) tftypes.Value {
			return tftypes.NewValue(setWrapType, vs)
		}
		vg := rapid.Map(rapid.SliceOfN(inner.valueGen, 0, 3), setWrap)
		return tv{
			schema: schema.Schema{
				// TODO[pulumi/pulumi-terraform-bridge#1862 alternative hash functions
				Type: schema.TypeSet,
				Elem: &inner.schema,
			},
			typ:      setWrapType,
			valueGen: vg,
		}
	})
	ge = tvg.WithSchemaTransform(ge)
	ge = tvg.WithNullAndUnknown(ge)
	return ge
}

// TF blocks can be resource or datasource inputs, or nested blocks.
func (tvg *tvGen) GenBlockWithDepth(depth int, parentName string) *rapid.Generator[tb] {
	return rapid.Custom[tb](func(t *rapid.T) tb {
		fieldSchemas := map[string]*schema.Schema{}
		fieldTypes := map[string]tftypes.Type{}
		fieldGenerators := map[string]*rapid.Generator[tftypes.Value]{}
		nFields := rapid.IntRange(0, 3).Draw(t, "nFields")
		for i := 0; i < nFields; i++ {
			fieldName := fmt.Sprintf("%sd%df%d", parentName, depth, i)
			fieldTV := tvg.GenBlockOrAttrWithDepth(depth, fieldName).Draw(t, fieldName)
			fieldSchemas[fieldName] = &fieldTV.schema
			fieldGenerators[fieldName] = fieldTV.valueGen
			fieldTypes[fieldName] = fieldTV.typ
		}
		objType := tftypes.Object{AttributeTypes: fieldTypes}
		var objGen *rapid.Generator[tftypes.Value]
		if len(fieldGenerators) > 0 {
			objGen = rapid.Custom[tftypes.Value](func(t *rapid.T) tftypes.Value {
				fields := map[string]tftypes.Value{}
				for f, fg := range fieldGenerators {
					fv := tvg.drawValue(t, fg, f)
					fields[f] = fv
				}
				return tftypes.NewValue(objType, fields)
			})
		} else {
			objGen = rapid.Just(tftypes.NewValue(objType, map[string]tftypes.Value{}))
		}
		err := schema.InternalMap(fieldSchemas).InternalValidate(nil)
		if err != nil {
			t.Log(fieldSchemas)
			t.Errorf("rapid_tv_gen generated an invalid schema: please fix: %s", err)
		}
		return tb{fieldSchemas, objType, objGen}
	})
}

// Single-nested blocks represent object types. In schemav2 providers there is no natural encoding for these, so they
// are typically encoded as MaxItems=1 lists with a *Resource Elem.
//
// See https://developer.hashicorp.com/terraform/plugin/framework/handling-data/blocks/single-nested
func (tvg *tvGen) GenSingleNestedBlock(depth int, parentName string) *rapid.Generator[tv] {
	ge := rapid.Custom[tv](func(t *rapid.T) tv {
		bl := tvg.GenBlockWithDepth(depth, parentName).Draw(t, "block")
		listWrapType := tftypes.List{ElementType: bl.typ}
		listWrap := func(v tftypes.Value) tftypes.Value {
			return tftypes.NewValue(listWrapType, []tftypes.Value{v})
		}
		return tv{
			schema: schema.Schema{
				Type:     schema.TypeList,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: bl.schemaMap,
				},
			},
			typ: listWrapType,
			// A few open questions here, can these values ever be unknown (likely yes) and how is that
			// represented in TF? Also, can these values be null or this is just represented as an empty
			// list? Should an empty list be part of the values here?
			//
			// This should also account for required schemas.
			//
			// valueGen: tvg.withNullAndUnknown(listWrapType, rapid.Map(bl.valueGen, listWrap)),
			valueGen: rapid.Map(bl.valueGen, listWrap),
		}
	})

	return tvg.WithSchemaTransform(ge)
}

func (tvg *tvGen) GenListNestedBlock(depth int, parentName string) *rapid.Generator[tv] {
	ge := rapid.Custom[tv](func(t *rapid.T) tv {
		bl := tvg.GenBlockWithDepth(depth, parentName).Draw(t, "block")
		listWrapType := tftypes.List{ElementType: bl.typ}
		listWrap := func(vs []tftypes.Value) tftypes.Value {
			return tftypes.NewValue(listWrapType, vs)
		}

		maxItems := rapid.IntRange(0, 1).Draw(t, "maxItemsOne")
		maxPropValues := 3
		if maxItems == 1 {
			maxPropValues = 1
		}
		// TODO: fix empty lists and revert the min value here to be randomly 0
		minVals := 1
		vg := rapid.Map(rapid.SliceOfN(bl.valueGen, minVals, maxPropValues), listWrap)
		return tv{
			schema: schema.Schema{
				Type:     schema.TypeList,
				MaxItems: maxItems,
				Elem: &schema.Resource{
					Schema: bl.schemaMap,
				},
			},
			typ:      listWrapType,
			valueGen: vg,
		}
	})
	ge = tvg.WithSchemaTransform(ge)
	ge = tvg.WithNullAndUnknown(ge)
	return ge
}

func (tvg *tvGen) GenSetNestedBlock(depth int, parentName string) *rapid.Generator[tv] {
	ge := rapid.Custom[tv](func(t *rapid.T) tv {
		bl := tvg.GenBlockWithDepth(depth, parentName).Draw(t, "block")
		setWrapType := tftypes.Set{ElementType: bl.typ}
		setWrap := func(vs []tftypes.Value) tftypes.Value {
			return tftypes.NewValue(setWrapType, vs)
		}

		maxItems := rapid.IntRange(0, 1).Draw(t, "maxItemsOne")
		maxPropValues := 3
		if maxItems == 1 {
			// We only generate valid values for the schema.
			maxPropValues = 1
		}
		vg := rapid.Map(rapid.SliceOfN(bl.valueGen, 0, maxPropValues), setWrap)
		return tv{
			schema: schema.Schema{
				Type:     schema.TypeSet,
				MaxItems: maxItems,
				// TODO: get a bit inventive with custom hash functions
				Elem: &schema.Resource{
					Schema: bl.schemaMap,
				},
			},
			typ:      setWrapType,
			valueGen: vg,
		}
	})
	ge = tvg.WithSchemaTransform(ge)
	ge = tvg.WithNullAndUnknown(ge)
	return ge
}

func (tvg *tvGen) GenAttrKind() *rapid.Generator[attrKind] {
	return rapid.SampledFrom([]attrKind{
		optionalAttr,
		requiredAttr,
		computedAttr,
		computedOptionalAttr,
	})
}

func (tvg *tvGen) GenString() *rapid.Generator[tv] {
	vals := []tftypes.Value{
		tftypes.NewValue(tftypes.String, "text"),
		tftypes.NewValue(tftypes.String, ""),
	}
	return tvg.GenScalar(schema.TypeString, vals)
}

func (tvg *tvGen) GenBool() *rapid.Generator[tv] {
	return tvg.GenScalar(schema.TypeBool, []tftypes.Value{
		tftypes.NewValue(tftypes.Bool, false),
		tftypes.NewValue(tftypes.Bool, true),
	})
}

func (tvg *tvGen) GenInt() *rapid.Generator[tv] {
	return tvg.GenScalar(schema.TypeInt, []tftypes.Value{
		tftypes.NewValue(tftypes.Number, 0),
		tftypes.NewValue(tftypes.Number, -1),
		tftypes.NewValue(tftypes.Number, 42),
	})
}

func (tvg *tvGen) GenFloat() *rapid.Generator[tv] {
	return tvg.GenScalar(schema.TypeInt, []tftypes.Value{
		tftypes.NewValue(tftypes.Number, float64(0.0)),
		tftypes.NewValue(tftypes.Number, float64(-1.0)),
		tftypes.NewValue(tftypes.Number, float64(42.0)),
	})
}

func (tvg *tvGen) GenScalar(vt schema.ValueType, values []tftypes.Value) *rapid.Generator[tv] {
	s := schema.Schema{
		Type: vt,
	}
	g := tv{
		schema:   s,
		typ:      values[0].Type(),
		valueGen: rapid.SampledFrom(values),
	}
	gen := tvg.WithSchemaTransform(rapid.Just(g))
	gen = tvg.WithNullAndUnknown(gen)
	return gen
}

func (tvg *tvGen) WithSchemaTransform(gen *rapid.Generator[tv]) *rapid.Generator[tv] {
	return rapid.Custom[tv](func(t *rapid.T) tv {
		tv0 := gen.Draw(t, "tv")
		st := tvg.GenSchemaTransform().Draw(t, "schemaTransform")
		return tv{
			schema:   st(tv0.schema),
			typ:      tv0.typ,
			valueGen: tv0.valueGen,
		}
	})
}

func (tvg *tvGen) WithNullAndUnknown(gen *rapid.Generator[tv]) *rapid.Generator[tv] {
	return rapid.Custom[tv](func(t *rapid.T) tv {
		tv0 := gen.Draw(t, "tv")
		gen := tv0.valueGen
		options := []*rapid.Generator[tftypes.Value]{gen}
		if tvg.generateUnknowns {
			unkGen := rapid.Just(tftypes.NewValue(tv0.typ, tftypes.UnknownValue))
			options = append(options, unkGen)
		}
		if !tv0.schema.Required {
			nullGen := rapid.Just(tftypes.NewValue(tv0.typ, nil))
			options = append(options, nullGen)
		}
		gen = rapid.OneOf(options...)
		return tv{
			schema:   tv0.schema,
			typ:      tv0.typ,
			valueGen: gen,
		}
	})
}

func (tvg *tvGen) GenSchemaTransform() *rapid.Generator[schemaT] {
	return rapid.Custom[schemaT](func(t *rapid.T) schemaT {
		attrKind := tvg.GenAttrKind().Draw(t, "attrKind")
		secret := rapid.Bool().Draw(t, "secret")
		forceNew := rapid.Bool().Draw(t, "forceNew")
		configModeAttr := rapid.Bool().Draw(t, "configMode")

		return func(s schema.Schema) schema.Schema {
			switch attrKind {
			case optionalAttr:
				s.Optional = true
			case requiredAttr:
				s.Required = true
			case computedAttr:
				// TODO this currently triggers errors in the tests because the provider needs to be
				// taught to polyfill computed values instead of passing them as inputs.
				s.Optional = true
			// s.Computed = true
			case computedOptionalAttr:
				s.Computed = true
				s.Optional = true
			}
			if forceNew {
				s.ForceNew = true
			}
			if secret {
				s.Sensitive = true
			}

			if tvg.generateConfigModeAttr && configModeAttr {
				if _, ok := s.Elem.(*schema.Resource); ok {
					if s.Type == schema.TypeSet || s.Type == schema.TypeList {
						s.ConfigMode = schema.SchemaConfigModeAttr
					}
				}
			}

			return s
		}
	})
}

func (*tvGen) drawValue(t *rapid.T, g *rapid.Generator[tftypes.Value], label string) tftypes.Value {
	return rapid.Map(g, newPrettyValueWrapper).Draw(t, label).Value()
}
