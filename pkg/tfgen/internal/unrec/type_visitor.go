// Copyright 2016-2024, Pulumi Corporation.
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

package unrec

import (
	"slices"

	pschema "github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

type typeVisitor struct {
	Schema *pschema.PackageSpec
	Visit  func(ancestors []tokens.Type, current tokens.Type)

	parent   *typeVisitor             // internal
	visiting tokens.Type              // internal
	seen     map[tokens.Type]struct{} // internal
}

func (tv *typeVisitor) VisitRoots(roots []tokens.Type) {
	for _, ty := range roots {
		tv.visitLocalType(ty)
	}
}

func (tv *typeVisitor) push(ty tokens.Type) *typeVisitor {
	copy := *tv
	copy.parent = tv
	copy.visiting = ty
	return &copy
}

func (tv *typeVisitor) ancestors() []tokens.Type {
	var result []tokens.Type
	x := tv
	for x != nil {
		if x.visiting != "" {
			result = append(result, x.visiting)
		}
		x = x.parent
	}
	slices.Reverse(result)
	return result
}

func (tv *typeVisitor) visitLocalType(ty tokens.Type) {
	cts, found := tv.Schema.Types[string(ty)]
	if !found {
		return
	}
	if tv.seen == nil {
		tv.seen = map[tokens.Type]struct{}{}
	}
	if _, seen := tv.seen[ty]; seen {
		return
	}
	tv.seen[ty] = struct{}{}
	tv.Visit(tv.ancestors(), ty)
	tv.push(ty).visitComplexTypeSpec(cts)
}

func (tv *typeVisitor) visitComplexTypeSpec(cts pschema.ComplexTypeSpec) {
	if cts.Enum != nil {
		return
	}
	tv.visitObjectTypeSpec(cts.ObjectTypeSpec)
}

func (tv *typeVisitor) visitObjectTypeSpec(ots pschema.ObjectTypeSpec) {
	for _, prop := range ots.Properties {
		tv.visitPropertySpec(prop)
	}
}

func (tv *typeVisitor) visitPropertySpec(ots pschema.PropertySpec) {
	tv.visitTypeSpec(ots.TypeSpec)
}

func (tv *typeVisitor) visitTypeSpec(ots pschema.TypeSpec) {
	if localType, ok := parseLocalRef(ots.Ref); ok {
		tv.visitLocalType(localType)
		return
	}
	if ots.Items != nil {
		tv.visitTypeSpec(*ots.Items)
	}
	if ots.AdditionalProperties != nil {
		tv.visitTypeSpec(*ots.AdditionalProperties)
	}
	for _, ty := range ots.OneOf {
		tv.visitTypeSpec(ty)
	}
}
