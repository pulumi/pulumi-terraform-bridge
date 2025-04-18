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

package valueshim_test

import (
	"math/big"
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/hexops/autogold/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/valueshim"
)

func Test_TValue_IsNull(t *testing.T) {
	t.Parallel()
	assert.True(t, valueshim.FromTValue(tftypes.NewValue(tftypes.String, nil)).IsNull())
	assert.False(t, valueshim.FromTValue(tftypes.NewValue(tftypes.String, "OK")).IsNull())
}

func Test_TValue_GoString(t *testing.T) {
	t.Parallel()
	v := valueshim.FromTValue(tftypes.NewValue(tftypes.String, "OK"))
	assert.Equal(t, `tftypes.String<"OK">`, v.GoString())
}

func Test_TValue_Type(t *testing.T) {
	t.Parallel()
	v := valueshim.FromTValue(tftypes.NewValue(tftypes.String, "OK"))
	assert.True(t, v.Type().IsStringType())
}

func Test_TValue_AsValueSlice(t *testing.T) {
	t.Parallel()
	for _, ls := range []tftypes.Type{
		tftypes.List{ElementType: tftypes.String},
		tftypes.Set{ElementType: tftypes.String},
	} {
		abc := []tftypes.Value{
			tftypes.NewValue(tftypes.String, "a"),
			tftypes.NewValue(tftypes.String, "b"),
			tftypes.NewValue(tftypes.String, "c"),
		}
		actual := valueshim.FromTValue(tftypes.NewValue(ls, abc)).AsValueSlice()
		assert.Equal(t, len(abc), len(actual))
		for i := 0; i < len(abc); i++ {
			assert.Equal(t, abc[i].String(), actual[i].GoString())
		}
	}
}

func Test_TValue_AsValueMap(t *testing.T) {
	t.Parallel()
	for _, ls := range []tftypes.Type{
		tftypes.Object{AttributeTypes: map[string]tftypes.Type{
			"x": tftypes.String,
			"y": tftypes.String,
		}},
		tftypes.Map{ElementType: tftypes.String},
	} {
		example := map[string]tftypes.Value{
			"x": tftypes.NewValue(tftypes.String, "a"),
			"y": tftypes.NewValue(tftypes.String, "b"),
		}
		actual := valueshim.FromTValue(tftypes.NewValue(ls, example)).AsValueMap()
		assert.Equal(t, len(example), len(actual))
		for k, v := range example {
			assert.Equal(t, v.String(), actual[k].GoString())
		}
	}
}

func Test_TValue_Marshal(t *testing.T) {
	t.Parallel()

	ok := tftypes.NewValue(tftypes.String, "OK")
	ok2 := tftypes.NewValue(tftypes.String, "OK2")
	n42 := tftypes.NewValue(tftypes.Number, 42)
	xyType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"x": tftypes.String,
		"y": tftypes.Number,
	}}

	bigI, _, err := big.ParseFloat("12345678901234567890", 0, 512, big.ToNearestEven)
	require.NoError(t, err)

	bigF, _, err := big.ParseFloat("1234567890.123456789", 0, 512, big.ToNearestEven)
	require.NoError(t, err)

	tupType := tftypes.Tuple{ElementTypes: []tftypes.Type{tftypes.String, tftypes.Number}}

	type testCase struct {
		v      tftypes.Value
		expect autogold.Value
	}

	testCases := []testCase{
		{
			v:      tftypes.NewValue(tftypes.String, nil),
			expect: autogold.Expect("null"),
		},
		{
			v:      tftypes.NewValue(tftypes.String, "OK"),
			expect: autogold.Expect(`"OK"`),
		},
		{
			v:      tftypes.NewValue(tftypes.Number, nil),
			expect: autogold.Expect("null"),
		},
		{
			v:      tftypes.NewValue(tftypes.Number, 42),
			expect: autogold.Expect("42"),
		},
		{
			v:      tftypes.NewValue(tftypes.Number, bigI),
			expect: autogold.Expect("12345678901234567890"),
		},
		{
			v:      tftypes.NewValue(tftypes.Number, bigF),
			expect: autogold.Expect("1234567890.123456789"),
		},
		{
			v:      tftypes.NewValue(tftypes.Bool, nil),
			expect: autogold.Expect("null"),
		},
		{
			v:      tftypes.NewValue(tftypes.Bool, true),
			expect: autogold.Expect("true"),
		},
		{
			v:      tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, nil),
			expect: autogold.Expect("null"),
		},
		{
			v:      tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, []tftypes.Value{}),
			expect: autogold.Expect("[]"),
		},
		{
			v:      tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, []tftypes.Value{ok}),
			expect: autogold.Expect(`["OK"]`),
		},
		{
			v:      tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, []tftypes.Value{ok, ok2}),
			expect: autogold.Expect(`["OK","OK2"]`),
		},
		{
			v:      tftypes.NewValue(tftypes.Set{ElementType: tftypes.String}, nil),
			expect: autogold.Expect("null"),
		},
		{
			v:      tftypes.NewValue(tftypes.Set{ElementType: tftypes.String}, []tftypes.Value{}),
			expect: autogold.Expect("[]"),
		},
		{
			v:      tftypes.NewValue(tftypes.Set{ElementType: tftypes.String}, []tftypes.Value{ok}),
			expect: autogold.Expect(`["OK"]`),
		},
		{
			v:      tftypes.NewValue(tftypes.Set{ElementType: tftypes.String}, []tftypes.Value{ok, ok2}),
			expect: autogold.Expect(`["OK","OK2"]`),
		},
		{
			v:      tftypes.NewValue(tftypes.Map{ElementType: tftypes.String}, nil),
			expect: autogold.Expect("null"),
		},
		{
			v:      tftypes.NewValue(tftypes.Map{ElementType: tftypes.String}, map[string]tftypes.Value{}),
			expect: autogold.Expect("{}"),
		},
		{
			v: tftypes.NewValue(tftypes.Map{ElementType: tftypes.String},
				map[string]tftypes.Value{"x": ok}),
			expect: autogold.Expect(`{"x":"OK"}`),
		},
		{
			v: tftypes.NewValue(tftypes.Map{ElementType: tftypes.String},
				map[string]tftypes.Value{"x": ok, "y": ok2}),
			expect: autogold.Expect(`{"x":"OK","y":"OK2"}`),
		},
		{
			v:      tftypes.NewValue(xyType, nil),
			expect: autogold.Expect("null"),
		},
		{
			v:      tftypes.NewValue(xyType, map[string]tftypes.Value{"x": ok, "y": n42}),
			expect: autogold.Expect(`{"x":"OK","y":42}`),
		},
		{
			v:      tftypes.NewValue(tupType, nil),
			expect: autogold.Expect("null"),
		},
		{
			v:      tftypes.NewValue(tupType, []tftypes.Value{ok, n42}),
			expect: autogold.Expect(`["OK",42]`),
		},
	}

	for _, tc := range testCases {
		raw, err := valueshim.FromTValue(tc.v).Marshal()
		require.NoError(t, err)
		tc.expect.Equal(t, string(raw))
	}
}

func Test_TValue_Remove(t *testing.T) {
	t.Parallel()

	type testCase struct {
		v   tftypes.Value
		exp autogold.Value
	}

	testCases := []testCase{
		{
			v:   tftypes.NewValue(tftypes.Object{}, map[string]tftypes.Value{}),
			exp: autogold.Expect("tftypes.Object[]<>"),
		},
		{
			v: tftypes.NewValue(tftypes.Object{
				AttributeTypes: map[string]tftypes.Type{"x": tftypes.String},
			}, map[string]tftypes.Value{"x": tftypes.NewValue(tftypes.String, "OK")}),
			exp: autogold.Expect("tftypes.Object[]<>"),
		},
		{
			v: tftypes.NewValue(tftypes.Object{
				AttributeTypes: map[string]tftypes.Type{
					"x": tftypes.String,
					"y": tftypes.Number,
				},
			}, map[string]tftypes.Value{
				"x": tftypes.NewValue(tftypes.String, "OK"),
				"y": tftypes.NewValue(tftypes.Number, 42),
			}),
			exp: autogold.Expect(`tftypes.Object["y":tftypes.Number]<"y":tftypes.Number<"42">>`),
		},
	}

	for _, tc := range testCases {
		tc.exp.Equal(t, valueshim.FromTValue(tc.v).Remove("x").GoString())
	}
}

func Test_TType(t *testing.T) {
	t.Parallel()

	assert.True(t, valueshim.FromTType(tftypes.Bool).IsBooleanType())
	assert.True(t, valueshim.FromTType(tftypes.String).IsStringType())
	assert.True(t, valueshim.FromTType(tftypes.Number).IsNumberType())
	assert.True(t, valueshim.FromTType(tftypes.List{ElementType: tftypes.Number}).IsListType())
	assert.True(t, valueshim.FromTType(tftypes.Set{ElementType: tftypes.Number}).IsSetType())
	assert.True(t, valueshim.FromTType(tftypes.Map{ElementType: tftypes.Number}).IsMapType())
	assert.True(t, valueshim.FromTType(tftypes.Object{}).IsObjectType())
	assert.Equal(t, "tftypes.Object[]", valueshim.FromTType(tftypes.Object{}).GoString())
}

func Test_TValue_ToX(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "OK", valueshim.FromTValue(tftypes.NewValue(tftypes.String, "OK")).StringValue())
	assert.Equal(t, 42.41, valueshim.FromTValue(tftypes.NewValue(tftypes.Number, 42.41)).NumberValue())
	assert.Equal(t, true, valueshim.FromTValue(tftypes.NewValue(tftypes.Bool, true)).BoolValue())
}
