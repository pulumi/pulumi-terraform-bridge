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
	"context"

	pschema "github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

func SimplifyRecursiveTypes(ctx context.Context, schema *pschema.PackageSpec) error {
	rewriteRules, err := computeAllRewriteRules(schema)
	if err != nil {
		return err
	}
	return rewriteTypeRefs(rewriteRules, schema)
}

func computeAllRewriteRules(schema *pschema.PackageSpec) (map[tokens.Type]tokens.Type, error) {
	cmp := &comparer{schema: schema}

	rewriteRules := make(map[tokens.Type]tokens.Type)

	for _, r := range schema.Resources {
		typeRefs, err := findResourceTypeReferences(r)
		if err != nil {
			return nil, err
		}
		if err := computeRewriteRules(cmp, schema, typeRefs, rewriteRules); err != nil {
			return nil, err
		}
	}

	for _, f := range schema.Functions {
		typeRefs, err := findFunctionTypeReferences(f)
		if err != nil {
			return nil, err
		}
		if err := computeRewriteRules(cmp, schema, typeRefs, rewriteRules); err != nil {
			return nil, err
		}
	}

	return rewriteRules, nil
}

// Starting from starterTypes, detect recursive type roots. Simplify all types reachable from the recursive type roots.
// Simplification rewrites type A with type B if A<=B according to [comparer.LessThanTypeRefs].
//
// Although scoped to the progeny of recursive types, this may still end up over-eagerly rewriting logically distinct
// but structurally identical types and may require some refinement.
func computeRewriteRules(
	cmp *comparer,
	schema *pschema.PackageSpec,
	starterTypes typeRefs,
	rewriteRules map[tokens.Type]tokens.Type,
) error {
	// Detect recursive type patterns first.
	rd := newRecursionDetector(schema)
	recursionGraph := rd.Detect(starterTypes.Slice())

	// Rewrite every recursive type into its root.
	for root, instances := range recursionGraph {
		for instance := range instances {
			rewriteRules[instance] = root
		}
	}

	// Find all types reachable from the recursive type references.
	recursiveRefs := newTypeRefs()
	for root, instances := range recursionGraph {
		recursiveRefs.Add(root)
		for instance := range instances {
			recursiveRefs.Add(instance)
		}
	}
	allRefs, err := findTypeReferenceTransitiveClosure(schema, recursiveRefs)
	if err != nil {
		return err
	}

	// Rewrite identical types into "best" types minimizing token length.
	for _, eqc := range typeEqualityClasses(cmp.WithRewrites(rewriteRules), allRefs) {
		best := eqc.Best()
		for _, typ := range eqc.Slice() {
			if typ != best {
				rewriteRules[typ] = best
			}
		}
	}

	return nil
}

func typeEqualityClasses(cmp *comparer, types typeRefs) []typeRefs {
	eq := func(a, b tokens.Type) bool {
		return cmp.EqualTypeRefs(a, b)
	}
	eClasses := equalityClasses(eq, types.Slice())
	var result []typeRefs
	for _, c := range eClasses {
		tr := make(typeRefs)
		tr.Add(c...)
		result = append(result, tr)
	}
	return result
}
