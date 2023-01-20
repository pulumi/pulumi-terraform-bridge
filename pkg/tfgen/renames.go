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

package tfgen

import (
	"fmt"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfgen/internal/paths"

	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

// Tabulates information about custom inflected or pluralied names. Pulumi bridged providers and Terraform conversion
// tooling may need to consult these tables to accurately translate between Pulumi and Terraform names.
type Renames struct {
	// Resources[t] stores the Terraform name for a resource identified by the Pulumi token t.
	Resources map[tokens.Type]string `json:"resources,omitempty"`

	// Functions[t] stores the Terraform name for a data source that maps to a Pulumi function with token t.
	Functions map[tokens.ModuleMember]string `json:"functions,omitempty"`

	// RenamedProperties[t][p] stores the Terraform name of a Pulumi property p on a type identified by Pulumi token
	// t, when this name is changed, that is RenamedProperties[t][p] != p. Here the t token can represent a Resource
	// type, a datasource (function) type, or an auxiliary type.
	RenamedProperties map[tokens.Token]map[tokens.Name]string `json:"renamedProperties,omitempty"`

	// Similar to RenamedProperties but pertains to provider-level config.
	RenamedConfigProperties map[tokens.Name]string `json:"renamedConfigProperties,omitempty"`
}

func newRenames() Renames {
	return Renames{
		Resources:               map[tokens.Type]string{},
		Functions:               map[tokens.ModuleMember]string{},
		RenamedProperties:       map[tokens.Token]map[tokens.Name]string{},
		RenamedConfigProperties: map[tokens.Name]string{},
	}
}

func (r Renames) renamedProps(tok tokens.Token) map[tokens.Name]string {
	props, ok := r.RenamedProperties[tok]
	if ok {
		return props
	}
	props = map[tokens.Name]string{}
	r.RenamedProperties[tok] = props
	return props
}

type rename struct {
	parent paths.TypePath
	name   paths.PropertyName
}

type renamesBuilder struct {
	pkg            tokens.Package
	resourcePrefix string
	renames        []rename
	resources      map[tokens.Type]string
	functions      map[tokens.ModuleMember]string
	objectTypes    map[string]tokens.Type
}

func newRenamesBuilder(pkg tokens.Package, resourcePrefix string) *renamesBuilder {
	return &renamesBuilder{
		pkg:            pkg,
		resourcePrefix: resourcePrefix,
		resources:      map[tokens.Type]string{},
		functions:      map[tokens.ModuleMember]string{},
		objectTypes:    map[string]tokens.Type{},
	}
}

func (r *renamesBuilder) registerResource(rp *paths.ResourcePath) {
	if rp.IsProvider() {
		// Provider resources are not a Terraform concept and can be skipped here.
		return
	}
	r.resources[rp.Token()] = rp.Key()
}

func (r *renamesBuilder) registerDataSource(dp *paths.DataSourcePath) {
	r.functions[dp.Token()] = dp.Key()
}

func (r *renamesBuilder) registerNamedObjectType(paths paths.TypePathSet, token tokens.Type) {
	if r.objectTypes == nil {
		r.objectTypes = map[string]tokens.Type{}
	}
	for _, p := range paths.Paths() {
		r.objectTypes[p.String()] = token
	}
}

func (r *renamesBuilder) registerProperty(parent paths.TypePath, name paths.PropertyName) {
	if name.Key == string(name.Name) {
		return
	}
	r.renames = append(r.renames, rename{parent, name})
}

func (r *renamesBuilder) build() Renames {
	re := newRenames()

	for _, item := range r.renames {
		var m map[tokens.Name]string
		switch parent := item.parent.(type) {
		case *paths.ConfigPath:
			m = re.RenamedConfigProperties
		case *paths.ResourceMemberPath:
			m = re.renamedProps(tokens.Token(parent.ResourcePath.Token()))
		case *paths.DataSourceMemberPath:
			m = re.renamedProps(tokens.Token(parent.DataSourcePath.Token()))
		default:
			tok, err := r.findObjectTypeToken(parent)
			if err != nil {
				panic(err)
			}
			m = re.renamedProps(tokens.Token(tok))
		}
		m[item.name.Name] = item.name.Key
	}

	re.Functions = r.functions
	re.Resources = r.resources
	return re
}

func (r renamesBuilder) findObjectTypeToken(path paths.TypePath) (tokens.Type, error) {
	if p, ok := r.objectTypes[path.String()]; ok {
		return p, nil
	}
	// As an implementation quirk, sometimes for a provider registerNamedObjectType gets called on an input propety
	// but not the state property, though they represent the same thing and have the same type. Rewrite the path
	// to replace state with inputs in this case and try the lookup again.
	//
	// See Test_ProviderWithObjectTypesInConfigCanGenerateRenames
	path = r.normalizeProviderStateToProviderInputs(path)
	if p, ok := r.objectTypes[path.String()]; ok {
		return p, nil
	}
	return "", fmt.Errorf("expected registerNamedObjectType to be called for %s", path.String())
}

func (r renamesBuilder) normalizeProviderStateToProviderInputs(p paths.TypePath) paths.TypePath {
	switch pp := p.(type) {
	case *paths.ResourceMemberPath:
		if pp.ResourcePath.IsProvider() && pp.ResourceMemberKind == paths.ResourceState {
			return pp.ResourcePath.Inputs()
		}
		return p
	case *paths.DataSourceMemberPath:
		return p
	case *paths.ConfigPath:
		return p
	case *paths.ElementPath:
		return paths.NewElementPath(r.normalizeProviderStateToProviderInputs(pp.Parent()))
	case *paths.PropertyPath:
		return paths.NewProperyPath(r.normalizeProviderStateToProviderInputs(pp.Parent()), pp.PropertyName)
	default:
		panic("impossible")
	}
}
