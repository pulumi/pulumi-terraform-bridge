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

type LookupResult struct {
	IsAttr         bool
	IsBlock        bool
	IsNestedObject bool
	Attr           Attr
	Block          Block
}

func LookupTerraformPath(schema Schema, path *tftypes.AttributePath) (LookupResult, error) {
	res, ok, err := tryLookupAttrOrBlock(schema, path)
	if err != nil {
		return res, err
	}
	if ok {
		return res, nil
	}

	// Indirectly for fwschema.NestedBlockObject or fwschema.NestedAttributeObject.
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

	return res, fmt.Errorf("LookupTerraformPath failed at path %s, got unexpected %v", path, res)
}

func tryLookupAttrOrBlock(schema Schema, path *tftypes.AttributePath) (LookupResult, bool, error) {
	res, remaining, err := tftypes.WalkAttributePath(schema, path)
	if err != nil {
		return LookupResult{}, false, fmt.Errorf("%v still remains in the path: %w", remaining, err)
	}
	attrLike, ok := res.(AttrLike)
	if ok {
		return LookupResult{IsAttr: true, Attr: FromAttrLike(attrLike)}, true, nil
	}
	blockLike, ok := res.(BlockLike)
	if ok {
		return LookupResult{IsBlock: true, Block: FromBlockLike(blockLike)}, true, nil
	}
	return LookupResult{}, false, nil
}
