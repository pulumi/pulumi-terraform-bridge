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
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

var (
	_ = shim.Resource(object{})
	_ = shim.SchemaMap(attrMap{})
	_ = shim.Resource(tftypesObject{})
)

type object struct {
	pseudoResource
	obj tfprotov6.SchemaObject
}

func (o object) Schema() shim.SchemaMap {
	attrMap := attrMap{}
	for _, v := range o.obj.Attributes {
		attrMap[v.Name] = v
	}
	return attrMap
}

func (o object) DeprecationMessage() string { return "" }

type attrMap map[string]*tfprotov6.SchemaAttribute

func (m attrMap) Len() int { return len(m) }

func (m attrMap) Get(key string) shim.Schema { return getSchemaMap(m, key) }

func (m attrMap) GetOk(key string) (shim.Schema, bool) {
	v, ok := m[key]
	if !ok {
		return nil, false
	}
	return attribute{*v}, true
}

func (m attrMap) Range(each func(key string, value shim.Schema) bool) {
	for k, v := range m {
		if !each(k, attribute{*v}) {
			return
		}
	}
}

func (m attrMap) Set(key string, value shim.Schema) {
	v, ok := value.(attribute)
	contract.Assertf(ok, "Must set an %T, found %T", v, value)
	m[key] = &v.attr
}

func (m attrMap) Delete(key string) {
	delete(m, key)
}

func (m attrMap) Validate() error { return nil }

type tftypesObject struct {
	pseudoResource
	obj tftypes.Object
}

func (o tftypesObject) Schema() shim.SchemaMap {
	attrMap := attrMap{}
	for k, v := range o.obj.AttributeTypes {
		_, optional := o.obj.OptionalAttributes[k]
		attrMap[k] = &tfprotov6.SchemaAttribute{Type: v, Optional: optional}
	}
	return attrMap
}

func (o tftypesObject) DeprecationMessage() string { return "" }
