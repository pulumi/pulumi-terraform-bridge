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

	"github.com/hashicorp/terraform-plugin-go/tftypes"

	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

type typeSchema struct {
	t tftypes.Type
}

var _ shim.Schema = (*typeSchema)(nil)

func (s *typeSchema) Type() shim.ValueType {
	ctx := context.TODO()
	vt, err := convertType(ctx, s.t)
	if err != nil {
		panic(err)
	}
	return vt
}

// Return zero values the following methods. GenerateSchema calls them, although they make no sense
// at the level of typeSchema. While attrSchema may be Optional or Required, typeSchema is cannot.
func (*typeSchema) Optional() bool { return false }
func (*typeSchema) Required() bool { return false }
func (*typeSchema) Computed() bool { return false }
func (*typeSchema) ForceNew() bool { return false }

func (s *typeSchema) Elem() interface{} {
	if s.t.Is(tftypes.Object{}) {
		obj := s.t.(tftypes.Object)
		var m shim.SchemaMap = &objectMap{obj}
		return m
	}
	return nil
}

func (*typeSchema) MaxItems() int      { return 0 }
func (*typeSchema) MinItems() int      { return 0 }
func (*typeSchema) Deprecated() string { return "" }

func (*typeSchema) Default() interface{}                               { panic("TODO") }
func (*typeSchema) DefaultFunc() shim.SchemaDefaultFunc                { panic("TODO") }
func (*typeSchema) DefaultValue() (interface{}, error)                 { panic("TODO") }
func (*typeSchema) Description() string                                { panic("TODO") }
func (*typeSchema) StateFunc() shim.SchemaStateFunc                    { panic("TODO") }
func (*typeSchema) ConflictsWith() []string                            { panic("TODO") }
func (*typeSchema) ExactlyOneOf() []string                             { panic("TODO") }
func (*typeSchema) Removed() string                                    { panic("TODO") }
func (*typeSchema) Sensitive() bool                                    { panic("TODO") }
func (*typeSchema) UnknownValue() interface{}                          { panic("TODO") }
func (*typeSchema) SetElement(config interface{}) (interface{}, error) { panic("TODO") }
func (*typeSchema) SetHash(v interface{}) int                          { panic("TODO") }
