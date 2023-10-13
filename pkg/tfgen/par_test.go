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

package tfgen

import (
	"fmt"
	"math"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParTransformMap(t *testing.T) {

	mkMap := func(n int) map[int]int {
		m := map[int]int{}
		for i := 0; i < n; i++ {
			m[i] = 2 * i
		}
		return m
	}

	inputs := mkMap(1000)

	inputsBad := mkMap(1000)
	inputsBad[4] = -8

	type testCase struct {
		inputs  map[int]int
		workers int
		batch   int
	}

	increment := func(m map[int]int) (map[int]int, error) {
		out := map[int]int{}
		for k, v := range m {
			if v < 0 {
				return nil, fmt.Errorf("neg")
			}
			out[k] = v + 1
		}
		return out, nil
	}

	testCases := []testCase{
		{inputs, -1, 3},
		{inputs, 2, 3},
		{inputs, 4, 3},
		{inputsBad, -1, 3},
		{inputsBad, 2, 3},
		{inputsBad, 4, 3},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(fmt.Sprintf("w%d__b%d", tc.workers, tc.batch), func(t *testing.T) {
			var ops atomic.Uint64

			inc := func(m map[int]int) (map[int]int, error) {
				assert.LessOrEqual(t, len(m), tc.batch)
				ops.Add(1)
				return increment(m)
			}

			actual, actualErr := parTransformMap(tc.inputs, inc, tc.workers, tc.batch)
			expect, expectErr := increment(tc.inputs)
			assert.Equal(t, int(math.Ceil(float64(len(tc.inputs))/float64(tc.batch))),
				int(ops.Load()))
			assert.Equal(t, expectErr, actualErr)
			assert.Equal(t, expect, actual)
		})
	}
}
