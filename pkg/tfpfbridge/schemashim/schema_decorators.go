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
	shim.Schema
	optional func(shim.Schema) bool
	required func(shim.Schema) bool
}

var _ shim.Schema = (*schemaDecorator)(nil)

func (d *schemaDecorator) Optional() bool {
	if d.optional != nil {
		return d.optional(d.Schema)
	}
	return d.Schema.Optional()
}

func (d *schemaDecorator) Required() bool {
	if d.required != nil {
		return d.required(d.Schema)
	}
	return d.Schema.Required()
}
