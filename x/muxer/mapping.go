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

type DispatchTable struct {
	// mapping is an implementation detail.
	//
	// Right now, we want to expose the same information to the user, but that doesn't
	// need to be true forever.
	dispatchTable
}

func MergeSchemasAndComputeDispatchTable(schemas []schema.PackageSpec) (DispatchTable, schema.PackageSpec, error) {
	// TODO Insert sanity checks and return an error on conflicts:
	// https://github.com/pulumi/pulumi-terraform-bridge/issues/949 For example right
	// now, if different schemas define the same type in different ways, which ever
	// schema comes first dominates.

	muxedSchema := func() *schema.PackageSpec { x := schemas[0]; return &x }()
	dispatchTable := newDispatchTable()

	// We need to zero these maps out so our normal process can re-add them. This
	// maintains consistency.
	muxedSchema.Resources = map[string]schema.ResourceSpec{}
	muxedSchema.Types = map[string]schema.ComplexTypeSpec{}
	muxedSchema.Functions = map[string]schema.FunctionSpec{}
	muxedSchema.Config = schema.ConfigSpec{}
	muxedSchema.Provider = schema.ResourceSpec{}

	for i, s := range schemas {
		i, s := i, s
		dispatchTable.layerSchema(muxedSchema, &s, i)
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

	return DispatchTable{dispatchTable: dispatchTable}, *muxedSchema, nil
}

type dispatchTable struct {
	// Resources and functions can only map to a single provider
	Resources map[string]int `json:"resources"`
	Functions map[string]int `json:"functions"`

	// Config values can map to multiple providers
	Config map[string][]int `json:"config"`
}

func newDispatchTable() dispatchTable {
	return dispatchTable{
		Resources: make(map[string]int),
		Functions: make(map[string]int),
		Config:    make(map[string][]int),
	}
}

func (dispatchTable dispatchTable) isEmpty() bool {
	return dispatchTable.Resources == nil &&
		dispatchTable.Functions == nil &&
		dispatchTable.Config == nil
}

// Layer `srcSchema` under `dstSchema`, keeping track of where resources and functions
// were mapped from.
//
// `srcIndex` is the index of `srcSchema` and thus its server.
func (dispatchTable dispatchTable) layerSchema(dstSchema, srcSchema *schema.PackageSpec, srcIndex int) {
	m := dispatchTableCtx{dispatchTable, dstSchema, srcSchema, srcIndex}

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

	for k := range m.srcSchema.Config.Variables {
		m.dispatchTable.Config[k] = append(m.dispatchTable.Config[k], m.srcIndex)
	}
	layerMap(&m.dstSchema.Config.Variables, m.srcSchema.Config.Variables,
		func(_ string, t schema.PropertySpec) { m.addType(t.TypeSpec) })

	// layer in the schema.ProviderSpec
	m.layerProvider(&m.dstSchema.Provider, m.srcSchema.Provider)
}

// A helper function to merge two maps together, accounting for the maps being nil.
func layerMap[K comparable, V any](dst *map[K]V, src map[K]V, finalize func(k K, v V)) {
	if src == nil {
		return
	}
	if *dst == nil {
		*dst = map[K]V{}
	}
	for k, v := range src {
		_, skip := (*dst)[k]
		if !skip {
			(*dst)[k] = v
			if finalize != nil {
				finalize(k, v)
			}
		}
	}
}

// Layer a provider resource onto another resource.
func (m *dispatchTableCtx) layerProvider(dst *schema.ResourceSpec, src schema.ResourceSpec) {
	// As implemented, this could layer an arbitrary resource onto another arbitrary
	// resource. The only resource where we want to "merge" instead "pick one" is the
	// provider resource.
	contract.Assert(dst != nil)

	addType := func(_ string, t schema.PropertySpec) { m.addType(t.TypeSpec) }
	layerString := func(dst *string, src string) {
		if *dst == "" {
			*dst = src
		}
	}

	// Layer ObjectTypeSpec properties
	layerString(&dst.Description, src.Description)
	layerMap(&dst.Properties, src.Properties, addType)
	layerString(&dst.Type, src.Type)
	dst.Required = appendUnique(dst.Required, src.Required)
	dst.Plain = appendUnique(dst.Plain, src.Plain)
	layerMap(&dst.Language, src.Language, nil)

	// Layer Resource properties
	layerMap(&dst.InputProperties, src.InputProperties, addType)
	dst.RequiredInputs = appendUnique(dst.RequiredInputs, src.RequiredInputs)
	dst.PlainInputs = appendUnique(dst.PlainInputs, src.PlainInputs)
	dst.Aliases = appendUnique(dst.Aliases, src.Aliases)
	layerString(&dst.DeprecationMessage, src.DeprecationMessage)
	layerMap(&dst.Methods, src.Methods, nil)

	// Layer state inputs if non-nil
	if src.StateInputs == nil {
		return
	}
	if dst.StateInputs == nil {
		dst.StateInputs = &schema.ObjectTypeSpec{}
	}
	layerMap(&dst.StateInputs.Properties, src.StateInputs.Properties, addType)
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

type dispatchTableCtx struct {
	dispatchTable        dispatchTable
	dstSchema, srcSchema *schema.PackageSpec
	srcIndex             int
}

func (m *dispatchTableCtx) setResource(token string, resource schema.ResourceSpec) {
	_, ok := m.dispatchTable.Resources[token]
	if ok {
		return
	}
	m.dispatchTable.Resources[token] = m.srcIndex
	m.dstSchema.Resources[token] = resource
	m.addProperties(resource.InputProperties)
	m.addProperties(resource.Properties)
	if resource.StateInputs != nil {
		m.addProperties(resource.StateInputs.Properties)
	}
}

func (m *dispatchTableCtx) setFunction(token string, function schema.FunctionSpec) {
	_, ok := m.dispatchTable.Functions[token]
	if ok {
		return
	}
	m.dstSchema.Functions[token] = function

	m.addObjectType(function.Inputs)
	m.addObjectType(function.Outputs)
}

func (m *dispatchTableCtx) addProperties(properties map[string]schema.PropertySpec) {
	for _, t := range properties {
		m.addType(t.TypeSpec)
	}
}

func (m *dispatchTableCtx) addType(t schema.TypeSpec) {
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

func (m *dispatchTableCtx) addRefType(ref string) {
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

func (m *dispatchTableCtx) addComplexType(token string, typ schema.ComplexTypeSpec) {
	m.dstSchema.Types[token] = typ

	if typ.Type != "object" {
		// This was en enum, so we can just do the assignment.
		return
	}

	m.addObjectType(&typ.ObjectTypeSpec)
}

func (m *dispatchTableCtx) addObjectType(typ *schema.ObjectTypeSpec) {
	if typ == nil {
		return
	}
	m.addProperties(typ.Properties)
}
