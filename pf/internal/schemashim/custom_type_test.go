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
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi-terraform-bridge/pf/internal/pfutils"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

// This example is taken from aws:resourceexplorer/index:Index "timeouts" property.
func TestCustomTypeEmbeddingObjectType(t *testing.T) {
	type timeoutsType struct {
		types.ObjectType
	}

	raw := schema.SingleNestedBlock{
		Attributes: map[string]schema.Attribute{
			"create": schema.StringAttribute{},
			"read":   schema.StringAttribute{},
			"update": schema.StringAttribute{},
		},
		CustomType: timeoutsType{
			ObjectType: types.ObjectType{
				AttrTypes: map[string]attr.Type{
					"create": types.StringType,
					"read":   types.StringType,
					"update": types.StringType,
				},
			},
		},
	}

	shimmed := &blockSchema{"key", pfutils.FromBlockLike(raw)}
	assert.Equal(t, shim.TypeMap, shimmed.Type())
	assert.NotNil(t, shimmed.Elem())
	_, isPseudoResource := shimmed.Elem().(shim.Resource)
	assert.Truef(t, isPseudoResource, "expected shim.Elem() to be of type shim.Resource, encoding an object type")

	create := shimmed.Elem().(shim.Resource).Schema().Get("create")
	assert.Equal(t, shim.TypeString, create.Type())
}

func TestCustomListType(t *testing.T) {
	ctx := context.Background()

	raw := schema.ListNestedBlock{
		CustomType: newListNestedObjectTypeOf[searchFilterModel](ctx, types.ObjectType{
			AttrTypes: map[string]attr.Type{
				"filter_string": basetypes.StringType{},
			},
		}),
		NestedObject: schema.NestedBlockObject{
			Attributes: map[string]schema.Attribute{
				"filter_string": schema.StringAttribute{
					Required: true,
				},
			},
		},
	}

	shimmed := &blockSchema{"key", pfutils.FromBlockLike(raw)}
	assert.Equal(t, shim.TypeList, shimmed.Type())
	assert.NotNil(t, shimmed.Elem())
	_, isPseudoResource := shimmed.Elem().(shim.Resource)
	assert.Truef(t, isPseudoResource, "expected shim.Elem() to be of type shim.Resource, encoding an object type")

	create := shimmed.Elem().(shim.Resource).Schema().Get("filter_string")
	assert.Equal(t, shim.TypeString, create.Type())
}

func TestCustomListAttribute(t *testing.T) {
	ctx := context.Background()

	raw := schema.ListNestedAttribute{
		CustomType: newListNestedObjectTypeOf[searchFilterModel](ctx, types.ObjectType{
			AttrTypes: map[string]attr.Type{
				"filter_string": basetypes.StringType{},
			},
		}),
		NestedObject: schema.NestedAttributeObject{
			Attributes: map[string]schema.Attribute{
				"filter_string": schema.StringAttribute{
					Required: true,
				},
			},
		},
	}

	shimmed := &attrSchema{"key", pfutils.FromAttrLike(raw)}
	assert.Equal(t, shim.TypeList, shimmed.Type())
	assert.NotNil(t, shimmed.Elem())
	_, isPseudoResource := shimmed.Elem().(shim.Schema)
	assert.Truef(t, isPseudoResource, "expected shim.Elem() to be of type shim.Resource, encoding an object type")

	create := shimmed.Elem().(shim.Schema).Elem().(shim.Resource).Schema().Get("filter_string")
	assert.Equal(t, shim.TypeString, create.Type())
}

func TestCustomSetType(t *testing.T) {
	ctx := context.Background()

	raw := schema.SetNestedBlock{
		CustomType: newSetNestedObjectTypeOf[searchFilterModel](ctx, types.ObjectType{
			AttrTypes: map[string]attr.Type{
				"filter_string": basetypes.StringType{},
			},
		}),
		NestedObject: schema.NestedBlockObject{
			Attributes: map[string]schema.Attribute{
				"filter_string": schema.StringAttribute{
					Required: true,
				},
			},
		},
	}

	shimmed := &blockSchema{"key", pfutils.FromBlockLike(raw)}
	assert.Equal(t, shim.TypeSet, shimmed.Type())
	assert.NotNil(t, shimmed.Elem())
	_, isPseudoResource := shimmed.Elem().(shim.Resource)
	assert.Truef(t, isPseudoResource, "expected shim.Elem() to be of type shim.Resource, encoding an object type")

	create := shimmed.Elem().(shim.Resource).Schema().Get("filter_string")
	assert.Equal(t, shim.TypeString, create.Type())
}

type searchFilterModel struct {
	FilterString types.String `tfsdk:"filter_string"`
}

type listNestedObjectTypeOf[T any] struct {
	basetypes.ListType
}

type setNestedObjectTypeOf[T any] struct {
	basetypes.SetType
}

var (
	_ basetypes.ListTypable = (*listNestedObjectTypeOf[struct{}])(nil)
)

func newListNestedObjectTypeOf[T any](ctx context.Context, elemType attr.Type) listNestedObjectTypeOf[T] {
	return listNestedObjectTypeOf[T]{basetypes.ListType{ElemType: elemType}}
}

func newSetNestedObjectTypeOf[T any](ctx context.Context, elemType attr.Type) setNestedObjectTypeOf[T] {
	return setNestedObjectTypeOf[T]{basetypes.SetType{ElemType: elemType}}
}
