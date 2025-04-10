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

	"github.com/hashicorp/go-cty/cty"
	"github.com/hexops/autogold/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/valueshim"
)

func Test_HCtyValue_IsNull(t *testing.T) {
	t.Parallel()
	assert.True(t, valueshim.FromHCtyValue(cty.NullVal(cty.String)).IsNull())
	assert.False(t, valueshim.FromHCtyValue(cty.StringVal("OK")).IsNull())
}

func Test_HCtyValue_GoString(t *testing.T) {
	t.Parallel()
	v := valueshim.FromHCtyValue(cty.StringVal("OK"))
	assert.Equal(t, `cty.StringVal("OK")`, v.GoString())
}

func Test_HCtyValue_Type(t *testing.T) {
	t.Parallel()
	v := valueshim.FromHCtyValue(cty.StringVal("OK"))
	assert.True(t, v.Type().IsStringType())
}

func Test_HCtyValue_AsValueSlice(t *testing.T) {
	t.Parallel()

	assert.Equal(t, 0, len(valueshim.FromHCtyValue(cty.ListValEmpty(cty.String)).AsValueSlice()))
	assert.Equal(t, 0, len(valueshim.FromHCtyValue(cty.SetValEmpty(cty.String)).AsValueSlice()))

	abc := []cty.Value{
		cty.StringVal("a"),
		cty.StringVal("b"),
		cty.StringVal("c"),
	}

	actualList := valueshim.FromHCtyValue(cty.ListVal(abc)).AsValueSlice()
	assert.Equal(t, len(abc), len(actualList))
	for i := 0; i < len(abc); i++ {
		assert.Equal(t, abc[i].GoString(), actualList[i].GoString())
	}

	// cty.SetVal may reorder elements so this test may be overly strict.
	actualSet := valueshim.FromHCtyValue(cty.SetVal(abc)).AsValueSlice()
	assert.Equal(t, len(abc), len(actualSet))
	for i := 0; i < len(abc); i++ {
		assert.Equal(t, abc[i].GoString(), actualSet[i].GoString())
	}
}

func Test_HCtyValue_AsValueMap(t *testing.T) {
	t.Parallel()
	for _, mk := range []func(map[string]cty.Value) cty.Value{
		cty.ObjectVal,
		cty.MapVal,
	} {
		example := map[string]cty.Value{
			"x": cty.StringVal("a"),
			"y": cty.StringVal("b"),
		}
		actual := valueshim.FromHCtyValue(mk(example)).AsValueMap()
		assert.Equal(t, len(example), len(actual))
		for k, v := range example {
			assert.Equal(t, v.GoString(), actual[k].GoString())
		}
	}
}

func Test_HCtyValue_Marshal(t *testing.T) {
	t.Parallel()

	ok := cty.StringVal("OK")
	ok2 := cty.StringVal("OK2")
	n42 := cty.NumberIntVal(42)

	xyType := cty.Object(map[string]cty.Type{
		"x": cty.String,
		"y": cty.Number,
	})

	bigI, _, err := big.ParseFloat("12345678901234567890", 0, 512, big.ToNearestEven)
	require.NoError(t, err)

	bigF, _, err := big.ParseFloat("1234567890.123456789", 0, 512, big.ToNearestEven)
	require.NoError(t, err)

	tupType := cty.Tuple([]cty.Type{cty.String, cty.Number})

	type testCase struct {
		v      cty.Value
		expect autogold.Value
	}

	testCases := []testCase{
		{
			v:      cty.NullVal(cty.String),
			expect: autogold.Expect("null"),
		},
		{
			v:      cty.StringVal("OK"),
			expect: autogold.Expect(`"OK"`),
		},
		{
			v:      cty.NullVal(cty.Number),
			expect: autogold.Expect("null"),
		},
		{
			v:      cty.NumberIntVal(42),
			expect: autogold.Expect("42"),
		},
		{
			v:      cty.NumberVal(bigI),
			expect: autogold.Expect("12345678901234567890"),
		},
		{
			v:      cty.NumberVal(bigF),
			expect: autogold.Expect("1234567890.123456789"),
		},
		{
			v:      cty.NullVal(cty.Bool),
			expect: autogold.Expect("null"),
		},
		{
			v:      cty.BoolVal(true),
			expect: autogold.Expect("true"),
		},
		{
			v:      cty.NullVal(cty.List(cty.String)),
			expect: autogold.Expect("null"),
		},
		{
			v:      cty.ListValEmpty(cty.String),
			expect: autogold.Expect("[]"),
		},
		{
			v:      cty.ListVal([]cty.Value{ok}),
			expect: autogold.Expect(`["OK"]`),
		},
		{
			v:      cty.ListVal([]cty.Value{ok, ok2}),
			expect: autogold.Expect(`["OK","OK2"]`),
		},
		{
			v:      cty.NullVal(cty.Set(cty.String)),
			expect: autogold.Expect("null"),
		},
		{
			v:      cty.SetValEmpty(cty.String),
			expect: autogold.Expect("[]"),
		},
		{
			v:      cty.SetVal([]cty.Value{ok}),
			expect: autogold.Expect(`["OK"]`),
		},
		{
			v:      cty.SetVal([]cty.Value{ok, ok2}),
			expect: autogold.Expect(`["OK","OK2"]`),
		},
		{
			v:      cty.NullVal(cty.Map(cty.String)),
			expect: autogold.Expect("null"),
		},
		{
			v:      cty.MapValEmpty(cty.String),
			expect: autogold.Expect("{}"),
		},
		{
			v:      cty.MapVal(map[string]cty.Value{"x": ok}),
			expect: autogold.Expect(`{"x":"OK"}`),
		},
		{
			v:      cty.MapVal(map[string]cty.Value{"x": ok, "y": ok2}),
			expect: autogold.Expect(`{"x":"OK","y":"OK2"}`),
		},
		{
			v:      cty.NullVal(xyType),
			expect: autogold.Expect("null"),
		},
		{
			v:      cty.ObjectVal(map[string]cty.Value{"x": ok, "y": n42}),
			expect: autogold.Expect(`{"x":"OK","y":42}`),
		},
		{
			v:      cty.NullVal(tupType),
			expect: autogold.Expect("null"),
		},
		{
			v:      cty.TupleVal([]cty.Value{ok, n42}),
			expect: autogold.Expect(`["OK",42]`),
		},
	}

	for _, tc := range testCases {
		raw, err := valueshim.FromHCtyValue(tc.v).Marshal()
		require.NoError(t, err)
		tc.expect.Equal(t, string(raw))
	}
}

func Test_HCtyValue_Remove(t *testing.T) {
	t.Parallel()

	type testCase struct {
		v   cty.Value
		exp autogold.Value
	}

	x := cty.ObjectVal(map[string]cty.Value{"x": cty.NumberIntVal(42)})
	xy := cty.ObjectVal(map[string]cty.Value{"x": cty.NumberIntVal(42), "y": cty.StringVal("OK")})

	testCases := []testCase{
		{
			v:   cty.EmptyObjectVal,
			exp: autogold.Expect("cty.EmptyObjectVal"),
		},
		{
			v:   x,
			exp: autogold.Expect("cty.EmptyObjectVal"),
		},
		{
			v:   xy,
			exp: autogold.Expect(`cty.ObjectVal(map[string]cty.Value{"y":cty.StringVal("OK")})`),
		},
	}

	for _, tc := range testCases {
		tc.exp.Equal(t, valueshim.FromHCtyValue(tc.v).Remove("x").GoString())
	}
}

func Test_HCtyType(t *testing.T) {
	t.Parallel()

	assert.True(t, valueshim.FromHCtyType(cty.Bool).IsBooleanType())
	assert.True(t, valueshim.FromHCtyType(cty.String).IsStringType())
	assert.True(t, valueshim.FromHCtyType(cty.Number).IsNumberType())
	assert.True(t, valueshim.FromHCtyType(cty.List(cty.Number)).IsListType())
	assert.True(t, valueshim.FromHCtyType(cty.Set(cty.Number)).IsSetType())
	assert.True(t, valueshim.FromHCtyType(cty.Map(cty.Number)).IsMapType())
	x := cty.Object(map[string]cty.Type{"x": cty.Number})
	assert.True(t, valueshim.FromHCtyType(x).IsObjectType())
	assert.Equal(t, `cty.Object(map[string]cty.Type{"x":cty.Number})`, valueshim.FromHCtyType(x).GoString())
}

func Test_HCty_ToX(t *testing.T) {
	assert.Equal(t, "OK", valueshim.FromHCtyValue(cty.StringVal("OK")).StringValue())
	assert.Equal(t, 42.41, valueshim.FromHCtyValue(cty.NumberFloatVal(42.41)).NumberValue())
	assert.Equal(t, true, valueshim.FromHCtyValue(cty.BoolVal(true)).BoolValue())
}
