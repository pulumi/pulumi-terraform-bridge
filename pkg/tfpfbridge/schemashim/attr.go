// Copyright 2016-2022, Pulumi Corporation.
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

package schemashim

import (
	"fmt"
	"reflect"

	pfattr "github.com/hashicorp/terraform-plugin-framework/attr"
)

type attr interface {
	attrLike
	NestedAttributes() map[string]attr
}

func newAttr(inner attrLike) attr {
	nested := map[string]attr{}
	for k, v := range getAttributes(inner) {
		nested[k] = newAttr(v)
	}
	return &attrImpl{
		attrLike: inner,
		nested:   nested,
	}
}

type attrLike interface {
	FrameworkType() pfattr.Type
	IsComputed() bool
	IsOptional() bool
	IsRequired() bool
	IsSensitive() bool
	GetDeprecationMessage() string
	GetDescription() string
	GetMarkdownDescription() string
}

// Dropping to reflection here since GetAttributes() refers to internal types from terraform-plugin-framework and cannot
// be easily called safely.
func getAttributes(a interface{}) map[string]attrLike {
	attrs := map[string]attrLike{}
	attrMap := callGetAttributes(reflect.ValueOf(a))

	if attrMap.IsNil() {
		return attrs
	}

	// Sometimes need to call a.GetAttributes().GetAttributes() to get to a map.
	if attrMap.Type().Kind() != reflect.Map {
		attrMap = callGetAttributes(attrMap)
	}

	if attrMap.Type().Kind() != reflect.Map {
		panic(fmt.Sprintf("Expecting a map, got %v", attrMap.Type().String()))
	}

	if attrMap.IsNil() {
		return attrs
	}

	attrMapIterator := attrMap.MapRange()
	for attrMapIterator.Next() {
		key := attrMapIterator.Key().Interface().(string)
		value := attrMapIterator.Value().Interface().(attrLike)
		attrs[key] = value
	}
	return attrs
}

func callGetAttributes(v reflect.Value) reflect.Value {
	m := v.MethodByName("GetAttributes")
	t := v.Type().String()
	if !m.IsValid() {
		panic(fmt.Sprintf("Expected a value of type %s to implement GetAttributes method", t))
	}
	attrCallResult := m.Call(nil)
	if len(attrCallResult) != 1 {
		panic(fmt.Sprintf("Expected GetAttributes from type %s to return 1 value", t))
	}
	return attrCallResult[0]
}

type attrImpl struct {
	attrLike
	nested map[string]attr
}

func (i *attrImpl) NestedAttributes() map[string]attr {
	return i.nested
}

var _ attr = (*attrImpl)(nil)
