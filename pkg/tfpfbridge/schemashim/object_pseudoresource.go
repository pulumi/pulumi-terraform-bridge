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
	"sort"

	"github.com/hashicorp/terraform-plugin-framework/types"

	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

// An Object type that masquerades as a Resource. This is a workaround to reusing tfgen code for generating schemas,
// which assumes schema.Elem() would return either a Resource or a Schema. This struct packages the object field names
// an types schema through a pseudo-Resource.
type objectPseudoResource struct {
	obj types.ObjectType

	attrs map[string]attr
}

func newObjectPseudoResource(t types.ObjectType, attrs map[string]attr) *objectPseudoResource {
	return &objectPseudoResource{obj: t, attrs: attrs}
}

var _ shim.Resource = (*objectPseudoResource)(nil)
var _ shim.SchemaMap = (*objectPseudoResource)(nil)

func (r *objectPseudoResource) Schema() shim.SchemaMap {
	return r
}

func (*objectPseudoResource) SchemaVersion() int         { panic("TODO") }
func (*objectPseudoResource) DeprecationMessage() string { panic("TODO") }

func (*objectPseudoResource) Importer() shim.ImportFunc {
	panic("objectPseudoResource does not implement runtime operation ImporterFunc")
}

func (*objectPseudoResource) Timeouts() *shim.ResourceTimeout {
	panic("objectPseudoResource does not implement runtime operation Timeouts")
}

func (*objectPseudoResource) InstanceState(id string, object,
	meta map[string]interface{}) (shim.InstanceState, error) {
	panic("objectPseudoResource does not implement runtime operation InstanceState")
}

func (*objectPseudoResource) DecodeTimeouts(
	config shim.ResourceConfig) (*shim.ResourceTimeout, error) {
	panic("objectPseudoResource does not implement runtime operation DecodeTimeouts")
}

func (r *objectPseudoResource) Len() int {
	return len(r.obj.AttrTypes)
}

func (r *objectPseudoResource) Get(key string) shim.Schema {
	s, ok := r.GetOk(key)
	if !ok {
		panic(fmt.Sprintf("Missing key: %v", key))
	}
	return s
}

func (r *objectPseudoResource) GetOk(key string) (shim.Schema, bool) {
	if attr, ok := r.attrs[key]; ok {
		return &attrSchema{key, attr}, true
	}

	if t, ok := r.obj.AttrTypes[key]; ok {
		return newTypeSchema(t, nil), true
	}

	return nil, false
}

func (r *objectPseudoResource) Range(each func(key string, value shim.Schema) bool) {
	var attrs []string
	for attr := range r.obj.AttrTypes {
		attrs = append(attrs, attr)
	}
	sort.Strings(attrs)
	for _, attr := range attrs {
		if !each(attr, r.Get(attr)) {
			return
		}
	}
}

func (*objectPseudoResource) Set(key string, value shim.Schema) {
	panic("Set not supported - is it possible to treat this as immutable?")
}

func (*objectPseudoResource) Delete(key string) {
	panic("Delete not supported - is it possible to treat this as immutable?")
}
