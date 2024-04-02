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

type tvGen struct {
	generateUnknowns bool
}

func (tvg *tvGen) GenTV(maxDepth int) *rapid.Generator[tv] {
	opts := []*rapid.Generator[tv]{
		tvg.GenString(),
	}
	if maxDepth > 1 {
		opts = append(opts, tvg.GenSingleNestedBlock(maxDepth-1))
	}
	return rapid.OneOf(opts...)
}

// Generate a resource or datasource inputs, which are always blocks in TF.
func (tvg *tvGen) GenBlock(maxDepth int) *rapid.Generator[tb] {
	return rapid.Custom[tb](func(t *rapid.T) tb {
		fieldSchemas := map[string]*schema.Schema{}
		fieldTypes := map[string]tftypes.Type{}
		fieldGenerators := map[string]*rapid.Generator[tftypes.Value]{}
		nFields := rapid.IntRange(0, 3).Draw(t, "nFields")
		for i := 0; i < nFields; i++ {
			fieldName := fmt.Sprintf("f%d", i)
			fieldTV := tvg.GenTV(maxDepth-1).Draw(t, fieldName)
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
		// for k, v := range fieldSchemas {
		// 	fmt.Printf("###### field %q %#v\n\n", k, v)
		// }
		return tb{fieldSchemas, objType, objGen}
	})
}

// Single-nested blocks represent object types. In schemav2 providers there is no natural encoding for these, so they
// are typically encoded as MaxItems=1 lists with a *Resource Elem.
//
// See https://developer.hashicorp.com/terraform/plugin/framework/handling-data/blocks/single-nested
func (tvg *tvGen) GenSingleNestedBlock(maxDepth int) *rapid.Generator[tv] {
	return rapid.Custom[tv](func(t *rapid.T) tv {
		st := tvg.GenSchemaTransform().Draw(t, "schemaTransform")
		bl := tvg.GenBlock(maxDepth).Draw(t, "block")
		listWrapType := tftypes.List{ElementType: bl.typ}
		listWrap := func(v tftypes.Value) tftypes.Value {
			return tftypes.NewValue(listWrapType, []tftypes.Value{v})
		}
		return tv{
			schema: st(schema.Schema{
				Type:     schema.TypeList,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: bl.schemaMap,
				},
			}),
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
}

func (tvg *tvGen) GenString() *rapid.Generator[tv] {
	s := schema.Schema{
		Type: schema.TypeString,
	}
	//nilValue := tftypes.NewValue(tftypes.String, nil)
	values := []tftypes.Value{
		tftypes.NewValue(tftypes.String, ""),
		tftypes.NewValue(tftypes.String, "text"),
	}
	if tvg.generateUnknowns {
		values = append(values, tftypes.NewValue(tftypes.String, tftypes.UnknownValue))
	}
	valueGen := rapid.SampledFrom(values)
	return rapid.Custom[tv](func(t *rapid.T) tv {
		st := tvg.GenSchemaTransform().Draw(t, "schemaTransform")
		s := st(s)
		//if s.Required {
		return tv{s, tftypes.String, valueGen}
		//}
		//return tv{s, tftypes.String, rapid.OneOf(valueGen, rapid.Just(nilValue))}
	})
}

// func (tvg *tvGen) withNullAndUnknown(
// 	t tftypes.Type,
// 	v *rapid.Generator[tftypes.Value],
// ) *rapid.Generator[tftypes.Value] {
// 	nullV := tftypes.NewValue(t, nil)
// 	if tvg.generateUnknowns {
// 		unknownV := tftypes.NewValue(t, tftypes.UnknownValue)
// 		return rapid.OneOf(v,
// 			rapid.Just(nullV),
// 			rapid.Just(unknownV))
// 	}
// 	return rapid.OneOf(v, rapid.Just(nullV))
// }

func (*tvGen) GenSchemaTransform() *rapid.Generator[schemaT] {
	return rapid.Custom[schemaT](func(t *rapid.T) schemaT {
		k := rapid.SampledFrom([]string{"o", "r", "c", "co"}).Draw(t, "optionalKind")
		// secret := rapid.Bool().Draw(t, "secret")
		// forceNew := rapid.Bool().Draw(t, "forceNew")

		return func(s schema.Schema) schema.Schema {
			switch k {
			case "o":
				s.Optional = true
			case "r":
				s.Required = true
			case "c":
				s.Optional = true
			// TODO this currently triggers Value for unconfigurable attribute
			// because the provider
			// s.Computed = true
			case "co":
				s.Computed = true
				s.Optional = true
			}
			// if forceNew {
			// 	s.ForceNew = true
			// }
			// if secret {
			// 	s.Sensitive = true
			// }
			return s
		}
	})
}

func (*tvGen) drawValue(t *rapid.T, g *rapid.Generator[tftypes.Value], label string) tftypes.Value {
	return rapid.Map(g, newPrettyValueWrapper).Draw(t, label).Value()
}
