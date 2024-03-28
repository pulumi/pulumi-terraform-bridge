package crosstests

import (
	"bytes"
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
	valueGen *rapid.Generator[wrappedValue]
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
		opts = append(opts, tvg.GenObjectValue(maxDepth-1))
	}
	return rapid.OneOf(opts...)
}

func (tvg *tvGen) GenObject(maxDepth int) *rapid.Generator[tv] {
	return rapid.Custom[tv](func(t *rapid.T) tv {
		fieldSchemas := map[string]*schema.Schema{}
		fieldTypes := map[string]tftypes.Type{}
		fieldGenerators := map[string]*rapid.Generator[wrappedValue]{}
		nFields := rapid.IntRange(0, 3).Draw(t, "nFields")
		for i := 0; i < nFields; i++ {
			fieldName := fmt.Sprintf("f%d", i)
			fieldTV := tvg.GenTV(maxDepth-1).Draw(t, fieldName)
			fieldSchemas[fieldName] = &fieldTV.schema
			fieldGenerators[fieldName] = fieldTV.valueGen
			fieldTypes[fieldName] = fieldTV.typ
		}
		objSchema := schema.Schema{
			Type: schema.TypeMap,
			Elem: &schema.Resource{
				Schema: fieldSchemas,
			},
		}
		st := tvg.GenSchemaTransform().Draw(t, "schemaTransform")
		objType := tftypes.Object{AttributeTypes: fieldTypes}
		var objGen *rapid.Generator[tftypes.Value]
		if len(fieldGenerators) > 0 {
			objGen = rapid.Custom[tftypes.Value](func(t *rapid.T) tftypes.Value {
				fields := map[string]tftypes.Value{}
				for f, fg := range fieldGenerators {
					fv := fg.Draw(t, f)
					fields[f] = fv.inner
				}
				return tftypes.NewValue(objType, fields)
			})
		} else {
			objGen = rapid.Just(tftypes.NewValue(objType, map[string]tftypes.Value{}))
		}
		return tv{st(objSchema), objType, rapid.Map(objGen, newWrappedValue)}
	})
}

// Like [GenObject] but can also return top-level nil or unknown.
func (tvg *tvGen) GenObjectValue(maxDepth int) *rapid.Generator[tv] {
	return rapid.Map(tvg.GenObject(maxDepth), func(x tv) tv {
		x.valueGen = tvg.withNullAndUnknown(x.typ, x.valueGen)
		return x
	})
}

func (tvg *tvGen) GenString() *rapid.Generator[tv] {
	s := schema.Schema{
		Type: schema.TypeString,
	}
	nilValue := tftypes.NewValue(tftypes.String, nil)
	values := []tftypes.Value{
		tftypes.NewValue(tftypes.String, ""),
		tftypes.NewValue(tftypes.String, "text"),
	}
	if tvg.generateUnknowns {
		values = append(values, tftypes.NewValue(tftypes.String, tftypes.UnknownValue))
	}
	valueGen := rapid.Map(rapid.SampledFrom(values), newWrappedValue)
	return rapid.Custom[tv](func(t *rapid.T) tv {
		st := tvg.GenSchemaTransform().Draw(t, "schemaTransform")
		s := st(s)
		if s.Required {
			return tv{s, tftypes.String, valueGen}
		}
		return tv{s, tftypes.String, rapid.OneOf(valueGen, rapid.Just(newWrappedValue(nilValue)))}
	})
}

func (tvg *tvGen) withNullAndUnknown(
	t tftypes.Type,
	v *rapid.Generator[wrappedValue],
) *rapid.Generator[wrappedValue] {
	nullV := tftypes.NewValue(t, nil)
	if tvg.generateUnknowns {
		unknownV := tftypes.NewValue(t, tftypes.UnknownValue)
		return rapid.OneOf(v,
			rapid.Just(newWrappedValue(nullV)),
			rapid.Just(newWrappedValue(unknownV)))
	}
	return rapid.OneOf(v, rapid.Just(newWrappedValue(nullV)))
}

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
				s.Computed = true
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

// Wrapping tftypes.Value to provide a friendlier GoString implementation. Whenever rapid draws a value it logs it, and
// it is really nice to be able to actually read the result.
type wrappedValue struct {
	inner tftypes.Value
}

func newWrappedValue(v tftypes.Value) wrappedValue {
	return wrappedValue{v}
}

func (s wrappedValue) GoString() string {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "<<<\n")
	tftypes.Walk(s.inner, func(ap *tftypes.AttributePath, v tftypes.Value) (bool, error) {
		switch {
		case v.Type().Is(tftypes.Object{}) || v.Type().Is(tftypes.Set{}) ||
			v.Type().Is(tftypes.Map{}) || v.Type().Is(tftypes.List{}):
			return true, nil
		default:
			fmt.Fprintf(&buf, "%s: %s\n", ap.String(), v.String())
			return true, nil
		}
	})
	fmt.Fprintf(&buf, ">>>\n")
	return buf.String() + ":" + fmt.Sprintf("%#v", s.inner)
}
