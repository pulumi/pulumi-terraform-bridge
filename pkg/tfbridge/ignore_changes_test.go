package tfbridge

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	v2Schema "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	shimv2 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v2"
)

type diffAndUpdateTestCase struct {
	diffTestCase
	expectedUpdateProps []interface{}
}

func diffAndUpdateTest(t *testing.T, tc diffAndUpdateTestCase) {
	ctx := context.Background()
	res := &v2Schema.Resource{
		Schema: tc.resourceSchema,
		CreateContext: func(ctx context.Context, rd *v2Schema.ResourceData, i interface{}) diag.Diagnostics {
			rd.SetId("id0")
			return nil
		},
		UpdateContext: func(ctx context.Context, rd *v2Schema.ResourceData, i interface{}) diag.Diagnostics {
			return nil
		},
	}
	provider := shimv2.NewProvider(&v2Schema.Provider{
		ResourcesMap: map[string]*v2Schema.Resource{
			"p_resource": res,
		},
	})
	state, err := plugin.MarshalProperties(tc.state, plugin.MarshalOptions{})
	require.NoError(t, err)

	inputs, err := plugin.MarshalProperties(tc.inputs, plugin.MarshalOptions{})
	require.NoError(t, err)

	p := Provider{
		tf: provider,
		info: ProviderInfo{
			Resources: map[string]*ResourceInfo{
				"p_resource": {
					Tok:    "pkg:index:PResource",
					Fields: tc.resourceFields,
				},
			},
		},
	}

	urn := "urn:pulumi:test::test::pkg:index:PResource::n1"
	p.initResourceMaps()
	checkResp, err := p.Check(ctx, &pulumirpc.CheckRequest{
		Urn:  urn,
		News: state,
	})
	assert.NoError(t, err)
	createResp, err := p.Create(ctx, &pulumirpc.CreateRequest{
		Urn:        urn,
		Preview:    false,
		Properties: checkResp.GetInputs(),
	})
	require.NoError(t, err)

	resp, err := p.Diff(ctx, &pulumirpc.DiffRequest{
		Id:            "myResource",
		Urn:           urn,
		Olds:          createResp.GetProperties(),
		News:          inputs,
		IgnoreChanges: tc.ignoreChanges,
	})
	require.NoError(t, err)
	updateResp, err := p.Update(context.Background(), &pulumirpc.UpdateRequest{
		Id:            "myResource",
		Urn:           urn,
		Olds:          state,
		News:          inputs,
		IgnoreChanges: tc.ignoreChanges,
	})
	require.NoError(t, err)

	outs, err := plugin.UnmarshalProperties(updateResp.GetProperties(), plugin.MarshalOptions{})
	require.NoError(t, err)

	assert.Equal(t, tc.expectedDiffChanges, resp.Changes)
	delete(outs, "__pulumi_raw_state_delta")
	if tc.expectedUpdateProps != nil {
		assert.Equal(t, resource.NewPropertyMapFromMap(map[string]interface{}{
			"id":    "myResource",
			"items": tc.expectedUpdateProps,
		}), outs)
	}
	require.Equal(t, tc.expected, resp.DetailedDiff)
}

// Collection of tests that test ignoreChanges functionality _without_ core involved.
// Both core and bridge process ignoreChanges. These tests test _only_ the bridge behavior
// These tests compliment the tests in `tests/ignore_changes_test.go`
func TestIgnoreChanges_bridge(t *testing.T) {
	t.Parallel()

	type tc struct {
		name                string
		schema              map[string]*v2Schema.Schema
		olds                resource.PropertyMap
		news                resource.PropertyMap
		ignoreChanges       []string
		expected            map[string]*pulumirpc.PropertyDiff
		expectedDiffChanges pulumirpc.DiffResponse_DiffChanges
		expectedUpdateProps []interface{}
	}

	cases := []tc{
		{
			name: "TopLevelString",
			schema: map[string]*v2Schema.Schema{
				"prop": {Type: v2Schema.TypeString, Optional: true},
			},
			olds: resource.NewPropertyMapFromMap(map[string]interface{}{
				"prop": "old",
			}),
			news: resource.NewPropertyMapFromMap(map[string]interface{}{
				"prop": "new",
			}),
			ignoreChanges: []string{"prop"},
		},
		{
			name: "ListIndexNestedField",
			schema: map[string]*v2Schema.Schema{
				"items": {
					Type: v2Schema.TypeList,
					Elem: &v2Schema.Resource{
						Schema: map[string]*v2Schema.Schema{
							"weight": {Type: v2Schema.TypeInt, Optional: true},
						},
					},
				},
			},
			olds: resource.PropertyMap{
				"items": resource.NewArrayProperty([]resource.PropertyValue{
					resource.NewObjectProperty(resource.PropertyMap{
						"weight": resource.NewNumberProperty(100),
					}),
				}),
			},
			news: resource.PropertyMap{
				"items": resource.NewArrayProperty([]resource.PropertyValue{
					resource.NewObjectProperty(resource.PropertyMap{
						"weight": resource.NewNumberProperty(200),
					}),
				}),
			},
			ignoreChanges: []string{"items[0].weight"},
		},
		{
			name: "ListIndexNestedFieldMaxItemsOne",
			schema: map[string]*v2Schema.Schema{
				"items": {
					Type:     v2Schema.TypeList,
					MaxItems: 1,
					Elem: &v2Schema.Resource{
						Schema: map[string]*v2Schema.Schema{
							"weight": {Type: v2Schema.TypeInt, Optional: true},
						},
					},
				},
			},
			olds: resource.PropertyMap{
				"items": resource.NewObjectProperty(resource.PropertyMap{
					"weight": resource.NewNumberProperty(100),
				}),
			},
			news: resource.PropertyMap{
				"items": resource.NewObjectProperty(resource.PropertyMap{
					"weight": resource.NewNumberProperty(200),
				}),
			},
			ignoreChanges: []string{"items.weight"},
		},
		{
			name: "ListIndexNestedFieldAddition",
			schema: map[string]*v2Schema.Schema{
				"items": {
					Type: v2Schema.TypeList,
					Elem: &v2Schema.Resource{
						Schema: map[string]*v2Schema.Schema{
							"weight": {Type: v2Schema.TypeInt, Optional: true},
						},
					},
				},
			},
			olds: resource.PropertyMap{
				"items": resource.NewArrayProperty([]resource.PropertyValue{
					resource.NewObjectProperty(resource.PropertyMap{
						"weight": resource.NewNumberProperty(100),
					}),
				}),
			},
			news: resource.PropertyMap{
				"items": resource.NewArrayProperty([]resource.PropertyValue{
					resource.NewObjectProperty(resource.PropertyMap{
						"weight": resource.NewNumberProperty(200),
					}),
					resource.NewObjectProperty(resource.PropertyMap{
						"weight": resource.NewNumberProperty(300),
					}),
				}),
			},
			ignoreChanges:       []string{"items[0].weight"},
			expectedDiffChanges: pulumirpc.DiffResponse_DIFF_SOME,
			expected: map[string]*pulumirpc.PropertyDiff{
				"items[1]": {},
			},
			expectedUpdateProps: []interface{}{
				map[string]any{"weight": 100},
				map[string]any{"weight": 300},
			},
		},
		{
			name: "ListIndexNestedFieldAdditionIgnored",
			schema: map[string]*v2Schema.Schema{
				"items": {
					Type: v2Schema.TypeList,
					Elem: &v2Schema.Resource{
						Schema: map[string]*v2Schema.Schema{
							"weight": {Type: v2Schema.TypeInt, Optional: true},
						},
					},
				},
			},
			olds: resource.PropertyMap{
				"items": resource.NewArrayProperty([]resource.PropertyValue{
					resource.NewObjectProperty(resource.PropertyMap{
						"weight": resource.NewNumberProperty(100),
					}),
				}),
			},
			news: resource.PropertyMap{
				"items": resource.NewArrayProperty([]resource.PropertyValue{
					resource.NewObjectProperty(resource.PropertyMap{
						"weight": resource.NewNumberProperty(200),
					}),
					resource.NewObjectProperty(resource.PropertyMap{
						"weight": resource.NewNumberProperty(300),
					}),
				}),
			},
			ignoreChanges:       []string{"items[0].weight", "items[1].weight"},
			expectedDiffChanges: pulumirpc.DiffResponse_DIFF_SOME,
			expected: map[string]*pulumirpc.PropertyDiff{
				"items[1]": {},
			},
			expectedUpdateProps: []interface{}{
				map[string]any{"weight": 100},
				// TODO: [pulumi/pulumi-terraform-bridge#3186]
				// new items should not be removed
				map[string]any{"weight": nil},
			},
		},
		{
			name: "ListWildcardNestedField",
			schema: map[string]*v2Schema.Schema{
				"items": {
					Type: v2Schema.TypeList,
					Elem: &v2Schema.Resource{
						Schema: map[string]*v2Schema.Schema{
							"weight": {Type: v2Schema.TypeInt, Optional: true},
						},
					},
				},
			},
			olds: resource.PropertyMap{
				"items": resource.NewArrayProperty([]resource.PropertyValue{
					resource.NewObjectProperty(resource.PropertyMap{"weight": resource.NewNumberProperty(100)}),
					resource.NewObjectProperty(resource.PropertyMap{"weight": resource.NewNumberProperty(300)}),
				}),
			},
			news: resource.PropertyMap{
				"items": resource.NewArrayProperty([]resource.PropertyValue{
					resource.NewObjectProperty(resource.PropertyMap{"weight": resource.NewNumberProperty(200)}),
					resource.NewObjectProperty(resource.PropertyMap{"weight": resource.NewNumberProperty(400)}),
				}),
			},
			// FIXME: ignoreChanges ignored!
			expectedDiffChanges: pulumirpc.DiffResponse_DIFF_SOME,
			expected: map[string]*pulumirpc.PropertyDiff{
				"items[0].weight": {Kind: pulumirpc.PropertyDiff_UPDATE},
				"items[1].weight": {Kind: pulumirpc.PropertyDiff_UPDATE},
			},
			ignoreChanges: []string{"items[*].weight"},
			expectedUpdateProps: []interface{}{
				map[string]any{"weight": 200},
				map[string]any{"weight": 400},
			},
		},
		{
			name: "ListWildcardNestedFieldAddition",
			schema: map[string]*v2Schema.Schema{
				"items": {
					Type: v2Schema.TypeList,
					Elem: &v2Schema.Resource{
						Schema: map[string]*v2Schema.Schema{
							"weight": {Type: v2Schema.TypeInt, Optional: true},
						},
					},
				},
			},
			olds: resource.PropertyMap{
				"items": resource.NewArrayProperty([]resource.PropertyValue{
					resource.NewObjectProperty(resource.PropertyMap{"weight": resource.NewNumberProperty(100)}),
					resource.NewObjectProperty(resource.PropertyMap{"weight": resource.NewNumberProperty(300)}),
				}),
			},
			news: resource.PropertyMap{
				"items": resource.NewArrayProperty([]resource.PropertyValue{
					resource.NewObjectProperty(resource.PropertyMap{"weight": resource.NewNumberProperty(200)}),
					resource.NewObjectProperty(resource.PropertyMap{"weight": resource.NewNumberProperty(400)}),
					resource.NewObjectProperty(resource.PropertyMap{"weight": resource.NewNumberProperty(500)}),
				}),
			},
			ignoreChanges: []string{"items[*].weight"},
			// FIXME: ignoreChanges ignored!
			expectedDiffChanges: pulumirpc.DiffResponse_DIFF_SOME,
			expected: map[string]*pulumirpc.PropertyDiff{
				"items[0].weight": {Kind: pulumirpc.PropertyDiff_UPDATE},
				"items[1].weight": {Kind: pulumirpc.PropertyDiff_UPDATE},
				"items[2]":        {},
			},
			expectedUpdateProps: []interface{}{
				map[string]any{"weight": 200},
				map[string]any{"weight": 400},
				map[string]any{"weight": 500},
			},
		},
		{
			name: "ObjectNestedField",
			schema: map[string]*v2Schema.Schema{
				"items": {
					Type: v2Schema.TypeMap,
					Elem: &v2Schema.Schema{Type: v2Schema.TypeInt},
				},
			},
			olds: resource.PropertyMap{
				"items": resource.NewObjectProperty(
					resource.PropertyMap{
						"weight": resource.NewNumberProperty(100),
						"other":  resource.NewNumberProperty(100),
					},
				),
			},
			news: resource.PropertyMap{
				"items": resource.NewObjectProperty(
					resource.PropertyMap{
						"weight": resource.NewNumberProperty(200),
						"other":  resource.NewNumberProperty(200),
					},
				),
			},
			ignoreChanges:       []string{"items.weight"},
			expectedDiffChanges: pulumirpc.DiffResponse_DIFF_SOME,
			expected: map[string]*pulumirpc.PropertyDiff{
				"items.other": {Kind: pulumirpc.PropertyDiff_UPDATE},
			},
		},
		{
			name: "ObjectNestedFieldWildcard",
			schema: map[string]*v2Schema.Schema{
				"items": {
					Type: v2Schema.TypeMap,
					Elem: &v2Schema.Schema{Type: v2Schema.TypeInt},
				},
			},
			olds: resource.PropertyMap{
				"items": resource.NewObjectProperty(
					resource.PropertyMap{
						"weight": resource.NewNumberProperty(100),
						"other":  resource.NewNumberProperty(100),
					},
				),
			},
			news: resource.PropertyMap{
				"items": resource.NewObjectProperty(
					resource.PropertyMap{
						"weight": resource.NewNumberProperty(200),
						"other":  resource.NewNumberProperty(200),
					},
				),
			},
			ignoreChanges: []string{"items.*"},
			// FIXME: ignoreChanges ignored!
			expectedDiffChanges: pulumirpc.DiffResponse_DIFF_SOME,
			expected: map[string]*pulumirpc.PropertyDiff{
				"items.other":  {Kind: pulumirpc.PropertyDiff_UPDATE},
				"items.weight": {Kind: pulumirpc.PropertyDiff_UPDATE},
			},
		},
		{
			name: "ObjectNestedFieldWildcardAddition",
			schema: map[string]*v2Schema.Schema{
				"items": {
					Type: v2Schema.TypeMap,
					Elem: &v2Schema.Schema{Type: v2Schema.TypeInt},
				},
			},
			olds: resource.PropertyMap{
				"items": resource.NewObjectProperty(
					resource.PropertyMap{
						"weight": resource.NewNumberProperty(100),
						"other":  resource.NewNumberProperty(100),
					},
				),
			},
			news: resource.PropertyMap{
				"items": resource.NewObjectProperty(
					resource.PropertyMap{
						"weight": resource.NewNumberProperty(200),
						"other":  resource.NewNumberProperty(200),
						"third":  resource.NewNumberProperty(300),
					},
				),
			},
			ignoreChanges: []string{"items.*"},
			// FIXME: ignoreChanges ignored!
			expectedDiffChanges: pulumirpc.DiffResponse_DIFF_SOME,
			expected: map[string]*pulumirpc.PropertyDiff{
				"items.other":  {Kind: pulumirpc.PropertyDiff_UPDATE},
				"items.weight": {Kind: pulumirpc.PropertyDiff_UPDATE},
				"items.third":  {},
			},
		},
		{
			name: "SetIndexNestedField",
			schema: map[string]*v2Schema.Schema{
				"items": {
					Type: v2Schema.TypeSet,
					Elem: &v2Schema.Resource{
						Schema: map[string]*v2Schema.Schema{
							"weight": {Type: v2Schema.TypeInt, Optional: true},
						},
					},
				},
			},
			olds: resource.PropertyMap{
				"items": resource.NewArrayProperty([]resource.PropertyValue{
					resource.NewObjectProperty(resource.PropertyMap{"weight": resource.NewNumberProperty(100)}),
					resource.NewObjectProperty(resource.PropertyMap{"weight": resource.NewNumberProperty(300)}),
				}),
			},
			news: resource.PropertyMap{
				"items": resource.NewArrayProperty([]resource.PropertyValue{
					resource.NewObjectProperty(resource.PropertyMap{"weight": resource.NewNumberProperty(200)}),
					resource.NewObjectProperty(resource.PropertyMap{"weight": resource.NewNumberProperty(400)}),
				}),
			},
			ignoreChanges:       []string{"items[0].weight"},
			expectedDiffChanges: pulumirpc.DiffResponse_DIFF_SOME,
			expected: map[string]*pulumirpc.PropertyDiff{
				// TODO: set indexing doesn't work at all
				// Note that this does work in `pkg/tests/ignore_changes_test.go` when you include
				// the engine ignore changes processing
				// [pulumi/pulumi-terraform-bridge#1756]
				"items[0]":        {Kind: pulumirpc.PropertyDiff_DELETE},
				"items[1].weight": {Kind: pulumirpc.PropertyDiff_UPDATE},
			},
			expectedUpdateProps: []interface{}{
				map[string]any{"weight": float64(400)},
			},
		},
		{
			name: "SetNestedFieldWildcard",
			schema: map[string]*v2Schema.Schema{
				"items": {
					Type: v2Schema.TypeSet,
					Elem: &v2Schema.Resource{
						Schema: map[string]*v2Schema.Schema{
							"weight": {Type: v2Schema.TypeInt, Optional: true},
						},
					},
				},
			},
			olds: resource.PropertyMap{
				"items": resource.NewArrayProperty([]resource.PropertyValue{
					resource.NewObjectProperty(resource.PropertyMap{"weight": resource.NewNumberProperty(100)}),
					resource.NewObjectProperty(resource.PropertyMap{"weight": resource.NewNumberProperty(300)}),
				}),
			},
			news: resource.PropertyMap{
				"items": resource.NewArrayProperty([]resource.PropertyValue{
					resource.NewObjectProperty(resource.PropertyMap{"weight": resource.NewNumberProperty(200)}),
					resource.NewObjectProperty(resource.PropertyMap{"weight": resource.NewNumberProperty(400)}),
				}),
			},
			ignoreChanges: []string{"items[*].weight"},
			// FIXME: ignoreChanges ignored!
			expectedDiffChanges: pulumirpc.DiffResponse_DIFF_SOME,
			expected: map[string]*pulumirpc.PropertyDiff{
				"items[0].weight": {Kind: pulumirpc.PropertyDiff_UPDATE},
				"items[1].weight": {Kind: pulumirpc.PropertyDiff_UPDATE},
			},
			expectedUpdateProps: []interface{}{
				map[string]any{"weight": float64(200)},
				map[string]any{"weight": float64(400)},
			},
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			expected := c.expected
			if c.expected == nil {
				expected = map[string]*pulumirpc.PropertyDiff{}
			}
			expectedDiffChanges := pulumirpc.DiffResponse_DIFF_NONE
			if c.expectedDiffChanges != 0 {
				expectedDiffChanges = c.expectedDiffChanges
			}

			diffAndUpdateTest(t, diffAndUpdateTestCase{
				diffTestCase: diffTestCase{
					resourceSchema:      c.schema,
					resourceFields:      nil,
					state:               c.olds,
					inputs:              c.news,
					ignoreChanges:       c.ignoreChanges,
					expectedDiffChanges: expectedDiffChanges,
					expected:            expected,
				},
				expectedUpdateProps: c.expectedUpdateProps,
			})
		})
	}
}
