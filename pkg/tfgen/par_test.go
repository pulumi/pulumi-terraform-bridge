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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParTransformMap(t *testing.T) {
	inputs := map[int]int{
		1:  2,
		2:  4,
		4:  8,
		8:  16,
		16: 32,
	}

	inputsBad := map[int]int{
		1:  2,
		2:  4,
		4:  -8,
		8:  16,
		16: 32,
	}

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
			actual, actualErr := parTransformMap(tc.inputs, increment, tc.workers, tc.batch)
			expect, expectErr := increment(tc.inputs)
			assert.Equal(t, expectErr, actualErr)
			assert.Equal(t, expect, actual)
		})
	}
}
