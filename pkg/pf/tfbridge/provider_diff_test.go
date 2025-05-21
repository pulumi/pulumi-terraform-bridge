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

package tfbridge

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/require"

	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/schema"
)

func TestTopLevelPropertyKeySet(t *testing.T) {
	t.Parallel()
	str := (&schema.Schema{
		Type: shim.TypeString,
	}).Shim()
	ints := (&schema.Schema{
		Type: shim.TypeInt,
	}).Shim()
	sch := schema.SchemaMap{
		"str": str,
		"obj": (&schema.Schema{
			Type: shim.TypeMap,
			Elem: (&schema.Resource{
				Schema: schema.SchemaMap{
					"string_prop": str,
					"int_prop":    ints,
				},
			}).Shim(),
		}).Shim(),
	}
	type testCase struct {
		name   string
		paths  []*tftypes.AttributePath
		expect []resource.PropertyKey
	}
	testCases := []testCase{
		{
			"empty",
			nil,
			nil,
		},
		{
			"str",
			[]*tftypes.AttributePath{tftypes.NewAttributePath().WithAttributeName("str")},
			[]resource.PropertyKey{"str"},
		},
		{
			"clipped-dedup-obj-keys",
			[]*tftypes.AttributePath{
				tftypes.NewAttributePath().WithAttributeName("obj").WithAttributeName("string_prop"),
				tftypes.NewAttributePath().WithAttributeName("obj").WithAttributeName("int_prop"),
			},
			[]resource.PropertyKey{"obj"},
		},
		{
			"sorted-str-and-obj",
			[]*tftypes.AttributePath{
				tftypes.NewAttributePath().WithAttributeName("str"),
				tftypes.NewAttributePath().WithAttributeName("obj").WithAttributeName("string_prop"),
			},
			[]resource.PropertyKey{"obj", "str"},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := topLevelPropertyKeySet(sch, nil, tc.paths)
			require.Equal(t, tc.expect, actual)
		})
	}
}

func TestTrimElementKeyValueFromTFPath(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name   string
		input  *tftypes.AttributePath
		expect *tftypes.AttributePath
	}{
		{
			name:   "single attribute name",
			input:  tftypes.NewAttributePath().WithAttributeName("foo"),
			expect: tftypes.NewAttributePath().WithAttributeName("foo"),
		},
		{
			name:   "attribute name and string key",
			input:  tftypes.NewAttributePath().WithAttributeName("foo").WithElementKeyString("bar"),
			expect: tftypes.NewAttributePath().WithAttributeName("foo").WithElementKeyString("bar"),
		},
		{
			name:   "attribute name and int key",
			input:  tftypes.NewAttributePath().WithAttributeName("foo").WithElementKeyInt(42),
			expect: tftypes.NewAttributePath().WithAttributeName("foo").WithElementKeyInt(42),
		},
		{
			name: "path with ElementKeyValue truncated",
			input: func() *tftypes.AttributePath {
				steps := []tftypes.AttributePathStep{
					tftypes.AttributeName("foo"),
					tftypes.ElementKeyValue(tftypes.NewValue(tftypes.String, "bar")),
				}
				return tftypes.NewAttributePathWithSteps(steps)
			}(),
			expect: tftypes.NewAttributePath().WithAttributeName("foo"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := trimElementKeyValueFromTFPath(tc.input)
			require.Equal(t, tc.expect.String(), actual.String())
		})
	}
}

func TestCheckRequiresReplace(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		priorVal       interface{}
		plannedVal     interface{}
		replacePaths   []*tftypes.AttributePath
		expectPaths    []*tftypes.AttributePath
		plannedIsKnown bool
	}

	objectType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{"attr": tftypes.String}}

	makeValue := func(typ tftypes.Type, val interface{}, isKnown bool) tftypes.Value {
		v := tftypes.NewValue(typ, val)
		if !isKnown {
			return tftypes.NewValue(typ, tftypes.UnknownValue)
		}
		return v
	}

	testCases := []testCase{
		{
			name:         "null prior state returns empty",
			priorVal:     nil,
			plannedVal:   map[string]tftypes.Value{"attr": tftypes.NewValue(tftypes.String, "foo")},
			replacePaths: []*tftypes.AttributePath{tftypes.NewAttributePath().WithAttributeName("attr")},
			expectPaths:  []*tftypes.AttributePath{},
		},
		{
			name:         "empty replace keys returns empty",
			priorVal:     map[string]tftypes.Value{"attr": tftypes.NewValue(tftypes.String, "foo")},
			plannedVal:   map[string]tftypes.Value{"attr": tftypes.NewValue(tftypes.String, "bar")},
			replacePaths: []*tftypes.AttributePath{},
			expectPaths:  []*tftypes.AttributePath{},
		},
		{
			name:         "value differs triggers replace",
			priorVal:     map[string]tftypes.Value{"attr": tftypes.NewValue(tftypes.String, "foo")},
			plannedVal:   map[string]tftypes.Value{"attr": tftypes.NewValue(tftypes.String, "bar")},
			replacePaths: []*tftypes.AttributePath{tftypes.NewAttributePath().WithAttributeName("attr")},
			expectPaths:  []*tftypes.AttributePath{tftypes.NewAttributePath().WithAttributeName("attr")},
		},
		{
			name:         "value equal does not trigger replace",
			priorVal:     map[string]tftypes.Value{"attr": tftypes.NewValue(tftypes.String, "foo")},
			plannedVal:   map[string]tftypes.Value{"attr": tftypes.NewValue(tftypes.String, "foo")},
			replacePaths: []*tftypes.AttributePath{tftypes.NewAttributePath().WithAttributeName("attr")},
			expectPaths:  []*tftypes.AttributePath{},
		},
		{
			name:           "planned value unknown triggers replace",
			priorVal:       map[string]tftypes.Value{"attr": tftypes.NewValue(tftypes.String, "foo")},
			plannedVal:     nil,
			replacePaths:   []*tftypes.AttributePath{tftypes.NewAttributePath().WithAttributeName("attr")},
			expectPaths:    []*tftypes.AttributePath{tftypes.NewAttributePath().WithAttributeName("attr")},
			plannedIsKnown: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			priorVal := makeValue(objectType, tc.priorVal, true)
			if tc.priorVal == nil {
				priorVal = tftypes.NewValue(objectType, nil)
			}
			plannedVal := makeValue(objectType, tc.plannedVal, tc.plannedIsKnown || tc.plannedVal != nil)
			priorState := &upgradedResourceState{Value: priorVal}
			paths, err := checkRequiresReplace(priorState, plannedVal, tc.replacePaths)
			require.NoError(t, err)
			require.ElementsMatch(t, tc.expectPaths, paths)
		})
	}

	t.Run("error walking attribute path", func(t *testing.T) {
		priorVal := tftypes.NewValue(objectType, map[string]tftypes.Value{"attr": tftypes.NewValue(tftypes.String, "foo")})
		plannedVal := tftypes.NewValue(objectType, map[string]tftypes.Value{"attr": tftypes.NewValue(tftypes.String, "bar")})
		priorState := &upgradedResourceState{Value: priorVal}
		badPath := tftypes.NewAttributePath().WithAttributeName("doesnotexist")
		_, err := checkRequiresReplace(priorState, plannedVal, []*tftypes.AttributePath{badPath})
		require.Error(t, err)
	})
}
