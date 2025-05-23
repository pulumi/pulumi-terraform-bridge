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
	"strings"
	"testing"

	sdkv2schema "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-terraform-bridge/v3/internal/testprovider"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/schema"
	shimv2 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v2"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/walk"
)

func TestPropertyPathToSchemaPath(t *testing.T) {
	t.Parallel()
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
		"list_obj": (&schema.Schema{
			Type: shim.TypeList,
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
		"list_obj": {
			Name: "listObj",
			Elem: &SchemaInfo{
				Fields: map[string]*SchemaInfo{
					"x_prop": {Name: "xOverride"},
				},
			},
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
		{
			name:     "override list nested object property",
			pp:       resource.PropertyPath{"listObj", 0, "xOverride"},
			expected: walk.NewSchemaPath().GetAttr("list_obj").Element().GetAttr("x_prop"),
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
	t.Parallel()
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

func TestTraverseProperties(t *testing.T) {
	t.Parallel()
	testTFProviderV2 := testprovider.ProviderV2()

	prov := &ProviderInfo{
		P:              shimv2.NewProvider(testTFProviderV2),
		IgnoreMappings: []string{"nested_secret_resource"},
		MetadataInfo:   NewProviderMetadata(nil),
	}

	tfToken := func(i PropertyVisitInfo) string {
		switch root := i.Root.(type) {
		case VisitResourceRoot:
			return root.TfToken
		case VisitDataSourceRoot:
			return root.TfToken
		default:
			return ""
		}
	}

	hasEffect := func(i PropertyVisitInfo) (PropertyVisitResult, error) {
		return PropertyVisitResult{
			HasEffect: strings.Contains(i.SchemaPath().GoString(), "bool_property_value") ||
				strings.Contains(i.SchemaPath().GoString(), "opt_bool"),
		}, nil
	}

	seenPaths := map[string][]SchemaPath{}
	err := TraverseProperties(prov, t.Name(), func(i PropertyVisitInfo) (PropertyVisitResult, error) {
		paths := seenPaths[tfToken(i)]
		seenPaths[tfToken(i)] = append(paths, i.SchemaPath())
		return hasEffect(i)
	}, TraverseForEffect(false))
	require.NoError(t, err)

	for k, v := range seenPaths {
		walk.SortSchemaPaths(v)
		seenPaths[k] = v
	}

	assert.Equal(t, map[string][]walk.SchemaPath{
		"": {
			walk.NewSchemaPath().GetAttr("config_value"),
		},
		"example_resource": {
			walk.NewSchemaPath().GetAttr("array_property_value"),
			walk.NewSchemaPath().GetAttr("array_property_value"),
			walk.NewSchemaPath().GetAttr("array_property_value").Element(),
			walk.NewSchemaPath().GetAttr("array_property_value").Element(),
			walk.NewSchemaPath().GetAttr("bool_property_value"),
			walk.NewSchemaPath().GetAttr("bool_property_value"),
			walk.NewSchemaPath().GetAttr("float_property_value"),
			walk.NewSchemaPath().GetAttr("float_property_value"),
			walk.NewSchemaPath().GetAttr("map_property_value"),
			walk.NewSchemaPath().GetAttr("nested_resources"),
			walk.NewSchemaPath().GetAttr("nested_resources"),
			walk.NewSchemaPath().GetAttr("nested_resources").Element().GetAttr("configuration"),
			walk.NewSchemaPath().GetAttr("nested_resources").Element().GetAttr("configuration"),
			walk.NewSchemaPath().GetAttr("nested_resources").Element().GetAttr("kind"),
			walk.NewSchemaPath().GetAttr("nested_resources").Element().GetAttr("opt_bool"),
			walk.NewSchemaPath().GetAttr("nil_property_value"),
			walk.NewSchemaPath().GetAttr("nil_property_value"),
			walk.NewSchemaPath().GetAttr("number_property_value"),
			walk.NewSchemaPath().GetAttr("number_property_value"),
			walk.NewSchemaPath().GetAttr("object_property_value"),
			walk.NewSchemaPath().GetAttr("object_property_value"),
			walk.NewSchemaPath().GetAttr("set_property_value"),
			walk.NewSchemaPath().GetAttr("set_property_value"),
			walk.NewSchemaPath().GetAttr("set_property_value").Element(),
			walk.NewSchemaPath().GetAttr("set_property_value").Element(),
			walk.NewSchemaPath().GetAttr("string_property_value"),
			walk.NewSchemaPath().GetAttr("string_property_value"),
			walk.NewSchemaPath().GetAttr("string_with_bad_interpolation"),
			walk.NewSchemaPath().GetAttr("string_with_bad_interpolation"),
		},
		"second_resource": {
			walk.NewSchemaPath().GetAttr("array_property_value"),
			walk.NewSchemaPath().GetAttr("array_property_value").Element(),
			walk.NewSchemaPath().GetAttr("bool_property_value"),
			walk.NewSchemaPath().GetAttr("conflicting_property"),
			walk.NewSchemaPath().GetAttr("conflicting_property2"),
			walk.NewSchemaPath().GetAttr("conflicting_property_unidirectional"),
			walk.NewSchemaPath().GetAttr("float_property_value"),
			walk.NewSchemaPath().GetAttr("nested_resources"),
			walk.NewSchemaPath().GetAttr("nested_resources").Element().GetAttr("configuration"),
			walk.NewSchemaPath().GetAttr("nil_property_value"),
			walk.NewSchemaPath().GetAttr("number_property_value"),
			walk.NewSchemaPath().GetAttr("object_property_value"),
			walk.NewSchemaPath().GetAttr("set_property_value"),
			walk.NewSchemaPath().GetAttr("set_property_value").Element(),
			walk.NewSchemaPath().GetAttr("string_property_value"),
			walk.NewSchemaPath().GetAttr("string_with_bad_interpolation"),
		},
	}, seenPaths)

	seenPaths = map[string][]SchemaPath{}
	err = TraverseProperties(prov, t.Name(), func(i PropertyVisitInfo) (PropertyVisitResult, error) {
		paths := seenPaths[tfToken(i)]
		seenPaths[tfToken(i)] = append(paths, i.SchemaPath())
		return hasEffect(i)
	}, TraverseForEffect(true))
	require.NoError(t, err)

	for k, v := range seenPaths {
		walk.SortSchemaPaths(v)
		seenPaths[k] = v
	}

	assert.Equal(t, map[string][]walk.SchemaPath{
		"example_resource": {
			walk.NewSchemaPath().GetAttr("bool_property_value"),
			walk.NewSchemaPath().GetAttr("bool_property_value"),
			walk.NewSchemaPath().GetAttr("nested_resources").Element().GetAttr("opt_bool"),
		},
		"second_resource": {
			walk.NewSchemaPath().GetAttr("bool_property_value"),
		},
	}, seenPaths)
}

func TestTraversePropertiesSchemaInfo(t *testing.T) {
	t.Parallel()
	md := NewProviderMetadata(nil)
	testTFProviderV2 := testprovider.ProviderV2()

	prov := &ProviderInfo{
		P:            shimv2.NewProvider(testTFProviderV2),
		MetadataInfo: md,
	}

	isTarget := func(i PropertyVisitInfo) bool {
		return strings.Contains(i.SchemaPath().GoString(), "bool_property_value") ||
			strings.Contains(i.SchemaPath().GoString(), "opt_bool")
	}

	visitor := func(i PropertyVisitInfo) (PropertyVisitResult, error) {
		// Force the schema info to be produced for all visit props
		info := i.SchemaInfo()
		if isTarget(i) {
			info.ForceNew = BoolRef(true)
			return PropertyVisitResult{HasEffect: true}, nil
		}
		return PropertyVisitResult{}, nil
	}

	verify := func(prov *ProviderInfo) {
		assert.Equal(t, BoolRef(true), prov.Resources["example_resource"].
			Fields["nested_resources"].Elem.
			Fields["opt_bool"].ForceNew)
		assert.Equal(t, BoolRef(true), prov.Resources["example_resource"].
			Fields["bool_property_value"].ForceNew)
		assert.Equal(t, BoolRef(true), prov.Resources["second_resource"].
			Fields["bool_property_value"].ForceNew)
	}

	err := TraverseProperties(prov, t.Name(), visitor, TraverseForEffect(false))
	require.NoError(t, err)

	assert.NotNil(t, prov.Resources["example_resource"].
		Fields["nested_resources"].Elem.
		Fields["configuration"])

	verify(prov)

	// Reset prov - We are now testing for effect
	prov = &ProviderInfo{
		P:            shimv2.NewProvider(testTFProviderV2),
		MetadataInfo: ExtractRuntimeMetadata(md),
	}
	err = TraverseProperties(prov, t.Name(), visitor, TraverseForEffect(true))
	require.NoError(t, err)
	verify(prov)

	// This property did not have an effect, so it should not have been visited.
	assert.Nil(t, prov.Resources["example_resource"].
		Fields["nested_resources"].Elem.
		Fields["configuration"])
}

func TestLookupSchemas(t *testing.T) {
	t.Parallel()

	t.Run("string schema", func(t *testing.T) {
		schemaMap := shimv2.NewSchemaMap(map[string]*sdkv2schema.Schema{
			"foo": {Type: sdkv2schema.TypeString},
		})

		sch, _, err := LookupSchemas(walk.NewSchemaPath().GetAttr("foo"), schemaMap, nil)
		require.NoError(t, err)
		require.Equal(t, shim.TypeString, sch.Type())
	})

	t.Run("list schema", func(t *testing.T) {
		tfs := shimv2.NewSchemaMap(map[string]*sdkv2schema.Schema{
			"myList": {
				Type:     sdkv2schema.TypeList,
				Optional: true,
				Elem: &sdkv2schema.Schema{
					Type: sdkv2schema.TypeString,
				},
			},
		})

		sch, _, err := LookupSchemas(walk.NewSchemaPath().GetAttr("myList"), tfs, nil)
		require.NoError(t, err)
		require.Equal(t, shim.TypeList, sch.Type())
	})

	t.Run("list element schema", func(t *testing.T) {
		tfs := shimv2.NewSchemaMap(map[string]*sdkv2schema.Schema{
			"myList": {
				Type:     sdkv2schema.TypeList,
				Optional: true,
				Elem: &sdkv2schema.Schema{
					Type: sdkv2schema.TypeString,
				},
			},
		})

		sch, _, err := LookupSchemas(walk.NewSchemaPath().GetAttr("myList").Element(), tfs, nil)
		require.NoError(t, err)
		require.Equal(t, shim.TypeString, sch.Type())
	})

	t.Run("resource elem schema lookup", func(t *testing.T) {
		tfs := shimv2.NewSchemaMap(map[string]*sdkv2schema.Schema{
			"myList": {
				Type:     sdkv2schema.TypeList,
				Optional: true,
				Elem: &sdkv2schema.Resource{
					Schema: map[string]*sdkv2schema.Schema{
						"foo": {Type: sdkv2schema.TypeString},
					},
				},
			},
		})

		sch, _, err := LookupSchemas(walk.NewSchemaPath().GetAttr("myList").Element(), tfs, nil)
		require.NoError(t, err)

		// Note that the element of a list of resources is returned as a map. This is not technically correct
		// but is the what we currently do here.
		// A Map type with a resource element is not a valid schema in the SDKv2, so this is the only use of that combination.
		require.Equal(t, shim.TypeMap, sch.Type())
	})
}
