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
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/stretchr/testify/assert"
)

func TestGetTriviaFromIndex(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name             string
		input            string
		first            int
		last             int
		blockLike        bool
		expectedLeading  string
		expectedTrailing string
	}{
		{
			name:             "simple trivia",
			input:            "/* leading */  4 /* trailing */",
			first:            1,
			last:             1,
			blockLike:        false,
			expectedLeading:  "/* leading */",
			expectedTrailing: "/* trailing */",
		},
		{
			name:             "simple trivia in block",
			input:            "{ /* leading */  4 /* trailing */ }",
			first:            2,
			last:             2,
			blockLike:        true,
			expectedLeading:  "/* leading */",
			expectedTrailing: "/* trailing */",
		},
		{
			name:             "top simple trivia in multi block",
			input:            "{ /* leading */  4 /* trailing */\n/* ignore me */\n 5 /* ignore me */ }",
			first:            2,
			last:             2,
			blockLike:        true,
			expectedLeading:  "/* leading */",
			expectedTrailing: "/* trailing */",
		},
		{
			name:             "bottom simple trivia in multi block",
			input:            "{ /* ignore me */  4 /* ignore me */\n/* leading */\n 5 /* trailing */ }",
			first:            7,
			last:             7,
			blockLike:        true,
			expectedLeading:  "/* leading */",
			expectedTrailing: "/* trailing */",
		},
	}

	for _, tt := range cases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			//t.Parallel()

			tokens, diagnostics := hclsyntax.LexConfig([]byte(tt.input), "", hcl.Pos{Byte: 0, Line: 1, Column: 1})
			assert.Empty(t, diagnostics)

			leading, trailing := getTrivaFromIndex(tokens, tt.first, tt.last, tt.blockLike)
			assert.Equal(t, tt.expectedLeading, string(leading.Bytes()))
			assert.Equal(t, tt.expectedTrailing, string(trailing.Bytes()))
		})
	}
}
