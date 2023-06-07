// Copyright 2016-2023, Pulumi Corporation.
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

package sdkv2

import (
	hcty "github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/zclconf/go-cty/cty"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v2/internal/tf/configs/configschema"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v2/internal/tf/plans/objchange"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

func proposedNew(res *schema.Resource, prior, config hcty.Value) (hcty.Value, error) {
	schema, err := configschemaBlock(res)
	if err != nil {
		return hcty.False, err
	}
	priorC := hcty2cty(prior)
	configC := hcty2cty(config)
	return cty2hcty(objchange.ProposedNew(schema, priorC, configC)), nil
}

func htype2ctype(t hcty.Type) cty.Type {
	// Used t.HasDynamicTypes() as an example of how to pattern-match types.
	switch {
	case t == hcty.NilType:
		return cty.NilType
	case t == hcty.DynamicPseudoType:
		return cty.DynamicPseudoType
	case t.IsPrimitiveType():
		switch {
		case t.Equals(hcty.Bool):
			return cty.Bool
		case t.Equals(hcty.String):
			return cty.String
		case t.Equals(hcty.Number):
			return cty.Number
		default:
			contract.Failf("Match failure on hcty.Type with t.IsPrimitiveType()")
		}
	case t.IsListType():
		return cty.List(htype2ctype(*t.ListElementType()))
	case t.IsMapType():
		return cty.Map(htype2ctype(*t.MapElementType()))
	case t.IsSetType():
		return cty.Set(htype2ctype(*t.SetElementType()))
	case t.IsObjectType():
		attrTypes := t.AttributeTypes()
		if len(attrTypes) == 0 {
			return cty.EmptyObject
		}
		converted := map[string]cty.Type{}
		for a, at := range attrTypes {
			converted[a] = htype2ctype(at)
		}
		return cty.Object(converted)
	case t.IsTupleType():
		elemTypes := t.TupleElementTypes()
		if len(elemTypes) == 0 {
			return cty.EmptyTuple
		}
		converted := []cty.Type{}
		for _, et := range elemTypes {
			converted = append(converted, htype2ctype(et))
		}
		return cty.Tuple(converted)
	case t.IsCapsuleType():
		contract.Assertf(false, "Capsule types are not yet supported")
	}
	contract.Assertf(false, "Match failure on hcty.Type: %v", t.GoString())
	return cty.NilType
}

func hcty2cty(val hcty.Value) cty.Value {
	if val.IsKnown() && val.Equals(hcty.NilVal).True() {
		return cty.NilVal
	}
	ty := htype2ctype(val.Type())
	return hcty2ctyWithType(ty, val)
}

func hcty2ctyWithType(ty cty.Type, val hcty.Value) cty.Value {
	// Pattern-matching below is based on how hcty.Transform is written.
	switch {
	case val.IsNull():
		return cty.NullVal(ty)
	case !val.IsKnown():
		return cty.UnknownVal(ty)
	case ty.Equals(cty.EmptyTuple):
		return cty.EmptyTupleVal
	case ty.Equals(cty.String):
		return cty.StringVal(val.AsString())
	case ty.Equals(cty.Number):
		return cty.NumberVal(val.AsBigFloat())
	case ty.Equals(cty.Bool):
		if val.True() {
			return cty.True
		}
		return cty.False
	case ty.IsListType(), ty.IsSetType(), ty.IsTupleType():
		l := val.LengthInt()
		elems := make([]cty.Value, 0, l)
		idx := 0
		for it := val.ElementIterator(); it.Next(); {
			_, ev := it.Element()
			var eT cty.Type
			if ty.IsTupleType() {
				eT = ty.TupleElementType(idx)
			} else {
				eT = ty.ElementType()
			}
			newEv := hcty2ctyWithType(eT, ev)
			elems = append(elems, newEv)
			idx++
		}
		switch {
		case ty.IsListType():
			if len(elems) == 0 {
				return cty.ListValEmpty(ty.ElementType())
			}
			return cty.ListVal(elems)
		case ty.IsSetType():
			if len(elems) == 0 {
				return cty.SetValEmpty(ty.ElementType())
			}
			return cty.SetVal(elems)
		case ty.IsTupleType():
			return cty.TupleVal(elems)
		default:
			contract.Assertf(false, "match failure on cty.Type: unknown sequence type")
		}
	case ty.IsMapType():
		eT := ty.ElementType()
		elems := make(map[string]cty.Value)
		for it := val.ElementIterator(); it.Next(); {
			kv, ev := it.Element()
			newEv := hcty2ctyWithType(eT, ev)
			elems[kv.AsString()] = newEv
		}
		if len(elems) == 0 {
			return cty.MapValEmpty(eT)
		}
		return cty.MapVal(elems)
	case ty.IsObjectType():
		atys := ty.AttributeTypes()
		newAVs := make(map[string]cty.Value)
		for name, aT := range atys {
			av := val.GetAttr(name)
			newAV := hcty2ctyWithType(aT, av)
			newAVs[name] = newAV
		}
		return cty.ObjectVal(newAVs)
	case ty.IsCapsuleType():
		contract.Assertf(false, "Capsule types are not yet supported")
	}
	contract.Assertf(false, "match failure on hcty.Value: %v", val.GoString())
	return cty.DynamicVal
}

func ctype2htype(t cty.Type) hcty.Type {
	// Used t.HasDynamicTypes() as an example of how to pattern-match types.
	switch {
	case t == cty.NilType:
		return hcty.NilType
	case t == cty.DynamicPseudoType:
		return hcty.DynamicPseudoType
	case t.IsPrimitiveType():
		switch {
		case t.Equals(cty.Bool):
			return hcty.Bool
		case t.Equals(cty.String):
			return hcty.String
		case t.Equals(cty.Number):
			return hcty.Number
		default:
			contract.Assertf(false, "Match failure on hcty.Type with t.IsPrimitiveType()")
		}
	case t.IsListType():
		return hcty.List(ctype2htype(*t.ListElementType()))
	case t.IsMapType():
		return hcty.Map(ctype2htype(*t.MapElementType()))
	case t.IsSetType():
		return hcty.Set(ctype2htype(*t.SetElementType()))
	case t.IsObjectType():
		attrTypes := t.AttributeTypes()
		if len(attrTypes) == 0 {
			return hcty.EmptyObject
		}
		converted := map[string]hcty.Type{}
		for a, at := range attrTypes {
			converted[a] = ctype2htype(at)
		}
		return hcty.Object(converted)
	case t.IsTupleType():
		elemTypes := t.TupleElementTypes()
		if len(elemTypes) == 0 {
			return hcty.EmptyTuple
		}
		converted := []hcty.Type{}
		for _, et := range elemTypes {
			converted = append(converted, ctype2htype(et))
		}
		return hcty.Tuple(converted)
	case t.IsCapsuleType():
		contract.Assertf(false, "Capsule types are not yet supported")
	}
	contract.Assertf(false, "Match failure on hcty.Type")
	return hcty.NilType
}

func cty2hcty(val cty.Value) hcty.Value {
	if val.IsKnown() && val.Equals(cty.NilVal).True() {
		return hcty.NilVal
	}
	ty := ctype2htype(val.Type())
	return cty2hctyWithType(ty, val)
}

func cty2hctyWithType(ty hcty.Type, val cty.Value) hcty.Value {
	// Pattern-matching below is based on how hcty.Transform is written.
	switch {
	case val.IsNull():
		return hcty.NullVal(ty)
	case !val.IsKnown():
		return hcty.UnknownVal(ty)
	case ty.Equals(hcty.EmptyTuple):
		return hcty.EmptyTupleVal
	case ty.Equals(hcty.String):
		return hcty.StringVal(val.AsString())
	case ty.Equals(hcty.Number):
		return hcty.NumberVal(val.AsBigFloat())
	case ty.Equals(hcty.Bool):
		if val.True() {
			return hcty.True
		}
		return hcty.False
	case ty.IsListType(), ty.IsSetType(), ty.IsTupleType():
		l := val.LengthInt()
		elems := make([]hcty.Value, 0, l)
		idx := 0
		for it := val.ElementIterator(); it.Next(); {
			_, ev := it.Element()
			var eT hcty.Type
			if ty.IsTupleType() {
				eT = ty.TupleElementType(idx)
			} else {
				eT = ty.ElementType()
			}
			newEv := cty2hctyWithType(eT, ev)
			elems = append(elems, newEv)
			idx++
		}
		switch {
		case ty.IsListType():
			if len(elems) == 0 {
				return hcty.ListValEmpty(ty.ElementType())
			}
			return hcty.ListVal(elems)
		case ty.IsSetType():
			if len(elems) == 0 {
				return hcty.SetValEmpty(ty.ElementType())
			}
			return hcty.SetVal(elems)
		case ty.IsTupleType():
			return hcty.TupleVal(elems)
		default:
			contract.Assertf(false, "match failure on cty.Type: unknown sequence type")
		}
	case ty.IsMapType():
		eT := ty.ElementType()
		elems := make(map[string]hcty.Value)
		for it := val.ElementIterator(); it.Next(); {
			kv, ev := it.Element()
			newEv := cty2hctyWithType(eT, ev)
			elems[kv.AsString()] = newEv
		}
		if len(elems) == 0 {
			return hcty.MapValEmpty(eT)
		}
		return hcty.MapVal(elems)
	case ty.IsObjectType():
		atys := ty.AttributeTypes()
		newAVs := make(map[string]hcty.Value)
		for name, aT := range atys {
			av := val.GetAttr(name)
			newAV := cty2hctyWithType(aT, av)
			newAVs[name] = newAV
		}
		return hcty.ObjectVal(newAVs)
	case ty.IsCapsuleType():
		contract.Assertf(false, "Capsule types are not yet supported")
	}
	contract.Assertf(false, "match failure on cty.Value")
	return hcty.DynamicVal
}

func configschemaBlock(res *schema.Resource) (*configschema.Block, error) {
	schema := res.CoreConfigSchema()

	block := &configschema.Block{
		Attributes:      map[string]*configschema.Attribute{},
		BlockTypes:      map[string]*configschema.NestedBlock{},
		Description:     schema.Description,
		DescriptionKind: configschema.StringKind(int(schema.DescriptionKind)),
		Deprecated:      schema.Deprecated,
	}

	for name, a := range schema.Attributes {
		t := htype2ctype(a.Type)
		block.Attributes[name] = &configschema.Attribute{
			Type:            t,
			Description:     a.Description,
			DescriptionKind: configschema.StringKind(int(schema.DescriptionKind)),
			Required:        a.Required,
			Optional:        a.Optional,
			Computed:        a.Computed,
			Sensitive:       a.Sensitive,
			Deprecated:      a.Deprecated,
		}
	}

	// The code below converts each schema.BlockTypes block to *configschema.NestedBlock, and populates
	// block.BlockTypes. This is a trivial conversion (copying identical fields) that is necessary because Go
	// toolchain currently sees the two NestedBlock structs as distinct types. If the type of schema.BlockType was
	// not internal, this could have been expressed as a recursive func, but given that it is internal, Go compiler
	// rejects such function definition. To workaround, an explicit queue is introduced to track all NestedBlock
	// values that need converting, and destinations structure is introduced to track where the conversion results
	// should go. The code can then proceed without an explicit recursive func definition or resorting to
	// reflection.
	queue := newQueue(schema.BlockTypes)

	destinations := map[interface{}]map[string]*configschema.NestedBlock{}
	for _, b := range schema.BlockTypes {
		destinations[b] = block.BlockTypes
	}

	for !queue.empty() {
		name, b := queue.dequeue()

		nested := configschema.Block{
			Attributes:      map[string]*configschema.Attribute{},
			BlockTypes:      map[string]*configschema.NestedBlock{},
			Description:     b.Description,
			DescriptionKind: configschema.StringKind(int(schema.DescriptionKind)),
			Deprecated:      b.Deprecated,
		}

		for name, a := range b.Attributes {
			t := htype2ctype(a.Type)
			nested.Attributes[name] = &configschema.Attribute{
				Type:            t,
				Description:     a.Description,
				DescriptionKind: configschema.StringKind(int(schema.DescriptionKind)),
				Required:        a.Required,
				Optional:        a.Optional,
				Computed:        a.Computed,
				Sensitive:       a.Sensitive,
				Deprecated:      a.Deprecated,
			}
		}

		for name, nb := range b.BlockTypes {
			destinations[nb] = nested.BlockTypes
			queue.enqueue(name, nb)
		}

		destinations[b][name] = &configschema.NestedBlock{
			Block:    nested,
			Nesting:  configschema.NestingMode(int(b.Nesting)),
			MinItems: b.MinItems,
			MaxItems: b.MaxItems,
		}
	}

	return block, nil
}

type queue[T any] struct {
	elems []struct {
		key   string
		value *T
	}
}

func (q *queue[T]) dequeue() (string, *T) {
	k := q.elems[0].key
	v := q.elems[0].value
	q.elems = q.elems[1:]
	return k, v
}

func (q *queue[T]) empty() bool {
	return len(q.elems) == 0
}

func (q *queue[T]) enqueue(key string, value *T) {
	q.elems = append(q.elems, struct {
		key   string
		value *T
	}{key: key, value: value})
}

func newQueue[T any](starter map[string]*T) *queue[T] {
	q := &queue[T]{}
	for k, v := range starter {
		q.enqueue(k, v)
	}
	return q
}
