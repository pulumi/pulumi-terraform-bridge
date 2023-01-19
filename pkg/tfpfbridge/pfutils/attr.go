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
	"fmt"
	"reflect"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	pschema "github.com/hashicorp/terraform-plugin-framework/provider/schema"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// Attr type works around not being able to link to fwschema.Attribute from
// "github.com/hashicorp/terraform-plugin-framework/internal/fwschema"
//
// Most methods from fwschema.Attribute have simple signatures and are copied into attrLike interface. Casting to
// attrLike exposes these methods.
//
// GetAttributes method is special since it returns a NestedAttributes interface that is also internal and cannot be
// linked to. Instead, NestedAttriutes information is recorded in a dedicated new field.
type Attr interface {
	AttrLike
	IsNested() bool
	Nested() map[string]Attr
	NestingMode() NestingMode
}

type AttrLike interface {
	IsComputed() bool
	IsOptional() bool
	IsRequired() bool
	IsSensitive() bool
	GetDeprecationMessage() string
	GetDescription() string
	GetMarkdownDescription() string
	GetType() attr.Type
}

func FromProviderAttribute(x pschema.Attribute) Attr {
	return FromAttrLike(x)
}

func FromDataSourceAttribute(x dschema.Attribute) Attr {
	return FromAttrLike(x)
}

func FromResourceAttribute(x rschema.Attribute) Attr {
	return FromAttrLike(x)
}

func FromAttrLike(attrLike AttrLike) Attr {
	nested, nestingMode := extractNestedAttributes(attrLike)
	return &attrAdapter{
		nested:      nested,
		nestingMode: nestingMode,
		AttrLike:    attrLike,
	}
}

func extractNestedAttributes(attrLike AttrLike) (map[string]Attr, NestingMode) {
	attrLikeValue := reflect.ValueOf(attrLike)

	// Check if attrLike implements fwschema.NestedAttribute. Use reflection because linkage is impossible.
	getNestedObject := attrLikeValue.MethodByName("GetNestedObject")
	if !getNestedObject.IsValid() {
		return nil, 0
	}
	getNestedObjectResult := getNestedObject.Call(nil)
	contract.Assertf(len(getNestedObjectResult) == 1,
		"Expected NestedAttribute.GetNestedObject() to return 1 value")

	// var nestedAttributeObject fwschema.NestedAttributeObject
	nestedAttributeObject := getNestedObjectResult[0]

	getAttributes := nestedAttributeObject.MethodByName("GetAttributes")
	contract.Assertf(getAttributes.IsValid(), "No NestedAttributeObject.GetAttributes method")

	getAttributesResult := getAttributes.Call(nil)
	contract.Assertf(len(getNestedObjectResult) == 1,
		"Expected NestedAttributeObject.GetAttributes to return 1 value")

	// type UnderlyingAttributes = map[string]fwschema.Attribute
	// var underlyingAttributes fwchema.UnderlyingAttributes
	underlyingAttributes := getAttributesResult[0]

	result := map[string]Attr{}

	mapIterator := underlyingAttributes.MapRange()
	for mapIterator.Next() {
		key := mapIterator.Key().Interface().(string)
		value := mapIterator.Value().Interface().(AttrLike)
		result[key] = FromAttrLike(value)
	}

	getNestingMode := nestedAttributeObject.MethodByName("GetNestingMode")
	contract.Assertf(getNestingMode.IsValid(), "No NestedAttributeObject.GetNestingMode method")

	getNestingModeResult := getNestingMode.Call(nil)
	contract.Assertf(len(getNestingModeResult) == 1,
		"Expected NestedAttributeObject.GetNestingMode to return 1 value")

	nestingModeValue := getNestedObjectResult[0]
	nestingMode := NestingMode(nestingModeValue.Interface().(uint8))

	return result, nestingMode
}

type attrAdapter struct {
	nested      map[string]Attr
	nestingMode NestingMode
	AttrLike
}

var _ Attr = (*attrAdapter)(nil)

func (a *attrAdapter) IsNested() bool {
	return a.nested != nil
}

func (a *attrAdapter) Nested() map[string]Attr {
	return a.nested
}

func (a *attrAdapter) NestingMode() NestingMode {
	return a.nestingMode
}

type NestingMode uint8

const (
	NestingModeUnknown NestingMode = 0
	NestingModeSingle  NestingMode = 1
	NestingModeList    NestingMode = 2
	NestingModeSet     NestingMode = 3
	NestingModeMap     NestingMode = 4
)

func AttributeAtTerraformPath(schema Schema, path *tftypes.AttributePath) (Attr, error) {
	// schema needs to implement AttributePathStepper here
	res, remaining, err := tftypes.WalkAttributePath(schema, path)
	if err != nil {
		return nil, fmt.Errorf("%v still remains in the path: %w", remaining, err)
	}
	attrLike, ok := res.(AttrLike)
	if !ok {
		msg := "expected WalkAttributePath to return an AttrLike at path %s, got: %s"
		return nil, fmt.Errorf(msg, path, reflect.TypeOf(res))
	}
	return FromAttrLike(attrLike), nil
}
