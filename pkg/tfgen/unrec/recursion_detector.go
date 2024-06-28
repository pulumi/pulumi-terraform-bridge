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
// This needs to use an approximate and not strict equality because the leaf node of a recursively unrolled type will
// drop recursive properties and therefore not strictly match the ancestor.
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

func (rd *recursionDetector) Detect(types []tokens.Type) []tokens.Type {
	vis := &typeVisitor{Schema: rd.schema, Visit: rd.visit}
	vis.VisitTypes(types...)
	result := []tokens.Type{}
	for t := range rd.detectedRecursiveTypes {
		result = append(result, t)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i] < result[j]
	})
	return result
}

func (rd *recursionDetector) visit(ancestors []tokens.Type, current tokens.Type) bool {
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
	return rd.cmp.LessThanOrEqualTypeRefs(t3, t2) && rd.cmp.LessThanOrEqualTypeRefs(t2, t1)
}
