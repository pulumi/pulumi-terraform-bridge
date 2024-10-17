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
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"pgregory.net/rapid"
)

func attrType(depth int) *rapid.Generator[attr.Type] {
	if depth <= 1 {
		return attrPrimitiveType()
	}

	types := []*rapid.Generator[attr.Type]{
		attrPrimitiveType(),
		rapid.Map(attrListType(depth-1), castToAttrType),
		rapid.Map(attrMapType(depth-1), castToAttrType),
		rapid.Map(attrSetType(depth-1), castToAttrType),
		rapid.Map(attrObjectType(depth-1), castToAttrType),
	}

	if false { // TODO: Enable testing tuples
		types = append(types, rapid.Map(attrTupleType(depth-1), castToAttrType))
	}

	return rapid.OneOf(types...)
}

func attrPrimitiveType() *rapid.Generator[attr.Type] {
	return rapid.OneOf(
		rapid.Map(rapid.Just(types.BoolType), castToAttrType),
		rapid.Map(rapid.Just(types.NumberType), castToAttrType),
		rapid.Map(rapid.Just(types.Float64Type), castToAttrType),
		rapid.Map(rapid.Just(types.StringType), castToAttrType),
		rapid.Map(rapid.Just(types.Int64Type), castToAttrType),
	)
}

func attrListType(depth int) *rapid.Generator[types.ListType] {
	return rapid.Map(attrType(depth-1), func(t attr.Type) types.ListType {
		return types.ListType{ElemType: t}
	})
}

func attrMapType(depth int) *rapid.Generator[types.MapType] {
	return rapid.Map(attrType(depth-1), func(t attr.Type) types.MapType {
		return types.MapType{ElemType: t}
	})
}

func attrSetType(depth int) *rapid.Generator[types.SetType] {
	return rapid.Map(attrType(depth-1), func(t attr.Type) types.SetType {
		return types.SetType{ElemType: t}
	})
}

func attrTupleType(depth int) *rapid.Generator[types.TupleType] {
	return rapid.Custom(func(t *rapid.T) types.TupleType {
		return types.TupleType{
			// 1 is assumed to be a minimum.
			// 4 is chosen as an arbitrary maximum.
			ElemTypes: rapid.SliceOfN(attrType(depth-1), 1, 4).Draw(t, "ElemTypes"),
		}
	})
}

func attrObjectType(depth int) *rapid.Generator[types.ObjectType] {
	return rapid.Custom(func(t *rapid.T) types.ObjectType {
		return types.ObjectType{
			AttrTypes: rapid.MapOfN(
				rapid.StringMatching(tfIdentifierRegexp),
				attrType(depth-1),
				1, maxObjectSize,
			).Draw(t, "AttrTypes"),
		}
	})
}

func castToAttrType[T attr.Type](v T) attr.Type { return v }
