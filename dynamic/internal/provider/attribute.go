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

package provider

import (
	"fmt"

	otshim "github.com/opentofu/opentofu/shim"
	"github.com/zclconf/go-cty/cty"

	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

var _ = shim.Schema(attribute{})

type attribute struct{ attr otshim.SchemaAttribute }

// Simple schema options

func (a attribute) Optional() bool      { return a.attr.Optional }
func (a attribute) Required() bool      { return a.attr.Required }
func (a attribute) Description() string { return a.attr.Description }
func (a attribute) Computed() bool      { return a.attr.Computed }
func (a attribute) ForceNew() bool      { return false }
func (a attribute) Deprecated() string  { return a.attr.Description }
func (a attribute) Sensitive() bool     { return a.attr.Sensitive }
func (a attribute) Removed() string     { return "" }

// Type information

func (a attribute) MaxItems() int { return 0 }
func (a attribute) MinItems() int { return 0 }
func (a attribute) Type() shim.ValueType {
	switch {
	case a.attr.Type.Equals(cty.Bool):
		return shim.TypeBool
	case a.attr.Type.Equals(cty.Number):
		// TODO: It looks like this interface only exposes "number", not integer.
		//
		// We should see if there is a work-around here.
		return shim.TypeFloat
	case a.attr.Type.Equals(cty.String):
		return shim.TypeString
	case a.attr.Type.IsListType() || a.attr.Type.IsSetType():
		return shim.TypeList
	case a.attr.Type.IsMapType() || a.attr.Type.IsObjectType() || a.attr.NestedType != nil:
		return shim.TypeMap
	default:
		panic(fmt.Sprintf("UNKNOWN TYPE of %#v", a.attr.Type)) // TODO: Remove for release
		return shim.TypeInvalid
	}
}

func (a attribute) Elem() interface{} {
	switch {
	case a.attr.NestedType != nil:
		return object{obj: *a.attr.NestedType}
	case a.attr.Type.IsObjectType():
		panic("Not implemented, this feature may not be used :)")
	case a.attr.Type.IsCollectionType():
		return attribute{otshim.SchemaAttribute{Type: a.attr.Type}}
	default:
		return nil
	}
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
