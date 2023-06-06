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

package sdkv2

import (
	"fmt"
	"testing"

	"github.com/hashicorp/go-cty/cty"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func TestProposedNew(t *testing.T) {
	cases := []struct {
		name   string
		res    *schema.Resource
		config cty.Value
		prior  cty.Value
		expect cty.Value
	}{
		{
			name: "configured set overrides null set",
			res: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"id": {
						Type:     schema.TypeString,
						Computed: true,
					},
					"set_property_value": {
						Type:     schema.TypeSet,
						Elem:     &schema.Schema{Type: schema.TypeString},
						Optional: true,
					},
				},
			},
			prior: cty.ObjectVal(map[string]cty.Value{
				"id":                 cty.StringVal("r1"),
				"set_property_value": cty.NullVal(cty.Set(cty.String)),
			}),
			config: cty.ObjectVal(map[string]cty.Value{
				"id":                 cty.NullVal(cty.String),
				"set_property_value": cty.SetVal([]cty.Value{cty.StringVal("foo")}),
			}),
			expect: cty.ObjectVal(map[string]cty.Value{
				"id":                 cty.StringVal("r1"),
				"set_property_value": cty.SetVal([]cty.Value{cty.StringVal("foo")}),
			}),
		},
	}

	for _, cc := range cases {
		c := cc

		t.Run(c.name, func(t *testing.T) {
			actual, err := proposedNew(c.res, c.prior, c.config)
			require.NoError(t, err)
			assert.Equal(t, c.expect.GoString(), actual.GoString())
		})
	}
}

func TestCtyTurnaround(t *testing.T) {
	type testCase struct {
		value cty.Value
	}

	testCases := []testCase{
		{cty.BoolVal(false)},
		{cty.BoolVal(true)},
		{cty.NumberIntVal(0)},
		{cty.NumberIntVal(42)},
		{cty.NumberFloatVal(3.17)},
		{cty.StringVal("")},
		{cty.StringVal("OK")},
		{cty.EmptyTupleVal},
		{cty.EmptyObjectVal},
		{cty.ListValEmpty(cty.Number)},
		{cty.MapValEmpty(cty.Number)},
		{cty.SetValEmpty(cty.Number)},
		{cty.TupleVal([]cty.Value{cty.True, cty.Zero})},
		{cty.ObjectVal(map[string]cty.Value{"x": cty.False, "y": cty.Zero})},
		{cty.ListVal([]cty.Value{cty.True, cty.False})},
		{cty.MapVal(map[string]cty.Value{"0": cty.False, "1": cty.True})},
		{cty.SetVal([]cty.Value{cty.Zero, cty.NumberIntVal(42)})},
	}

	for _, tc := range testCases {
		testCases = append(testCases,
			testCase{cty.ListVal([]cty.Value{tc.value})},
			testCase{cty.MapVal(map[string]cty.Value{"x": tc.value})},
			testCase{cty.SetVal([]cty.Value{tc.value})},
			testCase{cty.TupleVal([]cty.Value{tc.value})},
			testCase{cty.ObjectVal(map[string]cty.Value{"x": tc.value})})
	}

	// Assuming that NilVal should not be nested in real-world use cases so it is added here after the nesting
	// cases. It is slightly problematic to test as it can makes Equals() and other methods panic.
	testCases = append(testCases, testCase{cty.NilVal})

	for i, tc := range testCases {
		tc := tc
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			assert.Truef(t, tc.value.Equals(cty2hcty(hcty2cty(tc.value))).True(), "value turnaround")

			nullValue := cty.NullVal(tc.value.Type())
			assert.Truef(t, nullValue.Equals(cty2hcty(hcty2cty(nullValue))).True(), "null turnaround")

			unkValue := cty.UnknownVal(tc.value.Type())
			assert.Falsef(t, cty2hcty(hcty2cty(unkValue)).IsKnown(), "unknown turnaround")
		})
	}
}
