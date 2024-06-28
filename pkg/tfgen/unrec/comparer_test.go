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
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/stretchr/testify/require"
)

func TestComparerAndStatement(t *testing.T) {
	t1 := "myprov:index/WebAclStatementAndStatementStatementAndStatementStatement:WebAclStatementAndStatementStatementAndStatementStatement"
	t2 := "myprov:index/WebAclStatementAndStatementStatement:WebAclStatementAndStatementStatement"
	s := exampleSchema(t)
	cmp := &comparer{s, nil}
	require.Truef(t, cmp.LessThanTypeRefs(tokens.Type(t1), tokens.Type(t2)), "A<B")
}

func TestComparer(t *testing.T) {
	s := exampleSchema(t)
	cmp := &comparer{s, nil}

	allTypes := []tokens.Type{}
	for t := range s.Types {
		allTypes = append(allTypes, tokens.Type(t))
	}

	t.Run("A=B is reflexive", func(t *testing.T) {
		checkReflexive(t, allTypes, cmp.EqualTypeRefs)
	})

	t.Run("A=B is symmetric", func(t *testing.T) {
		checkSymmetric(t, allTypes, cmp.EqualTypeRefs)
	})

	t.Run("A=B is transitive", func(t *testing.T) {
		checkTransitive(t, allTypes, cmp.EqualTypeRefs)
	})

	t.Run("A <= B is reflexive", func(t *testing.T) {
		checkReflexive(t, allTypes, cmp.LessThanOrEqualTypeRefs)
	})

	t.Run("A <= B is antisymmetric", func(t *testing.T) {
		checkAntisymmetric(t, allTypes, cmp.EqualTypeRefs, cmp.LessThanOrEqualTypeRefs)
	})

	t.Run("A <= B is transitive", func(t *testing.T) {
		checkTransitive(t, allTypes, cmp.LessThanOrEqualTypeRefs)
	})
}

func checkReflexive[T any](t *testing.T, universe []T, r func(a, b T) bool) {
	for _, a := range universe {
		require.Truef(t, r(a, a), "reflexivity does not hold: a=%v", a)
	}
}

func checkSymmetric[T any](t *testing.T, universe []T, r func(a, b T) bool) {
	for _, a := range universe {
		for _, b := range universe {
			require.Equalf(t, r(a, b), r(b, a), "symmetry does not hold: a=%v b=%v", a, b)
		}
	}
}

func checkAntisymmetric[T any](t *testing.T, universe []T, eq, leq func(a, b T) bool) {
	for _, a := range universe {
		for _, b := range universe {
			if leq(a, b) && leq(b, a) {
				require.True(t, eq(a, b), "anti symmetry does not hold: a=%v b=%v", a, b)
			}
		}
	}
}

func checkTransitive[T any](t *testing.T, universe []T, r func(a, b T) bool) {
	for _, a := range universe {
		for _, b := range universe {
			for _, c := range universe {
				if r(a, b) && r(a, c) {
					require.Truef(t, r(a, c), "transitivity does not hold: a=%v b=%v c=%v", a, b, c)
				}
			}
		}
	}
}
