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
	"fmt"
	"sort"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/pulumi/pulumi-terraform-bridge/pf/internal/pfutils"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// An Object type that masquerades as a Resource. This is a workaround to reusing tfgen code for generating schemas,
// which assumes schema.Elem() would return either a Resource or a Schema. This struct packages the object field names
// an types schema through a pseudo-Resource.
type objectPseudoResource struct {
	schemaOnly
	obj          basetypes.ObjectTypable
	nestedAttrs  map[string]pfutils.Attr
	nestedBlocks map[string]pfutils.Block // should have disjoint keys from nestedAttrs
	allAttrNames []string
}

func newObjectPseudoResource(t basetypes.ObjectTypable,
	nestedAttrs map[string]pfutils.Attr,
	nestedBlocks map[string]pfutils.Block) *objectPseudoResource {
	lowerType := t.TerraformType(context.Background())
	objType, ok := lowerType.(tftypes.Object)
	contract.Assertf(ok, "t basetypes.ObjectTypable should produce a tftypes.Object "+
		"in t.TerraformType(): found %T", lowerType)
	var attrs []string
	for k := range objType.AttributeTypes {
		attrs = append(attrs, k)
	}
	sort.Strings(attrs)
	return &objectPseudoResource{
		schemaOnly:   schemaOnly{"objectPseudoResource"},
		obj:          t,
		nestedAttrs:  nestedAttrs,
		nestedBlocks: nestedBlocks,
		allAttrNames: attrs,
	}
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
	lowerType := r.obj.TerraformType(context.Background())
	objType := lowerType.(tftypes.Object)
	return len(objType.AttributeTypes)
}

func (r *objectPseudoResource) Get(key string) shim.Schema {
	s, ok := r.GetOk(key)
	contract.Assertf(ok, "Missing key: %v", key)
	return s
}

func (r *objectPseudoResource) GetOk(key string) (shim.Schema, bool) {
	// There is something here that possibly could be done better.
	//
	// When moving down a property identified by key, we are interested in keeping track of Attr for that property,
	// and recurse using attrSchema. This information may be coming out of band from the ObjectTypeable value itself
	// when using blocks, see TestCustomTypeEmbeddingObjectType.
	if attr, ok := r.nestedAttrs[key]; ok {
		return &attrSchema{key, attr}, true
	}

	// Nested blocks are similar to attributes:
	if block, ok := r.nestedBlocks[key]; ok {
		return newBlockSchema(key, block), true
	}

	if t, err := r.obj.ApplyTerraform5AttributePathStep(tftypes.AttributeName(key)); err == nil {
		typ, ok := t.(attr.Type)
		msg := "Failing to translate schema for attribute %q, unexpected attribute of type %T"
		contract.Assertf(ok, msg, key, typ)
		return newTypeSchema(typ, nil), true
	}

	// Check if key is a valid attribute.
	for _, a := range r.allAttrNames {
		// If key is a valid attribute, then we have failed to find it:
		if key == a {
			contract.Failf("[pf/internal/schemashim] Failing to translate schema "+
				"for attribute %q of object type %#v. "+
				"This should never happen, please report an issue.",
				key, r.obj)
		}
	}

	// Otherwise key is not a valid attribute, so we can just return
	return nil, false
}

func (r *objectPseudoResource) Range(each func(key string, value shim.Schema) bool) {
	for _, attr := range r.allAttrNames {
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
	tuple attr.TypeWithElementTypes
}

type tupElementAttr struct{ e attr.Type }

func (tupElementAttr) GetDeprecationMessage() string  { return "" }
func (tupElementAttr) GetDescription() string         { return "" }
func (tupElementAttr) GetMarkdownDescription() string { return "" }
func (tupElementAttr) IsOptional() bool               { return false }
func (tupElementAttr) IsRequired() bool               { return true }
func (tupElementAttr) IsSensitive() bool              { return false }
func (tupElementAttr) IsComputed() bool               { return false }

func (t tupElementAttr) GetType() attr.Type { return t.e }

func newTuplePseudoResource(t attr.TypeWithElementTypes) shim.Resource {
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
	contract.Assertf(ok, "Missing key: '%s' in tuple with %d elements", key, r.Len())
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
