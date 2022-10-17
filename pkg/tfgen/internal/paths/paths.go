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

	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

type ResourcePath struct {
	token      tokens.Type
	IsProvider bool
}

func NewResourcePath(resourceToken tokens.Type, isProvider bool) *ResourcePath {
	return &ResourcePath{
		token:      resourceToken,
		IsProvider: isProvider,
	}
}

func (p *ResourcePath) Token() tokens.Type {
	return p.token
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
	return fmt.Sprintf("resource[%s]", p.token.String())
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

func (p *ResourceMemberPath) Property(name string) *TypePath {
	return &TypePath{
		parent:     p,
		path:       name,
		parentKind: ResourceMemberPathParent,
	}
}

func (p *ResourceMemberPath) String() string {
	return fmt.Sprintf("%s.%s",
		p.ResourcePath.String(),
		p.ResourceMemberKind.String())
}

type DataSourcePath struct {
	token tokens.ModuleMember
}

func NewDataSourcePath(token tokens.ModuleMember) *DataSourcePath {
	return &DataSourcePath{token: token}
}

func (p *DataSourcePath) Token() tokens.ModuleMember {
	return p.token
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
	return fmt.Sprintf("datasource[%s]", p.token.String())
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

func (p *DataSourceMemberPath) Property(name string) *TypePath {
	return &TypePath{
		parent:     p,
		path:       name,
		parentKind: DataSourceMemberPathParent,
	}
}

func (p *DataSourceMemberPath) String() string {
	return fmt.Sprintf("%s.%s",
		p.DataSourcePath.String(),
		p.DataSourceMemberKind.String())
}

type ConfigPath struct{}

func NewConfigPath() *ConfigPath {
	return &ConfigPath{}
}

func (p *ConfigPath) Property(name string) *TypePath {
	return &TypePath{
		parent:     p,
		path:       name,
		parentKind: ConfigPathParent,
	}
}

func (p *ConfigPath) String() string {
	return "config"
}

const ElementPathFragment = "$"

type TypePath struct {
	parent     interface{}
	path       string
	parentKind ResourceTypeParentKind
}

func (p *TypePath) Property(name string) *TypePath {
	return &TypePath{
		parent:     p,
		path:       name,
		parentKind: TypePathParent,
	}
}

func (p *TypePath) Element() *TypePath {
	return &TypePath{
		parent:     p,
		path:       ElementPathFragment,
		parentKind: TypePathParent,
	}
}

func (p *TypePath) ParentKind() ResourceTypeParentKind {
	return p.parentKind
}

func (p *TypePath) TypePathParent() *TypePath {
	if p.parentKind == TypePathParent {
		return p.parent.(*TypePath)
	}
	return nil
}

func (p *TypePath) DataSourceParent() *DataSourceMemberPath {
	if p.parentKind == DataSourceMemberPathParent {
		return p.parent.(*DataSourceMemberPath)
	}
	return nil
}

func (p *TypePath) ResourceParent() *ResourceMemberPath {
	if p.parentKind == ResourceMemberPathParent {
		return p.parent.(*ResourceMemberPath)
	}
	return nil
}

func (p *TypePath) ConfigParent() *ConfigPath {
	if p.parentKind == ConfigPathParent {
		return p.parent.(*ConfigPath)
	}
	return nil
}

func (p *TypePath) String() string {
	path := p.path
	if path != "" {
		path = "." + path
	}
	return fmt.Sprintf("%v%s", p.parent, path)
}

type ResourceTypeParentKind int

const (
	DataSourceMemberPathParent ResourceTypeParentKind = iota
	ResourceMemberPathParent
	ConfigPathParent
	TypePathParent
)

func (s ResourceTypeParentKind) String() string {
	switch s {
	case DataSourceMemberPathParent:
		return "datasource"
	case ResourceMemberPathParent:
		return "resource"
	case ConfigPathParent:
		return "config"
	case TypePathParent:
		return "type"
	}
	return "unknown"
}

type NamedTypePathContainer interface {
	Property(name string) *TypePath
}
