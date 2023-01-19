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
	"context"
	"fmt"
	"reflect"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	pschema "github.com/hashicorp/terraform-plugin-framework/provider/schema"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// Attr type works around not being able to link to fwschema.Schema from
// "github.com/hashicorp/terraform-plugin-framework/internal/fwschema"
type Schema interface {
	tftypes.AttributePathStepper

	Type() attr.Type

	Attrs() map[string]Attr
	Blocks() map[string]Block

	DeprecationMessage() string
	AttributeAtPath(context.Context, path.Path) (Attr, diag.Diagnostics)
}

func FromProviderSchema(x pschema.Schema) Schema {
	attrs := convertMap(FromProviderAttribute, x.Attributes)
	blocks := convertMap(FromProviderBlock, x.Blocks)
	return newSchemaAdapter(x, x.Type(), x.DeprecationMessage, attrs, blocks, x.AttributeAtPath)
}

func FromDataSourceSchema(x dschema.Schema) Schema {
	attrs := convertMap(FromDataSourceAttribute, x.Attributes)
	blocks := convertMap(FromDataSourceBlock, x.Blocks)
	return newSchemaAdapter(x, x.Type(), x.DeprecationMessage, attrs, blocks, x.AttributeAtPath)
}

func FromResourceSchema(x rschema.Schema) Schema {
	attrs := convertMap(FromResourceAttribute, x.Attributes)
	blocks := convertMap(FromResourceBlock, x.Blocks)
	return newSchemaAdapter(x, x.Type(), x.DeprecationMessage, attrs, blocks, x.AttributeAtPath)
}

type schemaAdapter[T any] struct {
	tftypes.AttributePathStepper
	attrType           attr.Type
	deprecationMessage string
	attrs              map[string]Attr
	blocks             map[string]Block
	attributeAtPath    func(context.Context, path.Path) (T, diag.Diagnostics)
}

var _ Schema = (*schemaAdapter[interface{}])(nil)

func newSchemaAdapter[T any](
	stepper tftypes.AttributePathStepper,
	t attr.Type,
	deprecationMessage string,
	attrs map[string]Attr,
	blocks map[string]Block,
	atPath func(context.Context, path.Path) (T, diag.Diagnostics),
) *schemaAdapter[T] {
	return &schemaAdapter[T]{
		AttributePathStepper: stepper,
		attrType:             t,
		deprecationMessage:   deprecationMessage,
		attributeAtPath:      atPath,
		attrs:                attrs,
		blocks:               blocks,
	}
}

func (a *schemaAdapter[T]) DeprecationMessage() string {
	return a.deprecationMessage
}

func (a *schemaAdapter[T]) AttributeAtPath(ctx context.Context, p path.Path) (Attr, diag.Diagnostics) {
	raw, diag := a.attributeAtPath(ctx, p)
	var rawbox interface{} = raw
	attrLike, ok := rawbox.(AttrLike)
	if !ok {
		detail := fmt.Sprintf("Expected an AttrLike at path %s, got %s", p, reflect.TypeOf(raw))
		diag.AddError("Bad attributeAtPath result", detail)
	}
	return FromAttrLike(attrLike), diag
}

func (a *schemaAdapter[T]) Attrs() map[string]Attr {
	return a.attrs
}

func (a *schemaAdapter[T]) Blocks() map[string]Block {
	return a.blocks
}

func (a *schemaAdapter[T]) Type() attr.Type {
	return a.attrType
}

func convertMap[A any, B any](f func(A) B, m map[string]A) map[string]B {
	r := map[string]B{}
	for k, v := range m {
		r[k] = f(v)
	}
	return r
}
