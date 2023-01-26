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

package pfutils

import (
	"reflect"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

func extractNestedAttributes(attrLike AttrLike) (map[string]Attr, NestingMode) {
	obj := reflect.ValueOf(attrLike)
	// Check if attrLike implements fwschema.NestedAttribute. Use reflection because linkage is impossible.
	if !hasMethod(obj, "GetNestedObject") {
		return nil, 0
	}
	nestedAttributeObject := callGetter(obj, "GetNestedObject")                // fwschema.NestedAttributeObject
	underlyingAttributes := callGetter(nestedAttributeObject, "GetAttributes") // fwschema.UnderlyingAttributes
	nestingMode := callGetter(obj, "GetNestingMode")
	attrs := fromUnderlyingAttributes(underlyingAttributes)
	mode := NestingMode(asUInt8(nestingMode))
	return attrs, mode
}

func extractBlockNesting(blockLike BlockLike) (map[string]Attr, map[string]Block, BlockNestingMode) {
	obj := reflect.ValueOf(blockLike)
	nestedObj := callGetter(obj, "GetNestedObject") // fwschema.NestedBlockObject
	nestingMode := callGetter(obj, "GetNestingMode")
	underlyingAttributes := callGetter(nestedObj, "GetAttributes") // fwschema.UnderlyingAttributes
	blockMap := callGetter(nestedObj, "GetBlocks")                 // map[string]fwschema.Block
	blocks := map[string]Block{}
	iter := blockMap.MapRange()
	for iter.Next() {
		key := iter.Key().Interface().(string)
		block := FromBlockLike(iter.Value().Interface().(BlockLike))
		blocks[key] = block
	}
	attrs := fromUnderlyingAttributes(underlyingAttributes)
	mode := BlockNestingMode(asUInt8(nestingMode))
	return attrs, blocks, mode
}

func asUInt8(v reflect.Value) uint8 {
	return v.Convert(reflect.TypeOf(uint8(0))).Interface().(uint8)
}

func hasMethod(obj reflect.Value, method string) bool {
	return obj.MethodByName(method).IsValid()
}

func callGetter(obj reflect.Value, method string) reflect.Value {
	m := obj.MethodByName(method)
	contract.Assertf(m.IsValid(), "No method %q on type %s", method, obj.Type())
	result := m.Call(nil)
	contract.Assertf(len(result) == 1, "Expected %q method to return 1 value", method)
	return result[0]
}

func fromUnderlyingAttributes(underlyingAttributes reflect.Value) map[string]Attr {
	result := map[string]Attr{}

	mapIterator := underlyingAttributes.MapRange()
	for mapIterator.Next() {
		key := mapIterator.Key().Interface().(string)
		value := mapIterator.Value().Interface().(AttrLike)
		result[key] = FromAttrLike(value)
	}

	return result
}
