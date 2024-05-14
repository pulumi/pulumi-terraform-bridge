// Copyright 2016-2024, Pulumi Corporation.
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

package proto

import (
	"fmt"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

var _ = shim.Schema(attribute{})

type attribute struct{ attr tfprotov6.SchemaAttribute }

// Simple schema options

func (a attribute) Optional() bool      { return a.attr.Optional }
func (a attribute) Required() bool      { return a.attr.Required }
func (a attribute) Description() string { return a.attr.Description }
func (a attribute) Computed() bool      { return a.attr.Computed }
func (a attribute) ForceNew() bool      { return false }
func (a attribute) Sensitive() bool     { return a.attr.Sensitive }
func (a attribute) Removed() string     { return "" }
func (a attribute) Deprecated() string  { return deprecated(a.attr.Deprecated) }

// Type information

func (a attribute) MaxItems() int { return 0 }
func (a attribute) MinItems() int { return 0 }
func (a attribute) Type() shim.ValueType {
	t := a.attr.ValueType()
	switch {
	case t.Is(tftypes.Bool):
		return shim.TypeBool
	case t.Is(tftypes.Number):
		// TODO: It looks like this interface only exposes "number", not integer.
		//
		// We should see if there is a work-around here.
		return shim.TypeFloat
	case t.Is(tftypes.String):
		return shim.TypeString
	case t.Is(tftypes.List{}) || t.Is(tftypes.Set{}):
		return shim.TypeList
	case t.Is(tftypes.Map{}) || t.Is(tftypes.Object{}) || a.attr.NestedType != nil:
		return shim.TypeMap
	default:
		panic(fmt.Sprintf("UNKNOWN TYPE of %#v", t)) // TODO: Remove for release
		// return shim.TypeInvalid
	}
}

func (a attribute) Elem() interface{} {
	t := a.attr.ValueType()
	switch {
	case a.attr.NestedType != nil:
		return object{obj: *a.attr.NestedType}
	case t.Is(tftypes.Object{}):
		obj := t.(tftypes.Object)
		attrs := make([]*tfprotov6.SchemaAttribute, 0, len(obj.AttributeTypes))
		for k, v := range obj.AttributeTypes {
			_, optional := obj.OptionalAttributes[k]
			attrs = append(attrs, &tfprotov6.SchemaAttribute{
				Name:     k,
				Optional: optional,
				Type:     v,
			})
		}

		return object{obj: tfprotov6.SchemaObject{
			Attributes: attrs,
			Nesting:    tfprotov6.SchemaObjectNestingModeMap,
		}}
	}

	switch t := t.(type) {
	case tftypes.Set:
		return attribute{tfprotov6.SchemaAttribute{Type: t.ElementType}}
	case tftypes.Map:
		return attribute{tfprotov6.SchemaAttribute{Type: t.ElementType}}
	case tftypes.List:
		return attribute{tfprotov6.SchemaAttribute{Type: t.ElementType}}
	}

	return nil

}

// Defaults are applied in the provider binary, not here

func (a attribute) Default() interface{}                { return nil }
func (a attribute) DefaultFunc() shim.SchemaDefaultFunc { return nil }
func (a attribute) DefaultValue() (interface{}, error)  { return nil, nil }

func (a attribute) StateFunc() shim.SchemaStateFunc { return nil }
func (a attribute) ConflictsWith() []string         { return nil }
func (a attribute) ExactlyOneOf() []string          { return nil }

func (a attribute) SetElement(config interface{}) (interface{}, error) {
	panic("UNIMPLIMENTED")
}

func (a attribute) SetHash(v interface{}) int { panic("UNIMPLIMENTED") }
