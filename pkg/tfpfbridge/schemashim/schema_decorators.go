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
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

type schemaDecorator struct {
	innerSchema shim.Schema
	optional    func(shim.Schema) bool
	required    func(shim.Schema) bool
}

var _ shim.Schema = (*schemaDecorator)(nil)

func (d *schemaDecorator) Type() shim.ValueType {
	return d.innerSchema.Type()
}

func (d *schemaDecorator) Computed() bool { return d.innerSchema.Computed() }
func (d *schemaDecorator) ForceNew() bool { return d.innerSchema.ForceNew() }

func (d *schemaDecorator) Optional() bool {
	if d.optional != nil {
		return d.optional(d.innerSchema)
	}
	return d.innerSchema.Optional()
}

func (d *schemaDecorator) Required() bool {
	if d.required != nil {
		return d.required(d.innerSchema)
	}
	return d.innerSchema.Required()
}

func (d *schemaDecorator) Sensitive() bool { return d.innerSchema.Sensitive() }

func (d *schemaDecorator) Elem() interface{} { return d.innerSchema.Elem() }

func (d *schemaDecorator) MaxItems() int      { return d.innerSchema.MaxItems() }
func (d *schemaDecorator) MinItems() int      { return d.innerSchema.MinItems() }
func (d *schemaDecorator) Deprecated() string { return d.innerSchema.Deprecated() }

func (d *schemaDecorator) Default() interface{}                { return d.innerSchema.Default() }
func (d *schemaDecorator) DefaultFunc() shim.SchemaDefaultFunc { return d.innerSchema.DefaultFunc() }
func (d *schemaDecorator) DefaultValue() (interface{}, error)  { return d.innerSchema.DefaultValue() }
func (d *schemaDecorator) Description() string                 { return d.innerSchema.Description() }
func (d *schemaDecorator) StateFunc() shim.SchemaStateFunc     { return d.innerSchema.StateFunc() }
func (d *schemaDecorator) ConflictsWith() []string             { return d.innerSchema.ConflictsWith() }
func (d *schemaDecorator) ExactlyOneOf() []string              { return d.innerSchema.ExactlyOneOf() }
func (d *schemaDecorator) Removed() string                     { return d.innerSchema.Removed() }

func (d *schemaDecorator) UnknownValue() interface{} { return d.innerSchema.UnknownValue() }

func (d *schemaDecorator) SetElement(config interface{}) (interface{}, error) {
	return d.innerSchema.SetElement(config)
}

func (d *schemaDecorator) SetHash(v interface{}) int {
	return d.innerSchema.SetHash(v)
}
