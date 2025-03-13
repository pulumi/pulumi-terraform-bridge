// Copyright 2016-2025, Pulumi Corporation.
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

package tfbridge

import (
	"encoding/json"
	"testing"

	"github.com/hashicorp/go-cty/cty"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/require"

	"github.com/hexops/autogold/v2"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

func Test_rawStateInflections_turnaround(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name        string
		schemaMap   shim.SchemaMap
		schemaInfos map[string]*SchemaInfo
		pv          resource.PropertyValue
		cv          cty.Value
	}

	testCases := []testCase{
		{
			name: "null-string",
			pv:   resource.NewNullProperty(),
			cv:   cty.NullVal(cty.String),
		},
		{
			name: "null-number",
			pv:   resource.NewNumberProperty(42.5),
			cv:   cty.NumberFloatVal(42.5),
		},
		{
			name: "empty-string",
			pv:   resource.NewStringProperty(""),
			cv:   cty.StringVal(""),
		},
		{
			name: "simple-string",
			pv:   resource.NewStringProperty("simple"),
			cv:   cty.StringVal("simple"),
		},
		{
			name: "simple-bool",
			pv:   resource.NewBoolProperty(true),
			cv:   cty.BoolVal(true),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ih := inflectHelper{
				schemaMap:   tc.schemaMap,
				schemaInfos: tc.schemaInfos,
			}

			t.Logf("pv: %v", tc.pv.String())
			t.Logf("cv: %v", tc.cv.GoString())

			infl, err := ih.inflections(tc.pv, tc.cv)
			require.NoError(t, err)

			t.Logf("inflections: %#v", infl)

			recoveredCtyValue, err := rawStateRecover(tc.pv, infl)
			require.NoError(t, err)

			t.Logf("cv2:%v", recoveredCtyValue.GoString())

			require.True(t, recoveredCtyValue.RawEquals(tc.cv))
		})
	}
}

func Test_rawstate_inflections_serialization(t *testing.T) {
	type testCase struct {
		name   string
		infl   rawStateInflections
		expect autogold.Value
	}

	testCases := []testCase{
		{
			name: "typedNull",
			infl: rawStateInflections{TypedNull: &typedNull{T: cty.Object(map[string]cty.Type{
				"x": cty.String,
				"y": cty.Number,
			})}},
			expect: autogold.Expect(`{
 "null": {
  "t": [
   "object",
   {
    "x": "string",
    "y": "number"
   }
  ]
 }
}`),
		},
		{
			name: "pluralize-null",
			infl: rawStateInflections{Pluralize: &pluralize{ElementType: &cty.String}},
			expect: autogold.Expect(`{
 "plu": {
  "i": {},
  "t": "string"
 }
}`),
		},
		{
			name: "pluralize-inner",
			infl: rawStateInflections{Pluralize: &pluralize{
				Inner: rawStateInflections{TypedNull: &typedNull{T: cty.String}},
			}},
			expect: autogold.Expect(`{
 "plu": {
  "i": {
   "null": {
    "t": "string"
   }
  }
 }
}`),
		},
		{
			name: "map-empty",
			infl: rawStateInflections{Map: &mapInflections{
				T: &cty.String,
			}},
			expect: autogold.Expect(`{
 "map": {
  "t": "string"
 }
}`),
		},
		{
			name: "map-regular",
			infl: rawStateInflections{
				Map: &mapInflections{
					ElementInflections: map[resource.PropertyKey]rawStateInflections{
						"x": {TypedNull: &typedNull{T: cty.Bool}},
					},
				},
			},
			expect: autogold.Expect(`{
 "map": {
  "m": {
   "x": {
    "null": {
     "t": "bool"
    }
   }
  }
 }
}`),
		},
		{
			name: "obj",
			infl: rawStateInflections{
				Obj: &objInflections{
					Ignored: map[resource.PropertyKey]struct{}{
						"__meta": {},
					},
					Renamed: map[resource.PropertyKey]string{
						"fooBar": "foo_bar",
					},
					ElementInflections: map[resource.PropertyKey]rawStateInflections{
						"fooBar": {
							TypedNull: &typedNull{
								T: cty.Bool,
							},
						},
					},
				},
			},
			expect: autogold.Expect(`{
 "obj": {
  "ignored": {
   "__meta": {}
  },
  "o": {
   "fooBar": {
    "null": {
     "t": "bool"
    }
   }
  },
  "renamed": {
   "fooBar": "foo_bar"
  }
 }
}`),
		},
		{
			name: "array-empty",
			infl: rawStateInflections{
				Array: &arrayInflections{
					T: &cty.Bool,
				},
			},
			expect: autogold.Expect(`{
 "arr": {
  "arr": null,
  "t": "bool"
 }
}`),
		},
		{
			name: "array-regular",
			infl: rawStateInflections{
				Array: &arrayInflections{
					ElementInflections: map[int]rawStateInflections{
						1: {
							TypedNull: &typedNull{T: cty.String},
						},
					},
				},
			},
			expect: autogold.Expect(`{
 "arr": {
  "arr": {
   "1": {
    "null": {
     "t": "string"
    }
   }
  }
 }
}`),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			encoded, err := rawStateEncodeInflections(tc.infl)
			require.NoError(t, err)

			t.Logf("encoded: %#v", encoded)

			back, err := rawStateParseInflections(encoded)
			require.NoError(t, err)

			require.Equalf(t, tc.infl, back, "turnaround")

			encodedJ, err := json.MarshalIndent(encoded, "", " ")
			require.NoError(t, err)

			tc.expect.Equal(t, string(encodedJ))
		})
	}
}
