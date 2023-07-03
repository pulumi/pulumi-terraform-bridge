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
	"fmt"
	"sort"

	hcty "github.com/hashicorp/go-cty/cty"
	hctypack "github.com/hashicorp/go-cty/cty/msgpack"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfplan"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/zclconf/go-cty/cty"
)

func proposedNew(res *schema.Resource, prior, config hcty.Value) (hcty.Value, error) {
	t := res.CoreConfigSchema().ImpliedType()
	tt := htype2tftypes(t)
	schema, err := configschemaBlock(res)
	if err != nil {
		return hcty.False, err
	}
	priorV, err := hcty2tftypes(t, tt, prior)
	if err != nil {
		return hcty.Value{}, fmt.Errorf("Failed to convert an hcty.Value to a tftypes.Value: %w", err)
	}
	configV, err := hcty2tftypes(t, tt, config)
	if err != nil {
		return hcty.Value{}, fmt.Errorf("Failed to convert an hcty.Value to a tftypes.Value: %w", err)
	}
	planned, err := tfplan.ProposedNew(schema, priorV, configV)
	if err != nil {
		return hcty.Value{}, fmt.Errorf("Failure in ProposedNew: %w", err)
	}
	result, err := tftypes2hcty(t, tt, planned)
	if err != nil {
		return hcty.Value{}, fmt.Errorf("Failed to convert a tftypes.Value to a hcty.Value: %w", err)
	}
	return result, nil
}

func htype2tftypes(t hcty.Type) tftypes.Type {
	// Used t.HasDynamicTypes() as an example of how to pattern-match types.
	switch {
	case t == hcty.NilType:
		// From docs: NilType is an invalid type used when a function is returning an error and has no useful
		// type to return.
		contract.Assertf(false, "NilType is not supported")
	case t == hcty.DynamicPseudoType:
		return tftypes.DynamicPseudoType
	case t.IsPrimitiveType():
		switch {
		case t.Equals(hcty.Bool):
			return tftypes.Bool
		case t.Equals(hcty.String):
			return tftypes.String
		case t.Equals(hcty.Number):
			return tftypes.Number
		default:
			contract.Failf("Match failure on hcty.Type with t.IsPrimitiveType()")
		}
	case t.IsListType():
		return tftypes.List{ElementType: htype2tftypes(*t.ListElementType())}
	case t.IsMapType():
		return tftypes.Map{ElementType: htype2tftypes(*t.MapElementType())}
	case t.IsSetType():
		return tftypes.Set{ElementType: htype2tftypes(*t.SetElementType())}
	case t.IsObjectType():
		attrTypes := t.AttributeTypes()
		if len(attrTypes) == 0 {
			return tftypes.Object{AttributeTypes: map[string]tftypes.Type{}}
		}
		converted := map[string]tftypes.Type{}
		for a, at := range attrTypes {
			converted[a] = htype2tftypes(at)
		}
		return tftypes.Object{AttributeTypes: converted}
	case t.IsTupleType():
		elemTypes := t.TupleElementTypes()
		if len(elemTypes) == 0 {
			return tftypes.Tuple{ElementTypes: []tftypes.Type{}}
		}
		converted := []tftypes.Type{}
		for _, et := range elemTypes {
			converted = append(converted, htype2tftypes(et))
		}
		return tftypes.Tuple{ElementTypes: converted}
	case t.IsCapsuleType():
		contract.Assertf(false, "Capsule types are not yet supported")
	}
	contract.Assertf(false, "Match failure on hcty.Type: %v", t.GoString())
	var undefined tftypes.Type
	return undefined
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

func hcty2tftypes(t hcty.Type, tt tftypes.Type, val hcty.Value) (tftypes.Value, error) {
	msgpack, err := hctypack.Marshal(val, t)
	if err != nil {
		return tftypes.Value{}, err
	}
	return tfprotov6.DynamicValue{MsgPack: msgpack}.Unmarshal(tt)
}

func tftypes2hcty(t hcty.Type, tt tftypes.Type, val tftypes.Value) (hcty.Value, error) {
	dv, err := tfprotov6.NewDynamicValue(tt, val)
	if err != nil {
		return hcty.NilVal, nil
	}
	return hctypack.Unmarshal(dv.MsgPack, t)
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

func configschemaBlock(res *schema.Resource) (*tfprotov6.SchemaBlock, error) {
	schema := res.CoreConfigSchema()

	block := &tfprotov6.SchemaBlock{
		Attributes:      []*tfprotov6.SchemaAttribute{},
		BlockTypes:      []*tfprotov6.SchemaNestedBlock{},
		Description:     schema.Description,
		DescriptionKind: tfprotov6.StringKind(int32(schema.DescriptionKind)),
		Deprecated:      schema.Deprecated,
	}

	attrNames := []string{}
	for name := range schema.Attributes {
		attrNames = append(attrNames, name)
	}
	sort.Strings(attrNames)

	for _, name := range attrNames {
		a := schema.Attributes[name]
		t := htype2tftypes(a.Type)
		block.Attributes = append(block.Attributes, &tfprotov6.SchemaAttribute{
			Name:            name,
			Type:            t,
			Description:     a.Description,
			DescriptionKind: tfprotov6.StringKind(int32(schema.DescriptionKind)),
			Required:        a.Required,
			Optional:        a.Optional,
			Computed:        a.Computed,
			Sensitive:       a.Sensitive,
			Deprecated:      a.Deprecated,
		})
	}

	// The code below converts each schema.BlockTypes block to *tfprotov6.NestedBlock, and populates
	// block.BlockTypes. This is a trivial conversion (copying identical fields) that is necessary because Go
	// toolchain currently sees the two NestedBlock structs as distinct types. If the type of schema.BlockType was
	// not internal, this could have been expressed as a recursive func, but given that it is internal, Go compiler
	// rejects such function definition. To workaround, an explicit queue is introduced to track all NestedBlock
	// values that need converting, and destinations structure is introduced to track where the conversion results
	// should go. The code can then proceed without an explicit recursive func definition or resorting to
	// reflection.
	queue := newQueue(schema.BlockTypes)

	destinations := map[interface{}][]*tfprotov6.SchemaNestedBlock{}
	for _, b := range schema.BlockTypes {
		destinations[b] = block.BlockTypes
	}

	for !queue.empty() {
		name, b := queue.dequeue()

		nested := tfprotov6.SchemaBlock{
			Attributes:      []*tfprotov6.SchemaAttribute{},
			BlockTypes:      []*tfprotov6.SchemaNestedBlock{},
			Description:     b.Description,
			DescriptionKind: tfprotov6.StringKind(int(schema.DescriptionKind)),
			Deprecated:      b.Deprecated,
		}

		for name, a := range b.Attributes {
			t := htype2tftypes(a.Type)
			nested.Attributes = append(nested.Attributes, &tfprotov6.SchemaAttribute{
				Name:            name,
				Type:            t,
				Description:     a.Description,
				DescriptionKind: tfprotov6.StringKind(int(schema.DescriptionKind)),
				Required:        a.Required,
				Optional:        a.Optional,
				Computed:        a.Computed,
				Sensitive:       a.Sensitive,
				Deprecated:      a.Deprecated,
			})
		}

		sort.Slice(nested.Attributes, func(i, j int) bool {
			return nested.Attributes[i].Name < nested.Attributes[j].Name
		})

		for name, nb := range b.BlockTypes {
			destinations[nb] = nested.BlockTypes
			queue.enqueue(name, nb)
		}

		destinations[b] = append(destinations[b], &tfprotov6.SchemaNestedBlock{
			TypeName: name,
			Block:    &nested,
			Nesting:  tfprotov6.SchemaNestedBlockNestingMode(int32(b.Nesting)),
			MinItems: int64(b.MinItems),
			MaxItems: int64(b.MaxItems),
		})
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
