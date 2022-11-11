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
	"fmt"

	"github.com/hashicorp/terraform-plugin-go/tftypes"

	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

type objectMap struct {
	obj tftypes.Object
}

var _ shim.SchemaMap = (*objectMap)(nil)

func (m *objectMap) Len() int {
	return len(m.obj.AttributeTypes)
}

func (m *objectMap) Get(key string) shim.Schema {
	s, ok := m.GetOk(key)
	if !ok {
		panic(fmt.Sprintf("Missing key: %v", key))
	}
	return s
}

func (m *objectMap) GetOk(key string) (shim.Schema, bool) {
	attrs := m.obj.AttributeTypes
	ty, ok := attrs[key]
	if !ok {
		return nil, false
	}
	return &typeSchema{ty}, true
}

func (m *objectMap) Range(each func(key string, value shim.Schema) bool) {
	for key := range m.obj.AttributeTypes {
		if !each(key, m.Get(key)) {
			return
		}
	}
}

func (m *objectMap) Set(key string, value shim.Schema) {
	panic("Set not supported - is it possible to treat this as immutable?")
}

func (m *objectMap) Delete(key string) {
	panic("Delete not supported - is it possible to treat this as immutable?")
}
