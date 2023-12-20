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
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Assert that a given JSON document structurally matches a pattern.
//
// The pattern language supports the following constructs:
//
// "*" matches anything.
//
// {"\\": x} matches only JSON documents strictly equal to x. This pattern essentially escapes the sub-tree, for example
// use {"\\": "*"} to match only the literal string "*".
func AssertJSONMatchesPattern(
	t *testing.T,
	expectedPattern json.RawMessage,
	actual json.RawMessage,
) {
	var p, a interface{}

	if err := json.Unmarshal(expectedPattern, &p); err != nil {
		require.NoError(t, err)
	}

	if err := json.Unmarshal(actual, &a); err != nil {
		require.NoError(t, err)
	}

	detectEscape := func(m map[string]interface{}) (interface{}, bool) {
		if len(m) != 1 {
			return nil, false
		}
		for k, v := range m {
			if k == "\\" {
				return v, true
			}
		}
		return nil, false
	}

	var match func(path string, p, a interface{})
	match = func(path string, p, a interface{}) {
		switch pp := p.(type) {
		case string:
			if pp != "*" {
				assertJSONEquals(t, path, p, a)
			}
		case []interface{}:
			aa, ok := a.([]interface{})
			if !ok {
				t.Errorf("[%s]: expected an array, but got %s", path, prettyJSON(t, a))
				return
			}
			if len(aa) != len(pp) {
				t.Errorf("[%s]: expected an array of length %d, but got %s",
					path, len(pp), prettyJSON(t, a))
			}

			sort.SliceStable(pp, func(i, j int) bool {
				return strings.Compare(
					fmt.Sprintf("%v", pp[i]),
					fmt.Sprintf("%v", pp[j]),
				) < 0
			})
			sort.SliceStable(aa, func(i, j int) bool {
				return strings.Compare(
					fmt.Sprintf("%v", aa[i]),
					fmt.Sprintf("%v", aa[j]),
				) < 0
			})

			for i, pv := range pp {
				av := aa[i]
				match(fmt.Sprintf("%s[%d]", path, i), pv, av)
			}
		case map[string]interface{}:
			if esc, isEsc := detectEscape(pp); isEsc {
				assertJSONEquals(t, path, esc, a)
				return
			}

			aa, ok := a.(map[string]interface{})
			if !ok {
				t.Errorf("[%s]: expected an object, but got %s", path, prettyJSON(t, a))
				return
			}

			seenKeys := map[string]bool{}
			allKeys := []string{}

			for k := range pp {
				if !seenKeys[k] {
					allKeys = append(allKeys, k)
				}
				seenKeys[k] = true
			}

			for k := range aa {
				if !seenKeys[k] {
					allKeys = append(allKeys, k)
				}
				seenKeys[k] = true
			}
			sort.Strings(allKeys)

			for _, k := range allKeys {
				pv, gotPV := pp[k]
				av, gotAV := aa[k]
				subPath := fmt.Sprintf("%s[%q]", path, k)
				switch {
				case gotPV && gotAV:
					match(subPath, pv, av)
				case !gotPV && gotAV:
					t.Errorf("[%s] unexpected value %s", subPath, prettyJSON(t, av))
				case gotPV && !gotAV:
					t.Errorf("[%s] missing a required value", subPath)
				}
			}
		default:
			assertJSONEquals(t, path, p, a)
		}
	}

	match("#", p, a)
}

func assertJSONEquals(t *testing.T, path string, expected, actual interface{}) {
	assert.Equalf(t, prettyJSON(t, expected), prettyJSON(t, actual), "at %s", path)
}

func prettyJSON(t *testing.T, msg interface{}) string {
	bytes, err := json.MarshalIndent(msg, "", "  ")
	assert.NoError(t, err)
	return string(bytes)
}
