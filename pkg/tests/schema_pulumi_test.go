package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hexops/autogold/v2"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tests/internal/pulcheck"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tests/internal/tfcheck"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optpreview"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optrefresh"
	"github.com/stretchr/testify/require"
)

func TestBasic(t *testing.T) {
	resMap := map[string]*schema.Resource{
		"prov_test": {
			Schema: map[string]*schema.Schema{
				"test": {
					Type:     schema.TypeString,
					Optional: true,
				},
			},
		},
	}
	tfp := &schema.Provider{ResourcesMap: resMap}
	bridgedProvider := pulcheck.BridgedProvider(t, "prov", tfp)
	program := `
name: test
runtime: yaml
resources:
  mainRes:
    type: prov:index:Test
	properties:
	  test: "hello"
outputs:
  testOut: ${mainRes.test}
`
	pt := pulcheck.PulCheck(t, bridgedProvider, program)
	res := pt.Up()
	require.Equal(t, "hello", res.Outputs["testOut"].Value)
}

func TestUnknownHandling(t *testing.T) {
	resMap := map[string]*schema.Resource{
		"prov_test": {
			Schema: map[string]*schema.Schema{
				"test": {
					Type:     schema.TypeString,
					Optional: true,
				},
			},
		},
		"prov_aux": {
			Schema: map[string]*schema.Schema{
				"aux": {
					Type:     schema.TypeString,
					Computed: true,
					Optional: true,
				},
			},
			CreateContext: func(_ context.Context, d *schema.ResourceData, _ interface{}) diag.Diagnostics {
				d.SetId("aux")
				err := d.Set("aux", "aux")
				require.NoError(t, err)
				return nil
			},
		},
	}
	tfp := &schema.Provider{ResourcesMap: resMap}
	bridgedProvider := pulcheck.BridgedProvider(t, "prov", tfp)
	program := `
name: test
runtime: yaml
resources:
  auxRes:
    type: prov:index:Aux
  mainRes:
    type: prov:index:Test
    properties:
      test: ${auxRes.aux}
outputs:
  testOut: ${mainRes.test}
`
	pt := pulcheck.PulCheck(t, bridgedProvider, program)
	res := pt.Preview(optpreview.Diff())
	// Test that the test property is unknown at preview time
	require.Contains(t, res.StdOut, "test      : output<string>")
	resUp := pt.Up()
	// assert that the property gets resolved
	require.Equal(t, "aux", resUp.Outputs["testOut"].Value)
}

func TestCollectionsNullEmptyRefreshClean(t *testing.T) {
	for _, tc := range []struct {
		name               string
		planResourceChange bool
		schemaType         schema.ValueType
		cloudVal           interface{}
		programVal         string
		// If true, the cloud value will be set in the CreateContext
		// This is behaviour observed in both AWS and GCP providers, as well as a few others
		// where the provider returns an empty collections when a nil one was specified in inputs.
		// See [pulumi/pulumi-terraform-bridge#2047] for more details around this behavior
		createCloudValOverride bool
		expectedOutputTopLevel interface{}
		expectedOutputNested   interface{}
	}{
		{
			name:                   "map null with planResourceChange",
			planResourceChange:     true,
			schemaType:             schema.TypeMap,
			cloudVal:               map[string]interface{}{},
			programVal:             "null",
			expectedOutputTopLevel: nil,
			expectedOutputNested:   nil,
		},
		{
			name:               "map null without planResourceChange",
			planResourceChange: false,
			schemaType:         schema.TypeMap,
			cloudVal:           map[string]interface{}{},
			programVal:         "null",
			// Note the difference in expected output between top level and nested properties
			expectedOutputTopLevel: nil,
			expectedOutputNested:   map[string]interface{}{},
		},
		{
			name:                   "map null with planResourceChange with nil cloud value",
			planResourceChange:     true,
			schemaType:             schema.TypeMap,
			cloudVal:               nil,
			programVal:             "null",
			expectedOutputTopLevel: nil,
			expectedOutputNested:   nil,
		},
		{
			name:                   "map null without planResourceChange with nil cloud value",
			planResourceChange:     false,
			schemaType:             schema.TypeMap,
			cloudVal:               nil,
			programVal:             "null",
			expectedOutputTopLevel: nil,
			expectedOutputNested:   map[string]interface{}{},
		},
		{
			name:                   "map null with planResourceChange with cloud override",
			planResourceChange:     true,
			schemaType:             schema.TypeMap,
			cloudVal:               map[string]interface{}{},
			programVal:             "null",
			createCloudValOverride: true,
			expectedOutputTopLevel: nil,
			expectedOutputNested:   nil,
		},
		{
			name:                   "map null without planResourceChange with cloud override",
			planResourceChange:     false,
			schemaType:             schema.TypeMap,
			cloudVal:               map[string]interface{}{},
			programVal:             "null",
			createCloudValOverride: true,
			expectedOutputTopLevel: map[string]interface{}{},
			expectedOutputNested:   map[string]interface{}{},
		},
		{
			name:                   "map null with planResourceChange with nil cloud value and cloud override",
			planResourceChange:     true,
			schemaType:             schema.TypeMap,
			cloudVal:               nil,
			programVal:             "null",
			createCloudValOverride: true,
			expectedOutputTopLevel: nil,
			expectedOutputNested:   nil,
		},
		{
			name:                   "map null without planResourceChange with nil cloud value and cloud override",
			planResourceChange:     false,
			schemaType:             schema.TypeMap,
			cloudVal:               nil,
			programVal:             "null",
			createCloudValOverride: true,
			expectedOutputTopLevel: map[string]interface{}{},
			expectedOutputNested:   map[string]interface{}{},
		},
		{
			name:                   "map empty with planResourceChange",
			planResourceChange:     true,
			schemaType:             schema.TypeMap,
			cloudVal:               map[string]interface{}{},
			programVal:             "{}",
			expectedOutputTopLevel: nil,
			expectedOutputNested:   nil,
		},
		{
			name:               "map empty without planResourceChange",
			planResourceChange: false,
			schemaType:         schema.TypeMap,
			cloudVal:           map[string]interface{}{},
			programVal:         "{}",
			// Note the difference in expected output between top level and nested properties
			expectedOutputTopLevel: nil,
			expectedOutputNested:   map[string]interface{}{},
		},
		{
			name:                   "map empty with planResourceChange with cloud override",
			planResourceChange:     true,
			schemaType:             schema.TypeMap,
			cloudVal:               map[string]interface{}{},
			programVal:             "{}",
			createCloudValOverride: true,
			expectedOutputTopLevel: nil,
			expectedOutputNested:   nil,
		},
		{
			name:                   "map empty without planResourceChange with cloud override",
			planResourceChange:     false,
			schemaType:             schema.TypeMap,
			cloudVal:               map[string]interface{}{},
			programVal:             "{}",
			createCloudValOverride: true,
			expectedOutputTopLevel: map[string]interface{}{},
			expectedOutputNested:   map[string]interface{}{},
		},
		{
			name:                   "map nonempty with planResourceChange",
			planResourceChange:     true,
			schemaType:             schema.TypeMap,
			cloudVal:               map[string]interface{}{"val": "test"},
			programVal:             `{"val": "test"}`,
			expectedOutputTopLevel: map[string]interface{}{"val": "test"},
			expectedOutputNested:   map[string]interface{}{"val": "test"},
		},
		{
			name:                   "map nonempty without planResourceChange",
			planResourceChange:     false,
			schemaType:             schema.TypeMap,
			cloudVal:               map[string]interface{}{"val": "test"},
			programVal:             `{"val": "test"}`,
			expectedOutputTopLevel: map[string]interface{}{"val": "test"},
			expectedOutputNested:   map[string]interface{}{"val": "test"},
		},
		{
			name:                   "map nonempty with planResourceChange with cloud override",
			planResourceChange:     true,
			schemaType:             schema.TypeMap,
			cloudVal:               map[string]interface{}{"val": "test"},
			programVal:             `{"val": "test"}`,
			createCloudValOverride: true,
			expectedOutputTopLevel: map[string]interface{}{"val": "test"},
			expectedOutputNested:   map[string]interface{}{"val": "test"},
		},
		{
			name:                   "map nonempty without planResourceChange with cloud override",
			planResourceChange:     false,
			schemaType:             schema.TypeMap,
			cloudVal:               map[string]interface{}{"val": "test"},
			programVal:             `{"val": "test"}`,
			createCloudValOverride: true,
			expectedOutputTopLevel: map[string]interface{}{"val": "test"},
			expectedOutputNested:   map[string]interface{}{"val": "test"},
		},
		{
			name:                   "list null with planResourceChange",
			planResourceChange:     true,
			schemaType:             schema.TypeList,
			cloudVal:               []interface{}{},
			programVal:             "null",
			expectedOutputTopLevel: nil,
			expectedOutputNested:   nil,
		},
		{
			name:               "list null without planResourceChange",
			planResourceChange: false,
			schemaType:         schema.TypeList,
			cloudVal:           []interface{}{},
			programVal:         "null",
			// Note the difference in expected output between top level and nested properties
			expectedOutputTopLevel: nil,
			expectedOutputNested:   []interface{}{},
		},
		{
			name:                   "list null with planResourceChange with nil cloud value",
			planResourceChange:     true,
			schemaType:             schema.TypeList,
			cloudVal:               nil,
			programVal:             "null",
			expectedOutputTopLevel: nil,
			expectedOutputNested:   nil,
		},
		{
			name:               "list null without planResourceChange with nil cloud value",
			planResourceChange: false,
			schemaType:         schema.TypeList,
			cloudVal:           nil,
			programVal:         "null",
			// Note the difference in expected output between top level and nested properties
			expectedOutputTopLevel: nil,
			expectedOutputNested:   []interface{}{},
		},
		{
			name:                   "list null with planResourceChange with cloud override",
			planResourceChange:     true,
			schemaType:             schema.TypeList,
			cloudVal:               []interface{}{},
			programVal:             "null",
			createCloudValOverride: true,
			expectedOutputTopLevel: nil,
			expectedOutputNested:   nil,
		},
		{
			name:                   "list null without planResourceChange with cloud override",
			planResourceChange:     false,
			schemaType:             schema.TypeList,
			cloudVal:               []interface{}{},
			programVal:             "null",
			createCloudValOverride: true,
			expectedOutputTopLevel: []interface{}{},
			expectedOutputNested:   []interface{}{},
		},
		{
			name:                   "list null with planResourceChange with nil cloud value and cloud override",
			planResourceChange:     true,
			schemaType:             schema.TypeList,
			cloudVal:               nil,
			programVal:             "null",
			createCloudValOverride: true,
			expectedOutputTopLevel: nil,
			expectedOutputNested:   nil,
		},
		{
			name:                   "list null without planResourceChange with nil cloud value and cloud override",
			planResourceChange:     false,
			schemaType:             schema.TypeList,
			cloudVal:               nil,
			programVal:             "null",
			createCloudValOverride: true,
			expectedOutputTopLevel: []interface{}{},
			expectedOutputNested:   []interface{}{},
		},
		{
			name:                   "list empty with planResourceChange",
			planResourceChange:     true,
			schemaType:             schema.TypeList,
			cloudVal:               []string{},
			programVal:             "[]",
			expectedOutputTopLevel: []interface{}{},
			expectedOutputNested:   []interface{}{},
		},
		{
			name:               "list empty without planResourceChange",
			planResourceChange: false,
			schemaType:         schema.TypeList,
			cloudVal:           []string{},
			programVal:         "[]",
			// Note the difference in expected output between top level and nested properties
			expectedOutputTopLevel: nil,
			expectedOutputNested:   []interface{}{},
		},
		{
			name:                   "list empty with planResourceChange with cloud override",
			planResourceChange:     true,
			schemaType:             schema.TypeList,
			cloudVal:               []string{},
			programVal:             "[]",
			createCloudValOverride: true,
			expectedOutputTopLevel: []interface{}{},
			expectedOutputNested:   []interface{}{},
		},
		{
			name:                   "list empty without planResourceChange with cloud override",
			planResourceChange:     false,
			schemaType:             schema.TypeList,
			cloudVal:               []string{},
			programVal:             "[]",
			createCloudValOverride: true,
			expectedOutputTopLevel: []interface{}{},
			expectedOutputNested:   []interface{}{},
		},
		{
			name:                   "list nonempty with planResourceChange",
			planResourceChange:     true,
			schemaType:             schema.TypeList,
			cloudVal:               []interface{}{"val"},
			programVal:             `["val"]`,
			expectedOutputTopLevel: []interface{}{"val"},
			expectedOutputNested:   []interface{}{"val"},
		},
		{
			name:                   "list nonempty without planResourceChange",
			planResourceChange:     false,
			schemaType:             schema.TypeList,
			cloudVal:               []interface{}{"val"},
			programVal:             `["val"]`,
			expectedOutputTopLevel: []interface{}{"val"},
			expectedOutputNested:   []interface{}{"val"},
		},
		{
			name:                   "list nonempty with planResourceChange with cloud override",
			planResourceChange:     true,
			schemaType:             schema.TypeList,
			cloudVal:               []interface{}{"val"},
			programVal:             `["val"]`,
			createCloudValOverride: true,
			expectedOutputTopLevel: []interface{}{"val"},
			expectedOutputNested:   []interface{}{"val"},
		},
		{
			name:                   "list nonempty without planResourceChange with cloud override",
			planResourceChange:     false,
			schemaType:             schema.TypeList,
			cloudVal:               []interface{}{"val"},
			programVal:             `["val"]`,
			createCloudValOverride: true,
			expectedOutputTopLevel: []interface{}{"val"},
			expectedOutputNested:   []interface{}{"val"},
		},
		{
			name:                   "set null with planResourceChange",
			planResourceChange:     true,
			schemaType:             schema.TypeSet,
			cloudVal:               []interface{}{},
			programVal:             "null",
			expectedOutputTopLevel: nil,
			expectedOutputNested:   nil,
		},
		{
			name:               "set null without planResourceChange",
			planResourceChange: false,
			schemaType:         schema.TypeSet,
			cloudVal:           []interface{}{},
			programVal:         "null",
			// Note the difference in expected output between top level and nested properties
			expectedOutputTopLevel: nil,
			expectedOutputNested:   []interface{}{},
		},
		{
			name:                   "set null with planResourceChange with nil cloud value",
			planResourceChange:     true,
			schemaType:             schema.TypeSet,
			cloudVal:               nil,
			programVal:             "null",
			expectedOutputTopLevel: nil,
			expectedOutputNested:   nil,
		},
		{
			name:                   "set null without planResourceChange with nil cloud value",
			planResourceChange:     false,
			schemaType:             schema.TypeSet,
			cloudVal:               nil,
			programVal:             "null",
			expectedOutputTopLevel: nil,
			expectedOutputNested:   []interface{}{},
		},
		{
			name:                   "set null with planResourceChange with cloud override",
			planResourceChange:     true,
			schemaType:             schema.TypeSet,
			cloudVal:               []interface{}{},
			programVal:             "null",
			createCloudValOverride: true,
			expectedOutputTopLevel: nil,
			expectedOutputNested:   nil,
		},
		{
			name:                   "set null without planResourceChange with cloud override",
			planResourceChange:     false,
			schemaType:             schema.TypeSet,
			cloudVal:               []interface{}{},
			programVal:             "null",
			createCloudValOverride: true,
			expectedOutputTopLevel: []interface{}{},
			expectedOutputNested:   []interface{}{},
		},
		{
			name:                   "set null with planResourceChange with nil cloud value and cloud override",
			planResourceChange:     true,
			schemaType:             schema.TypeSet,
			cloudVal:               nil,
			programVal:             "null",
			createCloudValOverride: true,
			expectedOutputTopLevel: nil,
			expectedOutputNested:   nil,
		},
		{
			name:                   "set null without planResourceChange with nil cloud value and cloud override",
			planResourceChange:     false,
			schemaType:             schema.TypeSet,
			cloudVal:               nil,
			programVal:             "null",
			createCloudValOverride: true,
			expectedOutputTopLevel: []interface{}{},
			expectedOutputNested:   []interface{}{},
		},
		{
			name:                   "set empty with planResourceChange",
			planResourceChange:     true,
			schemaType:             schema.TypeSet,
			cloudVal:               []interface{}{},
			programVal:             "[]",
			expectedOutputTopLevel: nil,
			expectedOutputNested:   nil,
		},
		{
			name:               "set empty without planResourceChange",
			planResourceChange: false,
			schemaType:         schema.TypeSet,
			cloudVal:           []interface{}{},
			programVal:         "[]",
			// Note the difference in expected output between top level and nested properties
			expectedOutputTopLevel: nil,
			expectedOutputNested:   []interface{}{},
		},
		{
			name:                   "set empty with planResourceChange with cloud override",
			planResourceChange:     true,
			schemaType:             schema.TypeSet,
			cloudVal:               []interface{}{},
			programVal:             "[]",
			createCloudValOverride: true,
			expectedOutputTopLevel: nil,
			expectedOutputNested:   nil,
		},
		{
			name:                   "set empty without planResourceChange with cloud override",
			planResourceChange:     false,
			schemaType:             schema.TypeSet,
			cloudVal:               []interface{}{},
			programVal:             "[]",
			createCloudValOverride: true,
			expectedOutputTopLevel: []interface{}{},
			expectedOutputNested:   []interface{}{},
		},
		{
			name:                   "set nonempty with planResourceChange",
			schemaType:             schema.TypeSet,
			cloudVal:               []interface{}{"val"},
			programVal:             `["val"]`,
			expectedOutputTopLevel: []interface{}{"val"},
			expectedOutputNested:   []interface{}{"val"},
		},
		{
			name:                   "set nonempty without planResourceChange",
			planResourceChange:     false,
			schemaType:             schema.TypeSet,
			cloudVal:               []interface{}{"val"},
			programVal:             `["val"]`,
			expectedOutputTopLevel: []interface{}{"val"},
			expectedOutputNested:   []interface{}{"val"},
		},
		{
			name:                   "set nonempty with planResourceChange with cloud override",
			schemaType:             schema.TypeSet,
			cloudVal:               []interface{}{"val"},
			programVal:             `["val"]`,
			createCloudValOverride: true,
			expectedOutputTopLevel: []interface{}{"val"},
			expectedOutputNested:   []interface{}{"val"},
		},
		{
			name:                   "set nonempty without planResourceChange with cloud override",
			planResourceChange:     false,
			schemaType:             schema.TypeSet,
			cloudVal:               []interface{}{"val"},
			programVal:             `["val"]`,
			createCloudValOverride: true,
			expectedOutputTopLevel: []interface{}{"val"},
			expectedOutputNested:   []interface{}{"val"},
		},
	} {
		collectionPropPlural := ""
		pluralized := tc.schemaType == schema.TypeList || tc.schemaType == schema.TypeSet
		if pluralized {
			collectionPropPlural += "s"
		}

		opts := []pulcheck.BridgedProviderOpt{}
		if !tc.planResourceChange {
			opts = append(opts, pulcheck.DisablePlanResourceChange())
		}

		t.Run(tc.name, func(t *testing.T) {
			t.Run("top level", func(t *testing.T) {
				t.Parallel()
				resMap := map[string]*schema.Resource{
					"prov_test": {
						Schema: map[string]*schema.Schema{
							"collection_prop": {
								Type:     tc.schemaType,
								Optional: true,
								Elem:     &schema.Schema{Type: schema.TypeString},
							},
							"other_prop": {
								Type:     schema.TypeString,
								Optional: true,
							},
						},
						ReadContext: func(_ context.Context, rd *schema.ResourceData, _ interface{}) diag.Diagnostics {
							err := rd.Set("collection_prop", tc.cloudVal)
							require.NoError(t, err)
							err = rd.Set("other_prop", "test")
							require.NoError(t, err)
							return nil
						},
						CreateContext: func(_ context.Context, rd *schema.ResourceData, _ interface{}) diag.Diagnostics {
							if tc.createCloudValOverride {
								err := rd.Set("collection_prop", tc.cloudVal)
								require.NoError(t, err)
							}

							rd.SetId("id0")
							return nil
						},
					},
				}

				tfp := &schema.Provider{ResourcesMap: resMap}
				bridgedProvider := pulcheck.BridgedProvider(t, "prov", tfp, opts...)
				program := fmt.Sprintf(`
name: test
runtime: yaml
resources:
  mainRes:
    type: prov:index:Test
    properties:
      otherProp: "test"
      collectionProp%s: %s
outputs:
  collectionOutput: ${mainRes.collectionProp%s}
`, collectionPropPlural, tc.programVal, collectionPropPlural)
				pt := pulcheck.PulCheck(t, bridgedProvider, program)
				upRes := pt.Up()
				require.Equal(t, tc.expectedOutputTopLevel, upRes.Outputs["collectionOutput"].Value)
				res, err := pt.CurrentStack().Refresh(pt.Context(), optrefresh.ExpectNoChanges())
				require.NoError(t, err)
				t.Logf(res.StdOut)
				prevRes, err := pt.CurrentStack().Preview(pt.Context(), optpreview.ExpectNoChanges(), optpreview.Diff())
				require.NoError(t, err)
				t.Logf(prevRes.StdOut)
			})

			t.Run("nested", func(t *testing.T) {
				t.Parallel()
				resMap := map[string]*schema.Resource{
					"prov_test": {
						Schema: map[string]*schema.Schema{
							"prop": {
								Type:     schema.TypeList,
								Optional: true,
								Elem: &schema.Resource{
									Schema: map[string]*schema.Schema{
										"collection_prop": {
											Type:     tc.schemaType,
											Optional: true,
											Elem:     &schema.Schema{Type: schema.TypeString},
										},
										"other_nested_prop": {
											Type:     schema.TypeString,
											Optional: true,
										},
									},
								},
							},
							"other_prop": {
								Type:     schema.TypeString,
								Optional: true,
							},
						},
						ReadContext: func(_ context.Context, rd *schema.ResourceData, _ interface{}) diag.Diagnostics {
							err := rd.Set("prop", []map[string]interface{}{{"collection_prop": tc.cloudVal, "other_nested_prop": "test"}})
							require.NoError(t, err)
							err = rd.Set("other_prop", "test")
							require.NoError(t, err)

							return nil
						},
						CreateContext: func(_ context.Context, rd *schema.ResourceData, _ interface{}) diag.Diagnostics {
							if tc.createCloudValOverride {
								err := rd.Set("prop", []map[string]interface{}{{"collection_prop": tc.cloudVal, "other_nested_prop": "test"}})
								require.NoError(t, err)
							}
							rd.SetId("id0")
							return nil
						},
					},
				}

				tfp := &schema.Provider{ResourcesMap: resMap}
				bridgedProvider := pulcheck.BridgedProvider(t, "prov", tfp, opts...)
				program := fmt.Sprintf(`
name: test
runtime: yaml
resources:
  mainRes:
    type: prov:index:Test
    properties:
      otherProp: "test"
      props:
        - collectionProp%s: %s
          otherNestedProp: "test"
outputs:
  collectionOutput: ${mainRes.props[0].collectionProp%s}
`, collectionPropPlural, tc.programVal, collectionPropPlural)
				pt := pulcheck.PulCheck(t, bridgedProvider, program)
				upRes := pt.Up()
				require.Equal(t, tc.expectedOutputNested, upRes.Outputs["collectionOutput"].Value)

				res, err := pt.CurrentStack().Refresh(pt.Context(), optrefresh.ExpectNoChanges())
				require.NoError(t, err)
				t.Logf(res.StdOut)
				prevRes, err := pt.CurrentStack().Preview(pt.Context(), optpreview.ExpectNoChanges(), optpreview.Diff())
				require.NoError(t, err)
				t.Logf(prevRes.StdOut)
			})
		})
	}
}

func TestUnknownBlocks(t *testing.T) {
	resMap := map[string]*schema.Resource{
		"prov_test": {
			Schema: map[string]*schema.Schema{
				"test": {
					Type:     schema.TypeList,
					Optional: true,
					Elem: &schema.Resource{
						Schema: map[string]*schema.Schema{
							"test_prop": {
								Type:     schema.TypeString,
								Optional: true,
							},
						},
					},
				},
			},
		},
		"prov_nested_test": {
			Schema: map[string]*schema.Schema{
				"test": {
					Type:     schema.TypeList,
					Optional: true,
					Elem: &schema.Resource{
						Schema: map[string]*schema.Schema{
							"nested_prop": {
								Type:     schema.TypeList,
								Optional: true,
								Elem: &schema.Resource{
									Schema: map[string]*schema.Schema{
										"test_prop": {
											Type:     schema.TypeList,
											Optional: true,
											Elem: &schema.Schema{
												Type: schema.TypeString,
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		"prov_aux": {
			Schema: map[string]*schema.Schema{
				"aux": {
					Type:     schema.TypeList,
					Computed: true,
					Optional: true,
					Elem: &schema.Resource{
						Schema: map[string]*schema.Schema{
							"test_prop": {
								Type:     schema.TypeString,
								Computed: true,
								Optional: true,
							},
						},
					},
				},
				"nested_aux": {
					Type:     schema.TypeList,
					Optional: true,
					Computed: true,
					Elem: &schema.Resource{
						Schema: map[string]*schema.Schema{
							"nested_prop": {
								Type:     schema.TypeList,
								Optional: true,
								Computed: true,
								Elem: &schema.Resource{
									Schema: map[string]*schema.Schema{
										"test_prop": {
											Type:     schema.TypeList,
											Optional: true,
											Computed: true,
											Elem: &schema.Schema{
												Type: schema.TypeString,
											},
										},
									},
								},
							},
						},
					},
				},
			},
			CreateContext: func(_ context.Context, d *schema.ResourceData, _ interface{}) diag.Diagnostics {
				d.SetId("aux")
				if d.Get("aux") == nil {
					err := d.Set("aux", []map[string]interface{}{{"test_prop": "aux"}})
					require.NoError(t, err)
				}
				if d.Get("nested_aux") == nil {
					err := d.Set("nested_aux", []map[string]interface{}{
						{
							"nested_prop": []map[string]interface{}{
								{"test_prop": []string{"aux"}},
							},
						},
					})
					require.NoError(t, err)
				}
				return nil
			},
		},
	}
	tfp := &schema.Provider{ResourcesMap: resMap}
	bridgedProvider := pulcheck.BridgedProvider(t, "prov", tfp)

	provTestKnownProgram := `
name: test
runtime: yaml
resources:
    mainRes:
        type: prov:index:Test
        properties:
            tests:
                - testProp: "known_val"
`
	nestedProvTestKnownProgram := `
name: test
runtime: yaml
resources:
    mainRes:
        type: prov:index:NestedTest
        properties:
            tests:
                - nestedProps:
                    - testProps:
                        - "known_val"
`

	for _, tc := range []struct {
		name                string
		program             string
		initialKnownProgram string
		expectedInitial     autogold.Value
		expectedUpdate      autogold.Value
	}{
		{
			"list of objects",
			`
name: test
runtime: yaml
resources:
    auxRes:
        type: prov:index:Aux
        properties:
            auxes: %s
            nestedAuxes: %s
    mainRes:
        type: prov:index:Test
        properties:
            tests: ${auxRes.auxes}
`,
			provTestKnownProgram,
			autogold.Expect(`Previewing update (test):
+ pulumi:pulumi:Stack: (create)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    + prov:index/test:Test: (create)
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
        tests     : output<string>
Resources:
    + 3 to create
`),
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      - tests: [
      -     [0]: {
              - testProp: "known_val"
            }
        ]
      + tests: output<string>
Resources:
    + 1 to create
    ~ 1 to update
    2 changes. 1 unchanged
`),
		},
		{
			"unknown object",
			`
name: test
runtime: yaml
resources:
    auxRes:
        type: prov:index:Aux
        properties:
            auxes: %s
            nestedAuxes: %s
    mainRes:
        type: prov:index:Test
        properties:
            tests:
                - ${auxRes.auxes[0]}
`,
			provTestKnownProgram,

			autogold.Expect(`Previewing update (test):
+ pulumi:pulumi:Stack: (create)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    + prov:index/test:Test: (create)
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
        tests     : [
            [0]: output<string>
        ]
Resources:
    + 3 to create
`),
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [0]: {
                  - testProp: "known_val"
                }
          + [0]: output<string>
        ]
Resources:
    + 1 to create
    ~ 1 to update
    2 changes. 1 unchanged
`),
		},
		{
			"unknown object with others",
			`
name: test
runtime: yaml
resources:
    auxRes:
        type: prov:index:Aux
        properties:
            auxes: %s
            nestedAuxes: %s
    mainRes:
        type: prov:index:Test
        properties:
            tests:
                - ${auxRes.auxes[0]}
                - {"testProp": "val"}
`,
			provTestKnownProgram,

			autogold.Expect(`Previewing update (test):
+ pulumi:pulumi:Stack: (create)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    + prov:index/test:Test: (create)
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
        tests     : [
            [0]: output<string>
            [1]: {
                testProp  : "val"
            }
        ]
Resources:
    + 3 to create
`),
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [0]: {
                  - testProp: "known_val"
                }
          + [0]: output<string>
          + [1]: {
                  + testProp  : "val"
                }
        ]
Resources:
    + 1 to create
    ~ 1 to update
    2 changes. 1 unchanged
`),
		},
		{
			"unknown nested",
			`
name: test
runtime: yaml
resources:
    auxRes:
        type: prov:index:Aux
        properties:
            auxes: %s
            nestedAuxes: %s
    mainRes:
        type: prov:index:NestedTest
        properties:
            tests: ${auxRes.nestedAuxes}
`,
			nestedProvTestKnownProgram,
			autogold.Expect(`Previewing update (test):
+ pulumi:pulumi:Stack: (create)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    + prov:index/nestedTest:NestedTest: (create)
        [urn=urn:pulumi:test::test::prov:index/nestedTest:NestedTest::mainRes]
        tests     : output<string>
Resources:
    + 3 to create
`),
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    ~ prov:index/nestedTest:NestedTest: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/nestedTest:NestedTest::mainRes]
      - tests: [
      -     [0]: {
              - nestedProps: [
              -     [0]: {
                      - testProps: [
                      -     [0]: "known_val"
                        ]
                    }
                ]
            }
        ]
      + tests: output<string>
Resources:
    + 1 to create
    ~ 1 to update
    2 changes. 1 unchanged
`),
		},
		{
			"unknown nested level 1",
			`
name: test
runtime: yaml
resources:
    auxRes:
        type: prov:index:Aux
        properties:
            auxes: %s
            nestedAuxes: %s
    mainRes:
        type: prov:index:NestedTest
        properties:
            tests:
                - ${auxRes.nestedAuxes[0]}
`,
			nestedProvTestKnownProgram,
			autogold.Expect(`Previewing update (test):
+ pulumi:pulumi:Stack: (create)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    + prov:index/nestedTest:NestedTest: (create)
        [urn=urn:pulumi:test::test::prov:index/nestedTest:NestedTest::mainRes]
        tests     : [
            [0]: output<string>
        ]
Resources:
    + 3 to create
`),
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    ~ prov:index/nestedTest:NestedTest: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/nestedTest:NestedTest::mainRes]
      ~ tests: [
          - [0]: {
                  - nestedProps: [
                  -     [0]: {
                          - testProps: [
                          -     [0]: "known_val"
                            ]
                        }
                    ]
                }
          + [0]: output<string>
        ]
Resources:
    + 1 to create
    ~ 1 to update
    2 changes. 1 unchanged
`),
		},
		{
			"unknown nested level 2",
			`
name: test
runtime: yaml
resources:
    auxRes:
        type: prov:index:Aux
        properties:
            auxes: %s
            nestedAuxes: %s
    mainRes:
        type: prov:index:NestedTest
        properties:
            tests:
                - nestedProps: ${auxRes.nestedAuxes[0].nestedProps}
`,
			nestedProvTestKnownProgram,
			autogold.Expect(`Previewing update (test):
+ pulumi:pulumi:Stack: (create)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    + prov:index/nestedTest:NestedTest: (create)
        [urn=urn:pulumi:test::test::prov:index/nestedTest:NestedTest::mainRes]
        tests     : [
            [0]: {
                nestedProps: output<string>
            }
        ]
Resources:
    + 3 to create
`),
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    ~ prov:index/nestedTest:NestedTest: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/nestedTest:NestedTest::mainRes]
      ~ tests: [
          ~ [0]: {
                  - nestedProps: [
                  -     [0]: {
                          - testProps: [
                          -     [0]: "known_val"
                            ]
                        }
                    ]
                  + nestedProps: output<string>
                }
        ]
Resources:
    + 1 to create
    ~ 1 to update
    2 changes. 1 unchanged
`),
		},
		{
			"unknown nested level 3",
			`
name: test
runtime: yaml
resources:
    auxRes:
        type: prov:index:Aux
        properties:
            auxes: %s
            nestedAuxes: %s
    mainRes:
        type: prov:index:NestedTest
        properties:
            tests:
                - nestedProps:
                    - ${auxRes.nestedAuxes[0].nestedProps[0]}
`,
			nestedProvTestKnownProgram,
			autogold.Expect(`Previewing update (test):
+ pulumi:pulumi:Stack: (create)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    + prov:index/nestedTest:NestedTest: (create)
        [urn=urn:pulumi:test::test::prov:index/nestedTest:NestedTest::mainRes]
        tests     : [
            [0]: {
                nestedProps: [
                    [0]: output<string>
                ]
            }
        ]
Resources:
    + 3 to create
`),
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    ~ prov:index/nestedTest:NestedTest: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/nestedTest:NestedTest::mainRes]
      ~ tests: [
          ~ [0]: {
                  ~ nestedProps: [
                      - [0]: {
                              - testProps: [
                              -     [0]: "known_val"
                                ]
                            }
                      + [0]: output<string>
                    ]
                }
        ]
Resources:
    + 1 to create
    ~ 1 to update
    2 changes. 1 unchanged
`),
		},
		{
			"unknown nested level 4",
			`
name: test
runtime: yaml
resources:
    auxRes:
        type: prov:index:Aux
        properties:
            auxes: %s
            nestedAuxes: %s
    mainRes:
        type: prov:index:NestedTest
        properties:
            tests:
                - nestedProps:
                    - testProps: ${auxRes.nestedAuxes[0].nestedProps[0].testProps}
`,
			nestedProvTestKnownProgram,
			autogold.Expect(`Previewing update (test):
+ pulumi:pulumi:Stack: (create)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    + prov:index/nestedTest:NestedTest: (create)
        [urn=urn:pulumi:test::test::prov:index/nestedTest:NestedTest::mainRes]
        tests     : [
            [0]: {
                nestedProps: [
                    [0]: {
                        testProps : output<string>
                    }
                ]
            }
        ]
Resources:
    + 3 to create
`),
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    ~ prov:index/nestedTest:NestedTest: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/nestedTest:NestedTest::mainRes]
      ~ tests: [
          ~ [0]: {
                  ~ nestedProps: [
                      ~ [0]: {
                              - testProps: [
                              -     [0]: "known_val"
                                ]
                              + testProps: output<string>
                            }
                    ]
                }
        ]
Resources:
    + 1 to create
    ~ 1 to update
    2 changes. 1 unchanged
`),
		},
		{
			"unknown nested level 5",
			`
name: test
runtime: yaml
resources:
    auxRes:
        type: prov:index:Aux
        properties:
            auxes: %s
            nestedAuxes: %s
    mainRes:
        type: prov:index:NestedTest
        properties:
            tests:
                - nestedProps:
                    - testProps:
                        - ${auxRes.nestedAuxes[0].nestedProps[0].testProps[0]}
`,
			nestedProvTestKnownProgram,
			autogold.Expect(`Previewing update (test):
+ pulumi:pulumi:Stack: (create)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    + prov:index/nestedTest:NestedTest: (create)
        [urn=urn:pulumi:test::test::prov:index/nestedTest:NestedTest::mainRes]
        tests     : [
            [0]: {
                nestedProps: [
                    [0]: {
                        testProps : [
                            [0]: output<string>
                        ]
                    }
                ]
            }
        ]
Resources:
    + 3 to create
`),
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    ~ prov:index/nestedTest:NestedTest: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/nestedTest:NestedTest::mainRes]
      ~ tests: [
          ~ [0]: {
                  ~ nestedProps: [
                      ~ [0]: {
                              ~ testProps: [
                                  ~ [0]: "known_val" => output<string>
                                ]
                            }
                    ]
                }
        ]
Resources:
    + 1 to create
    ~ 1 to update
    2 changes. 1 unchanged
`),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			computedProgram := fmt.Sprintf(tc.program, "null", "null")

			t.Run("initial preview", func(t *testing.T) {
				pt := pulcheck.PulCheck(t, bridgedProvider, computedProgram)
				res := pt.Preview(optpreview.Diff())
				t.Logf(res.StdOut)

				tc.expectedInitial.Equal(t, res.StdOut)
			})

			t.Run("update preview", func(t *testing.T) {
				t.Skipf("Skipping this test as it this case is not handled by the TF plugin sdk")
				// The TF plugin SDK does not handle removing an input for a computed value, even if the provider implements it.
				// The plugin SDK always fills an empty Computed property with the value from the state.
				// Diff in these cases always returns no diff and the old state value is used.
				nonComputedProgram := fmt.Sprintf(tc.program, "[{testProp: \"val1\"}]", "[{nestedProps: [{testProps: [\"val1\"]}]}]")
				pt := pulcheck.PulCheck(t, bridgedProvider, nonComputedProgram)
				pt.Up()

				pulumiYamlPath := filepath.Join(pt.CurrentStack().Workspace().WorkDir(), "Pulumi.yaml")

				err := os.WriteFile(pulumiYamlPath, []byte(computedProgram), 0o600)
				require.NoError(t, err)

				res := pt.Preview(optpreview.Diff())
				t.Logf(res.StdOut)
				tc.expectedUpdate.Equal(t, res.StdOut)
			})

			t.Run("update preview with computed", func(t *testing.T) {
				pt := pulcheck.PulCheck(t, bridgedProvider, tc.initialKnownProgram)
				pt.Up()

				pulumiYamlPath := filepath.Join(pt.CurrentStack().Workspace().WorkDir(), "Pulumi.yaml")

				err := os.WriteFile(pulumiYamlPath, []byte(computedProgram), 0o600)
				require.NoError(t, err)

				res := pt.Preview(optpreview.Diff())
				t.Logf(res.StdOut)
				tc.expectedUpdate.Equal(t, res.StdOut)
			})
		})
	}
}

func TestFullyComputedNestedAttribute(t *testing.T) {
	resMap := map[string]*schema.Resource{
		"prov_test": {
			Schema: map[string]*schema.Schema{
				"attached_disks": {
					Type:     schema.TypeList,
					Optional: true,
					Elem: &schema.Resource{
						Schema: map[string]*schema.Schema{
							"name": {
								Optional: true,
								Type:     schema.TypeString,
							},
							"key256": {
								Computed: true,
								Type:     schema.TypeString,
							},
						},
					},
				},
				"top_level_computed": {
					Type:     schema.TypeString,
					Computed: true,
				},
			},
		},
	}

	importer := func(val any) func(context.Context, *schema.ResourceData, interface{}) ([]*schema.ResourceData, error) {
		return func(ctx context.Context, rd *schema.ResourceData, i interface{}) ([]*schema.ResourceData, error) {
			elMap := map[string]any{
				"name":   "disk1",
				"key256": val,
			}
			err := rd.Set("attached_disks", []map[string]any{elMap})
			require.NoError(t, err)

			err = rd.Set("top_level_computed", "computed_val")
			require.NoError(t, err)

			return []*schema.ResourceData{rd}, nil
		}
	}
	tfp := &schema.Provider{ResourcesMap: resMap}
	bridgedProvider := pulcheck.BridgedProvider(t, "prov", tfp)

	program := `
name: test
runtime: yaml
`
	for _, tc := range []struct {
		name      string
		importVal any
	}{
		{
			"non-nil",
			"val1",
		},
		{
			"nil",
			nil,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resMap["prov_test"].Importer = &schema.ResourceImporter{
				StateContext: importer(tc.importVal),
			}

			pt := pulcheck.PulCheck(t, bridgedProvider, program)

			res := pt.Import("prov:index/test:Test", "res1", "id1", "")

			t.Logf(res.Stdout)

			require.NotContains(t, res.Stdout, "One or more imported inputs failed to validate")
		})
	}
}

func TestConfigureGetRawConfigDoesNotPanic(t *testing.T) {
	// Regression test for [pulumi/pulumi-terraform-bridge#2262]
	getOkExists := func(d *schema.ResourceData, key string) (interface{}, bool) {
		v := d.GetRawConfig().GetAttr(key)
		if v.IsNull() {
			return nil, false
		}
		return d.Get(key), true
	}
	resMap := map[string]*schema.Resource{
		"prov_test": {
			Schema: map[string]*schema.Schema{
				"test": {
					Type:     schema.TypeString,
					Optional: true,
				},
			},
		},
	}

	runConfigureTest := func(t *testing.T, configPresent bool) {
		tfp := &schema.Provider{
			ResourcesMap: resMap,
			Schema: map[string]*schema.Schema{
				"config": {
					Type:     schema.TypeString,
					Optional: true,
				},
			},
			ConfigureContextFunc: func(ctx context.Context, rd *schema.ResourceData) (interface{}, diag.Diagnostics) {
				_, ok := getOkExists(rd, "config")
				require.Equal(t, configPresent, ok, "Unexpected config value")
				return nil, nil
			},
		}
		bridgedProvider := pulcheck.BridgedProvider(t, "prov", tfp)
		configVal := "val"
		if !configPresent {
			configVal = "null"
		}
		program := fmt.Sprintf(`
name: test
runtime: yaml
resources:
  prov:
    type: pulumi:providers:prov
	defaultProvider: true
	properties:
	  config: %s
  mainRes:
    type: prov:index:Test
	properties:
	  test: "hello"
outputs:
  testOut: ${mainRes.test}
`, configVal)
		pt := pulcheck.PulCheck(t, bridgedProvider, program)
		pt.Up()
	}

	t.Run("config exists", func(t *testing.T) {
		runConfigureTest(t, true)
	})

	t.Run("config does not exist", func(t *testing.T) {
		runConfigureTest(t, false)
	})
}

// TODO[pulumi/pulumi-terraform-bridge#2274]: Move to actual cross-test suite once the plumbing is done
func TestConfigureCrossTest(t *testing.T) {
	resMap := map[string]*schema.Resource{
		"prov_test": {
			Schema: map[string]*schema.Schema{
				"test": {
					Type:     schema.TypeString,
					Optional: true,
				},
			},
		},
	}

	runTest := func(t *testing.T, sch map[string]*schema.Schema, pulumiProgram, tfProgram string) {
		var tfRd *schema.ResourceData
		var puRd *schema.ResourceData
		_ = puRd // ignore unused warning
		tfp := &schema.Provider{
			ResourcesMap: resMap,
			Schema:       sch,
			ConfigureContextFunc: func(ctx context.Context, rd *schema.ResourceData) (interface{}, diag.Diagnostics) {
				if tfRd == nil {
					tfRd = rd
				} else {
					puRd = rd
				}

				return nil, nil
			},
		}

		tfdriver := tfcheck.NewTfDriver(t, t.TempDir(), "prov", tfp)
		tfdriver.Write(t, tfProgram)
		tfdriver.Plan(t)
		require.NotNil(t, tfRd)
		require.Nil(t, puRd)

		bridgedProvider := pulcheck.BridgedProvider(t, "prov", tfp)

		pt := pulcheck.PulCheck(t, bridgedProvider, pulumiProgram)
		pt.Preview()
		require.NotNil(t, puRd)
		require.Equal(t, tfRd.GetRawConfig(), puRd.GetRawConfig())
	}

	t.Run("string attr", func(t *testing.T) {
		runTest(t,
			map[string]*schema.Schema{
				"config": {
					Type:     schema.TypeString,
					Optional: true,
				},
			},
			`
name: test
runtime: yaml
resources:
	prov:
		type: pulumi:providers:prov
		defaultProvider: true
		properties:
			config: val
	mainRes:
		type: prov:index:Test
		properties:
			test: "val"
`,
			`
provider "prov" {
	config = "val"
}

resource "prov_test" "test" {
	test = "val"
}`)
	})

	t.Run("object block", func(t *testing.T) {
		runTest(t,
			map[string]*schema.Schema{
				"config": {
					Type:     schema.TypeList,
					Optional: true,
					Elem: &schema.Resource{
						Schema: map[string]*schema.Schema{
							"prop": {
								Type:     schema.TypeString,
								Optional: true,
							},
						},
					},
					MaxItems: 1,
				},
			},
			`
name: test
runtime: yaml
resources:
	prov:
		type: pulumi:providers:prov
		defaultProvider: true
		properties:
			config: {"prop": "val"}
	mainRes:
		type: prov:index:Test
		properties:
			test: "val"
`,
			`
provider "prov" {
	config {
		prop = "val"
	}
}

resource "prov_test" "test" {
	test = "val"
}`)
	})

	t.Run("list config", func(t *testing.T) {
		runTest(t,
			map[string]*schema.Schema{
				"config": {
					Type:     schema.TypeList,
					Optional: true,
					Elem: &schema.Schema{
						Type: schema.TypeString,
					},
				},
			},
			`
name: test
runtime: yaml
resources:
	prov:
		type: pulumi:providers:prov
		defaultProvider: true
		properties:
			configs: ["val"]
	mainRes:
		type: prov:index:Test
		properties:
			test: "val"
`,
			`
provider "prov" {
	config = ["val"]
}

resource "prov_test" "test" {
	test = "val"
}`)
	})

	t.Run("set config", func(t *testing.T) {
		runTest(t,
			map[string]*schema.Schema{
				"config": {
					Type:     schema.TypeSet,
					Optional: true,
					Elem: &schema.Schema{
						Type: schema.TypeString,
					},
				},
			},
			`
name: test
runtime: yaml
resources:
	prov:
		type: pulumi:providers:prov
		defaultProvider: true
		properties:
			configs: ["val"]
	mainRes:
		type: prov:index:Test
		properties:
			test: "val"
`,
			`
provider "prov" {
	config = ["val"]
}

resource "prov_test" "test" {
	test = "val"
}`)
	})
}

func TestBigIntOverride(t *testing.T) {
	getZoneFromStack := func(data []byte) string {
		var stateMap map[string]interface{}
		err := json.Unmarshal(data, &stateMap)
		require.NoError(t, err)
		resourcesList := stateMap["resources"].([]interface{})
		// stack, provider, resource
		require.Len(t, resourcesList, 3)
		testResState := resourcesList[2].(map[string]interface{})
		resOutputs := testResState["outputs"].(map[string]interface{})
		return resOutputs["managedZoneId"].(string)
	}
	bigInt := 1<<62 + 1
	resMap := map[string]*schema.Resource{
		"prov_test": {
			Schema: map[string]*schema.Schema{
				"prop": {
					Type:     schema.TypeString,
					Optional: true,
				},
				"managed_zone_id": {
					Type:     schema.TypeInt,
					Computed: true,
				},
			},
			CreateContext: func(ctx context.Context, rd *schema.ResourceData, i interface{}) diag.Diagnostics {
				rd.SetId("1")
				err := rd.Set("managed_zone_id", bigInt)
				require.NoError(t, err)
				return nil
			},
			UpdateContext: func(ctx context.Context, rd *schema.ResourceData, i interface{}) diag.Diagnostics {
				require.Equal(t, bigInt, rd.Get("managed_zone_id").(int))
				return nil
			},
			UseJSONNumber: true,
		},
	}

	runTest := func(t *testing.T, PRC bool) {
		tfp := &schema.Provider{ResourcesMap: resMap}
		opts := []pulcheck.BridgedProviderOpt{}
		if !PRC {
			opts = append(opts, pulcheck.DisablePlanResourceChange())
		}
		bridgedProvider := pulcheck.BridgedProvider(t, "prov", tfp, opts...)
		bridgedProvider.Resources["prov_test"] = &tfbridge.ResourceInfo{
			Tok: "prov:index:Test",
			Fields: map[string]*tfbridge.SchemaInfo{
				"managed_zone_id": {
					Type: "string",
				},
			},
		}

		program := `
name: test
runtime: yaml
resources:
    mainRes:
        type: prov:index:Test
        properties:
            prop: %s
`

		pt := pulcheck.PulCheck(t, bridgedProvider, fmt.Sprintf(program, "val"))
		pt.Up()

		// Check the state is correct
		stack := pt.ExportStack()
		data, err := stack.Deployment.MarshalJSON()
		require.NoError(t, err)
		require.Equal(t, fmt.Sprint(bigInt), getZoneFromStack(data))

		program2 := fmt.Sprintf(program, "val2")
		pulumiYamlPath := filepath.Join(pt.CurrentStack().Workspace().WorkDir(), "Pulumi.yaml")
		err = os.WriteFile(pulumiYamlPath, []byte(program2), 0o600)
		require.NoError(t, err)

		pt.Up()
		// Check the state is correct
		stack = pt.ExportStack()
		data, err = stack.Deployment.MarshalJSON()
		require.NoError(t, err)
		require.Equal(t, fmt.Sprint(bigInt), getZoneFromStack(data))
	}

	t.Run("PRC disabled", func(t *testing.T) {
		runTest(t, false)
	})

	t.Run("PRC enabled", func(t *testing.T) {
		runTest(t, true)
	})
}

func TestDetailedDiffPlainTypes(t *testing.T) {
	resMap := map[string]*schema.Resource{
		"prov_test": {
			Schema: map[string]*schema.Schema{
				"string_prop": {
					Type:     schema.TypeString,
					Optional: true,
				},
				"list_prop": {
					Type:     schema.TypeList,
					Optional: true,
					Elem:     &schema.Schema{Type: schema.TypeString},
				},
				"set_prop": {
					Type:     schema.TypeSet,
					Optional: true,
					Elem:     &schema.Schema{Type: schema.TypeString},
				},
				"map_prop": {
					Type:     schema.TypeMap,
					Optional: true,
					Elem:     &schema.Schema{Type: schema.TypeString},
				},
			},
		},
	}
	tfp := &schema.Provider{ResourcesMap: resMap}
	bridgedProvider := pulcheck.BridgedProvider(t, "prov", tfp)

	program := `
name: test
runtime: yaml
resources:
    mainRes:
        type: prov:index:Test
        properties: %s
`

	for _, tc := range []struct {
		name     string
		props1   interface{}
		props2   interface{}
		expected autogold.Value
	}{
		{
			"string unchanged",
			map[string]interface{}{"stringProp": "val"},
			map[string]interface{}{"stringProp": "val"},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
Resources:
    2 unchanged
`),
		},
		{
			"string added",
			map[string]interface{}{},
			map[string]interface{}{"stringProp": "val"},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      + stringProp: "val"
Resources:
    ~ 1 to update
    1 unchanged
`),
		},
		{
			"string removed",
			map[string]interface{}{"stringProp": "val1"},
			map[string]interface{}{},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      - stringProp: "val1"
Resources:
    ~ 1 to update
    1 unchanged
`),
		},
		{
			"string changed",
			map[string]interface{}{"stringProp": "val1"},
			map[string]interface{}{"stringProp": "val2"},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ stringProp: "val1" => "val2"
Resources:
    ~ 1 to update
    1 unchanged
`),
		},
		{
			"list unchanged",
			map[string]interface{}{"listProps": []interface{}{"val"}},
			map[string]interface{}{"listProps": []interface{}{"val"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
Resources:
    2 unchanged
`),
		},
		// TODO[pulumi/pulumi-terraform-bridge#2234]: Duplicated diff
		{
			"list added",
			map[string]interface{}{},
			map[string]interface{}{"listProps": []interface{}{"val"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      + listProps: [
      +     [0]: "val"
        ]
      + listProps: [
      +     [0]: "val"
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
		},
		// TODO[pulumi/pulumi-terraform-bridge#2233]: Missing diff
		{
			"list added empty",
			map[string]interface{}{},
			map[string]interface{}{"listProps": []interface{}{}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
Resources:
    2 unchanged
`),
		},
		// TODO[pulumi/pulumi-terraform-bridge#2234]: Duplicated diff
		{
			"list removed",
			map[string]interface{}{"listProps": []interface{}{"val"}},
			map[string]interface{}{},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      - listProps: [
      -     [0]: "val"
        ]
      - listProps: [
      -     [0]: "val"
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
		},
		// TODO[pulumi/pulumi-terraform-bridge#2233]: Missing diff
		{
			"list removed empty",
			map[string]interface{}{"listProps": []interface{}{}},
			map[string]interface{}{},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
Resources:
    2 unchanged
`),
		},
		{
			"list element added front",
			map[string]interface{}{"listProps": []interface{}{"val2", "val3"}},
			map[string]interface{}{"listProps": []interface{}{"val1", "val2", "val3"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ listProps: [
          ~ [0]: "val2" => "val1"
          ~ [1]: "val3" => "val2"
          + [2]: "val3"
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
		},
		{
			"list element added back",
			map[string]interface{}{"listProps": []interface{}{"val1", "val2"}},
			map[string]interface{}{"listProps": []interface{}{"val1", "val2", "val3"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ listProps: [
            [0]: "val1"
            [1]: "val2"
          + [2]: "val3"
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
		},
		{
			"list element added middle",
			map[string]interface{}{"listProps": []interface{}{"val1", "val3"}},
			map[string]interface{}{"listProps": []interface{}{"val1", "val2", "val3"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ listProps: [
            [0]: "val1"
          ~ [1]: "val3" => "val2"
          + [2]: "val3"
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
		},
		{
			"list element removed front",
			map[string]interface{}{"listProps": []interface{}{"val1", "val2", "val3"}},
			map[string]interface{}{"listProps": []interface{}{"val2", "val3"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ listProps: [
          ~ [0]: "val1" => "val2"
          ~ [1]: "val2" => "val3"
          - [2]: "val3"
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
		},
		{
			"list element removed back",
			map[string]interface{}{"listProps": []interface{}{"val1", "val2", "val3"}},
			map[string]interface{}{"listProps": []interface{}{"val1", "val2"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ listProps: [
            [0]: "val1"
            [1]: "val2"
          - [2]: "val3"
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
		},
		{
			"list element removed middle",
			map[string]interface{}{"listProps": []interface{}{"val1", "val2", "val3"}},
			map[string]interface{}{"listProps": []interface{}{"val1", "val3"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ listProps: [
            [0]: "val1"
          ~ [1]: "val2" => "val3"
          - [2]: "val3"
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
		},
		{
			"list element changed",
			map[string]interface{}{"listProps": []interface{}{"val1"}},
			map[string]interface{}{"listProps": []interface{}{"val2"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ listProps: [
          ~ [0]: "val1" => "val2"
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
		},
		{
			"set unchanged",
			map[string]interface{}{"setProps": []interface{}{"val"}},
			map[string]interface{}{"setProps": []interface{}{"val"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
Resources:
    2 unchanged
`),
		},
		// TODO[pulumi/pulumi-terraform-bridge#2234]: Duplicated diff
		{
			"set added",
			map[string]interface{}{},
			map[string]interface{}{"setProps": []interface{}{"val"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      + setProps: [
      +     [0]: "val"
        ]
      + setProps: [
      +     [0]: "val"
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
		},
		// TODO[pulumi/pulumi-terraform-bridge#2233]: Missing diff
		{
			"set added empty",
			map[string]interface{}{},
			map[string]interface{}{"setProps": []interface{}{}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
Resources:
    2 unchanged
`),
		},
		// TODO[pulumi/pulumi-terraform-bridge#2234]: Duplicated diff
		{
			"set removed",
			map[string]interface{}{"setProps": []interface{}{"val"}},
			map[string]interface{}{},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      - setProps: [
      -     [0]: "val"
        ]
      - setProps: [
      -     [0]: "val"
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
		},
		// TODO[pulumi/pulumi-terraform-bridge#2233]: Missing diff
		{
			"set removed empty",
			map[string]interface{}{"setProps": []interface{}{}},
			map[string]interface{}{},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
Resources:
    2 unchanged
`),
		},
		// TODO[pulumi/pulumi-terraform-bridge#2235]: Wrong number of additions
		{
			"set element added front",
			map[string]interface{}{"setProps": []interface{}{"val2", "val3"}},
			map[string]interface{}{"setProps": []interface{}{"val1", "val2", "val3"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ setProps: [
          + [0]: "val1"
          ~ [1]: "val3" => "val2"
          + [2]: "val3"
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
		},
		{
			"set element added back",
			map[string]interface{}{"setProps": []interface{}{"val1", "val2"}},
			map[string]interface{}{"setProps": []interface{}{"val1", "val2", "val3"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ setProps: [
            [0]: "val1"
            [1]: "val2"
          + [2]: "val3"
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
		},
		{
			"set element added middle",
			map[string]interface{}{"setProps": []interface{}{"val1", "val3"}},
			map[string]interface{}{"setProps": []interface{}{"val1", "val2", "val3"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ setProps: [
            [0]: "val1"
          ~ [1]: "val3" => "val2"
          + [2]: "val3"
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
		},
		{
			"set element removed front",
			map[string]interface{}{"setProps": []interface{}{"val1", "val2", "val3"}},
			map[string]interface{}{"setProps": []interface{}{"val2", "val3"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ setProps: [
          ~ [0]: "val1" => "val2"
          ~ [1]: "val2" => "val3"
          - [2]: "val3"
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
		},
		{
			"set element removed back",
			map[string]interface{}{"setProps": []interface{}{"val1", "val2", "val3"}},
			map[string]interface{}{"setProps": []interface{}{"val1", "val2"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ setProps: [
            [0]: "val1"
            [1]: "val2"
          - [2]: "val3"
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
		},
		// TODO[pulumi/pulumi-terraform-bridge#2235]: Wrong number of removals
		{
			"set element removed middle",
			map[string]interface{}{"setProps": []interface{}{"val1", "val2", "val3"}},
			map[string]interface{}{"setProps": []interface{}{"val1", "val3"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ setProps: [
            [0]: "val1"
          - [1]: "val2"
          - [2]: "val3"
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
		},
		{
			"set element changed",
			map[string]interface{}{"setProps": []interface{}{"val1"}},
			map[string]interface{}{"setProps": []interface{}{"val2"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ setProps: [
          ~ [0]: "val1" => "val2"
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
		},
		{
			"map unchanged",
			map[string]interface{}{"mapProp": map[string]interface{}{"key": "val"}},
			map[string]interface{}{"mapProp": map[string]interface{}{"key": "val"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
Resources:
    2 unchanged
`),
		},
		// TODO[pulumi/pulumi-terraform-bridge#2234]: Duplicated diff
		{
			"map added",
			map[string]interface{}{},
			map[string]interface{}{"mapProp": map[string]interface{}{"key": "val"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      + mapProp: {
          + key: "val"
        }
      + mapProp: {
          + key: "val"
        }
Resources:
    ~ 1 to update
    1 unchanged
`),
		},
		// TODO[pulumi/pulumi-terraform-bridge#2233]: Missing diff
		{
			"map added empty",
			map[string]interface{}{},
			map[string]interface{}{"mapProp": map[string]interface{}{}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
Resources:
    2 unchanged
`),
		},
		// TODO[pulumi/pulumi-terraform-bridge#2234]: Duplicated diff
		{
			"map removed",
			map[string]interface{}{"mapProp": map[string]interface{}{"key": "val"}},
			map[string]interface{}{},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      - mapProp: {
          - key: "val"
        }
      - mapProp: {
          - key: "val"
        }
Resources:
    ~ 1 to update
    1 unchanged
`),
		},
		// TODO[pulumi/pulumi-terraform-bridge#2233]: Missing diff
		{
			"map removed empty",
			map[string]interface{}{"mapProp": map[string]interface{}{}},
			map[string]interface{}{},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
Resources:
    2 unchanged
`),
		},
		// TODO[pulumi/pulumi-terraform-bridge#2234]: Duplicated diff
		{
			"map element added",
			map[string]interface{}{"mapProp": map[string]interface{}{}},
			map[string]interface{}{"mapProp": map[string]interface{}{"key": "val"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      + mapProp: {
          + key: "val"
        }
      + mapProp: {
          + key: "val"
        }
Resources:
    ~ 1 to update
    1 unchanged
`),
		},
		{
			"map element removed",
			map[string]interface{}{"mapProp": map[string]interface{}{"key": "val"}},
			map[string]interface{}{"mapProp": map[string]interface{}{}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ mapProp: {
          - key: "val"
        }
Resources:
    ~ 1 to update
    1 unchanged
`),
		},
		{
			"map value changed",
			map[string]interface{}{"mapProp": map[string]interface{}{"key": "val1"}},
			map[string]interface{}{"mapProp": map[string]interface{}{"key": "val2"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ mapProp: {
          ~ key: "val1" => "val2"
        }
Resources:
    ~ 1 to update
    1 unchanged
`),
		},
		{
			"map key changed",
			map[string]interface{}{"mapProp": map[string]interface{}{"key1": "val"}},
			map[string]interface{}{"mapProp": map[string]interface{}{"key2": "val"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ mapProp: {
          - key1: "val"
          + key2: "val"
        }
Resources:
    ~ 1 to update
    1 unchanged
`),
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			props1, err := json.Marshal(tc.props1)
			require.NoError(t, err)
			program1 := fmt.Sprintf(program, string(props1))
			props2, err := json.Marshal(tc.props2)
			require.NoError(t, err)
			program2 := fmt.Sprintf(program, string(props2))
			pt := pulcheck.PulCheck(t, bridgedProvider, program1)
			pt.Up()

			pulumiYamlPath := filepath.Join(pt.CurrentStack().Workspace().WorkDir(), "Pulumi.yaml")

			err = os.WriteFile(pulumiYamlPath, []byte(program2), 0o600)
			require.NoError(t, err)

			res := pt.Preview(optpreview.Diff())
			t.Logf(res.StdOut)
			tc.expected.Equal(t, res.StdOut)
		})
	}
}
