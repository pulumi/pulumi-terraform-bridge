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

package paths

import (
	"fmt"
	"sort"

	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

// TypePath values uniquely identify locations within a Pulumi Package Schema that require generating types in a target
// programming language when a provider SDK for that language is being built. Examples of such types include resources
// (see ResourcePath), data sources (DataSourcePath), provider configuration (ConfigPath), and nested object types that
// are used to describe the type of resource properties.
type TypePath interface {
	// Parent path, can be nil for root paths.
	Parent() TypePath

	// Useful for comparing paths.
	UniqueKey() string

	// Human friendly representation.
	String() string
}

// Identifies a resource uniquely.
type ResourcePath struct {
	key        string
	token      tokens.Type
	isProvider bool
}

func NewResourcePath(terraformResourceKey string, resourceToken tokens.Type, isProvider bool) *ResourcePath {
	if isProvider && terraformResourceKey != "" {
		panic("terraformResourceKey should be empty when isProvider=true")
	}
	if !isProvider && terraformResourceKey == "" {
		panic("terraformResourceKey should not be empty when isProvider=false")
	}
	return &ResourcePath{
		key:        terraformResourceKey,
		token:      resourceToken,
		isProvider: isProvider,
	}
}

// Raw name from shim.ResourceMap, typically the Terraform name uniquely identifying the Resource. Will be empty if
// IsProvider() is true and the resource is a Provider resource, a Pulumi-only concept.
func (p *ResourcePath) Key() string {
	return p.key
}

// Unquely identifies the Resource in Pulumi Schema.
//
// See also: https://www.pulumi.com/docs/guides/pulumi-packages/schema/
func (p *ResourcePath) Token() tokens.Type {
	return p.token
}

func (p *ResourcePath) IsProvider() bool {
	return p.isProvider
}

func (p *ResourcePath) Inputs() *ResourceMemberPath {
	return &ResourceMemberPath{
		ResourcePath:       p,
		ResourceMemberKind: ResourceInputs,
	}
}

func (p *ResourcePath) Outputs() *ResourceMemberPath {
	return &ResourceMemberPath{
		ResourcePath:       p,
		ResourceMemberKind: ResourceOutputs,
	}
}

func (p *ResourcePath) State() *ResourceMemberPath {
	return &ResourceMemberPath{
		ResourcePath:       p,
		ResourceMemberKind: ResourceState,
	}
}

func (p *ResourcePath) String() string {
	if p.isProvider {
		return fmt.Sprintf("resource[provider=%q]", p.token.String())
	}
	return fmt.Sprintf("resource[key=%q,token=%q]",
		p.key,
		p.token.String())
}

type ResourceMemberKind int

const (
	ResourceInputs ResourceMemberKind = iota
	ResourceOutputs
	ResourceState
)

func (s ResourceMemberKind) String() string {
	switch s {
	case ResourceInputs:
		return "inputs"
	case ResourceOutputs:
		return "outputs"
	case ResourceState:
		return "state"
	}
	return "unknown"
}

type ResourceMemberPath struct {
	ResourcePath       *ResourcePath
	ResourceMemberKind ResourceMemberKind
}

var _ TypePath = (*ResourceMemberPath)(nil)

func (p *ResourceMemberPath) Parent() TypePath {
	return nil
}

func (p *ResourceMemberPath) UniqueKey() string {
	return p.String()
}

func (p *ResourceMemberPath) String() string {
	return fmt.Sprintf("%s.%s",
		p.ResourcePath.String(),
		p.ResourceMemberKind.String())
}

// Identifies a data source uniquely.
type DataSourcePath struct {
	key   string
	token tokens.ModuleMember
}

func NewDataSourcePath(key string, token tokens.ModuleMember) *DataSourcePath {
	return &DataSourcePath{
		key:   key,
		token: token,
	}
}

// Pulumi token uniquely identifiying the DataSource.
func (p *DataSourcePath) Token() tokens.ModuleMember {
	return p.token
}

// Unique identifier for the DataSource preserved from the shim layer, typically the Terraform name.
func (p *DataSourcePath) Key() string {
	return p.key
}

func (p *DataSourcePath) Args() *DataSourceMemberPath {
	return &DataSourceMemberPath{
		DataSourcePath:       p,
		DataSourceMemberKind: DataSourceArgs,
	}
}

func (p *DataSourcePath) Results() *DataSourceMemberPath {
	return &DataSourceMemberPath{
		DataSourcePath:       p,
		DataSourceMemberKind: DataSourceResults,
	}
}

func (p *DataSourcePath) String() string {
	return fmt.Sprintf("datasource[key=%q,token=%q]",
		p.key,
		p.token.String())
}

type DataSourceMemberKind int

const (
	DataSourceArgs DataSourceMemberKind = iota
	DataSourceResults
)

func (s DataSourceMemberKind) String() string {
	switch s {
	case DataSourceArgs:
		return "args"
	case DataSourceResults:
		return "results"
	}
	return "unknown"
}

type DataSourceMemberPath struct {
	DataSourcePath       *DataSourcePath
	DataSourceMemberKind DataSourceMemberKind
}

var _ TypePath = (*DataSourceMemberPath)(nil)

func (p *DataSourceMemberPath) Parent() TypePath {
	return nil
}

func (p *DataSourceMemberPath) String() string {
	return fmt.Sprintf("%s.%s",
		p.DataSourcePath.String(),
		p.DataSourceMemberKind.String())
}

func (p *DataSourceMemberPath) UniqueKey() string {
	return p.String()
}

type ConfigPath struct{}

var _ TypePath = (*ConfigPath)(nil)

func NewConfigPath() *ConfigPath {
	return &ConfigPath{}
}

func (p *ConfigPath) UniqueKey() string {
	return p.String()
}

func (p *ConfigPath) String() string {
	return "config"
}

func (p *ConfigPath) Parent() TypePath {
	return nil
}

type PropertyPath struct {
	parent       TypePath
	PropertyName PropertyName
}

var _ TypePath = (*PropertyPath)(nil)

func NewProperyPath(parent TypePath, name PropertyName) *PropertyPath {
	return &PropertyPath{parent: parent, PropertyName: name}
}

func (p *PropertyPath) Parent() TypePath {
	return p.parent
}

func (p *PropertyPath) UniqueKey() string {
	return p.String()
}

func (p *PropertyPath) String() string {
	path := p.PropertyName.String()
	if path != "" {
		path = "." + path
	}
	return fmt.Sprintf("%s%s", p.Parent().String(), path)
}

// Represents an element of List or Map or Set type represented by Parent.
type ElementPath struct {
	parent TypePath
}

var _ TypePath = (*ElementPath)(nil)

func NewElementPath(parent TypePath) *ElementPath {
	return &ElementPath{parent}
}

func (p *ElementPath) UniqueKey() string {
	return p.String()
}

func (p *ElementPath) String() string {
	return fmt.Sprintf("%s.$", p.parent.String())
}

func (p *ElementPath) Parent() TypePath {
	return p.parent
}

// Type paths include property names as path fragments.
type PropertyName struct {
	// The original name from the shim (typically the Terraform name). Example: "bcrypt_hash".
	Key string

	// Possibly inflected Pulumi name, typically but not always equal to Key. Example inflection: "bcryptHash".
	Name tokens.Name
}

func (pn PropertyName) String() string {
	if string(pn.Name) == pn.Key {
		return pn.Key
	}
	return fmt.Sprintf("%s[pulumi:%q]", pn.Key, pn.Name.String())
}

type TypePathSet map[string]TypePath

func NewTypePathSet() TypePathSet { return TypePathSet(map[string]TypePath{}) }

func SingletonTypePathSet(typePath TypePath) TypePathSet {
	s := NewTypePathSet()
	s.Add(typePath)
	return s
}

func (s TypePathSet) Add(p TypePath) {
	s[p.UniqueKey()] = p
}

func (s TypePathSet) Paths() []TypePath {
	res := []TypePath{}
	for _, v := range s {
		res = append(res, v)
	}
	sort.Slice(res, func(i, j int) bool {
		return res[i].UniqueKey() < res[j].UniqueKey()
	})
	return res
}

// RawTypePath represents a type anchored from an opaque user provided type.
type RawTypePath struct {
	t              tokens.Type
	structuralPath TypePath
}

func NewRawPath(typ tokens.Type, structuralPath TypePath) *RawTypePath {
	return &RawTypePath{typ, structuralPath}
}

var _ TypePath = (*RawTypePath)(nil)

func (p *RawTypePath) Parent() TypePath { return nil }

// Useful for comparing paths.
func (p *RawTypePath) UniqueKey() string { return p.t.String() }

// Human friendly representation.
func (p *RawTypePath) String() string { return p.t.Name().String() }

func (p *RawTypePath) Raw() tokens.Type { return tokens.Type(p.t) }

func (p *RawTypePath) StructuralPath() TypePath { return p.structuralPath }
