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
	"bytes"
	"reflect"

	pschema "github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

type comparer struct {
	// Local type-ref comparisons are scoped to a package schema.
	schema *pschema.PackageSpec

	// Set of implicit type rewrites to consider when comparing.
	rewrites map[tokens.Type]tokens.Type
}

func (cmp *comparer) WithRewrites(rewrites map[tokens.Type]tokens.Type) *comparer {
	return &comparer{
		schema:   cmp.schema,
		rewrites: rewrites,
	}
}

func (cmp *comparer) EqualTypeRefs(a, b tokens.Type) bool {
	g := &generalizedComparer{schema: cmp.schema, rewrites: cmp.rewrites}
	g.EqualXPropertyMaps = g.strictlyEqualXPropertyMaps
	return g.EqualTypeRefs(a, b)
}

func (cmp *comparer) LessThanTypeRefs(a, b tokens.Type) bool {
	return cmp.LessThanOrEqualTypeRefs(a, b) && !cmp.EqualTypeRefs(a, b)
}

// A type will be considered "less than" another type if both are locally defined object types and A defines a subset of
// B's properties. This is useful to deal with property dropout during recursive type expansions.
func (cmp *comparer) LessThanOrEqualTypeRefs(a, b tokens.Type) (eq bool) {
	g := &generalizedComparer{schema: cmp.schema, rewrites: cmp.rewrites}
	g.EqualXPropertyMaps = g.lessThanOrEqualXPropertyMaps
	return g.EqualTypeRefs(a, b)
}

// Generalizing structural comparisons to specialize for A=B and A<=B separately.
type generalizedComparer struct {
	schema             *pschema.PackageSpec
	rewrites           map[tokens.Type]tokens.Type
	EqualXPropertyMaps func(xPropertyMap, xPropertyMap) bool
}

func (cmp *generalizedComparer) lessThanOrEqualXPropertyMaps(a, b xPropertyMap) bool {
	// Empty objects are treated specially and are never {}<=X.
	if len(a) == 0 || len(b) == 0 {
		return len(a) == len(b)
	}
	for aK, aP := range a {
		bP, ok := b[aK]
		// Every key in A should also be a key in B.
		if !ok {
			return false
		}
		if !cmp.EqualXProperties(aP, bP) {
			return false
		}
	}
	return true
}

func (cmp *generalizedComparer) strictlyEqualXPropertyMaps(a, b xPropertyMap) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	if len(a) != len(b) {
		return false
	}
	for k, av := range a {
		bv, ok := b[k]
		if !ok {
			return false
		}
		if !cmp.EqualXProperties(av, bv) {
			return false
		}
	}
	return true
}

func (cmp *generalizedComparer) rewrite(a tokens.Type) tokens.Type {
	if cmp.rewrites == nil {
		return a
	}
	if x, ok := cmp.rewrites[a]; ok {
		return x
	}
	return a
}

func (cmp *generalizedComparer) EqualTypeRefs(a, b tokens.Type) bool {
	a, b = cmp.rewrite(a), cmp.rewrite(b)
	if a == b {
		return true
	}
	aT, gotA := cmp.schema.Types[string(a)]
	bT, gotB := cmp.schema.Types[string(b)]
	if gotA && gotB {
		return cmp.EqualComplexTypeSpecs(&aT, &bT)
	}
	return false
}

func (cmp *generalizedComparer) EqualRawRefs(a, b string) bool {
	if a == b {
		return true
	}
	aT, ok1 := parseLocalRef(a)
	bT, ok2 := parseLocalRef(b)
	if ok1 && ok2 {
		return cmp.EqualTypeRefs(aT, bT)
	}
	return false
}

func (cmp *generalizedComparer) EqualXProperties(a, b xProperty) bool {
	if a.IsRequired != b.IsRequired {
		return false
	}
	if a.IsPlain != b.IsPlain {
		return false
	}
	if !cmp.EqualPropertySpecs(&a.PropertySpec, &b.PropertySpec) {
		return false
	}
	return true
}

func (cmp *generalizedComparer) EqualObjectTypeSpecs(a, b pschema.ObjectTypeSpec) bool {
	if a.Type != b.Type {
		return false
	}
	if a.IsOverlay != b.IsOverlay {
		return false
	}
	if !cmp.EqualLanguageMaps(a.Language, b.Language) {
		return false
	}
	if a.Description != b.Description {
		return false
	}
	if !cmp.EqualXPropertyMaps(newXPropertyMap(a), newXPropertyMap(b)) {
		return false
	}
	return true
}

func (cmp *generalizedComparer) EqualComplexTypeSpecs(a, b *pschema.ComplexTypeSpec) bool {
	// Do not identify enum equality yet.
	if a.Enum != nil || b.Enum != nil {
		return false
	}
	if !cmp.EqualObjectTypeSpecs(a.ObjectTypeSpec, b.ObjectTypeSpec) {
		return false
	}
	return true
}

func (cmp *generalizedComparer) EqualLanguageMaps(a, b map[string]pschema.RawMessage) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	if len(a) != len(b) {
		return false
	}
	for k, av := range a {
		bv, ok := b[k]
		if !ok {
			return false
		}
		if !bytes.Equal(av, bv) {
			return false
		}
	}
	return true
}

func (cmp *generalizedComparer) EqualTypeSpecLists(a, b []pschema.TypeSpec) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !cmp.EqualTypeSpecs(&a[i], &b[i]) {
			return false
		}
	}
	return true
}

func (cmp *generalizedComparer) EqualStringSlices(a, b []string) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func (cmp *generalizedComparer) EqualStringMaps(a, b map[string]string) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	if len(a) != len(b) {
		return false
	}
	for k, av := range a {
		bv, ok := b[k]
		if !ok {
			return false
		}
		if av != bv {
			return false
		}
	}
	return true
}

func (cmp *generalizedComparer) EqualDiscriminatorSpecs(a, b *pschema.DiscriminatorSpec) bool {
	if a == nil || b == nil {
		return a == b
	}
	if a.PropertyName != b.PropertyName {
		return false
	}
	if !cmp.EqualStringMaps(a.Mapping, b.Mapping) {
		return false
	}
	return true
}

func (cmp *generalizedComparer) EqualTypeSpecs(a, b *pschema.TypeSpec) bool {
	if a == nil || b == nil {
		return a == b
	}
	if a.Type != b.Type {
		return false
	}
	if !cmp.EqualRawRefs(a.Ref, b.Ref) {
		return false
	}
	if !cmp.EqualTypeSpecs(a.AdditionalProperties, b.AdditionalProperties) {
		return false
	}
	if !cmp.EqualTypeSpecs(a.Items, b.Items) {
		return false
	}
	if !cmp.EqualTypeSpecLists(a.OneOf, b.OneOf) {
		return false
	}
	if !cmp.EqualDiscriminatorSpecs(a.Discriminator, b.Discriminator) {
		return false
	}
	if a.Plain != b.Plain {
		return false
	}
	return true
}

func (cmp *generalizedComparer) EqualPropertySpecs(a, b *pschema.PropertySpec) bool {
	if !cmp.EqualTypeSpecs(&a.TypeSpec, &b.TypeSpec) {
		return false
	}
	if a.Description != b.Description {
		return false
	}
	if !reflect.DeepEqual(a.Const, b.Const) {
		return false
	}
	if !reflect.DeepEqual(a.Default, b.Default) {
		return false
	}
	if !cmp.EqualDefaultSpecs(a.DefaultInfo, b.DefaultInfo) {
		return false
	}
	if a.DeprecationMessage != b.DeprecationMessage {
		return false
	}
	if !cmp.EqualLanguageMaps(a.Language, b.Language) {
		return false
	}
	if a.Secret != b.Secret {
		return false
	}
	if a.ReplaceOnChanges != b.ReplaceOnChanges {
		return false
	}
	if a.WillReplaceOnChanges != b.WillReplaceOnChanges {
		return false
	}
	return true
}

func (cmp *generalizedComparer) EqualDefaultSpecs(a, b *pschema.DefaultSpec) bool {
	if a == nil || b == nil {
		return a == b
	}
	if !cmp.EqualLanguageMaps(a.Language, b.Language) {
		return false
	}
	if !cmp.EqualStringSlices(a.Environment, b.Environment) {
		return false
	}
	return true
}
