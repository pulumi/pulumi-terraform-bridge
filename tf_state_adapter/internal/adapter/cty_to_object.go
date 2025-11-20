// From the bridge
package adapter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/zclconf/go-cty/cty"
	ctyjson "github.com/zclconf/go-cty/cty/json"
)

func objectFromCtyValue(v cty.Value) map[string]interface{} {
	var path cty.Path
	buf := &bytes.Buffer{}
	// The round trip here to JSON is redundant, we could instead convert from cty to map[string]interface{} directly
	err := marshal(v, v.Type(), path, buf)
	contract.AssertNoErrorf(err, "Failed to marshal cty.Value to a JSON string value")

	var m map[string]interface{}
	err = unmarshalJSON(buf.Bytes(), &m)
	contract.AssertNoErrorf(err, "failed to unmarshal: %s", buf.String())

	return m
}

// This is needed because json.Unmarshal uses float64 for numbers by default which truncates int64 numbers.
func unmarshalJSON(data []byte, v interface{}) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	return dec.Decode(v)
}

// marshal takes a cty.Value and converts it to a JSON string
// This is a fork of the marshal function for the hashicorp/go-cty package
// https://github.com/hashicorp/go-cty/blob/master/cty/json/marshal.go
//
// The only difference being the handling of Unknown values. In our case we want to convert
// the unknown value into a string value
func marshal(val cty.Value, t cty.Type, path cty.Path, b *bytes.Buffer) error {
	if val.IsMarked() {
		return path.NewErrorf("value has marks, so it cannot be serialized")
	}

	// If we're going to decode as DynamicPseudoType then we need to save
	// dynamic type information to recover the real type.
	if t == cty.DynamicPseudoType && val.Type() != cty.DynamicPseudoType {
		return marshalDynamic(val, path, b)
	}

	if val.IsNull() {
		b.WriteString("null")
		return nil
	}

	// This is the one difference between the hashicorp/go-cty marshal function
	// This function does not correctly handle Unknown values. At this point it could be any unknown type,
	// an unknown map, list, etc, but it is not possible to further recurse on an Unknown type
	// so the best we can do is just convert it to a string sentinel
	if !val.IsKnown() {
		fmt.Fprintf(b, "%q", tfbridge.TerraformUnknownVariableValue)
		return nil
	}

	// The caller should've guaranteed that the given val is conformant with
	// the given type t, so we'll proceed under that assumption here.
	switch {
	case t.IsPrimitiveType():
		switch t {
		case cty.String:
			json, err := json.Marshal(val.AsString())
			if err != nil {
				return path.NewErrorf("failed to serialize value: %s", err)
			}
			b.Write(json)
			return nil
		case cty.Number:
			if val.RawEquals(cty.PositiveInfinity) || val.RawEquals(cty.NegativeInfinity) {
				return path.NewErrorf("cannot serialize infinity as JSON")
			}
			b.WriteString(val.AsBigFloat().Text('f', -1))
			return nil
		case cty.Bool:
			if val.True() {
				b.WriteString("true")
			} else {
				b.WriteString("false")
			}
			return nil
		default:
			panic("unsupported primitive type")
		}
	case t.IsListType(), t.IsSetType():
		b.WriteRune('[')
		first := true
		ety := t.ElementType()
		it := val.ElementIterator()
		path := append(path, nil) // local override of 'path' with extra element
		for it.Next() {
			if !first {
				b.WriteRune(',')
			}
			ek, ev := it.Element()
			path[len(path)-1] = cty.IndexStep{
				Key: ek,
			}
			err := marshal(ev, ety, path, b)
			if err != nil {
				return err
			}
			first = false
		}
		b.WriteRune(']')
		return nil
	case t.IsMapType():
		b.WriteRune('{')
		first := true
		ety := t.ElementType()
		it := val.ElementIterator()
		path := append(path, nil) // local override of 'path' with extra element
		for it.Next() {
			if !first {
				b.WriteRune(',')
			}
			ek, ev := it.Element()
			path[len(path)-1] = cty.IndexStep{
				Key: ek,
			}
			var err error
			err = marshal(ek, ek.Type(), path, b)
			if err != nil {
				return err
			}
			b.WriteRune(':')
			err = marshal(ev, ety, path, b)
			if err != nil {
				return err
			}
			first = false
		}
		b.WriteRune('}')
		return nil
	case t.IsTupleType():
		b.WriteRune('[')
		etys := t.TupleElementTypes()
		it := val.ElementIterator()
		path := append(path, nil) // local override of 'path' with extra element
		i := 0
		for it.Next() {
			if i > 0 {
				b.WriteRune(',')
			}
			ety := etys[i]
			ek, ev := it.Element()
			path[len(path)-1] = cty.IndexStep{
				Key: ek,
			}
			err := marshal(ev, ety, path, b)
			if err != nil {
				return err
			}
			i++
		}
		b.WriteRune(']')
		return nil
	case t.IsObjectType():
		b.WriteRune('{')
		atys := t.AttributeTypes()
		path := append(path, nil) // local override of 'path' with extra element

		names := make([]string, 0, len(atys))
		for k := range atys {
			names = append(names, k)
		}
		sort.Strings(names)

		for i, k := range names {
			aty := atys[k]
			if i > 0 {
				b.WriteRune(',')
			}
			av := val.GetAttr(k)
			path[len(path)-1] = cty.GetAttrStep{
				Name: k,
			}
			var err error
			err = marshal(cty.StringVal(k), cty.String, path, b)
			if err != nil {
				return err
			}
			b.WriteRune(':')
			err = marshal(av, aty, path, b)
			if err != nil {
				return err
			}
		}
		b.WriteRune('}')
		return nil
	case t.IsCapsuleType():
		rawVal := val.EncapsulatedValue()
		jsonVal, err := json.Marshal(rawVal)
		if err != nil {
			return path.NewError(err)
		}
		b.Write(jsonVal)
		return nil
	default:
		// should never happen
		return path.NewErrorf("cannot JSON-serialize %s", t.FriendlyName())
	}
}

// marshalDynamic adds an extra wrapping object containing dynamic type
// information for the given value.
func marshalDynamic(val cty.Value, path cty.Path, b *bytes.Buffer) error {
	typeJSON, err := ctyjson.MarshalType(val.Type())
	if err != nil {
		return path.NewErrorf("failed to serialize type: %s", err)
	}
	b.WriteString(`{"value":`)
	if err := marshal(val, val.Type(), path, b); err != nil {
		return err
	}
	b.WriteString(`,"type":`)
	b.Write(typeJSON)
	b.WriteRune('}')
	return nil
}
