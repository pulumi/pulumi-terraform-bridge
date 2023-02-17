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
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	pfattr "github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/pulumi/pulumi-terraform-bridge/pf/internal/pfutils"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

// An Object type that masquerades as a Resource. This is a workaround to reusing tfgen code for generating schemas,
// which assumes schema.Elem() would return either a Resource or a Schema. This struct packages the object field names
// an types schema through a pseudo-Resource.
type objectPseudoResource struct {
	schemaOnly
	obj types.ObjectType

	attrs map[string]pfutils.Attr
}

func newObjectPseudoResource(t types.ObjectType, attrs map[string]pfutils.Attr) *objectPseudoResource {
	return &objectPseudoResource{
		schemaOnly: schemaOnly{"objectPseudoResource"},
		obj:        t, attrs: attrs}
}

var _ shim.Resource = (*objectPseudoResource)(nil)
var _ shim.SchemaMap = (*objectPseudoResource)(nil)

func (r *objectPseudoResource) Schema() shim.SchemaMap {
	return r
}

func (*objectPseudoResource) SchemaVersion() int {
	panic("This is an Object type encoded as a shim.Resource, and " +
		"SchemaVersion() should not be called on this entity during schema generation")
}

func (*objectPseudoResource) DeprecationMessage() string {
	return ""
}

func (*objectPseudoResource) Importer() shim.ImportFunc {
	panic("This is an Object type encoded as a shim.Resource, and " +
		"ImporterFunc() should not be called on this entity during schema generation")
}

func (*objectPseudoResource) Timeouts() *shim.ResourceTimeout {
	panic("This is an Object type encoded as a shim.Resource, and " +
		"Timeouts() should not be called on this entity during schema generation")
}

func (*objectPseudoResource) InstanceState(id string, object,
	meta map[string]interface{}) (shim.InstanceState, error) {
	panic("This is an Object type encoded as a shim.Resource, and " +
		"InstanceState() should not be called on this entity during schema generation")
}

func (*objectPseudoResource) DecodeTimeouts(
	config shim.ResourceConfig) (*shim.ResourceTimeout, error) {
	panic("This is an Object type encoded as a shim.Resource, and " +
		"DecodeTimeouts() should not be called on this entity during schema generation")
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

type tuplePseudoResource struct {
	schemaOnly
	attrs map[string]pfutils.Attr
	tuple pfattr.TypeWithElementTypes
}

type tupElementAttr struct{ e pfattr.Type }

func (tupElementAttr) GetDeprecationMessage() string  { return "" }
func (tupElementAttr) GetDescription() string         { return "" }
func (tupElementAttr) GetMarkdownDescription() string { return "" }
func (tupElementAttr) IsOptional() bool               { return false }
func (tupElementAttr) IsRequired() bool               { return true }
func (tupElementAttr) IsSensitive() bool              { return false }
func (tupElementAttr) IsComputed() bool               { return false }

func (t tupElementAttr) GetType() attr.Type { return t.e }

func newTuplePseudoResource(t pfattr.TypeWithElementTypes) shim.Resource {
	attrs := make(map[string]pfutils.Attr, len(t.ElementTypes()))
	for i, e := range t.ElementTypes() {
		k := fmt.Sprintf("t%d", i)
		attrs[k] = pfutils.FromAttrLike(tupElementAttr{e})
	}
	return &tuplePseudoResource{
		schemaOnly: schemaOnly{"tuplePseudoResource"},
		attrs:      attrs,
		tuple:      t}
}

func (*tuplePseudoResource) SchemaVersion() int         { panic("TODO") }
func (*tuplePseudoResource) DeprecationMessage() string { panic("TODO") }

func (r *tuplePseudoResource) Schema() shim.SchemaMap {
	return r
}

func (r *tuplePseudoResource) Get(key string) shim.Schema {
	v, ok := r.GetOk(key)
	if !ok {
		panic(fmt.Sprintf("Missing key: '%s' in tuple with %d elements", key, r.Len()))
	}
	return v
}

func (r *tuplePseudoResource) GetOk(key string) (shim.Schema, bool) {
	if key == "" || key[0] != 't' {
		return nil, false
	}
	i, err := strconv.Atoi(key[1:])
	types := r.tuple.ElementTypes()
	if err != nil || i > len(types) {
		return nil, false
	}
	return newTypeSchema(types[i], r.attrs), true
}

func (r *tuplePseudoResource) Len() int {
	return len(r.tuple.ElementTypes())
}

func (r *tuplePseudoResource) Range(each func(key string, value shim.Schema) bool) {
	for i, v := range r.tuple.ElementTypes() {
		k := fmt.Sprintf("t%d", i)
		if !each(k, newTypeSchema(v, r.attrs)) {
			break
		}
	}
}

func (*tuplePseudoResource) Set(key string, value shim.Schema) {
	panic("Set not supported - is it possible to treat this as immutable?")
}

func (*tuplePseudoResource) Delete(key string) {
	panic("Delete not supported - is it possible to treat this as immutable?")
}

type schemaOnly struct{ typ string }

func (s *schemaOnly) Importer() shim.ImportFunc {
	m := "type"
	if s != nil || s.typ != "" {
		m = s.typ
	}
	panic(m + " does not implement runtime operation ImporterFunc")
}

func (s *schemaOnly) Timeouts() *shim.ResourceTimeout {
	m := "type"
	if s != nil || s.typ != "" {
		m = s.typ
	}
	panic(m + " does not implement runtime operation Timeouts")
}

func (s *schemaOnly) InstanceState(id string, object,
	meta map[string]interface{}) (shim.InstanceState, error) {
	m := "type"
	if s != nil || s.typ != "" {
		m = s.typ
	}
	panic(m + " does not implement runtime operation InstanceState")
}

func (s *schemaOnly) DecodeTimeouts(
	config shim.ResourceConfig) (*shim.ResourceTimeout, error) {
	m := "type"
	if s != nil || s.typ != "" {
		m = s.typ
	}
	panic(m + " does not implement runtime operation DecodeTimeouts")
}
