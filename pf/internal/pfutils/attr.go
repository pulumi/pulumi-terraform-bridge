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

	"github.com/hashicorp/terraform-plugin-framework/attr"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	pschema "github.com/hashicorp/terraform-plugin-framework/provider/schema"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"

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
	HasNestedObject() bool
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

type attrAdapter struct {
	nested      map[string]Attr
	nestingMode NestingMode
	AttrLike
}

var _ Attr = (*attrAdapter)(nil)

func (a *attrAdapter) HasNestedObject() bool {
	switch a.NestingMode() {
	case NestingModeList, NestingModeMap, NestingModeSet:
		return true
	default:
		return false
	}
}

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

// Classifies the results of LookupTerraformPath. The interesting cases:
//
// 1. IsAttr=true, and Attr is set; this means an Attribute was found
// 2. IsBlock=true, and Block is set; this means a Block was found
// 3. IsNestedObject=true; this means the path is a nested object one level down from Attr or Block
// 4. IsMisc=true; this groups all other cases, such as resolving to a simple atomic type as String
type LookupResult struct {
	IsAttr         bool
	IsBlock        bool
	IsNestedObject bool
	IsMisc         bool
	Attr           Attr
	Block          Block
}

// Drills down a Schema with a given AttributePath to classify what is found at that path, see LookupResult.
func LookupTerraformPath(schema Schema, path *tftypes.AttributePath) (LookupResult, error) {
	res, ok, err := tryLookupAttrOrBlock(schema, path)
	if err != nil {
		return res, err
	}
	if ok {
		return res, nil
	}

	// Perhaps our parent path is an Attribute or a Block with a nested object, then path is a path to a nested
	// object. This is another way to detect, indirectly, if res is a fwschema.NestedBlockObject or
	// fwschema.NestedAttributeObject, without relying on reflection.
	if parent := path.WithoutLastStep(); parent != nil {
		parentRes, parentOk, parentErr := tryLookupAttrOrBlock(schema, parent)
		if parentErr == nil && parentOk {
			if parentRes.IsAttr && parentRes.Attr.HasNestedObject() {
				return LookupResult{IsNestedObject: true}, nil
			}
			if parentRes.IsBlock && parentRes.Block.HasNestedObject() {
				return LookupResult{IsNestedObject: true}, nil
			}
		}
	}

	return LookupResult{IsMisc: true}, nil
}

func tryLookupAttrOrBlock(schema Schema, path *tftypes.AttributePath) (LookupResult, bool, error) {
	res, remaining, err := tftypes.WalkAttributePath(schema, path)
	if err != nil {
		return LookupResult{}, false, fmt.Errorf("%v still remains in the path: %w", remaining, err)
	}
	switch res := res.(type) {
	case AttrLike:
		return LookupResult{IsAttr: true, Attr: FromAttrLike(res)}, true, nil
	case BlockLike:
		return LookupResult{IsBlock: true, Block: FromBlockLike(res)}, true, nil
	}
	return LookupResult{}, false, nil
}
