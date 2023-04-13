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

package convert

import (
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

// Subset of pschema.PackageSpec required for conversion.
type PackageSpec interface {
	Config() *schema.ConfigSpec
	Function(tok tokens.ModuleMember) *schema.FunctionSpec
	Resource(tok tokens.Type) *schema.ResourceSpec
	Type(tok tokens.Type) *schema.ComplexTypeSpec
}

func PrecomputedPackageSpec(s *schema.PackageSpec) PackageSpec {
	return &packageSpec{s}
}

type packageSpec struct {
	spec *schema.PackageSpec
}

var _ PackageSpec = (*packageSpec)(nil)

func (p packageSpec) Config() *schema.ConfigSpec {
	return &p.spec.Config
}

func (p packageSpec) Resource(tok tokens.Type) *schema.ResourceSpec {
	res, ok := p.spec.Resources[string(tok)]
	if ok {
		return &res
	}
	return nil
}

func (p packageSpec) Type(tok tokens.Type) *schema.ComplexTypeSpec {
	typ, ok := p.spec.Types[string(tok)]
	if ok {
		return &typ
	}
	return nil
}

func (p packageSpec) Function(tok tokens.ModuleMember) *schema.FunctionSpec {
	res, ok := p.spec.Functions[string(tok)]
	if ok {
		return &res
	}
	return nil
}
