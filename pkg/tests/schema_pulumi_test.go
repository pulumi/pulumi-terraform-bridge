package tests

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hexops/autogold/v2"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tests/pulcheck"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tests/tfcheck"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	shimv2 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v2"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optpreview"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optrefresh"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
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
	res := pt.Up(t)
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
	res := pt.Preview(t, optpreview.Diff())
	// Test that the test property is unknown at preview time
	require.Contains(t, res.StdOut, "test      : output<string>")
	resUp := pt.Up(t)
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
				upRes := pt.Up(t)
				require.Equal(t, tc.expectedOutputTopLevel, upRes.Outputs["collectionOutput"].Value)
				res, err := pt.CurrentStack().Refresh(pt.Context(), optrefresh.ExpectNoChanges())
				require.NoError(t, err)
				t.Log(res.StdOut)
				prevRes, err := pt.CurrentStack().Preview(pt.Context(), optpreview.ExpectNoChanges(), optpreview.Diff())
				require.NoError(t, err)
				t.Log(prevRes.StdOut)
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
				upRes := pt.Up(t)
				require.Equal(t, tc.expectedOutputNested, upRes.Outputs["collectionOutput"].Value)

				res, err := pt.CurrentStack().Refresh(pt.Context(), optrefresh.ExpectNoChanges())
				require.NoError(t, err)
				t.Log(res.StdOut)
				prevRes, err := pt.CurrentStack().Preview(pt.Context(), optpreview.ExpectNoChanges(), optpreview.Diff())
				require.NoError(t, err)
				t.Log(prevRes.StdOut)
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
				res := pt.Preview(t, optpreview.Diff())
				t.Log(res.StdOut)

				tc.expectedInitial.Equal(t, res.StdOut)
			})

			t.Run("update preview", func(t *testing.T) {
				t.Skipf("Skipping this test as it this case is not handled by the TF plugin sdk")
				// The TF plugin SDK does not handle removing an input for a computed value, even if the provider implements it.
				// The plugin SDK always fills an empty Computed property with the value from the state.
				// Diff in these cases always returns no diff and the old state value is used.
				nonComputedProgram := fmt.Sprintf(tc.program, "[{testProp: \"val1\"}]", "[{nestedProps: [{testProps: [\"val1\"]}]}]")
				pt := pulcheck.PulCheck(t, bridgedProvider, nonComputedProgram)
				pt.Up(t)

				pulumiYamlPath := filepath.Join(pt.CurrentStack().Workspace().WorkDir(), "Pulumi.yaml")

				err := os.WriteFile(pulumiYamlPath, []byte(computedProgram), 0o600)
				require.NoError(t, err)

				res := pt.Preview(t, optpreview.Diff())
				t.Log(res.StdOut)
				tc.expectedUpdate.Equal(t, res.StdOut)
			})

			t.Run("update preview with computed", func(t *testing.T) {
				pt := pulcheck.PulCheck(t, bridgedProvider, tc.initialKnownProgram)
				pt.Up(t)

				pulumiYamlPath := filepath.Join(pt.CurrentStack().Workspace().WorkDir(), "Pulumi.yaml")

				err := os.WriteFile(pulumiYamlPath, []byte(computedProgram), 0o600)
				require.NoError(t, err)

				res := pt.Preview(t, optpreview.Diff())
				t.Log(res.StdOut)
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

			res := pt.Import(t, "prov:index/test:Test", "res1", "id1", "")

			t.Log(res.Stdout)

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
		pt.Up(t)
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
		_, err := tfdriver.Plan(t)
		require.NoError(t, err)
		require.NotNil(t, tfRd)
		require.Nil(t, puRd)

		bridgedProvider := pulcheck.BridgedProvider(t, "prov", tfp)

		pt := pulcheck.PulCheck(t, bridgedProvider, pulumiProgram)
		pt.Preview(t)
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
		pt.Up(t)

		// Check the state is correct
		stack := pt.ExportStack(t)
		data, err := stack.Deployment.MarshalJSON()
		require.NoError(t, err)
		require.Equal(t, fmt.Sprint(bigInt), getZoneFromStack(data))

		program2 := fmt.Sprintf(program, "val2")
		pulumiYamlPath := filepath.Join(pt.CurrentStack().Workspace().WorkDir(), "Pulumi.yaml")
		err = os.WriteFile(pulumiYamlPath, []byte(program2), 0o600)
		require.NoError(t, err)

		pt.Up(t)
		// Check the state is correct
		stack = pt.ExportStack(t)
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
	// TODO: Remove this once accurate bridge previews are rolled out
	t.Setenv("PULUMI_TF_BRIDGE_ACCURATE_BRIDGE_PREVIEW", "true")
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
				"list_block": {
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
				},
				"set_block": {
					Type:     schema.TypeSet,
					Optional: true,
					Elem: &schema.Resource{
						Schema: map[string]*schema.Schema{
							"prop": {
								Type:     schema.TypeString,
								Optional: true,
							},
						},
					},
				},
				"max_items_one_block": {
					Type:     schema.TypeList,
					Optional: true,
					MaxItems: 1,
					Elem: &schema.Resource{
						Schema: map[string]*schema.Schema{
							"prop": {
								Type:     schema.TypeString,
								Optional: true,
							},
						},
					},
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
Resources:
    ~ 1 to update
    1 unchanged
`),
		},
		// pulumi/pulumi-terraform-bridge#2233: This is the intended behavior
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
Resources:
    ~ 1 to update
    1 unchanged
`),
		},
		// pulumi/pulumi-terraform-bridge#2233: This is the intended behavior
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
Resources:
    ~ 1 to update
    1 unchanged
`),
		},
		// pulumi/pulumi-terraform-bridge#2233: This is the intended behavior
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
Resources:
    ~ 1 to update
    1 unchanged
`),
		},
		// pulumi/pulumi-terraform-bridge#2233: This is the intended behavior
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
          + [1]: "val2"
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
          - [0]: "val1"
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
          - [2]: "val3"
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
		},
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
          - [1]: "val2"
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
Resources:
    ~ 1 to update
    1 unchanged
`),
		},
		// pulumi/pulumi-terraform-bridge#2233: This is the intended behavior
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
Resources:
    ~ 1 to update
    1 unchanged
`),
		},
		// pulumi/pulumi-terraform-bridge#2233: This is the intended behavior
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
		{
			"list block unchanged",
			map[string]interface{}{"listBlocks": []interface{}{map[string]interface{}{"prop": "val"}}},
			map[string]interface{}{"listBlocks": []interface{}{map[string]interface{}{"prop": "val"}}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
Resources:
    2 unchanged
`),
		},
		{
			"list block added",
			map[string]interface{}{},
			map[string]interface{}{"listBlocks": []interface{}{map[string]interface{}{"prop": "val"}}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      + listBlocks: [
      +     [0]: {
              + prop      : "val"
            }
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
		},
		// This is expected to be a no-op because blocks can not be nil in TF
		{
			"list block added empty",
			map[string]interface{}{},
			map[string]interface{}{"listBlocks": []interface{}{}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
Resources:
    2 unchanged
`),
		},
		{
			"list block added empty object",
			map[string]interface{}{},
			map[string]interface{}{"listBlocks": []interface{}{map[string]interface{}{}}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      + listBlocks: [
      +     [0]: {
            }
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
		},
		{
			"list block removed",
			map[string]interface{}{"listBlocks": []interface{}{map[string]interface{}{"prop": "val"}}},
			map[string]interface{}{},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      - listBlocks: [
      -     [0]: {
              - prop: "val"
            }
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
		},
		// This is expected to be a no-op because blocks can not be nil in TF
		{
			"list block removed empty",
			map[string]interface{}{"listBlocks": []interface{}{}},
			map[string]interface{}{},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
Resources:
    2 unchanged
`),
		},
		{
			"list block removed empty object",
			map[string]interface{}{"listBlocks": []interface{}{map[string]interface{}{}}},
			map[string]interface{}{},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      - listBlocks: [
      -     [0]: {
              - prop: <null>
            }
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
		},
		{
			"list block element added front",
			map[string]interface{}{"listBlocks": []interface{}{
				map[string]interface{}{"prop": "val2"},
				map[string]interface{}{"prop": "val3"},
			}},
			map[string]interface{}{"listBlocks": []interface{}{
				map[string]interface{}{"prop": "val1"},
				map[string]interface{}{"prop": "val2"},
				map[string]interface{}{"prop": "val3"},
			}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ listBlocks: [
          ~ [0]: {
                  ~ prop: "val2" => "val1"
                }
          ~ [1]: {
                  ~ prop: "val3" => "val2"
                }
          + [2]: {
                  + prop      : "val3"
                }
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
		},
		{
			"list block element added back",
			map[string]interface{}{"listBlocks": []interface{}{
				map[string]interface{}{"prop": "val1"},
				map[string]interface{}{"prop": "val2"},
			}},
			map[string]interface{}{"listBlocks": []interface{}{
				map[string]interface{}{"prop": "val1"},
				map[string]interface{}{"prop": "val2"},
				map[string]interface{}{"prop": "val3"},
			}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ listBlocks: [
          + [2]: {
                  + prop      : "val3"
                }
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
		},
		{
			"list block element added middle",
			map[string]interface{}{"listBlocks": []interface{}{
				map[string]interface{}{"prop": "val1"},
				map[string]interface{}{"prop": "val3"},
			}},
			map[string]interface{}{"listBlocks": []interface{}{
				map[string]interface{}{"prop": "val1"},
				map[string]interface{}{"prop": "val2"},
				map[string]interface{}{"prop": "val3"},
			}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ listBlocks: [
          ~ [1]: {
                  ~ prop: "val3" => "val2"
                }
          + [2]: {
                  + prop      : "val3"
                }
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
		},
		{
			"list block element removed front",
			map[string]interface{}{"listBlocks": []interface{}{
				map[string]interface{}{"prop": "val1"},
				map[string]interface{}{"prop": "val2"},
				map[string]interface{}{"prop": "val3"},
			}},
			map[string]interface{}{"listBlocks": []interface{}{
				map[string]interface{}{"prop": "val2"},
				map[string]interface{}{"prop": "val3"},
			}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ listBlocks: [
          ~ [0]: {
                  ~ prop: "val1" => "val2"
                }
          ~ [1]: {
                  ~ prop: "val2" => "val3"
                }
          - [2]: {
                  - prop: "val3"
                }
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
		},
		{
			"list block element removed back",
			map[string]interface{}{"listBlocks": []interface{}{
				map[string]interface{}{"prop": "val1"},
				map[string]interface{}{"prop": "val2"},
				map[string]interface{}{"prop": "val3"},
			}},
			map[string]interface{}{"listBlocks": []interface{}{
				map[string]interface{}{"prop": "val1"},
				map[string]interface{}{"prop": "val2"},
			}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ listBlocks: [
          - [2]: {
                  - prop: "val3"
                }
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
		},
		{
			"list block element removed middle",
			map[string]interface{}{"listBlocks": []interface{}{
				map[string]interface{}{"prop": "val1"},
				map[string]interface{}{"prop": "val2"},
				map[string]interface{}{"prop": "val3"},
			}},
			map[string]interface{}{"listBlocks": []interface{}{
				map[string]interface{}{"prop": "val1"},
				map[string]interface{}{"prop": "val3"},
			}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ listBlocks: [
          ~ [1]: {
                  ~ prop: "val2" => "val3"
                }
          - [2]: {
                  - prop: "val3"
                }
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
		},
		{
			"list block element changed",
			map[string]interface{}{"listBlocks": []interface{}{
				map[string]interface{}{"prop": "val1"},
			}},
			map[string]interface{}{"listBlocks": []interface{}{
				map[string]interface{}{"prop": "val2"},
			}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ listBlocks: [
          ~ [0]: {
                  ~ prop: "val1" => "val2"
                }
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
		},
		{
			"set block unchanged",
			map[string]interface{}{"setBlocks": []interface{}{map[string]interface{}{"prop": "val"}}},
			map[string]interface{}{"setBlocks": []interface{}{map[string]interface{}{"prop": "val"}}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
Resources:
    2 unchanged
`),
		},
		{
			"set block added",
			map[string]interface{}{},
			map[string]interface{}{"setBlocks": []interface{}{map[string]interface{}{"prop": "val"}}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      + setBlocks: [
      +     [0]: {
              + prop      : "val"
            }
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
		},
		// This is expected to be a no-op because blocks can not be nil in TF
		{
			"set block added empty",
			map[string]interface{}{},
			map[string]interface{}{"setBlocks": []interface{}{}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
Resources:
    2 unchanged
`),
		},
		{
			"set block added empty object",
			map[string]interface{}{},
			map[string]interface{}{"setBlocks": []interface{}{map[string]interface{}{}}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      + setBlocks: [
      +     [0]: {
            }
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
		},
		{
			"set block removed",
			map[string]interface{}{"setBlocks": []interface{}{map[string]interface{}{"prop": "val"}}},
			map[string]interface{}{},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      - setBlocks: [
      -     [0]: {
              - prop: "val"
            }
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
		},
		// This is expected to be a no-op because blocks can not be nil in TF
		{
			"set block removed empty",
			map[string]interface{}{"setBlocks": []interface{}{}},
			map[string]interface{}{},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
Resources:
    2 unchanged
`),
		},
		// TODO[pulumi/pulumi-terraform-bridge#2399] nested prop diff
		{
			"set block removed empty object",
			map[string]interface{}{"setBlocks": []interface{}{map[string]interface{}{}}},
			map[string]interface{}{},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      - setBlocks: [
      -     [0]: {
              - prop: ""
            }
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
		},
		{
			"set block element added front",
			map[string]interface{}{"setBlocks": []interface{}{
				map[string]interface{}{"prop": "val2"},
				map[string]interface{}{"prop": "val3"},
			}},
			map[string]interface{}{"setBlocks": []interface{}{
				map[string]interface{}{"prop": "val1"},
				map[string]interface{}{"prop": "val2"},
				map[string]interface{}{"prop": "val3"},
			}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ setBlocks: [
          + [0]: {
                  + prop      : "val1"
                }
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
		},
		{
			"set block element added back",
			map[string]interface{}{"setBlocks": []interface{}{
				map[string]interface{}{"prop": "val1"},
				map[string]interface{}{"prop": "val2"},
			}},
			map[string]interface{}{"setBlocks": []interface{}{
				map[string]interface{}{"prop": "val1"},
				map[string]interface{}{"prop": "val2"},
				map[string]interface{}{"prop": "val3"},
			}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ setBlocks: [
          + [2]: {
                  + prop      : "val3"
                }
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
		},
		{
			"set block element added middle",
			map[string]interface{}{"setBlocks": []interface{}{
				map[string]interface{}{"prop": "val1"},
				map[string]interface{}{"prop": "val3"},
			}},
			map[string]interface{}{"setBlocks": []interface{}{
				map[string]interface{}{"prop": "val1"},
				map[string]interface{}{"prop": "val2"},
				map[string]interface{}{"prop": "val3"},
			}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ setBlocks: [
          + [1]: {
                  + prop      : "val2"
                }
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
		},
		{
			"set block element removed front",
			map[string]interface{}{"setBlocks": []interface{}{
				map[string]interface{}{"prop": "val1"},
				map[string]interface{}{"prop": "val2"},
				map[string]interface{}{"prop": "val3"},
			}},
			map[string]interface{}{"setBlocks": []interface{}{
				map[string]interface{}{"prop": "val2"},
				map[string]interface{}{"prop": "val3"},
			}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ setBlocks: [
          - [0]: {
                  - prop: "val1"
                }
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
		},
		{
			"set block element removed back",
			map[string]interface{}{"setBlocks": []interface{}{
				map[string]interface{}{"prop": "val1"},
				map[string]interface{}{"prop": "val2"},
				map[string]interface{}{"prop": "val3"},
			}},
			map[string]interface{}{"setBlocks": []interface{}{
				map[string]interface{}{"prop": "val1"},
				map[string]interface{}{"prop": "val2"},
			}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ setBlocks: [
          - [2]: {
                  - prop: "val3"
                }
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
		},
		{
			"set block element removed middle",
			map[string]interface{}{"setBlocks": []interface{}{
				map[string]interface{}{"prop": "val1"},
				map[string]interface{}{"prop": "val2"},
				map[string]interface{}{"prop": "val3"},
			}},
			map[string]interface{}{"setBlocks": []interface{}{
				map[string]interface{}{"prop": "val1"},
				map[string]interface{}{"prop": "val3"},
			}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ setBlocks: [
          - [1]: {
                  - prop: "val2"
                }
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
		},
		{
			"set block element changed",
			map[string]interface{}{"setBlocks": []interface{}{
				map[string]interface{}{"prop": "val1"},
			}},
			map[string]interface{}{"setBlocks": []interface{}{
				map[string]interface{}{"prop": "val2"},
			}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ setBlocks: [
          ~ [0]: {
                  ~ prop: "val1" => "val2"
                }
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
		},
		{
			"maxItemsOne block unchanged",
			map[string]interface{}{"maxItemsOneBlock": map[string]interface{}{"prop": "val"}},
			map[string]interface{}{"maxItemsOneBlock": map[string]interface{}{"prop": "val"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
Resources:
    2 unchanged
`),
		},
		{
			"maxItemsOne block added",
			map[string]interface{}{},
			map[string]interface{}{"maxItemsOneBlock": map[string]interface{}{"prop": "val"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      + maxItemsOneBlock: {
          + prop      : "val"
        }
Resources:
    ~ 1 to update
    1 unchanged
`),
		},
		{
			"maxItemsOne block added empty",
			map[string]interface{}{},
			map[string]interface{}{"maxItemsOneBlock": map[string]interface{}{}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      + maxItemsOneBlock: {
        }
Resources:
    ~ 1 to update
    1 unchanged
`),
		},
		{
			"maxItemsOne block removed",
			map[string]interface{}{"maxItemsOneBlock": map[string]interface{}{"prop": "val"}},
			map[string]interface{}{},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      - maxItemsOneBlock: {
          - prop: "val"
        }
Resources:
    ~ 1 to update
    1 unchanged
`),
		},
		// TODO[pulumi/pulumi-terraform-bridge#2399] nested prop diff
		{
			"maxItemsOne block removed empty",
			map[string]interface{}{"maxItemsOneBlock": map[string]interface{}{}},
			map[string]interface{}{},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      - maxItemsOneBlock: {
          - prop: <null>
        }
Resources:
    ~ 1 to update
    1 unchanged
`),
		},
		{
			"maxItemsOne block changed",
			map[string]interface{}{"maxItemsOneBlock": map[string]interface{}{"prop": "val1"}},
			map[string]interface{}{"maxItemsOneBlock": map[string]interface{}{"prop": "val2"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ maxItemsOneBlock: {
          ~ prop: "val1" => "val2"
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
			pt.Up(t)

			pulumiYamlPath := filepath.Join(pt.CurrentStack().Workspace().WorkDir(), "Pulumi.yaml")

			err = os.WriteFile(pulumiYamlPath, []byte(program2), 0o600)
			require.NoError(t, err)

			res := pt.Preview(t, optpreview.Diff())
			t.Log(res.StdOut)
			tc.expected.Equal(t, res.StdOut)
		})
	}
}

func TestFailedValidatorOnReadHandling(t *testing.T) {
	type PulumiResources struct {
		Type       string                 `yaml:"type"`
		Properties map[string]interface{} `yaml:"properties"`
	}
	type PulumiYaml struct {
		Runtime   string                     `yaml:"runtime,omitempty"`
		Name      string                     `yaml:"name,omitempty"`
		Resources map[string]PulumiResources `yaml:"resources"`
	}

	tests := []struct {
		name          string
		schema        schema.Schema
		cloudVal      interface{}
		expectedProps map[string]interface{}
		expectFailure bool
	}{
		{
			name:     "TypeString no validate",
			cloudVal: "ABC",
			schema: schema.Schema{
				Type:     schema.TypeString,
				Computed: true,
				Optional: true,
			},
			expectedProps: map[string]interface{}{
				// input not dropped
				"collectionProp": "ABC",
			},
		},
		{
			name:     "TypeString ValidateFunc does not error",
			cloudVal: "ABC",
			schema: schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
				ValidateFunc: func(i interface{}, s string) ([]string, []error) {
					return []string{}, []error{}
				},
			},
			expectedProps: map[string]interface{}{
				// input not dropped
				"collectionProp": "ABC",
			},
		},
		{
			name:     "TypeString ValidateDiagFunc does not error",
			cloudVal: "ABC",
			schema: schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
				ValidateDiagFunc: func(i interface{}, p cty.Path) diag.Diagnostics {
					return nil
				},
			},
			expectedProps: map[string]interface{}{
				// input not dropped
				"collectionProp": "ABC",
			},
		},
		{
			name:     "TypeString ValidateDiagFunc returns error",
			cloudVal: "ABC",
			schema: schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
				ValidateDiagFunc: func(i interface{}, p cty.Path) diag.Diagnostics {
					return diag.Errorf("Error")
				},
			},
			// input dropped
			expectedProps: map[string]interface{}{},
		},
		{
			name: "TypeMap ValidateDiagFunc returns error",
			cloudVal: map[string]string{
				"nested_prop":       "ABC",
				"nested_other_prop": "value",
			},
			schema: schema.Schema{
				Type:     schema.TypeMap,
				Optional: true,
				Computed: true,
				ValidateDiagFunc: func(i interface{}, p cty.Path) diag.Diagnostics {
					return diag.Errorf("Error")
				},
				Elem: &schema.Schema{
					Type:     schema.TypeString,
					Optional: true,
				},
			},
			// input dropped
			expectedProps: map[string]interface{}{},
		},
		{
			name: "Non-Computed TypeMap ValidateDiagFunc does not drop",
			cloudVal: map[string]string{
				"nested_prop":       "ABC",
				"nested_other_prop": "value",
			},
			schema: schema.Schema{
				Type:     schema.TypeMap,
				Optional: true,
				Computed: false,
				ValidateDiagFunc: func(i interface{}, p cty.Path) diag.Diagnostics {
					return diag.Errorf("Error")
				},
				Elem: &schema.Schema{
					Type:     schema.TypeString,
					Optional: true,
				},
			},
			// input not dropped
			expectedProps: map[string]interface{}{
				"collectionProp": map[string]interface{}{
					"nested_prop":       "ABC",
					"nested_other_prop": "value",
				},
			},
			// we don't drop computed: false attributes, so they will
			// still fail
			expectFailure: true,
		},
		{
			name: "Required TypeMap ValidateDiagFunc does not drop",
			cloudVal: map[string]string{
				"nested_prop":       "ABC",
				"nested_other_prop": "value",
			},
			schema: schema.Schema{
				Type:     schema.TypeMap,
				Required: true,
				ValidateDiagFunc: func(i interface{}, p cty.Path) diag.Diagnostics {
					return diag.Errorf("Error")
				},
				Elem: &schema.Schema{
					Type:     schema.TypeString,
					Optional: true,
				},
			},
			expectedProps: map[string]interface{}{
				"collectionProp": map[string]interface{}{
					"nested_prop":       "ABC",
					"nested_other_prop": "value",
				},
			},
			expectFailure: true,
		},
		{
			name:     "TypeString ValidateFunc returns error",
			cloudVal: "ABC",
			schema: schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
				ValidateFunc: func(i interface{}, s string) ([]string, []error) {
					return []string{}, []error{errors.New("Error")}
				},
			},
			// input dropped
			expectedProps: map[string]interface{}{},
		},
		{
			name:     "TypeString ValidateFunc does not drop required fields",
			cloudVal: "ABC",
			schema: schema.Schema{
				Type:     schema.TypeString,
				Required: true,
				ValidateFunc: func(i interface{}, s string) ([]string, []error) {
					return []string{}, []error{errors.New("Error")}
				},
			},
			expectedProps: map[string]interface{}{
				// input not dropped
				"collectionProp": "ABC",
			},
			expectFailure: true,
		},
		{
			name: "TypeSet ValidateDiagFunc returns error",
			cloudVal: []interface{}{
				"ABC", "value",
			},
			schema: schema.Schema{
				Type:     schema.TypeSet,
				Optional: true,
				Computed: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
					ValidateDiagFunc: func(i interface{}, p cty.Path) diag.Diagnostics {
						if val, ok := i.(string); ok && val != "ABC" {
							return diag.Errorf("Error")
						}
						return nil
					},
				},
			},
			// if one element of the list fails validation
			// the entire list is removed. Terraform does not return
			// list indexes as part of the diagnostic attribute path
			expectedProps: map[string]interface{}{},
		},

		// ValidateDiagFunc & ValidateFunc are not supported for TypeList & TypeSet, but they
		// are supported on the nested elements. For now we are not processing the results of those with `schema.Resource` elements
		// since it can get complicated. Nothing will get dropped and the validation error will pass through
		{
			name: "TypeList do not validate nested fields",
			cloudVal: []interface{}{
				map[string]interface{}{
					"nested_prop":       "ABC",
					"nested_other_prop": "ABC",
				},
			},
			schema: schema.Schema{
				Type:     schema.TypeList,
				Required: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"nested_prop": {
							Type:     schema.TypeString,
							Optional: true,
							ValidateDiagFunc: func(i interface{}, p cty.Path) diag.Diagnostics {
								return diag.Errorf("Error")
							},
						},
						"nested_other_prop": {
							Type:     schema.TypeString,
							Optional: true,
							ValidateDiagFunc: func(i interface{}, p cty.Path) diag.Diagnostics {
								return nil
							},
						},
					},
				},
			},
			expectedProps: map[string]interface{}{
				"collectionProp": map[string]interface{}{
					"nestedOtherProp": "ABC",
					"nestedProp":      "ABC",
				},
			},
			expectFailure: true,
		},
		{
			name: "TypeSet Do not validate nested fields",
			cloudVal: []interface{}{
				map[string]interface{}{
					"nested_prop": "ABC",
				},
			},
			schema: schema.Schema{
				Type:     schema.TypeSet,
				Required: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"nested_prop": {
							Type:     schema.TypeString,
							Required: true,
							ValidateFunc: func(i interface{}, s string) ([]string, []error) {
								return []string{}, []error{errors.New("Error")}
							},
						},
					},
				},
			},
			expectedProps: map[string]interface{}{
				"collectionProps": []interface{}{
					map[string]interface{}{
						"nestedProp": "ABC",
					},
				},
			},
			expectFailure: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			resMap := map[string]*schema.Resource{
				"prov_test": {
					Schema: map[string]*schema.Schema{
						"collection_prop": &tc.schema,
						"other_prop": {
							Type:     schema.TypeString,
							Optional: true,
						},
					},
					Importer: &schema.ResourceImporter{
						StateContext: func(ctx context.Context, rd *schema.ResourceData, i interface{}) ([]*schema.ResourceData, error) {
							err := rd.Set("collection_prop", tc.cloudVal)
							assert.NoError(t, err)
							err = rd.Set("other_prop", "test")
							assert.NoError(t, err)
							return []*schema.ResourceData{rd}, nil
						},
					},
					ReadContext: func(ctx context.Context, rd *schema.ResourceData, i interface{}) diag.Diagnostics {
						err := rd.Set("collection_prop", tc.cloudVal)
						assert.NoError(t, err)
						err = rd.Set("other_prop", "test")
						assert.NoError(t, err)
						return nil
					},
				},
			}
			tfp := &schema.Provider{ResourcesMap: resMap}
			bridgedProvider := pulcheck.BridgedProvider(t, "prov", tfp, pulcheck.DisablePlanResourceChange())
			program := `
name: test
runtime: yaml
`
			pt := pulcheck.PulCheck(t, bridgedProvider, program)
			outPath := filepath.Join(pt.CurrentStack().Workspace().WorkDir(), "out.yaml")

			imp := pt.Import(t, "prov:index/test:Test", "mainRes", "mainRes", "", "--out", outPath)
			tc.expectedProps["otherProp"] = "test"

			contents, err := os.ReadFile(outPath)
			assert.NoError(t, err)
			expected := PulumiYaml{
				Resources: map[string]PulumiResources{
					"mainRes": {
						Type:       "prov:Test",
						Properties: tc.expectedProps,
					},
				},
			}
			var actual PulumiYaml
			err = yaml.Unmarshal(contents, &actual)
			assert.NoError(t, err)

			assert.Equal(t, expected, actual)
			if tc.expectFailure {
				assert.Contains(t, imp.Stdout, "One or more imported inputs failed to validate")
			} else {
				assert.NotContains(t, imp.Stdout, "One or more imported inputs failed to validate")

				f, err := os.OpenFile(filepath.Join(pt.CurrentStack().Workspace().WorkDir(), "Pulumi.yaml"), os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o600)
				assert.NoError(t, err)
				defer f.Close()
				_, err = f.WriteString(string(contents))
				assert.NoError(t, err)

				// run preview using the generated file
				pt.Preview(t, optpreview.Diff(), optpreview.ExpectNoChanges())
			}
		})
	}
}

func TestCreateCustomTimeoutsCrossTest(t *testing.T) {
	test := func(
		t *testing.T,
		schemaCreateTimeout *time.Duration,
		programTimeout *string,
		expected time.Duration,
		ExpectFail bool,
	) {
		var pulumiCapturedTimeout *time.Duration
		var tfCapturedTimeout *time.Duration
		prov := &schema.Provider{
			ResourcesMap: map[string]*schema.Resource{
				"prov_test": {
					Schema: map[string]*schema.Schema{
						"prop": {
							Type:     schema.TypeString,
							Optional: true,
						},
					},
					CreateContext: func(ctx context.Context, rd *schema.ResourceData, i interface{}) diag.Diagnostics {
						t := rd.Timeout(schema.TimeoutCreate)
						if pulumiCapturedTimeout == nil {
							pulumiCapturedTimeout = &t
						} else {
							tfCapturedTimeout = &t
						}
						rd.SetId("id")
						return diag.Diagnostics{}
					},
					Timeouts: &schema.ResourceTimeout{
						Create: schemaCreateTimeout,
					},
				},
			},
		}

		bridgedProvider := pulcheck.BridgedProvider(t, "prov", prov)
		pulumiTimeout := `""`
		if programTimeout != nil {
			pulumiTimeout = fmt.Sprintf(`"%s"`, *programTimeout)
		}

		tfTimeout := "null"
		if programTimeout != nil {
			tfTimeout = fmt.Sprintf(`"%s"`, *programTimeout)
		}

		program := fmt.Sprintf(`
name: test
runtime: yaml
resources:
	mainRes:
		type: prov:Test
		properties:
			prop: "val"
		options:
			customTimeouts:
				create: %s
`, pulumiTimeout)

		pt := pulcheck.PulCheck(t, bridgedProvider, program)
		pt.Up(t)
		// We pass custom timeouts in the program if the resource does not support them.

		require.NotNil(t, pulumiCapturedTimeout)
		require.Nil(t, tfCapturedTimeout)

		tfProgram := fmt.Sprintf(`
resource "prov_test" "mainRes" {
	prop = "val"
	timeouts {
		create = %s
	}
}`, tfTimeout)

		tfdriver := tfcheck.NewTfDriver(t, t.TempDir(), "prov", prov)
		tfdriver.Write(t, tfProgram)

		plan, err := tfdriver.Plan(t)
		if ExpectFail {
			require.Error(t, err)
			return
		}
		require.NoError(t, err)
		err = tfdriver.Apply(t, plan)
		require.NoError(t, err)
		require.NotNil(t, tfCapturedTimeout)

		assert.Equal(t, *pulumiCapturedTimeout, *tfCapturedTimeout)
		assert.Equal(t, *pulumiCapturedTimeout, expected)
	}

	oneSecString := "1s"
	oneSec := 1 * time.Second
	// twoSecString := "2s"
	twoSec := 2 * time.Second

	tests := []struct {
		name                string
		schemaCreateTimeout *time.Duration
		programTimeout      *string
		expected            time.Duration
		expectFail          bool
	}{
		{
			"schema specified timeout",
			&oneSec,
			nil,
			oneSec,
			false,
		},
		{
			"program specified timeout",
			&twoSec,
			&oneSecString,
			oneSec,
			false,
		},
		{
			"program specified without schema timeout",
			nil,
			&oneSecString,
			oneSec,
			true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			test(t, tc.schemaCreateTimeout, tc.programTimeout, tc.expected, tc.expectFail)
		})
	}
}

func TestStateFunc(t *testing.T) {
	resMap := map[string]*schema.Resource{
		"prov_test": {
			CreateContext: func(ctx context.Context, d *schema.ResourceData, i interface{}) diag.Diagnostics {
				d.SetId("id")
				var diags diag.Diagnostics
				v, ok := d.GetOk("test")
				assert.True(t, ok, "test property not set")

				err := d.Set("test", v.(string)+" world")
				require.NoError(t, err)
				return diags
			},
			Schema: map[string]*schema.Schema{
				"test": {
					Type:     schema.TypeString,
					Optional: true,
					ForceNew: true,
					StateFunc: func(v interface{}) string {
						return v.(string) + " world"
					},
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
	res := pt.Up(t)
	require.Equal(t, "hello world", res.Outputs["testOut"].Value)
	pt.Preview(t, optpreview.ExpectNoChanges())
}

// TestPlanStateEdit tests that [shimv2.WithPlanStateEdit] can be used to effectively edit
// planed state.
//
// The test is set up to reproduce https://github.com/pulumi/pulumi-gcp/issues/2372.
func TestPlanStateEdit(t *testing.T) {
	setLabelsDiff := func(_ context.Context, d *schema.ResourceDiff, _ interface{}) error {
		raw := d.Get("labels")
		if raw == nil {
			return nil
		}

		if d.Get("terraform_labels") == nil {
			return fmt.Errorf("`terraform_labels` field is not present in the resource schema")
		}

		// If "labels" field is computed, set "terraform_labels" and "effective_labels" to computed.
		// https://github.com/hashicorp/terraform-provider-google/issues/16217
		if !d.GetRawPlan().GetAttr("labels").IsWhollyKnown() {
			if err := d.SetNewComputed("terraform_labels"); err != nil {
				return fmt.Errorf("error setting terraform_labels to computed: %w", err)
			}

			return nil
		}

		// Merge provider default labels with the user defined labels in the resource to get terraform managed labels
		terraformLabels := make(map[string]string)

		labels := raw.(map[string]interface{})
		for k, v := range labels {
			terraformLabels[k] = v.(string)
		}

		if err := d.SetNew("terraform_labels", terraformLabels); err != nil {
			return fmt.Errorf("error setting new terraform_labels diff: %w", err)
		}

		return nil
	}

	const tfLabelsKey = "terraform_labels"

	fixEmptyLabels := func(ctx context.Context, req shimv2.PlanStateEditRequest) (cty.Value, error) {
		tfbridge.GetLogger(ctx).Debug("Invoked") // ctx is correctly passed and the logger is available

		assert.Equal(t, "prov_test", req.TfToken)
		assert.Equal(t, resource.PropertyMap{
			"__defaults": resource.NewProperty([]resource.PropertyValue{}),
			"labels": resource.NewProperty(resource.PropertyMap{
				"empty": resource.NewProperty(""),
				"key":   resource.NewProperty("val"),
			}),
		}, req.NewInputs)
		assert.Equal(t, resource.PropertyMap{
			"configValue": resource.NewProperty("configured"),
		}, req.ProviderConfig)

		m := req.PlanState.AsValueMap()
		effectiveLabels := m[tfLabelsKey].AsValueMap()
		effectiveLabels["empty"] = cty.StringVal("")
		m[tfLabelsKey] = cty.MapVal(effectiveLabels)
		return cty.ObjectVal(m), nil
	}

	res := &schema.Resource{
		Schema: map[string]*schema.Schema{
			"labels": {
				Type:     schema.TypeMap,
				Optional: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},
			tfLabelsKey: {
				Type:     schema.TypeMap,
				Computed: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},
		},
		CustomizeDiff: setLabelsDiff,
	}

	tfp := &schema.Provider{
		Schema: map[string]*schema.Schema{"config_value": {
			Type:     schema.TypeString,
			Optional: true,
		}},
		ResourcesMap: map[string]*schema.Resource{"prov_test": res},
	}
	bridgedProvider := pulcheck.BridgedProvider(t, "prov", tfp,
		pulcheck.WithStateEdit(fixEmptyLabels))
	program := `
name: test
runtime: yaml
resources:
  _:
    type: pulumi:providers:prov
    properties:
        configValue: "configured"
    defaultProvider: true
  mainRes:
    type: prov:index:Test
    properties:
      labels: { "key": "val", "empty": "" }
outputs:
  keyValue: ${mainRes.terraformLabels["key"]}
  emptyValue: ${mainRes.terraformLabels["empty"]}`
	pt := pulcheck.PulCheck(t, bridgedProvider, program)
	out := pt.Up(t)

	assert.Equal(t, "val", out.Outputs["keyValue"].Value)
	assert.Equal(t, "", out.Outputs["emptyValue"].Value)
}

func TestMakeTerraformResultNilVsEmptyMap(t *testing.T) {
	// Nil and empty maps are not equal
	nilMap := resource.NewObjectProperty(nil)
	emptyMap := resource.NewObjectProperty(resource.PropertyMap{})

	assert.True(t, nilMap.DeepEquals(emptyMap))
	assert.NotEqual(t, emptyMap.ObjectValue(), nilMap.ObjectValue())

	// Check that MakeTerraformResult maintains that difference
	const resName = "prov_test"
	resMap := map[string]*schema.Resource{
		"prov_test": {
			Schema: map[string]*schema.Schema{
				"test": {
					Type:     schema.TypeMap,
					Optional: true,
					Elem: &schema.Schema{
						Type: schema.TypeString,
					},
				},
			},
		},
	}

	prov := &schema.Provider{
		ResourcesMap: resMap,
	}
	bridgedProvider := pulcheck.BridgedProvider(t, "prov", prov)

	ctx := context.Background()
	shimProv := bridgedProvider.P

	res := shimProv.ResourcesMap().Get(resName)

	t.Run("NilMap", func(t *testing.T) {
		// Create a resource with a nil map
		state, err := res.InstanceState("0", map[string]interface{}{}, map[string]interface{}{})
		assert.NoError(t, err)

		props, err := tfbridge.MakeTerraformResult(ctx, shimProv, state, res.Schema(), nil, nil, true)
		assert.NoError(t, err)
		assert.NotNil(t, props)
		assert.True(t, props["test"].V == nil)
	})

	t.Run("EmptyMap", func(t *testing.T) {
		// Create a resource with an empty map
		state, err := res.InstanceState("0", map[string]interface{}{"test": map[string]interface{}{}}, map[string]interface{}{})
		assert.NoError(t, err)

		props, err := tfbridge.MakeTerraformResult(ctx, shimProv, state, res.Schema(), nil, nil, true)
		assert.NoError(t, err)
		assert.NotNil(t, props)
		assert.True(t, props["test"].DeepEquals(emptyMap))
	})
}

func runDetailedDiffTest(
	t *testing.T, resMap map[string]*schema.Resource, program1, program2 string,
) (string, map[string]interface{}) {
	tfp := &schema.Provider{ResourcesMap: resMap}
	bridgedProvider := pulcheck.BridgedProvider(t, "prov", tfp)
	pt := pulcheck.PulCheck(t, bridgedProvider, program1)
	pt.Up(t)
	pulumiYamlPath := filepath.Join(pt.CurrentStack().Workspace().WorkDir(), "Pulumi.yaml")

	err := os.WriteFile(pulumiYamlPath, []byte(program2), 0o600)
	require.NoError(t, err)

	pt.ClearGrpcLog(t)
	res := pt.Preview(t, optpreview.Diff())
	t.Log(res.StdOut)

	diffResponse := struct {
		DetailedDiff map[string]interface{} `json:"detailedDiff"`
	}{}

	for _, entry := range pt.GrpcLog(t).Entries {
		if entry.Method == "/pulumirpc.ResourceProvider/Diff" {
			err := json.Unmarshal(entry.Response, &diffResponse)
			require.NoError(t, err)
		}
	}

	return res.StdOut, diffResponse.DetailedDiff
}

func TestDetailedDiffSet(t *testing.T) {
	// TODO: Remove this once accurate bridge previews are rolled out
	t.Setenv("PULUMI_TF_BRIDGE_ACCURATE_BRIDGE_PREVIEW", "true")
	runTest := func(t *testing.T, resMap map[string]*schema.Resource, props1, props2 interface{},
		expected autogold.Value, expectedDetailedDiff map[string]any,
	) {
		program := `
name: test
runtime: yaml
resources:
  mainRes:
    type: prov:index:Test
    properties:
      tests: %s
`
		props1JSON, err := json.Marshal(props1)
		require.NoError(t, err)
		program1 := fmt.Sprintf(program, string(props1JSON))
		props2JSON, err := json.Marshal(props2)
		require.NoError(t, err)
		program2 := fmt.Sprintf(program, string(props2JSON))
		out, detailedDiff := runDetailedDiffTest(t, resMap, program1, program2)

		expected.Equal(t, out)
		require.Equal(t, expectedDetailedDiff, detailedDiff)
	}

	type setDetailedDiffTestCase struct {
		name                              string
		props1                            []string
		props2                            []string
		expectedAttrDetailedDiff          map[string]any
		expectedAttr                      autogold.Value
		expectedAttrForceNewDetailedDiff  map[string]any
		expectedAttrForceNew              autogold.Value
		expectedBlockDetailedDiff         map[string]any
		expectedBlock                     autogold.Value
		expectedBlockForceNewDetailedDiff map[string]any
		expectedBlockForceNew             autogold.Value
	}

	testCases := []setDetailedDiffTestCase{
		{
			"unchanged",
			[]string{"val1"},
			[]string{"val1"},
			nil,
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
Resources:
    2 unchanged
`),
			nil,
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
Resources:
    2 unchanged
`),
			nil,
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
Resources:
    2 unchanged
`),
			nil,
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
Resources:
    2 unchanged
`),
		},
		{
			"changed non-empty",
			[]string{"val1"},
			[]string{"val2"},
			map[string]interface{}{"tests[0]": map[string]interface{}{"kind": "UPDATE"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          ~ [0]: "val1" => "val2"
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
			map[string]interface{}{"tests[0]": map[string]interface{}{"kind": "UPDATE_REPLACE"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          ~ [0]: "val1" => "val2"
        ]
Resources:
    +-1 to replace
    1 unchanged
`),
			map[string]interface{}{"tests[0].nested": map[string]interface{}{"kind": "UPDATE"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          ~ [0]: {
                  ~ nested: "val1" => "val2"
                }
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
			map[string]interface{}{"tests[0].nested": map[string]interface{}{"kind": "UPDATE_REPLACE"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          ~ [0]: {
                  ~ nested: "val1" => "val2"
                }
        ]
Resources:
    +-1 to replace
    1 unchanged
`),
		},
		{
			"changed from empty",
			[]string{},
			[]string{"val1"},
			map[string]interface{}{"tests": map[string]interface{}{}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      + tests: [
      +     [0]: "val1"
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
			map[string]interface{}{"tests": map[string]interface{}{"kind": "ADD_REPLACE"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      + tests: [
      +     [0]: "val1"
        ]
Resources:
    +-1 to replace
    1 unchanged
`),
			map[string]interface{}{"tests": map[string]interface{}{}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      + tests: [
      +     [0]: {
              + nested    : "val1"
            }
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
			map[string]interface{}{"tests": map[string]interface{}{"kind": "ADD_REPLACE"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      + tests: [
      +     [0]: {
              + nested    : "val1"
            }
        ]
Resources:
    +-1 to replace
    1 unchanged
`),
		},
		{
			"changed to empty",
			[]string{"val1"},
			[]string{},
			map[string]interface{}{"tests": map[string]interface{}{"kind": "DELETE"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      - tests: [
      -     [0]: "val1"
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
			map[string]interface{}{"tests": map[string]interface{}{"kind": "DELETE_REPLACE"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      - tests: [
      -     [0]: "val1"
        ]
Resources:
    +-1 to replace
    1 unchanged
`),
			map[string]interface{}{"tests": map[string]interface{}{"kind": "DELETE"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      - tests: [
      -     [0]: {
              - nested: "val1"
            }
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
			map[string]interface{}{"tests": map[string]interface{}{"kind": "DELETE_REPLACE"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      - tests: [
      -     [0]: {
              - nested: "val1"
            }
        ]
Resources:
    +-1 to replace
    1 unchanged
`),
		},
		{
			"removed front",
			[]string{"val1", "val2", "val3"},
			[]string{"val2", "val3"},
			map[string]interface{}{"tests[0]": map[string]interface{}{"kind": "DELETE"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [0]: "val1"
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
			map[string]interface{}{"tests[0]": map[string]interface{}{"kind": "DELETE_REPLACE"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [0]: "val1"
        ]
Resources:
    +-1 to replace
    1 unchanged
`),
			map[string]interface{}{"tests[0]": map[string]interface{}{"kind": "DELETE"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [0]: {
                  - nested: "val1"
                }
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
			map[string]interface{}{"tests[0]": map[string]interface{}{"kind": "DELETE_REPLACE"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [0]: {
                  - nested: "val1"
                }
        ]
Resources:
    +-1 to replace
    1 unchanged
`),
		},
		{
			"removed front unordered",
			[]string{"val2", "val1", "val3"},
			[]string{"val1", "val3"},
			map[string]interface{}{"tests[1]": map[string]interface{}{"kind": "DELETE"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [1]: "val2"
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
			map[string]interface{}{"tests[1]": map[string]interface{}{"kind": "DELETE_REPLACE"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [1]: "val2"
        ]
Resources:
    +-1 to replace
    1 unchanged
`),
			map[string]interface{}{"tests[1]": map[string]interface{}{"kind": "DELETE"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [1]: {
                  - nested: "val2"
                }
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
			map[string]interface{}{"tests[1]": map[string]interface{}{"kind": "DELETE_REPLACE"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [1]: {
                  - nested: "val2"
                }
        ]
Resources:
    +-1 to replace
    1 unchanged
`),
		},
		{
			"removed middle",
			[]string{"val1", "val2", "val3"},
			[]string{"val1", "val3"},
			map[string]interface{}{"tests[1]": map[string]interface{}{"kind": "DELETE"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [1]: "val2"
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
			map[string]interface{}{"tests[1]": map[string]interface{}{"kind": "DELETE_REPLACE"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [1]: "val2"
        ]
Resources:
    +-1 to replace
    1 unchanged
`),
			map[string]interface{}{"tests[1]": map[string]interface{}{"kind": "DELETE"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [1]: {
                  - nested: "val2"
                }
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
			map[string]interface{}{"tests[1]": map[string]interface{}{"kind": "DELETE_REPLACE"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [1]: {
                  - nested: "val2"
                }
        ]
Resources:
    +-1 to replace
    1 unchanged
`),
		},
		{
			"removed middle unordered",
			[]string{"val2", "val3", "val1"},
			[]string{"val2", "val1"},
			map[string]interface{}{"tests[2]": map[string]interface{}{"kind": "DELETE"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [2]: "val3"
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
			map[string]interface{}{"tests[2]": map[string]interface{}{"kind": "DELETE_REPLACE"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [2]: "val3"
        ]
Resources:
    +-1 to replace
    1 unchanged
`),
			map[string]interface{}{"tests[2]": map[string]interface{}{"kind": "DELETE"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [2]: {
                  - nested: "val3"
                }
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
			map[string]interface{}{"tests[2]": map[string]interface{}{"kind": "DELETE_REPLACE"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [2]: {
                  - nested: "val3"
                }
        ]
Resources:
    +-1 to replace
    1 unchanged
`),
		},
		{
			"removed end",
			[]string{"val1", "val2", "val3"},
			[]string{"val1", "val2"},
			map[string]interface{}{"tests[2]": map[string]interface{}{"kind": "DELETE"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [2]: "val3"
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
			map[string]interface{}{"tests[2]": map[string]interface{}{"kind": "DELETE_REPLACE"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [2]: "val3"
        ]
Resources:
    +-1 to replace
    1 unchanged
`),
			map[string]interface{}{"tests[2]": map[string]interface{}{"kind": "DELETE"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [2]: {
                  - nested: "val3"
                }
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
			map[string]interface{}{"tests[2]": map[string]interface{}{"kind": "DELETE_REPLACE"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [2]: {
                  - nested: "val3"
                }
        ]
Resources:
    +-1 to replace
    1 unchanged
`),
		},
		{
			"removed end unordered",
			[]string{"val2", "val3", "val1"},
			[]string{"val2", "val3"},
			map[string]interface{}{"tests[0]": map[string]interface{}{"kind": "DELETE"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [0]: "val1"
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
			map[string]interface{}{"tests[0]": map[string]interface{}{"kind": "DELETE_REPLACE"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [0]: "val1"
        ]
Resources:
    +-1 to replace
    1 unchanged
`),
			map[string]interface{}{"tests[0]": map[string]interface{}{"kind": "DELETE"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [0]: {
                  - nested: "val1"
                }
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
			map[string]interface{}{"tests[0]": map[string]interface{}{"kind": "DELETE_REPLACE"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [0]: {
                  - nested: "val1"
                }
        ]
Resources:
    +-1 to replace
    1 unchanged
`),
		},
		{
			"added front",
			[]string{"val2", "val3"},
			[]string{"val1", "val2", "val3"},
			map[string]interface{}{"tests[0]": map[string]interface{}{}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [0]: "val1"
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
			map[string]interface{}{"tests[0]": map[string]interface{}{"kind": "ADD_REPLACE"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [0]: "val1"
        ]
Resources:
    +-1 to replace
    1 unchanged
`),
			map[string]interface{}{"tests[0]": map[string]interface{}{}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [0]: {
                  + nested    : "val1"
                }
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
			map[string]interface{}{"tests[0]": map[string]interface{}{"kind": "ADD_REPLACE"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [0]: {
                  + nested    : "val1"
                }
        ]
Resources:
    +-1 to replace
    1 unchanged
`),
		},
		{
			"added front unordered",
			[]string{"val3", "val1"},
			[]string{"val2", "val2", "val1"},
			map[string]interface{}{"tests[1]": map[string]interface{}{"kind": "UPDATE"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          ~ [1]: "val3" => "val2"
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
			map[string]interface{}{"tests[1]": map[string]interface{}{"kind": "UPDATE_REPLACE"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          ~ [1]: "val3" => "val2"
        ]
Resources:
    +-1 to replace
    1 unchanged
`),
			map[string]interface{}{"tests[1].nested": map[string]interface{}{"kind": "UPDATE"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          ~ [1]: {
                  ~ nested: "val3" => "val2"
                }
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
			map[string]interface{}{"tests[1].nested": map[string]interface{}{"kind": "UPDATE_REPLACE"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          ~ [1]: {
                  ~ nested: "val3" => "val2"
                }
        ]
Resources:
    +-1 to replace
    1 unchanged
`),
		},
		{
			"added middle",
			[]string{"val1", "val3"},
			[]string{"val1", "val2", "val3"},
			map[string]interface{}{"tests[1]": map[string]interface{}{}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [1]: "val2"
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
			map[string]interface{}{"tests[1]": map[string]interface{}{"kind": "ADD_REPLACE"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [1]: "val2"
        ]
Resources:
    +-1 to replace
    1 unchanged
`),
			map[string]interface{}{"tests[1]": map[string]interface{}{}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [1]: {
                  + nested    : "val2"
                }
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
			map[string]interface{}{"tests[1]": map[string]interface{}{"kind": "ADD_REPLACE"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [1]: {
                  + nested    : "val2"
                }
        ]
Resources:
    +-1 to replace
    1 unchanged
`),
		},
		{
			"added middle unordered",
			[]string{"val2", "val1"},
			[]string{"val2", "val3", "val1"},
			map[string]interface{}{"tests[2]": map[string]interface{}{}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [2]: "val1"
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
			map[string]interface{}{"tests[2]": map[string]interface{}{"kind": "ADD_REPLACE"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [2]: "val1"
        ]
Resources:
    +-1 to replace
    1 unchanged
`),
			map[string]interface{}{"tests[2]": map[string]interface{}{}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [2]: {
                  + nested    : "val1"
                }
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
			map[string]interface{}{"tests[2]": map[string]interface{}{"kind": "ADD_REPLACE"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [2]: {
                  + nested    : "val1"
                }
        ]
Resources:
    +-1 to replace
    1 unchanged
`),
		},
		{
			"added end",
			[]string{"val1", "val2"},
			[]string{"val1", "val2", "val3"},
			map[string]interface{}{"tests[2]": map[string]interface{}{}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [2]: "val3"
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
			map[string]interface{}{"tests[2]": map[string]interface{}{"kind": "ADD_REPLACE"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [2]: "val3"
        ]
Resources:
    +-1 to replace
    1 unchanged
`),
			map[string]interface{}{"tests[2]": map[string]interface{}{}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [2]: {
                  + nested    : "val3"
                }
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
			map[string]interface{}{"tests[2]": map[string]interface{}{"kind": "ADD_REPLACE"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [2]: {
                  + nested    : "val3"
                }
        ]
Resources:
    +-1 to replace
    1 unchanged
`),
		},
		{
			"added end unordered",
			[]string{"val2", "val3"},
			[]string{"val2", "val3", "val1"},
			map[string]interface{}{"tests[0]": map[string]interface{}{}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [0]: "val2"
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
			map[string]interface{}{"tests[0]": map[string]interface{}{"kind": "ADD_REPLACE"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [0]: "val2"
        ]
Resources:
    +-1 to replace
    1 unchanged
`),
			map[string]interface{}{"tests[0]": map[string]interface{}{}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [0]: {
                  + nested    : "val2"
                }
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
			map[string]interface{}{"tests[0]": map[string]interface{}{"kind": "ADD_REPLACE"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [0]: {
                  + nested    : "val2"
                }
        ]
Resources:
    +-1 to replace
    1 unchanged
`),
		},
		{
			"same element updated",
			[]string{"val1", "val2", "val3"},
			[]string{"val1", "val4", "val3"},
			map[string]interface{}{"tests[1]": map[string]interface{}{"kind": "DELETE"}, "tests[2]": map[string]interface{}{}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [1]: "val2"
          + [2]: "val3"
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
			map[string]interface{}{"tests[1]": map[string]interface{}{"kind": "DELETE_REPLACE"}, "tests[2]": map[string]interface{}{"kind": "ADD_REPLACE"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [1]: "val2"
          + [2]: "val3"
        ]
Resources:
    +-1 to replace
    1 unchanged
`),
			map[string]interface{}{"tests[1]": map[string]interface{}{"kind": "DELETE"}, "tests[2]": map[string]interface{}{}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [1]: {
                  - nested: "val2"
                }
          + [2]: {
                  + nested    : "val3"
                }
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
			map[string]interface{}{"tests[1]": map[string]interface{}{"kind": "DELETE_REPLACE"}, "tests[2]": map[string]interface{}{"kind": "ADD_REPLACE"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [1]: {
                  - nested: "val2"
                }
          + [2]: {
                  + nested    : "val3"
                }
        ]
Resources:
    +-1 to replace
    1 unchanged
`),
		},
		{
			"same element updated unordered",
			[]string{"val2", "val3", "val1"},
			[]string{"val2", "val4", "val1"},
			map[string]interface{}{"tests[2]": map[string]interface{}{"kind": "UPDATE"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          ~ [2]: "val3" => "val1"
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
			map[string]interface{}{"tests[2]": map[string]interface{}{"kind": "UPDATE_REPLACE"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          ~ [2]: "val3" => "val1"
        ]
Resources:
    +-1 to replace
    1 unchanged
`),
			map[string]interface{}{"tests[2].nested": map[string]interface{}{"kind": "UPDATE"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          ~ [2]: {
                  ~ nested: "val3" => "val1"
                }
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
			map[string]interface{}{"tests[2].nested": map[string]interface{}{"kind": "UPDATE_REPLACE"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          ~ [2]: {
                  ~ nested: "val3" => "val1"
                }
        ]
Resources:
    +-1 to replace
    1 unchanged
`),
		},
		{
			"shuffled",
			[]string{"val1", "val2", "val3"},
			[]string{"val3", "val1", "val2"},
			nil,
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
Resources:
    2 unchanged
`),
			nil,
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
Resources:
    2 unchanged
`),
			nil,
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
Resources:
    2 unchanged
`),
			nil,
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
Resources:
    2 unchanged
`),
		},
		{
			"shuffled unordered",
			[]string{"val2", "val3", "val1"},
			[]string{"val3", "val1", "val2"},
			nil,
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
Resources:
    2 unchanged
`),
			nil,
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
Resources:
    2 unchanged
`),
			nil,
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
Resources:
    2 unchanged
`),
			nil,
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
Resources:
    2 unchanged
`),
		},
		{
			"shuffled with duplicates",
			[]string{"val1", "val2", "val3"},
			[]string{"val3", "val1", "val2", "val3"},
			nil,
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
Resources:
    2 unchanged
`),
			nil,
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
Resources:
    2 unchanged
`),
			nil,
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
Resources:
    2 unchanged
`),
			nil,
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
Resources:
    2 unchanged
`),
		},
		{
			"shuffled with duplicates unordered",
			[]string{"val2", "val3", "val1"},
			[]string{"val3", "val1", "val2", "val3"},
			nil,
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
Resources:
    2 unchanged
`),
			nil,
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
Resources:
    2 unchanged
`),
			nil,
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
Resources:
    2 unchanged
`),
			nil,
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
Resources:
    2 unchanged
`),
		},
		{
			"shuffled added front",
			[]string{"val2", "val3"},
			[]string{"val1", "val3", "val2"},
			map[string]interface{}{"tests[0]": map[string]interface{}{}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [0]: "val1"
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
			map[string]interface{}{"tests[0]": map[string]interface{}{"kind": "ADD_REPLACE"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [0]: "val1"
        ]
Resources:
    +-1 to replace
    1 unchanged
`),
			map[string]interface{}{"tests[0]": map[string]interface{}{}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [0]: {
                  + nested    : "val1"
                }
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
			map[string]interface{}{"tests[0]": map[string]interface{}{"kind": "ADD_REPLACE"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [0]: {
                  + nested    : "val1"
                }
        ]
Resources:
    +-1 to replace
    1 unchanged
`),
		},
		{
			"shuffled added front unordered",
			[]string{"val3", "val1"},
			[]string{"val2", "val1", "val3"},
			map[string]interface{}{"tests[1]": map[string]interface{}{}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [1]: "val1"
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
			map[string]interface{}{"tests[1]": map[string]interface{}{"kind": "ADD_REPLACE"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [1]: "val1"
        ]
Resources:
    +-1 to replace
    1 unchanged
`),
			map[string]interface{}{"tests[1]": map[string]interface{}{}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [1]: {
                  + nested    : "val1"
                }
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
			map[string]interface{}{"tests[1]": map[string]interface{}{"kind": "ADD_REPLACE"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [1]: {
                  + nested    : "val1"
                }
        ]
Resources:
    +-1 to replace
    1 unchanged
`),
		},
		{
			"shuffled added middle",
			[]string{"val1", "val3"},
			[]string{"val3", "val2", "val1"},
			map[string]interface{}{"tests[1]": map[string]interface{}{}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [1]: "val2"
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
			map[string]interface{}{"tests[1]": map[string]interface{}{"kind": "ADD_REPLACE"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [1]: "val2"
        ]
Resources:
    +-1 to replace
    1 unchanged
`),
			map[string]interface{}{"tests[1]": map[string]interface{}{}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [1]: {
                  + nested    : "val2"
                }
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
			map[string]interface{}{"tests[1]": map[string]interface{}{"kind": "ADD_REPLACE"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [1]: {
                  + nested    : "val2"
                }
        ]
Resources:
    +-1 to replace
    1 unchanged
`),
		},
		{
			"shuffled added middle unordered",
			[]string{"val2", "val1"},
			[]string{"val1", "val3", "val2"},
			map[string]interface{}{"tests[2]": map[string]interface{}{}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [2]: "val2"
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
			map[string]interface{}{"tests[2]": map[string]interface{}{"kind": "ADD_REPLACE"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [2]: "val2"
        ]
Resources:
    +-1 to replace
    1 unchanged
`),
			map[string]interface{}{"tests[2]": map[string]interface{}{}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [2]: {
                  + nested    : "val2"
                }
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
			map[string]interface{}{"tests[2]": map[string]interface{}{"kind": "ADD_REPLACE"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [2]: {
                  + nested    : "val2"
                }
        ]
Resources:
    +-1 to replace
    1 unchanged
`),
		},
		{
			"shuffled added end",
			[]string{"val1", "val2"},
			[]string{"val2", "val1", "val3"},
			map[string]interface{}{"tests[2]": map[string]interface{}{}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [2]: "val3"
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
			map[string]interface{}{"tests[2]": map[string]interface{}{"kind": "ADD_REPLACE"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [2]: "val3"
        ]
Resources:
    +-1 to replace
    1 unchanged
`),
			map[string]interface{}{"tests[2]": map[string]interface{}{}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [2]: {
                  + nested    : "val3"
                }
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
			map[string]interface{}{"tests[2]": map[string]interface{}{"kind": "ADD_REPLACE"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [2]: {
                  + nested    : "val3"
                }
        ]
Resources:
    +-1 to replace
    1 unchanged
`),
		},
		{
			"shuffled removed front",
			[]string{"val1", "val2", "val3"},
			[]string{"val3", "val2"},
			map[string]interface{}{"tests[0]": map[string]interface{}{"kind": "DELETE"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [0]: "val1"
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
			map[string]interface{}{"tests[0]": map[string]interface{}{"kind": "DELETE_REPLACE"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [0]: "val1"
        ]
Resources:
    +-1 to replace
    1 unchanged
`),
			map[string]interface{}{"tests[0]": map[string]interface{}{"kind": "DELETE"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [0]: {
                  - nested: "val1"
                }
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
			map[string]interface{}{"tests[0]": map[string]interface{}{"kind": "DELETE_REPLACE"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [0]: {
                  - nested: "val1"
                }
        ]
Resources:
    +-1 to replace
    1 unchanged
`),
		},
		{
			"shuffled removed middle",
			[]string{"val1", "val2", "val3"},
			[]string{"val3", "val1"},
			map[string]interface{}{"tests[1]": map[string]interface{}{"kind": "DELETE"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [1]: "val2"
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
			map[string]interface{}{"tests[1]": map[string]interface{}{"kind": "DELETE_REPLACE"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [1]: "val2"
        ]
Resources:
    +-1 to replace
    1 unchanged
`),
			map[string]interface{}{"tests[1]": map[string]interface{}{"kind": "DELETE"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [1]: {
                  - nested: "val2"
                }
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
			map[string]interface{}{"tests[1]": map[string]interface{}{"kind": "DELETE_REPLACE"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [1]: {
                  - nested: "val2"
                }
        ]
Resources:
    +-1 to replace
    1 unchanged
`),
		},
		{
			"shuffled removed end",
			[]string{"val1", "val2", "val3"},
			[]string{"val2", "val1"},
			map[string]interface{}{"tests[2]": map[string]interface{}{"kind": "DELETE"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [2]: "val3"
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
			map[string]interface{}{"tests[2]": map[string]interface{}{"kind": "DELETE_REPLACE"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [2]: "val3"
        ]
Resources:
    +-1 to replace
    1 unchanged
`),
			map[string]interface{}{"tests[2]": map[string]interface{}{"kind": "DELETE"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [2]: {
                  - nested: "val3"
                }
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
			map[string]interface{}{"tests[2]": map[string]interface{}{"kind": "DELETE_REPLACE"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [2]: {
                  - nested: "val3"
                }
        ]
Resources:
    +-1 to replace
    1 unchanged
`),
		},
		{
			"two added",
			[]string{"val1", "val2"},
			[]string{"val1", "val2", "val3", "val4"},
			map[string]interface{}{"tests[2]": map[string]interface{}{}, "tests[3]": map[string]interface{}{}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [2]: "val3"
          + [3]: "val4"
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
			map[string]interface{}{"tests[2]": map[string]interface{}{"kind": "ADD_REPLACE"}, "tests[3]": map[string]interface{}{"kind": "ADD_REPLACE"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [2]: "val3"
          + [3]: "val4"
        ]
Resources:
    +-1 to replace
    1 unchanged
`),
			map[string]interface{}{"tests[2]": map[string]interface{}{}, "tests[3]": map[string]interface{}{}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [2]: {
                  + nested    : "val3"
                }
          + [3]: {
                  + nested    : "val4"
                }
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
			map[string]interface{}{"tests[2]": map[string]interface{}{"kind": "ADD_REPLACE"}, "tests[3]": map[string]interface{}{"kind": "ADD_REPLACE"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [2]: {
                  + nested    : "val3"
                }
          + [3]: {
                  + nested    : "val4"
                }
        ]
Resources:
    +-1 to replace
    1 unchanged
`),
		},
		{
			"two removed",
			[]string{"val1", "val2", "val3", "val4"},
			[]string{"val1", "val2"},
			map[string]interface{}{"tests[2]": map[string]interface{}{"kind": "DELETE"}, "tests[3]": map[string]interface{}{"kind": "DELETE"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [2]: "val3"
          - [3]: "val4"
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
			map[string]interface{}{"tests[2]": map[string]interface{}{"kind": "DELETE_REPLACE"}, "tests[3]": map[string]interface{}{"kind": "DELETE_REPLACE"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [2]: "val3"
          - [3]: "val4"
        ]
Resources:
    +-1 to replace
    1 unchanged
`),
			map[string]interface{}{"tests[2]": map[string]interface{}{"kind": "DELETE"}, "tests[3]": map[string]interface{}{"kind": "DELETE"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [2]: {
                  - nested: "val3"
                }
          - [3]: {
                  - nested: "val4"
                }
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
			map[string]interface{}{"tests[2]": map[string]interface{}{"kind": "DELETE_REPLACE"}, "tests[3]": map[string]interface{}{"kind": "DELETE_REPLACE"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [2]: {
                  - nested: "val3"
                }
          - [3]: {
                  - nested: "val4"
                }
        ]
Resources:
    +-1 to replace
    1 unchanged
`),
		},
		{
			"two added and two removed",
			[]string{"val1", "val2", "val3", "val4"},
			[]string{"val1", "val2", "val5", "val6"},
			map[string]interface{}{"tests[2]": map[string]interface{}{"kind": "UPDATE"}, "tests[3]": map[string]interface{}{"kind": "UPDATE"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          ~ [2]: "val3" => "val5"
          ~ [3]: "val4" => "val6"
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
			map[string]interface{}{"tests[2]": map[string]interface{}{"kind": "UPDATE_REPLACE"}, "tests[3]": map[string]interface{}{"kind": "UPDATE_REPLACE"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          ~ [2]: "val3" => "val5"
          ~ [3]: "val4" => "val6"
        ]
Resources:
    +-1 to replace
    1 unchanged
`),
			map[string]interface{}{"tests[2].nested": map[string]interface{}{"kind": "UPDATE"}, "tests[3].nested": map[string]interface{}{"kind": "UPDATE"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          ~ [2]: {
                  ~ nested: "val3" => "val5"
                }
          ~ [3]: {
                  ~ nested: "val4" => "val6"
                }
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
			map[string]interface{}{"tests[2].nested": map[string]interface{}{"kind": "UPDATE_REPLACE"}, "tests[3].nested": map[string]interface{}{"kind": "UPDATE_REPLACE"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          ~ [2]: {
                  ~ nested: "val3" => "val5"
                }
          ~ [3]: {
                  ~ nested: "val4" => "val6"
                }
        ]
Resources:
    +-1 to replace
    1 unchanged
`),
		},
		{
			"two added and two removed shuffled, one overlaps",
			[]string{"val1", "val2", "val3", "val4"},
			[]string{"val1", "val5", "val6", "val2"},
			map[string]interface{}{"tests[2]": map[string]interface{}{"kind": "UPDATE"}, "tests[3]": map[string]interface{}{"kind": "UPDATE"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          ~ [2]: "val3" => "val6"
          ~ [3]: "val4" => "val2"
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
			map[string]interface{}{"tests[2]": map[string]interface{}{"kind": "UPDATE_REPLACE"}, "tests[3]": map[string]interface{}{"kind": "UPDATE_REPLACE"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          ~ [2]: "val3" => "val6"
          ~ [3]: "val4" => "val2"
        ]
Resources:
    +-1 to replace
    1 unchanged
`),
			map[string]interface{}{"tests[2].nested": map[string]interface{}{"kind": "UPDATE"}, "tests[3].nested": map[string]interface{}{"kind": "UPDATE"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          ~ [2]: {
                  ~ nested: "val3" => "val6"
                }
          ~ [3]: {
                  ~ nested: "val4" => "val2"
                }
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
			map[string]interface{}{"tests[2].nested": map[string]interface{}{"kind": "UPDATE_REPLACE"}, "tests[3].nested": map[string]interface{}{"kind": "UPDATE_REPLACE"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          ~ [2]: {
                  ~ nested: "val3" => "val6"
                }
          ~ [3]: {
                  ~ nested: "val4" => "val2"
                }
        ]
Resources:
    +-1 to replace
    1 unchanged
`),
		},
		{
			"two added and two removed shuffled, no overlaps",
			[]string{"val1", "val2", "val3", "val4"},
			[]string{"val5", "val6", "val1", "val2"},
			map[string]interface{}{"tests[2]": map[string]interface{}{"kind": "UPDATE"}, "tests[3]": map[string]interface{}{"kind": "UPDATE"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          ~ [2]: "val3" => "val1"
          ~ [3]: "val4" => "val2"
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
			map[string]interface{}{"tests[2]": map[string]interface{}{"kind": "UPDATE_REPLACE"}, "tests[3]": map[string]interface{}{"kind": "UPDATE_REPLACE"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          ~ [2]: "val3" => "val1"
          ~ [3]: "val4" => "val2"
        ]
Resources:
    +-1 to replace
    1 unchanged
`),
			map[string]interface{}{"tests[2].nested": map[string]interface{}{"kind": "UPDATE"}, "tests[3].nested": map[string]interface{}{"kind": "UPDATE"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          ~ [2]: {
                  ~ nested: "val3" => "val1"
                }
          ~ [3]: {
                  ~ nested: "val4" => "val2"
                }
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
			map[string]interface{}{"tests[2].nested": map[string]interface{}{"kind": "UPDATE_REPLACE"}, "tests[3].nested": map[string]interface{}{"kind": "UPDATE_REPLACE"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          ~ [2]: {
                  ~ nested: "val3" => "val1"
                }
          ~ [3]: {
                  ~ nested: "val4" => "val2"
                }
        ]
Resources:
    +-1 to replace
    1 unchanged
`),
		},
		{
			"two added and two removed shuffled, with duplicates",
			[]string{"val1", "val2", "val3", "val4"},
			[]string{"val1", "val5", "val6", "val2", "val1", "val2"},
			map[string]interface{}{"tests[2]": map[string]interface{}{"kind": "UPDATE"}, "tests[3]": map[string]interface{}{"kind": "UPDATE"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          ~ [2]: "val3" => "val6"
          ~ [3]: "val4" => "val2"
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
			map[string]interface{}{"tests[2]": map[string]interface{}{"kind": "UPDATE_REPLACE"}, "tests[3]": map[string]interface{}{"kind": "UPDATE_REPLACE"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          ~ [2]: "val3" => "val6"
          ~ [3]: "val4" => "val2"
        ]
Resources:
    +-1 to replace
    1 unchanged
`),
			map[string]interface{}{"tests[2].nested": map[string]interface{}{"kind": "UPDATE"}, "tests[3].nested": map[string]interface{}{"kind": "UPDATE"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          ~ [2]: {
                  ~ nested: "val3" => "val6"
                }
          ~ [3]: {
                  ~ nested: "val4" => "val2"
                }
        ]
Resources:
    ~ 1 to update
    1 unchanged
`),
			map[string]interface{}{"tests[2].nested": map[string]interface{}{"kind": "UPDATE_REPLACE"}, "tests[3].nested": map[string]interface{}{"kind": "UPDATE_REPLACE"}},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          ~ [2]: {
                  ~ nested: "val3" => "val6"
                }
          ~ [3]: {
                  ~ nested: "val4" => "val2"
                }
        ]
Resources:
    +-1 to replace
    1 unchanged
`),
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			for _, forceNew := range []bool{false, true} {
				t.Run(fmt.Sprintf("ForceNew=%v", forceNew), func(t *testing.T) {
					expected := tc.expectedAttr
					if forceNew {
						expected = tc.expectedAttrForceNew
					}

					expectedDetailedDiff := tc.expectedAttrDetailedDiff
					if forceNew {
						expectedDetailedDiff = tc.expectedAttrForceNewDetailedDiff
					}
					t.Run("Attribute", func(t *testing.T) {
						res := &schema.Resource{
							Schema: map[string]*schema.Schema{
								"test": {
									Type:     schema.TypeSet,
									Optional: true,
									Elem: &schema.Schema{
										Type: schema.TypeString,
									},
									ForceNew: forceNew,
								},
							},
						}
						runTest(t, map[string]*schema.Resource{"prov_test": res}, tc.props1, tc.props2, expected, expectedDetailedDiff)
					})

					expected = tc.expectedBlock
					if forceNew {
						expected = tc.expectedBlockForceNew
					}
					expectedDetailedDiff = tc.expectedBlockDetailedDiff
					if forceNew {
						expectedDetailedDiff = tc.expectedBlockForceNewDetailedDiff
					}

					t.Run("Block", func(t *testing.T) {
						res := &schema.Resource{
							Schema: map[string]*schema.Schema{
								"test": {
									Type:     schema.TypeSet,
									Optional: true,
									Elem: &schema.Resource{
										Schema: map[string]*schema.Schema{
											"nested": {
												Type:     schema.TypeString,
												Optional: true,
												ForceNew: forceNew,
											},
										},
									},
								},
							},
						}

						props1 := make([]interface{}, len(tc.props1))
						for i, v := range tc.props1 {
							props1[i] = map[string]interface{}{"nested": v}
						}

						props2 := make([]interface{}, len(tc.props2))
						for i, v := range tc.props2 {
							props2[i] = map[string]interface{}{"nested": v}
						}

						runTest(t, map[string]*schema.Resource{"prov_test": res}, props1, props2, expected, expectedDetailedDiff)
					})
				})
			}
		})
	}

	t.Run("Ensure detailed diff order", func(t *testing.T) {
		testCaseMap := make(map[string]setDetailedDiffTestCase, len(testCases))
		for _, tc := range testCases {
			testCaseMap[tc.name] = tc
		}

		// Ensure that the detailed diff is the same when the set is re-ordered
		for name, tc := range testCaseMap {
			if !strings.Contains(name, " unordered") {
				continue
			}

			t.Run(name, func(t *testing.T) {
				orderedName := strings.ReplaceAll(name, " unordered", "")

				orderedTC, ok := testCaseMap[orderedName]

				if !ok {
					t.Fatalf("missing ordered test case %s", orderedName)
				}

				require.Equal(t, orderedTC.expectedAttrDetailedDiff, tc.expectedAttrDetailedDiff)
				require.Equal(t, orderedTC.expectedBlockDetailedDiff, tc.expectedBlockDetailedDiff)
				require.Equal(t, orderedTC.expectedAttrForceNewDetailedDiff, tc.expectedAttrForceNewDetailedDiff)
				require.Equal(t, orderedTC.expectedBlockForceNewDetailedDiff, tc.expectedBlockForceNewDetailedDiff)
			})
		}
	})
}

// "UNKNOWN" for unknown values
func testDetailedDiffWithUnknowns(t *testing.T, resMap map[string]*schema.Resource, unknownString string, props1, props2 interface{}, expected, expectedDetailedDiff autogold.Value) {
	originalProgram := `
name: test
runtime: yaml
resources:
  mainRes:
    type: prov:index:Test
	properties:
		tests: %s
outputs:
  testOut: ${mainRes.tests}
	`
	props1JSON, err := json.Marshal(props1)
	require.NoError(t, err)
	program1 := fmt.Sprintf(originalProgram, string(props1JSON))

	programWithUnknown := `
name: test
runtime: yaml
resources:
  auxRes:
    type: prov:index:Aux
  mainRes:
    type: prov:index:Test
    properties:
      tests: %s
outputs:
  testOut: ${mainRes.tests}
`
	props2JSON, err := json.Marshal(props2)
	require.NoError(t, err)
	program2 := fmt.Sprintf(programWithUnknown, string(props2JSON))
	program2 = strings.ReplaceAll(program2, "UNKNOWN", unknownString)

	out, detailedDiff := runDetailedDiffTest(t, resMap, program1, program2)
	expected.Equal(t, out)
	expectedDetailedDiff.Equal(t, detailedDiff)
}

func TestUnknownSetAttributeElementDiff(t *testing.T) {
	// TODO: Remove this once accurate bridge previews are rolled out
	t.Setenv("PULUMI_TF_BRIDGE_ACCURATE_BRIDGE_PREVIEW", "true")
	resMap := map[string]*schema.Resource{
		"prov_test": {
			Schema: map[string]*schema.Schema{
				"test": {
					Type:     schema.TypeSet,
					Optional: true,
					Elem: &schema.Schema{
						Type: schema.TypeString,
					},
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

	t.Run("empty to unknown element", func(t *testing.T) {
		testDetailedDiffWithUnknowns(t, resMap, "${auxRes.aux}",
			[]interface{}{},
			[]interface{}{"UNKNOWN"},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      + tests: [
      +     [0]: output<string>
        ]
    --outputs:--
  + testOut: output<string>
Resources:
    + 1 to create
    ~ 1 to update
    2 changes. 1 unchanged
`),
			autogold.Expect(map[string]interface{}{"tests": map[string]interface{}{}}))
	})

	t.Run("non-empty to unknown element", func(t *testing.T) {
		testDetailedDiffWithUnknowns(t, resMap, "${auxRes.aux}",
			[]interface{}{"val1"},
			[]interface{}{"UNKNOWN"},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          ~ [0]: "val1" => output<string>
        ]
Resources:
    + 1 to create
    ~ 1 to update
    2 changes. 1 unchanged
`),
			autogold.Expect(map[string]interface{}{"tests": map[string]interface{}{"kind": "UPDATE"}}))
	})

	t.Run("unknown element added front", func(t *testing.T) {
		testDetailedDiffWithUnknowns(t, resMap, "${auxRes.aux}",
			[]interface{}{"val2", "val3"},
			[]interface{}{"UNKNOWN", "val2", "val3"},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          ~ [0]: "val2" => output<string>
          ~ [1]: "val3" => "val2"
          + [2]: "val3"
        ]
Resources:
    + 1 to create
    ~ 1 to update
    2 changes. 1 unchanged
`),
			autogold.Expect(map[string]interface{}{"tests": map[string]interface{}{"kind": "UPDATE"}}),
		)
	})

	t.Run("unknown element added middle", func(t *testing.T) {
		testDetailedDiffWithUnknowns(t, resMap, "${auxRes.aux}",
			[]interface{}{"val1", "val3"},
			[]interface{}{"val1", "UNKNOWN", "val3"},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
            [0]: "val1"
          ~ [1]: "val3" => output<string>
          + [2]: "val3"
        ]
Resources:
    + 1 to create
    ~ 1 to update
    2 changes. 1 unchanged
`),
			autogold.Expect(map[string]interface{}{"tests": map[string]interface{}{"kind": "UPDATE"}}),
		)
	})

	t.Run("unknown element added end", func(t *testing.T) {
		testDetailedDiffWithUnknowns(t, resMap, "${auxRes.aux}",
			[]interface{}{"val1", "val2"},
			[]interface{}{"val1", "val2", "UNKNOWN"},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
            [0]: "val1"
            [1]: "val2"
          + [2]: output<string>
        ]
Resources:
    + 1 to create
    ~ 1 to update
    2 changes. 1 unchanged
`),
			autogold.Expect(map[string]interface{}{"tests": map[string]interface{}{"kind": "UPDATE"}}),
		)
	})

	t.Run("element updated to unknown", func(t *testing.T) {
		testDetailedDiffWithUnknowns(t, resMap, "${auxRes.aux}",
			[]interface{}{"val1", "val2", "val3"},
			[]interface{}{"val1", "UNKNOWN", "val3"},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
            [0]: "val1"
          ~ [1]: "val2" => output<string>
            [2]: "val3"
        ]
Resources:
    + 1 to create
    ~ 1 to update
    2 changes. 1 unchanged
`),
			autogold.Expect(map[string]interface{}{"tests": map[string]interface{}{"kind": "UPDATE"}}),
		)
	})

	t.Run("shuffled unknown added front", func(t *testing.T) {
		testDetailedDiffWithUnknowns(t, resMap, "${auxRes.aux}",
			[]interface{}{"val2", "val3"},
			[]interface{}{"UNKNOWN", "val3", "val2"},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          ~ [0]: "val2" => output<string>
            [1]: "val3"
          + [2]: "val2"
        ]
Resources:
    + 1 to create
    ~ 1 to update
    2 changes. 1 unchanged
`),
			autogold.Expect(map[string]interface{}{"tests": map[string]interface{}{"kind": "UPDATE"}}),
		)
	})

	t.Run("shuffled unknown added middle", func(t *testing.T) {
		testDetailedDiffWithUnknowns(t, resMap, "${auxRes.aux}",
			[]interface{}{"val1", "val3"},
			[]interface{}{"val3", "UNKNOWN", "val1"},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          ~ [0]: "val1" => "val3"
          ~ [1]: "val3" => output<string>
          + [2]: "val1"
        ]
Resources:
    + 1 to create
    ~ 1 to update
    2 changes. 1 unchanged
`),
			autogold.Expect(map[string]interface{}{"tests": map[string]interface{}{"kind": "UPDATE"}}),
		)
	})

	t.Run("shuffled unknown added end", func(t *testing.T) {
		testDetailedDiffWithUnknowns(t, resMap, "${auxRes.aux}",
			[]interface{}{"val1", "val2"},
			[]interface{}{"val2", "val1", "UNKNOWN"},
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          ~ [0]: "val1" => "val2"
          ~ [1]: "val2" => "val1"
          + [2]: output<string>
        ]
Resources:
    + 1 to create
    ~ 1 to update
    2 changes. 1 unchanged
`),
			autogold.Expect(map[string]interface{}{"tests": map[string]interface{}{"kind": "UPDATE"}}),
		)
	})
}

func TestUnknownSetAttributeDiff(t *testing.T) {
	// TODO: Remove this once accurate bridge previews are rolled out
	t.Setenv("PULUMI_TF_BRIDGE_ACCURATE_BRIDGE_PREVIEW", "true")
	resMap := map[string]*schema.Resource{
		"prov_test": {
			Schema: map[string]*schema.Schema{
				"test": {
					Type:     schema.TypeSet,
					Optional: true,
					Elem: &schema.Schema{
						Type: schema.TypeString,
					},
				},
			},
		},
		"prov_aux": {
			Schema: map[string]*schema.Schema{
				"aux": {
					Type:     schema.TypeSet,
					Computed: true,
					Optional: true,
					Elem: &schema.Schema{
						Type: schema.TypeString,
					},
				},
			},
			CreateContext: func(_ context.Context, d *schema.ResourceData, _ interface{}) diag.Diagnostics {
				d.SetId("aux")
				err := d.Set("aux", []interface{}{"aux"})
				require.NoError(t, err)
				return nil
			},
		},
	}

	t.Run("empty to unknown set", func(t *testing.T) {
		testDetailedDiffWithUnknowns(t, resMap, "${auxRes.auxes}",
			[]interface{}{},
			"UNKNOWN",
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      + tests: output<string>
    --outputs:--
  + testOut: output<string>
Resources:
    + 1 to create
    ~ 1 to update
    2 changes. 1 unchanged
`),
			autogold.Expect(map[string]interface{}{"tests": map[string]interface{}{}}),
		)
	})

	t.Run("non-empty to unknown set", func(t *testing.T) {
		testDetailedDiffWithUnknowns(t, resMap, "${auxRes.auxes}",
			[]interface{}{"val"},
			"UNKNOWN",
			autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      - tests: [
      -     [0]: "val"
        ]
      + tests: output<string>
Resources:
    + 1 to create
    ~ 1 to update
    2 changes. 1 unchanged
`),
			autogold.Expect(map[string]interface{}{"tests": map[string]interface{}{"kind": "UPDATE"}}),
		)
	})
}

// TODO: Cross test unknowns in sets
