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

package tfbridge

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/schema"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/walk"
)

func TestPropertyPathToSchemaPath(t *testing.T) {
	yes := true
	strSchema := (&schema.Schema{Type: shim.TypeString, Optional: true}).Shim()

	xySchema := (&schema.Resource{
		Schema: schema.SchemaMap{
			"x_prop": strSchema,
			"y_prop": strSchema,
		},
	}).Shim()

	schemaMap := &schema.SchemaMap{
		"string_prop": strSchema,
		"list_str": (&schema.Schema{
			Type: shim.TypeList,
			Elem: strSchema,
		}).Shim(),
		"list_str_named": (&schema.Schema{
			Type: shim.TypeList,
			Elem: strSchema,
		}).Shim(),
		"list_unknowns": (&schema.Schema{
			Type: shim.TypeList,
		}).Shim(),
		"flat_list": (&schema.Schema{
			Type:     shim.TypeList,
			Elem:     xySchema,
			MaxItems: 1,
		}).Shim(),
		"flat_list_via_schema_info": (&schema.Schema{
			Type: shim.TypeList,
			Elem: xySchema,
		}).Shim(),
		"obj_set": (&schema.Schema{
			Type: shim.TypeSet,
			Elem: xySchema,
		}).Shim(),
		"single_obj": (&schema.Schema{
			Type: shim.TypeMap,
			Elem: xySchema,
		}).Shim(),
	}

	schemaInfos := map[string]*SchemaInfo{
		"flat_list_via_schema_info": {
			MaxItemsOne: &yes,
		},
		"list_str_named": {
			Name: "listStr",
		},
	}

	type testCase struct {
		name     string
		pp       resource.PropertyPath
		expected SchemaPath
	}

	cases := []testCase{
		{
			name:     "simple string property",
			pp:       []any{"stringProp"},
			expected: walk.NewSchemaPath().GetAttr("string_prop"),
		},
		{
			name:     "simple not found property",
			pp:       []any{"notFoundProp"},
			expected: nil,
		},
		{
			name:     "simple not found drill-down property",
			pp:       []any{"stringProp", "notFoundProp"},
			expected: nil,
		},
		{
			name:     "list",
			pp:       []any{"listStrs"},
			expected: walk.NewSchemaPath().GetAttr("list_str"),
		},
		{
			name:     "named list",
			pp:       []any{"listStr"},
			expected: walk.NewSchemaPath().GetAttr("list_str_named"),
		},

		{
			name:     "list element",
			pp:       []any{"listStrs", 3},
			expected: walk.NewSchemaPath().GetAttr("list_str").Element(),
		},
		{
			name:     "list of unknowns",
			pp:       []any{"listUnknowns"},
			expected: walk.NewSchemaPath().GetAttr("list_unknowns"),
		},
		{
			name:     "list of unknowns element",
			pp:       []any{"listUnknowns", 3},
			expected: nil,
		},
		{
			name:     "single-nested block",
			pp:       []any{"singleObj", "xProp"},
			expected: walk.NewSchemaPath().GetAttr("single_obj").GetAttr("x_prop"),
		},
		{
			name:     "set-nested block 1",
			pp:       []any{"objSets"},
			expected: walk.NewSchemaPath().GetAttr("obj_set"),
		},
		{
			name:     "set-nested block 2",
			pp:       []any{"objSets", 0},
			expected: walk.NewSchemaPath().GetAttr("obj_set").Element(),
		},
		{
			name:     "set-nested block 3",
			pp:       []any{"objSets", 0, "xProp"},
			expected: walk.NewSchemaPath().GetAttr("obj_set").Element().GetAttr("x_prop"),
		},
		{
			name:     "max-items-1 list 1",
			pp:       []any{"flatList"},
			expected: walk.NewSchemaPath().GetAttr("flat_list"),
		},
		{
			name:     "max-items-1 list 3",
			pp:       []any{"flatList", "xProp"},
			expected: walk.NewSchemaPath().GetAttr("flat_list").Element().GetAttr("x_prop"),
		},
		{
			name:     "max-items-1 list 3 via schemainfo",
			pp:       []any{"flatListViaSchemaInfo", "xProp"},
			expected: walk.NewSchemaPath().GetAttr("flat_list_via_schema_info").Element().GetAttr("x_prop"),
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			pp := PropertyPathToSchemaPath(tc.pp, schemaMap, schemaInfos)
			assert.Equal(t, tc.expected, pp)

			t.Run("inverse", func(t *testing.T) {
				// If SchemaPath -> PP doesn't work, it doesn't make sense
				// to test round tripping.
				if tc.expected == nil {
					t.SkipNow()
				}

				// Element selection is not round-trippable, so we convert
				// some element index, like `3` into the generic element
				// index `"*"`.
				for i, p := range tc.pp {
					if _, ok := p.(int); ok {
						tc.pp[i] = "*"
					}
				}
				actual := SchemaPathToPropertyPath(pp, schemaMap, schemaInfos)
				assert.Equal(t, tc.pp, actual)
			})
		})
	}
}

func TestLookupSchemaInfoMapPath(t *testing.T) {
	yes := true

	schemaInfos := map[string]*SchemaInfo{
		"list_prop": {
			Elem: &SchemaInfo{
				Secret: &yes,
			},
		},
		"nested_obj_prop": {
			Fields: map[string]*SchemaInfo{
				"sub_prop": {
					Fields: map[string]*SchemaInfo{
						"p": {
							Secret: &yes,
						},
					},
				},
			},
		},
		"max_items_one_prop": {
			MaxItemsOne: &yes,
			Elem: &SchemaInfo{
				Fields: map[string]*SchemaInfo{
					"sub_prop": {
						Secret: &yes,
					},
				},
			},
		},
	}

	type testCase struct {
		name     string
		path     SchemaPath
		expected *SchemaInfo
	}

	testCases := []testCase{
		{
			"not-found",
			walk.NewSchemaPath().Element(),
			nil,
		},
		{
			"list",
			walk.NewSchemaPath().GetAttr("list_prop").Element(),
			schemaInfos["list_prop"].Elem,
		},
		{
			"nested object",
			walk.NewSchemaPath().GetAttr("nested_obj_prop").GetAttr("sub_prop").GetAttr("p"),
			schemaInfos["nested_obj_prop"].Fields["sub_prop"].Fields["p"],
		},
		{
			"oblivious to maxitemsone",
			walk.NewSchemaPath().GetAttr("max_items_one_prop").Element().GetAttr("sub_prop"),
			schemaInfos["max_items_one_prop"].Elem.Fields["sub_prop"],
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			actual := LookupSchemaInfoMapPath(tc.path, schemaInfos)
			assert.Equal(t, tc.expected, actual)
		})
	}
}
