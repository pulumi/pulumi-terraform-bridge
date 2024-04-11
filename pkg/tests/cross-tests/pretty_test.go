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

package crosstests

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/hexops/autogold/v2"
)

func TestPrettyPrint(t *testing.T) {
	type testCase struct {
		v tftypes.Value
		e autogold.Value
	}

	testCases := []testCase{
		{
			tftypes.NewValue(tftypes.Number, 42),
			autogold.Expect("\ntftypes.NewValue(tftypes.Number, 42)"),
		},
		{
			tftypes.NewValue(tftypes.Object{
				AttributeTypes: map[string]tftypes.Type{
					"f1": tftypes.Bool,
					"f0": tftypes.List{
						ElementType: tftypes.Object{
							AttributeTypes: map[string]tftypes.Type{
								"n": tftypes.Number,
							},
						},
					},
				},
			}, map[string]tftypes.Value{
				"f1": tftypes.NewValue(tftypes.Bool, true),
				"f0": tftypes.NewValue(tftypes.List{
					ElementType: tftypes.Object{
						AttributeTypes: map[string]tftypes.Type{
							"n": tftypes.Number,
						},
					},
				}, []tftypes.Value{
					tftypes.NewValue(tftypes.Object{
						AttributeTypes: map[string]tftypes.Type{
							"n": tftypes.Number,
						},
					}, map[string]tftypes.Value{
						"n": tftypes.NewValue(tftypes.Number, 42),
					}),
				}),
			}),
			autogold.Expect(`
t1 := tftypes.Object{AttributeTypes: map[string]tftypes.Type{
  "n": tftypes.Number,
}}
t0 := tftypes.Object{AttributeTypes: map[string]tftypes.Type{
  "f0": tftypes.List{ElementType: t1},
  "f1": tftypes.Bool,
}}
tftypes.NewValue(t0, map[string]tftypes.Value{
  "f0": tftypes.NewValue(tftypes.List{ElementType: t1}, []tftypes.Value{
.   tftypes.NewValue(t1, map[string]tftypes.Value{
      "n": tftypes.NewValue(tftypes.Number, 42),
    }),
  }),
  "f1": tftypes.NewValue(tftypes.Bool, true),
})`),
		},
	}

	for i, tc := range testCases {
		tc := tc
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			pw := prettyValueWrapper{tc.v}
			tc.e.Equal(t, pw.GoString())
		})
	}
}
