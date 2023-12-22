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

package testing

import (
	"testing"
)

func TestJsonMatch(t *testing.T) {
	AssertJSONMatchesPattern(t, []byte(`1`), []byte(`1`))
	AssertJSONMatchesPattern(t, []byte(`"*"`), []byte(`1`))
	AssertJSONMatchesPattern(t, []byte(`"*"`), []byte(`2`))
	AssertJSONMatchesPattern(t, []byte(`{"\\": "*"}`), []byte(`"*"`))
	AssertJSONMatchesPattern(t, []byte(`[1, "*", 3]`), []byte(`[1, 2, 3]`))
	AssertJSONMatchesPattern(t, []byte(`{"foo": "*", "bar": 3}`), []byte(`{"foo": 1, "bar": 3}`))
	AssertJSONMatchesPatternWithOpts(t, []byte(`[1, 2, 3]`), []byte(`[1, 3, 2]`),
		JsonMatchOptions{
			UnorderedArrayPaths: map[string]bool{"#": true},
		})
	AssertJSONMatchesPatternWithOpts(t,
		[]byte(`[{"key1":"val"}, {"key2":"val"}]`),
		[]byte(`[{"key2":"val"}, {"key1":"val"}]`),
		JsonMatchOptions{
			UnorderedArrayPaths: map[string]bool{"#": true},
		},
	)
	AssertJSONMatchesPatternWithOpts(t,
		[]byte(`[{"key":"val1"}, {"key":"val2"}]`),
		[]byte(`[{"key":"val2"}, {"key":"val1"}]`),
		JsonMatchOptions{
			UnorderedArrayPaths: map[string]bool{"#": true},
		},
	)
	AssertJSONMatchesPatternWithOpts(t,
		[]byte(`{"arr":[{"key":"val1"}, {"key":"val2"}]}`),
		[]byte(`{"arr":[{"key":"val2"}, {"key":"val1"}]}`),
		JsonMatchOptions{
			UnorderedArrayPaths: map[string]bool{`#["arr"]`: true},
		},
	)
}
