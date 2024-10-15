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

	"github.com/hashicorp/terraform-plugin-framework/attr"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	pschema "github.com/hashicorp/terraform-plugin-framework/provider/schema"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

type SchemaType interface {
	tftypes.AttributePathStepper

	Type(context.Context) tftypes.Type

	// Resource schemas are versioned for [State Upgrade].
	//
	// [State Upgrade]: https://developer.hashicorp.com/terraform/plugin/framework/resources/state-upgrade
	ResourceSchemaVersion() int64
}

// Attr type works around not being able to link to fwschema.Schema from
// "github.com/hashicorp/terraform-plugin-framework/internal/fwschema"
type Schema interface {
	SchemaType

	Attrs() map[string]Attr
	Blocks() map[string]Block

	DeprecationMessage() string
	ResourceProtoSchema(ctx context.Context) (*tfprotov6.Schema, error)
}

func FromProviderSchema(x pschema.Schema) Schema {
	attrs := convertMap(FromProviderAttribute, x.Attributes)
	blocks := convertMap(FromProviderBlock, x.Blocks)
	return newSchemaAdapter(x, x.Type(), x.DeprecationMessage, attrs, blocks, nil)
}

func FromDataSourceSchema(x dschema.Schema) Schema {
	attrs := convertMap(FromDataSourceAttribute, x.Attributes)
	blocks := convertMap(FromDataSourceBlock, x.Blocks)
	return newSchemaAdapter(x, x.Type(), x.DeprecationMessage, attrs, blocks, nil)
}

func FromResourceSchema(x rschema.Schema) Schema {
	attrs := convertMap(FromResourceAttribute, x.Attributes)
	blocks := convertMap(FromResourceBlock, x.Blocks)
	return newSchemaAdapter(x, x.Type(), x.DeprecationMessage, attrs, blocks, &x)
}

type schemaAdapter struct {
	tftypes.AttributePathStepper
	attrType           attr.Type
	deprecationMessage string
	attrs              map[string]Attr
	blocks             map[string]Block

	// Will only be non-nil for resources.
	resourceSchema *rschema.Schema
}

var _ Schema = (*schemaAdapter)(nil)

func newSchemaAdapter(
	stepper tftypes.AttributePathStepper,
	t attr.Type,
	deprecationMessage string,
	attrs map[string]Attr,
	blocks map[string]Block,
	resourceSchema *rschema.Schema,
) *schemaAdapter {
	return &schemaAdapter{
		AttributePathStepper: stepper,
		attrType:             t,
		deprecationMessage:   deprecationMessage,
		attrs:                attrs,
		blocks:               blocks,
		resourceSchema:       resourceSchema,
	}
}

func (a *schemaAdapter) ResourceProtoSchema(ctx context.Context) (*tfprotov6.Schema, error) {
	if a.resourceSchema != nil {
		return convertResourceSchemaToProto(ctx, a.resourceSchema)
	}
	return nil, nil
}

func (a *schemaAdapter) ResourceSchemaVersion() int64 {
	if a.resourceSchema != nil {
		return a.resourceSchema.Version
	}
	return 0
}

func (a *schemaAdapter) DeprecationMessage() string {
	return a.deprecationMessage
}

func (a *schemaAdapter) Attrs() map[string]Attr {
	return a.attrs
}

func (a *schemaAdapter) Blocks() map[string]Block {
	return a.blocks
}

func (a *schemaAdapter) Type(ctx context.Context) tftypes.Type {
	return a.attrType.TerraformType(ctx)
}

func convertMap[A any, B any](f func(A) B, m map[string]A) map[string]B {
	r := map[string]B{}
	for k, v := range m {
		r[k] = f(v)
	}
	return r
}
