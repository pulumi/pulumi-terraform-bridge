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
	"fmt"
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

func TestCustomObject(t *testing.T) {
	ctx := context.Background()

	obj := newObjectPseudoResource(NewObjectTypeOf[SomeType](ctx), nil, nil)

	s := obj.Schema().Get("s")
	assert.Equal(t, shim.TypeString, s.Type())
}

type SomeType struct {
	S types.String `tfsdk:"s"`
}

// --- custom object machinery ---
//
// copied from terraform-provider-aws/internal/framework/types/objectof.go
//
// Licensed under Apache Version 2.0 HashiCorp, Inc.

// objectTypeOf is the attribute type of an ObjectValueOf.
type objectTypeOf[T any] struct {
	basetypes.ObjectType
}

var (
	_ basetypes.ObjectTypable  = (*objectTypeOf[struct{}])(nil)
	_ basetypes.ObjectValuable = (*ObjectValueOf[struct{}])(nil)
)

func NewObjectTypeOf[T any](ctx context.Context) basetypes.ObjectTypable {
	return objectTypeOf[T]{basetypes.ObjectType{AttrTypes: AttributeTypesMust[T](ctx)}}
}

func (t objectTypeOf[T]) Equal(o attr.Type) bool {
	other, ok := o.(objectTypeOf[T])

	if !ok {
		return false
	}

	return t.ObjectType.Equal(other.ObjectType)
}

func (t objectTypeOf[T]) String() string {
	var zero T
	return fmt.Sprintf("ObjectTypeOf[%T]", zero)
}

func (t objectTypeOf[T]) ValueFromObject(
	ctx context.Context, in basetypes.ObjectValue,
) (basetypes.ObjectValuable, diag.Diagnostics) {
	var diags diag.Diagnostics

	if in.IsNull() {
		return NewObjectValueOfNull[T](ctx), diags
	}
	if in.IsUnknown() {
		return NewObjectValueOfUnknown[T](ctx), diags
	}

	objectValue, d := basetypes.NewObjectValue(AttributeTypesMust[T](ctx), in.Attributes())
	diags.Append(d...)
	if diags.HasError() {
		return NewObjectValueOfUnknown[T](ctx), diags
	}

	value := ObjectValueOf[T]{
		ObjectValue: objectValue,
	}

	return value, diags
}

func (t objectTypeOf[T]) ValueFromTerraform(ctx context.Context, in tftypes.Value) (attr.Value, error) {
	attrValue, err := t.ObjectType.ValueFromTerraform(ctx, in)

	if err != nil {
		return nil, err
	}

	objectValue, ok := attrValue.(basetypes.ObjectValue)

	if !ok {
		return nil, fmt.Errorf("unexpected value type of %T", attrValue)
	}

	objectValuable, diags := t.ValueFromObject(ctx, objectValue)

	if diags.HasError() {
		return nil, fmt.Errorf("unexpected error converting ObjectValue to ObjectValuable: %v", diags)
	}

	return objectValuable, nil
}

func (t objectTypeOf[T]) ValueType(ctx context.Context) attr.Value {
	return ObjectValueOf[T]{}
}

// ObjectValueOf represents a Terraform Plugin Framework Object value whose corresponding Go type is the structure T.
type ObjectValueOf[T any] struct {
	basetypes.ObjectValue
}

func (v ObjectValueOf[T]) Equal(o attr.Value) bool {
	other, ok := o.(ObjectValueOf[T])

	if !ok {
		return false
	}

	return v.ObjectValue.Equal(other.ObjectValue)
}

func (v ObjectValueOf[T]) Type(ctx context.Context) attr.Type {
	return NewObjectTypeOf[T](ctx)
}

func NewObjectValueOfNull[T any](ctx context.Context) ObjectValueOf[T] {
	return ObjectValueOf[T]{ObjectValue: basetypes.NewObjectNull(AttributeTypesMust[T](ctx))}
}

func NewObjectValueOfUnknown[T any](ctx context.Context) ObjectValueOf[T] {
	return ObjectValueOf[T]{ObjectValue: basetypes.NewObjectUnknown(AttributeTypesMust[T](ctx))}
}

func NewObjectValueOf[T any](ctx context.Context, t *T) ObjectValueOf[T] {
	return ObjectValueOf[T]{ObjectValue: must(basetypes.NewObjectValueFrom(ctx, AttributeTypesMust[T](ctx), t))}
}

// AttributeTypes returns a map of attribute types for the specified type T.
// T must be a struct and reflection is used to find exported fields of T with the `tfsdk` tag.
func AttributeTypes[T any](ctx context.Context) (map[string]attr.Type, error) {
	var t T
	val := reflect.ValueOf(t)
	typ := val.Type()

	if typ.Kind() == reflect.Ptr && typ.Elem().Kind() == reflect.Struct {
		val = reflect.New(typ.Elem()).Elem()
		typ = typ.Elem()
	}

	if typ.Kind() != reflect.Struct {
		return nil, fmt.Errorf("%T has unsupported type: %s", t, typ)
	}

	attributeTypes := make(map[string]attr.Type)
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if field.PkgPath != "" {
			continue // Skip unexported fields.
		}
		tag := field.Tag.Get(`tfsdk`)
		if tag == "-" {
			continue // Skip explicitly excluded fields.
		}
		if tag == "" {
			return nil, fmt.Errorf(`%T needs a struct tag for "tfsdk" on %s`, t, field.Name)
		}

		if v, ok := val.Field(i).Interface().(attr.Value); ok {
			attributeTypes[tag] = v.Type(ctx)
		}
	}

	fmt.Printf("Attrs = %#v (on %T)\n", attributeTypes, t)

	return attributeTypes, nil
}

func AttributeTypesMust[T any](ctx context.Context) map[string]attr.Type {
	return must(AttributeTypes[T](ctx))
}

func must[T any, E any](t T, err E) T {
	if v := reflect.ValueOf(err); v.IsValid() && !v.IsZero() {
		panic(err)
	}
	return t
}
