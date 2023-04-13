// Copyright 2016-2023, Pulumi Corporation.
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

package muxer

import (
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

func Mapping(schemas []schema.PackageSpec) (*DispatchTable, schema.PackageSpec, error) {
	// TODO Insert sanity checks and return an error on conflicts:
	// https://github.com/pulumi/pulumi-terraform-bridge/issues/949 For example right
	// now, if different schemas define the same type in different ways, which ever
	// schema comes first dominates.

	muxedSchema := func() *schema.PackageSpec { x := schemas[0]; return &x }()
	mapping := NewDispatchTable()

	// We need to zero these maps out so our normal process can re-add them. This
	// maintains consistency.
	muxedSchema.Resources = map[string]schema.ResourceSpec{}
	muxedSchema.Types = map[string]schema.ComplexTypeSpec{}
	muxedSchema.Functions = map[string]schema.FunctionSpec{}
	muxedSchema.Config = schema.ConfigSpec{}
	muxedSchema.Provider = schema.ResourceSpec{}

	for i, s := range schemas {
		s := s
		layerSchema(mapping, muxedSchema, &s, i)
	}

	if len(muxedSchema.Resources) == 0 {
		muxedSchema.Resources = nil
	}
	if len(muxedSchema.Types) == 0 {
		muxedSchema.Types = nil
	}
	if len(muxedSchema.Functions) == 0 {
		muxedSchema.Functions = nil
	}

	return mapping, *muxedSchema, nil
}

// Layer `srcSchema` under `dstSchema`, keeping track of where resources and functions
// were mapped from.
//
// `srcIndex` is the index of `srcSchema` and thus its server.
func layerSchema(mapping *DispatchTable, dstSchema, srcSchema *schema.PackageSpec, srcIndex int) {
	m := mappingCtx{mapping, dstSchema, srcSchema, srcIndex}

	for tk, r := range m.srcSchema.Resources {
		m.setResource(tk, r)
	}
	for tk, f := range m.srcSchema.Functions {
		m.setFunction(tk, f)
	}

	// Add non-repeating required config keys to the schema
	if len(srcSchema.Config.Required) > 0 {
		dstSchema.Config.Required = appendUnique(
			dstSchema.Config.Required,
			srcSchema.Config.Required)
	}

	for k, v := range m.srcSchema.Config.Variables {
		if m.dstSchema.Config.Variables == nil {
			m.dstSchema.Config.Variables = map[string]schema.PropertySpec{}
		}
		// TODO: Validity check that this value matches any other config value
		// with the same key
		m.dstSchema.Config.Variables[k] = v
		m.mapping.Config[k] = append(m.mapping.Config[k], m.srcIndex)
		m.addType(v.TypeSpec)
	}

	// layer in the schema.ProviderSpec
	m.layerResource(&m.dstSchema.Provider, m.srcSchema.Provider)
}

func (m *mappingCtx) layerResource(dst *schema.ResourceSpec, src schema.ResourceSpec) {
	contract.Assert(dst != nil)

	for k, v := range src.InputProperties {
		if _, has := dst.InputProperties[k]; has {
			// TODO: Validity check that these match
			continue
		}
		m.addType(v.TypeSpec)
		if dst.InputProperties == nil {
			dst.InputProperties = map[string]schema.PropertySpec{}
		}
		dst.InputProperties[k] = v
	}
	for k, v := range src.Properties {
		if _, has := dst.Properties[k]; has {
			// TODO: Validity check that these match
			continue
		}
		m.addType(v.TypeSpec)
		if dst.Properties == nil {
			dst.Properties = map[string]schema.PropertySpec{}
		}
		dst.Properties[k] = v
	}

	if src.StateInputs != nil {
		for k, v := range src.StateInputs.Properties {
			if dst.StateInputs == nil {
				dst.StateInputs = &schema.ObjectTypeSpec{}
			}
			if dst.StateInputs.Properties == nil {
				dst.StateInputs.Properties = map[string]schema.PropertySpec{}
			}

			if _, has := dst.StateInputs.Properties[k]; has {
				// TODO: Validity check that these match
				continue
			}
			m.addType(v.TypeSpec)
			dst.StateInputs.Properties[k] = v
		}
		dst.Plain = appendUnique(dst.Plain, src.Plain)
	}
}

// Append elements from src to dst if they are not already present in dst.
func appendUnique[T comparable](dst, src []T) []T {
	set := make(map[T]struct{}, len(dst))
	for _, v := range dst {
		set[v] = struct{}{}
	}
	for _, v := range src {
		if _, has := set[v]; has {
			continue
		}
		dst = append(dst, v)
	}
	return dst
}

type mappingCtx struct {
	mapping              *DispatchTable
	dstSchema, srcSchema *schema.PackageSpec
	srcIndex             int
}

func (m *mappingCtx) setResource(token string, resource schema.ResourceSpec) {
	_, ok := m.mapping.Resources[token]
	if ok {
		return
	}
	m.mapping.Resources[token] = m.srcIndex
	m.dstSchema.Resources[token] = resource
	m.addProperties(resource.InputProperties)
	m.addProperties(resource.Properties)
	if resource.StateInputs != nil {
		m.addProperties(resource.StateInputs.Properties)
	}
}

func (m *mappingCtx) setFunction(token string, function schema.FunctionSpec) {
	_, ok := m.mapping.Functions[token]
	if ok {
		return
	}
	m.dstSchema.Functions[token] = function

	m.addObjectType(function.Inputs)
	m.addObjectType(function.Outputs)
}

func (m *mappingCtx) addProperties(properties map[string]schema.PropertySpec) {
	for _, t := range properties {
		m.addType(t.TypeSpec)
	}
}

func (m *mappingCtx) addType(t schema.TypeSpec) {
	if t.Ref != "" {
		m.addRefType(t.Ref)
	}

	for _, one := range t.OneOf {
		m.addType(one)
	}

	switch t.Type {
	// We don't need to include primitive types
	case "bool", "integer", "number", "string":
		return

	// Simple compound types are also free
	case "array":
		m.addType(*t.Items)
		return
	case "object":
		if t.AdditionalProperties != nil {
			m.addType(*t.AdditionalProperties)
			return
		}
	}
}

func (m *mappingCtx) addRefType(ref string) {
	// External references don't need to be addressed, since there isn't any
	// associated data in this schema.
	//
	// Resource references don't need to be addressed, since they will be covered by
	// the resource sweep later.
	switch {
	case strings.HasPrefix(ref, "pulumi.json#/"):
		return
	}

	prefix := "#/types/"
	if !strings.HasPrefix(ref, prefix) {
		return
	}
	token := strings.TrimPrefix(ref, prefix)

	if _, alreadyExists := m.dstSchema.Types[token]; alreadyExists {
		// This resource is already defined, either by a more prominent server or
		// another resource in this one.
		return
	}

	typ, ok := m.srcSchema.Types[token]
	if !ok {
		// This is a token type (dangling ref) in the source schema, so it will
		// also be a token type in the muxed schema.
		return
	}

	// We have added a new type to the muxed schema, we now need to make sure that all
	// referenced types exist in the muxed schema.
	m.addComplexType(token, typ)
}

func (m *mappingCtx) addComplexType(token string, typ schema.ComplexTypeSpec) {
	m.dstSchema.Types[token] = typ

	if typ.Type != "object" {
		// This was en enum, so we can just do the assignment.
		return
	}

	m.addObjectType(&typ.ObjectTypeSpec)
}

func (m *mappingCtx) addObjectType(typ *schema.ObjectTypeSpec) {
	if typ == nil {
		return
	}
	m.addProperties(typ.Properties)
}
