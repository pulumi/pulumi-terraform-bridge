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
	"sort"

	pschema "github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

// Detects unrolled recursion.
//
// TF does not support recursive type definitions but Pulumi does. The idea here is to take a projection of TF recursion
// unrolling trick and fold it back.
//
// Suppose ancestor(T1, T2) if T2 describes a sub-property of T1.
//
// Suppose T1 <= T2 if T1 has the subset of properties of T2 with types that are themselves Tx <= Ty.
//
// The algo looks for this pattern to detect recursion:
//
//	ancestor(T1, T2), ancestor(T2, T3),  T3 <= T2 <= T1
//
// Such that:
//
//	set(props(T1)) = set(props(T2))
//
// This needs to use an approximate and not strict equality because the leaf node of a recursively unrolled type will
// drop recursive properties and therefore not strictly match the ancestor. The prop-set condition prevents accidentally
// identifying non-recursive subset instances as recursive instances.
type recursionDetector struct {
	schema                 *pschema.PackageSpec
	detectedRecursiveTypes map[tokens.Type]struct{}
	cmp                    *comparer
}

func newRecursionDetector(schema *pschema.PackageSpec) *recursionDetector {
	return &recursionDetector{
		schema:                 schema,
		detectedRecursiveTypes: map[tokens.Type]struct{}{},
		cmp:                    &comparer{schema: schema},
	}
}

// Starting from the set of starterTypes, detects recursion and reports it. The keys of the resulting map are the
// recursion roots, and the values are sets of recursive instances for each root.
func (rd *recursionDetector) Detect(starterTypes []tokens.Type) map[tokens.Type]map[tokens.Type]struct{} {
	// First pass: detect recursion roots.
	vis := &typeVisitor{Schema: rd.schema, Visit: rd.detectRootsVisitor}
	vis.VisitTypes(starterTypes...)

	roots := rd.roots()

	detected := map[tokens.Type]map[tokens.Type]struct{}{}
	for _, r := range roots {
		detected[r] = map[tokens.Type]struct{}{}
	}

	// Second pass: detect instances.
	vis2 := &typeVisitor{Schema: rd.schema, Visit: func(_ []tokens.Type, current tokens.Type) bool {
		for _, root := range roots {
			if rd.cmp.LessThanOrEqualTypeRefs(current, root) && current != root {
				detected[root][current] = struct{}{}
				return true
			}
		}
		return true
	}}

	vis2.VisitTypes(starterTypes...)
	return detected
}

func (rd *recursionDetector) sorted(types []tokens.Type) []tokens.Type {
	tokens := slices.Clone(types)
	sort.Slice(tokens, func(i, j int) bool {
		if len(tokens[i]) < len(tokens[j]) {
			return true
		}
		return tokens[i] < tokens[j]
	})
	return tokens
}

func (rd *recursionDetector) unique(types []tokens.Type) []tokens.Type {
	result := []tokens.Type{}
	for _, t := range types {
		seen := false
		for _, s := range result {
			if rd.cmp.EqualTypeRefs(s, t) {
				seen = true
				break
			}
		}
		if !seen {
			result = append(result, t)
		}
	}
	return result
}

func (rd *recursionDetector) roots() []tokens.Type {
	tt := []tokens.Type{}
	for t := range rd.detectedRecursiveTypes {
		tt = append(tt, t)
	}
	return rd.unique(rd.sorted(tt))
}

func (rd *recursionDetector) detectRootsVisitor(ancestors []tokens.Type, current tokens.Type) bool {
	for i, ai := range ancestors {
		if _, visited := rd.detectedRecursiveTypes[ai]; visited {
			continue
		}
		for _, aj := range ancestors[i+1:] {
			if rd.detect(ai, aj, current) {
				if rd.detectedRecursiveTypes == nil {
					rd.detectedRecursiveTypes = map[tokens.Type]struct{}{}
				}
				rd.detectedRecursiveTypes[ai] = struct{}{}
				return true
			}
		}
	}
	return true
}

func (rd *recursionDetector) detect(t1, t2, t3 tokens.Type) bool {
	return rd.cmp.LessThanTypeRefs(t3, t2) && rd.cmp.LessThanTypeRefs(t2, t1) && rd.sameProps(t1, t2)
}

func (rd *recursionDetector) sameProps(t1, t2 tokens.Type) bool {
	t1d, ok1 := rd.schema.Types[string(t1)]
	t2d, ok2 := rd.schema.Types[string(t2)]
	if !ok1 || !ok2 {
		return false
	}
	if len(t1d.Properties) != len(t2d.Properties) {
		return false
	}
	for k := range t1d.Properties {
		if _, ok := t2d.Properties[k]; !ok {
			return false
		}
	}
	return true
}