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

	"pgregory.net/rapid"
)

func TestEqualityClasses(t *testing.T) {
	eq := func(a int, b int) bool {
		return a%3 == b%3
	}

	rapid.Check(t, func(t *rapid.T) {
		s := rapid.SliceOf(rapid.Int()).Draw(t, "s")
		actual := equalityClasses(eq, s)
		t.Logf("actual %+v", actual)
		for i, c1 := range actual {
			for j, c2 := range actual {
				if i == j {
					continue
				}
				if sameEqualityClasses(eq, c1, c2) {
					t.Fatalf("found same equality classes in the result")
				}
			}
		}
		for _, c := range actual {
			for i, e1 := range c {
				for j, e2 := range c {
					if i == j {
						continue
					}
					if !eq(e1, e2) {
						t.Fatalf("found entries in the same eq class")
					}
				}
			}
		}
	})
}
