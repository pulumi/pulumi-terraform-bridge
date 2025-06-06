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

package schemashim

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/internal/pfutils"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

func TestObjectAttribute(t *testing.T) {
	t.Parallel()
	objectAttr := schema.ObjectAttribute{
		AttributeTypes: map[string]attr.Type{
			"s": basetypes.StringType{},
		},
	}
	shimmed := newAttrSchema("key", pfutils.FromAttrLike(objectAttr))
	assertIsObjectType(t, shimmed)
	s := shimmed.Elem().(shim.Resource).Schema().Get("s")
	assert.Equal(t, shim.TypeString, s.Type())
}

func TestTypeSchemaDescriptionIsEmpty(t *testing.T) {
	t.Parallel()
	shimmedType := &typeSchema{
		t:      basetypes.StringType{},
		nested: nil,
	}
	assert.Equal(t, shimmedType.Description(), "")
}

func TestSingleNestedBlock(t *testing.T) {
	t.Parallel()
	b := schema.SingleNestedBlock{
		Attributes: simpleObjectAttributes(),
	}
	shimmed := newBlockSchema("key", pfutils.FromResourceBlock(b))
	assertIsObjectType(t, shimmed)
	assert.Equal(t, "obj[c=str,co=str,desc=str,o=str,r=str]", schemaLogicalType(shimmed).String())
	r, ok := shimmed.Elem().(shim.Resource)
	require.True(t, ok, "Single-nested TF blocks should be represented as Elem() shim.Resource")
	assertHasSimpleObjectAttributes(t, r)
}

func TestListNestedBlock(t *testing.T) {
	t.Parallel()
	b := schema.ListNestedBlock{
		NestedObject: schema.NestedBlockObject{
			Attributes: simpleObjectAttributes(),
		},
	}
	shimmed := newBlockSchema("key", pfutils.FromResourceBlock(b))
	assert.Equal(t, "list[obj[c=str,co=str,desc=str,o=str,r=str]]", schemaLogicalType(shimmed).String())
	r, ok := shimmed.Elem().(shim.Resource)
	require.True(t, ok, "List-nested TF blocks should be represented as Elem() shim.Resource")
	assertHasSimpleObjectAttributes(t, r)
}

func TestSetNestedBlock(t *testing.T) {
	t.Parallel()
	b := schema.SetNestedBlock{
		NestedObject: schema.NestedBlockObject{
			Attributes: simpleObjectAttributes(),
		},
	}
	shimmed := newBlockSchema("key", pfutils.FromResourceBlock(b))
	assert.Equal(t, "set[obj[c=str,co=str,desc=str,o=str,r=str]]", schemaLogicalType(shimmed).String())
	r, ok := shimmed.Elem().(shim.Resource)
	require.True(t, ok, "Set-nested TF blocks should be represented as Elem() shim.Resource")
	assertHasSimpleObjectAttributes(t, r)
}

func TestDeeplyNestedBlock(t *testing.T) {
	t.Parallel()
	b := schema.ListNestedBlock{
		NestedObject: schema.NestedBlockObject{
			Blocks: map[string]schema.Block{
				"action_parameters": schema.ListNestedBlock{
					NestedObject: schema.NestedBlockObject{
						Blocks: map[string]schema.Block{
							"phases": schema.ListNestedBlock{
								NestedObject: schema.NestedBlockObject{
									Attributes: map[string]schema.Attribute{
										"p1": schema.BoolAttribute{
											Optional: true,
										},
										"p2": schema.BoolAttribute{
											Optional: true,
										},
									},
								},
							},
						},
						Attributes: map[string]schema.Attribute{
							"automatic_https_rewrites": schema.BoolAttribute{
								Optional: true,
							},
							"bic": schema.BoolAttribute{
								Optional: true,
							},
						},
					},
				},
			},
		},
	}
	shimmed := newBlockSchema("key", pfutils.FromResourceBlock(b))
	phasesType := "list[obj[p1=bool,p2=bool]]"
	topType := fmt.Sprintf("list[obj[action_parameters="+
		"list[obj[automatic_https_rewrites=bool,bic=bool,phases=%s]]]]", phasesType)
	assert.Equal(t, topType, schemaLogicalType(shimmed).String())
}

func simpleObjectAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"o": schema.StringAttribute{
			Optional: true,
		},
		"r": schema.StringAttribute{
			Required: true,
		},
		"c": schema.StringAttribute{
			Computed: true,
		},
		"co": schema.StringAttribute{
			Computed: true,
			Optional: true,
		},
		"desc": schema.StringAttribute{
			Description: "I am a description",
		},
	}
}

func assertHasSimpleObjectAttributes(t *testing.T, r shim.Resource) {
	assert.True(t, r.Schema().Get("o").Optional(), "o is optional")
	assert.True(t, r.Schema().Get("c").Computed(), "c is computed")
	assert.True(t, r.Schema().Get("r").Required(), "r is required")
	assert.True(t, r.Schema().Get("co").Computed() && r.Schema().Get("co").Optional(), "co is computed and optional")
	assert.Equal(t, r.Schema().Get("desc").Description(), "I am a description")
}

func assertIsObjectType(t *testing.T, shimmed shim.Schema) {
	assert.Equal(t, shim.TypeMap, shimmed.Type())
	assert.NotNil(t, shimmed.Elem())
	_, isPseudoResource := shimmed.Elem().(shim.Resource)
	assert.Truef(t, isPseudoResource, "expected shim.Elem() to be of type shim.Resource, encoding an object type")
}

type logicalType interface {
	LogicalType()
	String() string
}

type listT struct {
	elem logicalType
}

func (listT) LogicalType() {}

func (t listT) String() string {
	return fmt.Sprintf("list[%s]", t.elem.String())
}

type mapT struct {
	elem logicalType
}

func (t mapT) String() string {
	return fmt.Sprintf("map[%s]", t.elem.String())
}

func (mapT) LogicalType() {}

type setT struct {
	elem logicalType
}

func (t setT) String() string {
	return fmt.Sprintf("set[%s]", t.elem.String())
}

func (setT) LogicalType() {}

type objT map[string]logicalType

func (objT) LogicalType() {}

func (t objT) String() string {
	var fields []string
	for k, v := range t {
		fields = append(fields, fmt.Sprintf("%s=%s", k, v.String()))
	}
	sort.Strings(fields)
	return fmt.Sprintf("obj[%s]", strings.Join(fields, ","))
}

type strT struct{}

func (strT) LogicalType() {}

func (t strT) String() string {
	return "str"
}

type intT struct{}

func (t intT) String() string {
	return "int"
}

func (intT) LogicalType() {}

type boolT struct{}

func (t boolT) String() string {
	return "bool"
}

func (boolT) LogicalType() {}

type floatT struct{}

func (floatT) LogicalType() {}

func (t floatT) String() string {
	return "float"
}

type unknownT struct{}

func (unknownT) LogicalType() {}

func (t unknownT) String() string {
	return "unknown"
}

func schemaLogicalType(s shim.Schema) logicalType {
	switch elem := s.Elem().(type) {
	case shim.Resource:
		t := objT(make(map[string]logicalType))
		elem.Schema().Range(func(key string, value shim.Schema) bool {
			t[key] = schemaLogicalType(value)
			return true
		})
		switch s.Type() {
		case shim.TypeMap:
			return t
		case shim.TypeList:
			return &listT{t}
		case shim.TypeSet:
			return &setT{t}
		default:
			panic("invalid combination, Elem() is a Resource but Type() is not a collection")
		}
	case shim.Schema:
		switch s.Type() {
		case shim.TypeList:
			return &listT{schemaLogicalType(elem)}
		case shim.TypeMap:
			return &mapT{schemaLogicalType(elem)}
		case shim.TypeSet:
			return &setT{schemaLogicalType(elem)}
		default:
			panic("invalid combination, Elem() is a Schema but Type() is not a collection")
		}
	case nil:
		switch s.Type() {
		case shim.TypeList:
			return &listT{unknownT{}}
		case shim.TypeMap:
			return &mapT{unknownT{}}
		case shim.TypeSet:
			return &setT{unknownT{}}
		case shim.TypeString:
			return &strT{}
		case shim.TypeBool:
			return &boolT{}
		case shim.TypeInt:
			return &intT{}
		case shim.TypeFloat:
			return &floatT{}
		default:
			panic(fmt.Sprintf("unknown type: %v", s.Type()))
		}
	default:
		panic("invalid Elem()")
	}
}
