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
)

type ComputedMapping struct {
	// mapping is an implementation detail.
	//
	// Right now, we want to expose the same information to the user, but that doesn't
	// need to be true forever.
	mapping
}

func Mapping(schemas []schema.PackageSpec) (ComputedMapping, schema.PackageSpec, error) {
	// TODO Insert sanity checks and return an error on conflicts.  For example right
	// now, if different schemas define the same type in different ways, which ever
	// schema comes first dominates.

	muxedSchema := &schemas[0]
	mapping := newMapping()

	// We need to zero these maps out so our normal process can re-add them. This
	// maintains consistency.
	muxedSchema.Resources = map[string]schema.ResourceSpec{}
	muxedSchema.Types = map[string]schema.ComplexTypeSpec{}
	muxedSchema.Functions = map[string]schema.FunctionSpec{}

	for i, s := range schemas {
		s := s
		mapping.layerSchema(muxedSchema, &s, i)
	}

	return ComputedMapping{mapping}, *muxedSchema, nil
}

type mapping struct {
	// Resources and functions can only map to a single provider
	Resources map[string]int `json:"resources"`
	Functions map[string]int `json:"functions"`

	// Config values can map to multiple providers
	Config map[string][]int `json:"config"`
}

func newMapping() mapping {
	return mapping{
		Resources: make(map[string]int),
		Functions: make(map[string]int),
		Config:    make(map[string][]int),
	}
}

func (mapping mapping) isEmpty() bool {
	return mapping.Resources == nil &&
		mapping.Functions == nil &&
		mapping.Config == nil
}

// Layer `srcSchema` under `dstSchema`, keeping track of where resources and functions
// were mapped from.
//
// `srcIndex` is the index of `srcSchema` and thus its server.
func (mapping mapping) layerSchema(dstSchema, srcSchema *schema.PackageSpec, srcIndex int) {
	m := mappingCtx{
		mapping:   mapping,
		dstSchema: dstSchema,
		srcSchema: srcSchema,
		srcIndex:  srcIndex,
	}
	for tk, r := range m.srcSchema.Resources {
		m.setResource(tk, r)
	}
	for tk, f := range m.srcSchema.Functions {
		m.setFunction(tk, f)
	}
	for v := range m.srcSchema.Config.Variables {
		m.mapping.Config[v] = append(m.mapping.Config[v], m.srcIndex)
	}
}

type mappingCtx struct {
	mapping              mapping
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
	m.addProperties(resource.StateInputs.Properties)
}

func (m *mappingCtx) setFunction(token string, function schema.FunctionSpec) {
	_, ok := m.mapping.Functions[token]
	if ok {
		return
	}
	m.dstSchema.Functions[token] = function
	m.addProperties(function.Inputs.Properties)
	m.addProperties(function.Outputs.Properties)
}

func (m *mappingCtx) addProperties(properties map[string]schema.PropertySpec) {
	for _, t := range properties {
		m.addType(t.TypeSpec)
	}
}

func (m *mappingCtx) addType(t schema.TypeSpec) {
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

	for _, one := range t.OneOf {
		m.addType(one)
	}

	if t.Ref != "" {
		m.addRefType(t.Ref)
	}
}

func (m *mappingCtx) addRefType(ref string) {
	// External references don't need to be addressed, since there isn't any
	// associated data in this schema.
	//
	// Resource references don't need to be addressed, since they will be covered by
	// the resource sweep later.
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

	m.dstSchema.Types[token] = typ
	// We have added a new type to the muxed schema, we now need to make sure that all
	// referenced types exist in the muxed schema.
	m.addProperties(typ.Properties)
}
