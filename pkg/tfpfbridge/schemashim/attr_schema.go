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
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-go/tftypes"

	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

type attrSchema struct {
	key  string
	attr attr
}

var _ shim.Schema = (*typeSchema)(nil)

func (s *attrSchema) Type() shim.ValueType {
	ctx := context.TODO()
	ty := s.attr.FrameworkType().TerraformType(ctx)
	vt, err := convertType(ctx, ty)
	if err != nil {
		panic(err)
	}
	return vt
}

func (s *attrSchema) Optional() bool {
	return s.attr.IsOptional()
}

func (s *attrSchema) Required() bool {
	return s.attr.IsRequired()
}

func (*attrSchema) Default() interface{}                { panic("TODO") }
func (*attrSchema) DefaultFunc() shim.SchemaDefaultFunc { panic("TODO") }
func (*attrSchema) DefaultValue() (interface{}, error)  { panic("TODO") }

func (s *attrSchema) Description() string {
	return s.attr.GetMarkdownDescription()
}

func (s *attrSchema) Computed() bool {
	return s.attr.IsComputed()
}

func (s *attrSchema) ForceNew() bool {
	// TODO is there a way to detect this?
	//
	// Plugin Framework providers use plan modifiers to indicate something similar:
	// https://developer.hashicorp.com/terraform/plugin/framework/resources/plan-modification#attribute-plan-modification
	//
	// Detecting it may be tricky.
	return false
}

func (*attrSchema) StateFunc() shim.SchemaStateFunc { panic("TODO") }

// Needs to return a shim.Schema, a shim.Resource, or nil.
func (s *attrSchema) Elem() interface{} {
	ctx := context.TODO()
	t := s.attr.FrameworkType().TerraformType(ctx)
	switch {
	case t.Is(tftypes.Bool):
		return nil
	case t.Is(tftypes.String):
		return nil
	case t.Is(tftypes.Number):
		return nil
	case t.Is(tftypes.Map{}):
		mT := t.(tftypes.Map)
		var schema shim.Schema = &typeSchema{mT.ElementType}
		return schema
	case t.Is(tftypes.List{}):
		lT := t.(tftypes.List)
		var schema shim.Schema = &typeSchema{lT.ElementType}
		return schema
	case t.Is(tftypes.Object{}):
		// This case can be triggered through tfsdk.SingleNestedAttributes. Logically it defines an attribute
		// with a type that is an Object type. To encode the schema of the Object type in a way the shim layer
		// understands, Elem() needes to return a Resource value.
		//
		// See also: documentation on shim.Schema.Elem().
		return &objectPseudoResource{t.(tftypes.Object)}
	default:
		panic(fmt.Errorf("TODO: unhandled elem case: %v", t))
	}
}

func (*attrSchema) MaxItems() int {
	// TODO is there a way to find MaxItems?
	return 0
}

func (*attrSchema) MinItems() int {
	// TODO is there a way to find MinItems?
	return 0
}

func (*attrSchema) ConflictsWith() []string { panic("TODO") }
func (*attrSchema) ExactlyOneOf() []string  { panic("TODO") }

func (s *attrSchema) Deprecated() string {
	return s.attr.GetDeprecationMessage()
}

func (*attrSchema) Removed() string {
	return "" // TODO
}

func (s *attrSchema) Sensitive() bool {
	return s.attr.IsSensitive()
}

func (*attrSchema) UnknownValue() interface{}                          { panic("TODO") }
func (*attrSchema) SetElement(config interface{}) (interface{}, error) { panic("TODO") }
func (*attrSchema) SetHash(v interface{}) int                          { panic("TODO") }
