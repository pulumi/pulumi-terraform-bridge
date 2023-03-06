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

package testprovider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/pulumi/pulumi-terraform-bridge/pf/internal/pfutils"
)

type TupleType struct {
	Types []attr.Type
}

func (c TupleType) attr(i int) schema.Attribute {
	switch t := c.Types[i].(type) {
	case basetypes.BoolType:
		return schema.BoolAttribute{}
	case basetypes.StringType:
		return schema.StringAttribute{}
	default:
		panic(fmt.Sprintf("Unhandled type: %T", t))
	}
}

func (c TupleType) tftype(ctx context.Context) tftypes.Tuple {
	types := make([]tftypes.Type, len(c.Types))
	for i, v := range c.Types {
		types[i] = v.TerraformType(ctx)
	}
	return tftypes.Tuple{
		ElementTypes: types,
	}
}

func (c TupleType) ApplyTerraform5AttributePathStep(step tftypes.AttributePathStep) (any, error) {
	i, ok := step.(tftypes.ElementKeyInt)
	if !ok {
		return nil, fmt.Errorf("cannot index a tuple with %#v", step)
	}
	if int(i) >= len(c.Types) {
		return nil, fmt.Errorf("index %d out of bounds on tuple with length %d", i, len(c.Types))
	}
	return c.attr(int(i)), nil
}

var _ attr.TypeWithElementTypes = ((*TupleType)(nil))
var _ pfutils.BlockLike = ((*TupleType)(nil))

func (c TupleType) GetDeprecationMessage() string  { return "" }
func (c TupleType) GetDescription() string         { return "" }
func (c TupleType) GetMarkdownDescription() string { return "" }
func (c TupleType) Type() attr.Type                { panic("NA") }

func (c TupleType) Equal(o attr.Type) bool {
	tt, ok := o.(TupleType)
	if !ok {
		return false
	}
	if len(tt.Types) != len(c.Types) {
		return false
	}
	for i := range c.Types {
		if !c.Types[i].Equal(tt.Types[i]) {
			return false
		}
	}
	return true
}

func (c TupleType) String() string {
	return c.tftype(context.Background()).String()
}

func (c TupleType) TerraformType(ctx context.Context) tftypes.Type {
	return c.tftype(ctx)
}

func (c TupleType) ElementTypes() []attr.Type {
	return c.Types
}

func (c TupleType) ValueFromTerraform(ctx context.Context, val tftypes.Value) (attr.Value, error) {
	if !val.IsKnown() {
		return TupleValue{
			state: attr.ValueStateUnknown,
			typ:   c,
		}, nil
	}
	if !val.IsNull() {
		return TupleValue{
			state: attr.ValueStateNull,
			typ:   c,
		}, nil
	}
	var values []tftypes.Value
	if err := val.As(&values); err != nil {
		return nil, err
	}

	return TupleValue{
		state: attr.ValueStateKnown,
		typ:   c,
		val:   values,
	}, nil
}

func (c TupleType) ValueType(context.Context) attr.Value {
	return TupleValue{typ: c}
}

func (c TupleType) WithElementTypes(types []attr.Type) attr.TypeWithElementTypes {
	return TupleType{Types: types}
}

func (c TupleValue) IsNull() bool                   { return c.state == attr.ValueStateNull }
func (c TupleValue) IsUnknown() bool                { return c.state == attr.ValueStateUnknown }
func (c TupleValue) String() string                 { return fmt.Sprintf("%#v", c) }
func (c TupleValue) Type(context.Context) attr.Type { return c.typ }

func (c TupleValue) ToTerraformValue(ctx context.Context) (tftypes.Value, error) {
	t := c.typ.tftype(ctx)
	if c.state == attr.ValueStateKnown {
		return tftypes.NewValue(t, tftypes.UnknownValue), nil
	}
	if c.state == attr.ValueStateNull {
		return tftypes.NewValue(t, nil), nil
	}
	if err := tftypes.ValidateValue(t, c.val); err != nil {
		return tftypes.NewValue(t, nil), err
	}
	return tftypes.NewValue(t, c.val), nil
}

type TupleValue struct {
	state attr.ValueState
	typ   TupleType
	val   []tftypes.Value
}

func (c TupleValue) Equal(o attr.Value) bool {
	oV, ok := o.(TupleValue)
	if !ok {
		return false
	}
	if !c.typ.Equal(oV.typ) {
		return false
	}
	if c.IsNull() && o.IsNull() {
		return true
	}
	if c.IsUnknown() && o.IsUnknown() {
		return true
	}
	for i := range c.val {
		if !c.val[i].Equal(oV.val[i]) {
			return false
		}
	}
	return true
}
