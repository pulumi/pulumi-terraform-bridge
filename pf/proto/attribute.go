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
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"

	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

var _ = shim.Schema(attribute{})

type attribute struct{ attr tfprotov6.SchemaAttribute }

// Simple schema options

func (a attribute) Optional() bool      { return a.attr.Optional }
func (a attribute) Required() bool      { return a.attr.Required }
func (a attribute) Description() string { return a.attr.Description }
func (a attribute) Computed() bool      { return a.attr.Computed }
func (a attribute) ForceNew() bool      { return false } // The information is not available from tfprotov6
func (a attribute) Sensitive() bool     { return a.attr.Sensitive }
func (a attribute) Removed() string     { return "" }
func (a attribute) Deprecated() string  { return deprecated(a.attr.Deprecated) }

// Type information

func (a attribute) MaxItems() int { return 0 }
func (a attribute) MinItems() int { return 0 }

func (a attribute) Type() shim.ValueType { return element{typ: a.attr.ValueType()}.Type() }

func (a attribute) Elem() interface{} {
	if a.attr.NestedType != nil {
		obj := *a.attr.NestedType

		// How obj.NestedType.Nesting should be handled isn't obvious.
		//
		// The case analysis goes:
		//
		// [tfprotov6.SchemaObjectNestingModeSingle]: `a` represents an Object, so it's `.Type()`
		// should be [shim.TypeMap] and it's `.Elem()` should implement [shim.Resource]. This is
		// correctly handled by `.Type()` and [object] implements [shim.Resource].
		//
		// [tfprotov6.SchemaObjectNestingModeList] or [tfprotov6.SchemaObjectNestingModeSet]: `a`
		// represents a List<Object>, so it's `.Type()` should be [shim.TypeList] and it's `.Elem()`
		// should implement [shim.Resource]. This is correctly handled by `.Type()` and [object]
		// implements [shim.Resource].
		//
		// [tfprotov6.SchemaObjectNestingModeMap]: `a` represents a Map<Object>, so it's `.Type()`
		// should be [shim.TypeMap] and it's `.Elem()` should be a [shim.Schema] whose `.Type()` is
		// [shim.TypeMap] and whose `.Elem()` is a [shim.Resource].

		switch obj.Nesting {
		case tfprotov6.SchemaObjectNestingModeMap:
			// We are careful to only assign to variables on [attribute.Elem]'s stack. We *do not*
			// mutate the caller.
			obj.Nesting = tfprotov6.SchemaObjectNestingModeSingle
			a.attr.NestedType = &obj
			return a
		case tfprotov6.SchemaObjectNestingModeSingle,
			tfprotov6.SchemaObjectNestingModeList,
			tfprotov6.SchemaObjectNestingModeSet:
			return object{obj: obj}
		case tfprotov6.SchemaObjectNestingModeInvalid:
			fallthrough
		default:
			contract.Failf("Invalid attribute nesting: %s", a.attr.NestedType.Nesting)
		}
	}
	return element{a.attr.ValueType(), a.Optional()}.Elem()
}

// Defaults are applied in the provider binary, not here

func (a attribute) Default() interface{}                { return nil }
func (a attribute) DefaultFunc() shim.SchemaDefaultFunc { return nil }
func (a attribute) DefaultValue() (interface{}, error)  { return nil, nil }

func (a attribute) StateFunc() shim.SchemaStateFunc { return nil }
func (a attribute) ConflictsWith() []string         { return nil }
func (a attribute) ExactlyOneOf() []string          { return nil }
func (a attribute) AtLeastOneOf() []string          { return nil }
func (a attribute) RequiredWith() []string          { return nil }
func (a attribute) ConfigMode() shim.ConfigModeType { return 0 }

func (a attribute) SetElement(config interface{}) (interface{}, error) {
	panic("UNIMPLIMENTED")
}

func (a attribute) SetHash(v interface{}) int { panic("UNIMPLIMENTED") }
