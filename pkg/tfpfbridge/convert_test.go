// Copyright 2016-2022, Pulumi Corporation.
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
	"fmt"
	"math/big"
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type convertTurnaroundTestCase struct {
	name    string
	ty      tftypes.Type
	val     tftypes.Value
	prop    resource.PropertyValue
	normVal func(tftypes.Value) interface{}
}

func TestConvertTurnaround(t *testing.T) {
	t.Parallel()

	cases := convertTurnaroundTestCases(tftypes.String, resource.NewStringProperty, "", "test-string")
	cases = append(cases, convertTurnaroundTestCases(tftypes.Bool, resource.NewBoolProperty, false, true)...)
	cases = append(cases, convertTurnaroundTestCases(tftypes.Number, resource.NewNumberProperty, float64(0), 42, 3.12)...)

	cases = append(cases, convertTurnaroundTestCases(
		tftypes.List{ElementType: tftypes.String},
		arrayPV(resource.NewStringProperty),
		[]string{},
		[]string{""},
		[]string{"test-string"},
		[]string{"a", "", "b"},
	)...)

	cases = append(cases, []convertTurnaroundTestCase{
		{
			name:    "tftypes.Number/int",
			ty:      tftypes.Number,
			val:     tftypesNewValue(tftypes.Number, int64(42)),
			prop:    resource.NewNumberProperty(42),
			normVal: normNum,
		},
	}...)

	for _, testcase := range cases {
		testcase := testcase

		t.Run(testcase.name+"/tf2pu", func(t *testing.T) {
			t.Parallel()

			actual, err := ConvertTFValueToProperty(testcase.ty)(testcase.val)
			require.NoError(t, err)

			assert.Equal(t, testcase.prop, actual)
		})

		t.Run(testcase.name+"/pu2tf", func(t *testing.T) {
			t.Parallel()

			actual, err := ConvertPropertyToTFValue(testcase.ty)(testcase.prop)
			require.NoError(t, err)

			if testcase.normVal != nil {
				assert.Equal(t, testcase.normVal(testcase.val), testcase.normVal(actual))
			} else {
				assert.Equal(t, testcase.val, actual)
			}
		})
	}
}

func convertTurnaroundUnknownTestCase(ty tftypes.Type, zeroValue resource.PropertyValue) convertTurnaroundTestCase {
	return convertTurnaroundTestCase{
		name: ty.String() + "/unknown",
		ty:   ty,
		val:  tftypesNewValue(ty, tftypes.UnknownValue),
		prop: resource.NewComputedProperty(resource.Computed{Element: zeroValue}),
	}
}

func convertTurnaroundNilTestCase(ty tftypes.Type) convertTurnaroundTestCase {
	return convertTurnaroundTestCase{
		name: ty.String() + "/nil",
		ty:   ty,
		val:  tftypesNewValue(ty, nil),
		prop: resource.NewPropertyValue(nil),
	}
}

func convertTurnaroundTestCases[T any](ty tftypes.Type, topv func(x T) resource.PropertyValue, vals ...T) []convertTurnaroundTestCase {
	var zero T
	zeroValue := topv(zero)
	cases := []convertTurnaroundTestCase{
		convertTurnaroundNilTestCase(ty),
		convertTurnaroundUnknownTestCase(ty, zeroValue),
	}
	for j, v := range vals {
		cases = append(cases, convertTurnaroundTestCase{
			name: ty.String() + "/" + fmt.Sprintf("%d", j),
			ty:   ty,
			val:  tftypesNewValue(ty, v),
			prop: topv(v),
		})
	}
	return cases
}

func normNum(v tftypes.Value) interface{} {
	var f big.Float
	if err := v.As(&f); err != nil {
		panic(err)
	}
	return f.SetPrec(53)
}

func arrayPV[T any](topv func(T) resource.PropertyValue) func(data []T) resource.PropertyValue {
	return func(data []T) resource.PropertyValue {
		xs := []resource.PropertyValue{}
		for _, d := range data {
			xs = append(xs, topv(d))
		}
		return resource.NewArrayProperty(xs)
	}
}

// Enhance tftypes.NewValue to recur into lists. That is, be able to
// pass []string for example instead of []tftypes.Value.
func tftypesNewValue(t tftypes.Type, val interface{}) tftypes.Value {
	if val == nil || val == tftypes.UnknownValue {
		return tftypes.NewValue(t, val)
	}

	switch tt := t.(type) {
	case tftypes.List:
		var elems []tftypes.Value

		r := reflect.ValueOf(val)
		for i := 0; i < r.Len(); i++ {
			elem := tftypesNewValue(tt.ElementType, r.Index(i).Interface())
			elems = append(elems, elem)
		}

		return tftypes.NewValue(t, elems)
	default:
		return tftypes.NewValue(t, val)
	}
}
