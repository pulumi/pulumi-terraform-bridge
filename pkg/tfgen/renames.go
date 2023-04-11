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

func MergeRenames(renames []Renames) Renames {
	result := newRenames()
	for _, rename := range renames {
		for k, v := range rename.Resources {
			_, exists := result.Resources[k]
			if !exists {
				result.Resources[k] = v
			}
		}

		for k, v := range rename.Functions {
			_, exists := result.Functions[k]
			if !exists {
				result.Functions[k] = v
			}
		}
		for k, v := range rename.RenamedProperties {
			_, exists := result.RenamedProperties[k]
			if !exists {
				result.RenamedProperties[k] = v
			}
		}
		for k, v := range rename.RenamedConfigProperties {
			_, exists := result.RenamedConfigProperties[k]
			if !exists {
				result.RenamedConfigProperties[k] = v
			}
		}
	}
	return result
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

type renamesBuilder struct {
	pkg            tokens.Package
	resourcePrefix string
	properties     []struct {
		parent paths.TypePath
		name   paths.PropertyName
	}
	resources       map[tokens.Type]string
	functions       map[tokens.ModuleMember]string
	objectTypes     map[string]tokens.Type
	objectTypePaths map[tokens.Type]paths.TypePathSet
}

func newRenamesBuilder(pkg tokens.Package, resourcePrefix string) *renamesBuilder {
	return &renamesBuilder{
		pkg:             pkg,
		resourcePrefix:  resourcePrefix,
		resources:       map[tokens.Type]string{},
		functions:       map[tokens.ModuleMember]string{},
		objectTypes:     map[string]tokens.Type{},
		objectTypePaths: map[tokens.Type]paths.TypePathSet{},
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
	r.objectTypePaths[token] = paths
}

func (r *renamesBuilder) registerProperty(parent paths.TypePath, name paths.PropertyName) {
	r.properties = append(r.properties, struct {
		parent paths.TypePath
		name   paths.PropertyName
	}{parent: parent, name: name})
}

func (*renamesBuilder) isConfig(propertyParentPath paths.TypePath) bool {
	_, ok := propertyParentPath.(*paths.ConfigPath)
	return ok
}

func (r *renamesBuilder) propertyParentToken(propertyParentPath paths.TypePath) (tokens.Token, error) {
	switch parent := propertyParentPath.(type) {
	case *paths.ConfigPath:
		panic("ConfigPath should have been detected with isConfig")
	case *paths.ResourceMemberPath:
		return tokens.Token(parent.ResourcePath.Token()), nil
	case *paths.DataSourceMemberPath:
		return tokens.Token(parent.DataSourcePath.Token()), nil
	default:
		tok, err := r.findObjectTypeToken(parent)
		if err != nil {
			return "", err
		}
		return tokens.Token(tok), nil
	}
}

// Like BuildRenames().BuildConfigProperties  but retains all properties for validation.
func (r *renamesBuilder) BuildConfigProperties() []paths.PropertyName {
	configProps := []paths.PropertyName{}
	for _, item := range r.properties {
		if !r.isConfig(item.parent) {
			continue
		}
		configProps = append(configProps, item.name)
	}
	return r.dedup(configProps)
}

// Like BuildRenames().RenamedProperties but retains all properties for validation.
func (r *renamesBuilder) BuildProperties() (map[tokens.Token][]paths.PropertyName, error) {
	props := map[tokens.Token][]paths.PropertyName{}
	for _, item := range r.properties {
		if r.isConfig(item.parent) {
			continue
		}
		t, err := r.propertyParentToken(item.parent)
		if err != nil {
			return nil, err
		}
		props[t] = append(props[t], item.name)
	}
	for k := range props {
		props[k] = r.dedup(props[k])
	}
	return props, nil
}

// Multiple locations may share a named Pulumi object type. This reverse lookup finds all locations that map to an
// object type identified by tok.
func (r *renamesBuilder) ObjectTypePaths(tok tokens.Type) paths.TypePathSet {
	res := r.objectTypePaths[tok]
	if res == nil {
		res = paths.NewTypePathSet()
	}
	return res
}

func (*renamesBuilder) dedup(names []paths.PropertyName) []paths.PropertyName {
	seen := map[string]bool{}
	uniq := []paths.PropertyName{}
	for _, pn := range names {
		k := fmt.Sprintf("%d%s%d%s", len(pn.Key), pn.Key, len(pn.Name), pn.Name)
		if !seen[k] {
			uniq = append(uniq, pn)
		}
		seen[k] = true
	}
	return uniq
}

func (r *renamesBuilder) BuildRenames() (Renames, error) {
	re := newRenames()

	for _, item := range r.properties {
		if item.name.Key == string(item.name.Name) {
			continue
		}

		var m map[tokens.Name]string
		if r.isConfig(item.parent) {
			m = re.RenamedConfigProperties
		} else {
			t, err := r.propertyParentToken(item.parent)
			if err != nil {
				return Renames{}, err
			}
			m = re.renamedProps(t)
		}
		m[item.name.Name] = item.name.Key
	}

	re.Functions = r.functions
	re.Resources = r.resources
	return re, nil
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
		panic("normalizeProviderStateToProviderInputs: impossible case")
	}
}
