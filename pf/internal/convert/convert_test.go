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

package convert

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

	cases = append(cases, convertTurnaroundTestCases(
		tftypes.List{ElementType: tftypes.List{ElementType: tftypes.Number}},
		arrayPV(arrayPV(resource.NewNumberProperty)),
		[][]float64{},
		[][]float64{{0}},
		[][]float64{{0}, {}, {1.5}, {42, -10}},
	)...)

	cases = append(cases, convertTurnaroundTestCases(
		tftypes.Map{ElementType: tftypes.String},
		mapPV(resource.NewStringProperty),
		map[string]string{},
		map[string]string{"": ""},
		map[string]string{"test": "test-string"},
		map[string]string{"a": "a", "empty": "", "b": "b"},
	)...)

	cases = append(cases, convertTurnaroundTestCases(
		tftypes.Map{ElementType: tftypes.Map{ElementType: tftypes.String}},
		mapPV(mapPV(resource.NewStringProperty)),
		map[string]map[string]string{},
		map[string]map[string]string{"": {"": ""}},
		map[string]map[string]string{"x": {"test": "test-string"}},
		map[string]map[string]string{"x": {"a": "a"}, "y": {"empty": "", "b": "b"}},
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

		encoder, decoder, err := byType(testcase.ty)
		require.NoError(t, err)

		t.Run(testcase.name+"/tf2pu", func(t *testing.T) {
			t.Parallel()

			actual, err := decoder.ToPropertyValue(testcase.val)
			require.NoError(t, err)

			assert.Equal(t, testcase.prop, actual)
		})

		t.Run(testcase.name+"/pu2tf", func(t *testing.T) {
			t.Parallel()

			actual, err := encoder.FromPropertyValue(testcase.prop)
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

func mapPV[T any](topv func(T) resource.PropertyValue) func(data map[string]T) resource.PropertyValue {
	return func(data map[string]T) resource.PropertyValue {
		var entries resource.PropertyMap = make(resource.PropertyMap)
		for k, v := range data {
			entries[resource.PropertyKey(k)] = topv(v)
		}
		return resource.NewObjectProperty(entries)
	}
}

// Enhance tftypes.NewValue to recur into lists and maps. That is, be
// able to pass []string for example instead of []tftypes.Value.
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
	case tftypes.Map:
		elems := map[string]tftypes.Value{}
		r := reflect.ValueOf(val)
		iter := r.MapRange()
		for iter.Next() {
			key := iter.Key().Interface().(string)
			elems[key] = tftypesNewValue(tt.ElementType, iter.Value().Interface())
		}
		return tftypes.NewValue(t, elems)
	default:
		return tftypes.NewValue(t, val)
	}
}

func byType(typ tftypes.Type) (Encoder, Decoder, error) {
	switch {
	case typ.Equal(tftypes.Bool):
		return newBoolEncoder(), newBoolDecoder(), nil
	case typ.Equal(tftypes.Number):
		return newNumberEncoder(), newNumberDecoder(), nil
	case typ.Equal(tftypes.String):
		return newStringEncoder(), newStringDecoder(), nil
	case typ.Is(tftypes.List{}):
		lT := typ.(tftypes.List)
		elementEncoder, elementDecoder, err := byType(lT.ElementType)
		if err != nil {
			return nil, nil, err
		}
		enc, err := newListEncoder(lT.ElementType, elementEncoder)
		if err != nil {
			return nil, nil, err
		}
		dec, err := newListDecoder(elementDecoder)
		if err != nil {
			return nil, nil, err
		}
		return enc, dec, err
	case typ.Is(tftypes.Map{}):
		eT := typ.(tftypes.Map)
		elementEncoder, elementDecoder, err := byType(eT.ElementType)
		if err != nil {
			return nil, nil, err
		}
		enc, err := newMapEncoder(eT.ElementType, elementEncoder)
		if err != nil {
			return nil, nil, err
		}
		dec, err := newMapDecoder(elementDecoder)
		if err != nil {
			return nil, nil, err
		}
		return enc, dec, err
	default:

		return nil, nil, fmt.Errorf("Yet to support type: %v", typ.String())
	}
}
