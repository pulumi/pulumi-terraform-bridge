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
	attr attr
}

var _ shim.Schema = (*typeSchema)(nil)

func (s *attrSchema) Type() shim.ValueType {
	ctx := context.TODO()
	ty := s.attr.GetType().TerraformType(ctx)
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
func (*attrSchema) Description() string                 { panic("TODO") }

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

// Needs to return a shim.Schema, a shim.Resource, or nil. IN the case of attrSchema it is always a
// shim.Schema specifically a &typeSchema, or else nil if the type has no element.
func (s *attrSchema) Elem() interface{} {
	ctx := context.TODO()
	t := s.attr.GetType().TerraformType(ctx)
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
	default:
		panic(fmt.Errorf("TODO: unhanded elem case: %v", t))
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

func (*attrSchema) Removed() string { panic("TODO") }

func (s *attrSchema) Sensitive() bool {
	return s.attr.IsSensitive()
}

func (*attrSchema) UnknownValue() interface{}                          { panic("TODO") }
func (*attrSchema) SetElement(config interface{}) (interface{}, error) { panic("TODO") }
func (*attrSchema) SetHash(v interface{}) int                          { panic("TODO") }
