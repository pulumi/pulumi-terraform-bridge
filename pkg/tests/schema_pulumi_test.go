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
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
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

func trimDiff(t *testing.T, diff string) string {
	urnLine := "    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]"
	resourcesSummaryLine := "Resources:\n"
	require.Contains(t, diff, urnLine)
	require.Contains(t, diff, resourcesSummaryLine)

	// trim the diff to only include the contents after the URN line and before the summary
	urnIndex := strings.Index(diff, urnLine)
	resourcesSummaryIndex := strings.Index(diff, resourcesSummaryLine)
	return diff[urnIndex+len(urnLine) : resourcesSummaryIndex]
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
			autogold.Expect(`
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    + prov:index/test:Test: (create)
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
        tests     : output<string>
`),
			autogold.Expect(`
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

			autogold.Expect(`
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    + prov:index/test:Test: (create)
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
        tests     : [
            [0]: output<string>
        ]
`),
			autogold.Expect(`
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

			autogold.Expect(`
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
`),
			autogold.Expect(`
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
			autogold.Expect(`
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    + prov:index/nestedTest:NestedTest: (create)
        [urn=urn:pulumi:test::test::prov:index/nestedTest:NestedTest::mainRes]
        tests     : output<string>
`),
			autogold.Expect(`
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
			autogold.Expect(`
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    + prov:index/nestedTest:NestedTest: (create)
        [urn=urn:pulumi:test::test::prov:index/nestedTest:NestedTest::mainRes]
        tests     : [
            [0]: output<string>
        ]
`),
			autogold.Expect(`
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
			autogold.Expect(`
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    + prov:index/nestedTest:NestedTest: (create)
        [urn=urn:pulumi:test::test::prov:index/nestedTest:NestedTest::mainRes]
        tests     : [
            [0]: {
                nestedProps: output<string>
            }
        ]
`),
			autogold.Expect(`
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
			autogold.Expect(`
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
`),
			autogold.Expect(`
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
			autogold.Expect(`
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
`),
			autogold.Expect(`
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
			autogold.Expect(`
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
`),
			autogold.Expect(`
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
`),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			computedProgram := fmt.Sprintf(tc.program, "null", "null")

			t.Run("initial preview", func(t *testing.T) {
				pt := pulcheck.PulCheck(t, bridgedProvider, computedProgram)
				res := pt.Preview(t, optpreview.Diff())
				t.Log(res.StdOut)

				tc.expectedInitial.Equal(t, trimDiff(t, res.StdOut))
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
				tc.expectedUpdate.Equal(t, trimDiff(t, res.StdOut))
			})

			t.Run("update preview with computed", func(t *testing.T) {
				pt := pulcheck.PulCheck(t, bridgedProvider, tc.initialKnownProgram)
				pt.Up(t)

				pulumiYamlPath := filepath.Join(pt.CurrentStack().Workspace().WorkDir(), "Pulumi.yaml")

				err := os.WriteFile(pulumiYamlPath, []byte(computedProgram), 0o600)
				require.NoError(t, err)

				res := pt.Preview(t, optpreview.Diff())
				t.Log(res.StdOut)
				tc.expectedUpdate.Equal(t, trimDiff(t, res.StdOut))
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
	// TODO[pulumi/pulumi-terraform-bridge#2517]: Remove this once accurate bridge previews are rolled out
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
			autogold.Expect("\n"),
		},
		{
			"string added",
			map[string]interface{}{},
			map[string]interface{}{"stringProp": "val"},
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      + stringProp: "val"
`),
		},
		{
			"string removed",
			map[string]interface{}{"stringProp": "val1"},
			map[string]interface{}{},
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      - stringProp: "val1"
`),
		},
		{
			"string changed",
			map[string]interface{}{"stringProp": "val1"},
			map[string]interface{}{"stringProp": "val2"},
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ stringProp: "val1" => "val2"
`),
		},
		{
			"list unchanged",
			map[string]interface{}{"listProps": []interface{}{"val"}},
			map[string]interface{}{"listProps": []interface{}{"val"}},
			autogold.Expect("\n"),
		},
		{
			"list added",
			map[string]interface{}{},
			map[string]interface{}{"listProps": []interface{}{"val"}},
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      + listProps: [
      +     [0]: "val"
        ]
`),
		},
		// pulumi/pulumi-terraform-bridge#2233: This is the intended behavior
		{
			"list added empty",
			map[string]interface{}{},
			map[string]interface{}{"listProps": []interface{}{}},
			autogold.Expect("\n"),
		},
		{
			"list removed",
			map[string]interface{}{"listProps": []interface{}{"val"}},
			map[string]interface{}{},
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      - listProps: [
      -     [0]: "val"
        ]
`),
		},
		// pulumi/pulumi-terraform-bridge#2233: This is the intended behavior
		{
			"list removed empty",
			map[string]interface{}{"listProps": []interface{}{}},
			map[string]interface{}{},
			autogold.Expect("\n"),
		},
		{
			"list element added front",
			map[string]interface{}{"listProps": []interface{}{"val2", "val3"}},
			map[string]interface{}{"listProps": []interface{}{"val1", "val2", "val3"}},
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ listProps: [
          ~ [0]: "val2" => "val1"
          ~ [1]: "val3" => "val2"
          + [2]: "val3"
        ]
`),
		},
		{
			"list element added back",
			map[string]interface{}{"listProps": []interface{}{"val1", "val2"}},
			map[string]interface{}{"listProps": []interface{}{"val1", "val2", "val3"}},
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ listProps: [
          + [2]: "val3"
        ]
`),
		},
		{
			"list element added middle",
			map[string]interface{}{"listProps": []interface{}{"val1", "val3"}},
			map[string]interface{}{"listProps": []interface{}{"val1", "val2", "val3"}},
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ listProps: [
          ~ [1]: "val3" => "val2"
          + [2]: "val3"
        ]
`),
		},
		{
			"list element removed front",
			map[string]interface{}{"listProps": []interface{}{"val1", "val2", "val3"}},
			map[string]interface{}{"listProps": []interface{}{"val2", "val3"}},
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ listProps: [
          ~ [0]: "val1" => "val2"
          ~ [1]: "val2" => "val3"
          - [2]: "val3"
        ]
`),
		},
		{
			"list element removed back",
			map[string]interface{}{"listProps": []interface{}{"val1", "val2", "val3"}},
			map[string]interface{}{"listProps": []interface{}{"val1", "val2"}},
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ listProps: [
          - [2]: "val3"
        ]
`),
		},
		{
			"list element removed middle",
			map[string]interface{}{"listProps": []interface{}{"val1", "val2", "val3"}},
			map[string]interface{}{"listProps": []interface{}{"val1", "val3"}},
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ listProps: [
          ~ [1]: "val2" => "val3"
          - [2]: "val3"
        ]
`),
		},
		{
			"list element changed",
			map[string]interface{}{"listProps": []interface{}{"val1"}},
			map[string]interface{}{"listProps": []interface{}{"val2"}},
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ listProps: [
          ~ [0]: "val1" => "val2"
        ]
`),
		},
		{
			"set unchanged",
			map[string]interface{}{"setProps": []interface{}{"val"}},
			map[string]interface{}{"setProps": []interface{}{"val"}},
			autogold.Expect("\n"),
		},
		{
			"set added",
			map[string]interface{}{},
			map[string]interface{}{"setProps": []interface{}{"val"}},
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      + setProps: [
      +     [0]: "val"
        ]
`),
		},
		// pulumi/pulumi-terraform-bridge#2233: This is the intended behavior
		{
			"set added empty",
			map[string]interface{}{},
			map[string]interface{}{"setProps": []interface{}{}},
			autogold.Expect("\n"),
		},
		{
			"set removed",
			map[string]interface{}{"setProps": []interface{}{"val"}},
			map[string]interface{}{},
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      - setProps: [
      -     [0]: "val"
        ]
`),
		},
		// pulumi/pulumi-terraform-bridge#2233: This is the intended behavior
		{
			"set removed empty",
			map[string]interface{}{"setProps": []interface{}{}},
			map[string]interface{}{},
			autogold.Expect("\n"),
		},
		{
			"set element added front",
			map[string]interface{}{"setProps": []interface{}{"val2", "val3"}},
			map[string]interface{}{"setProps": []interface{}{"val1", "val2", "val3"}},
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ setProps: [
          + [0]: "val1"
        ]
`),
		},
		{
			"set element added back",
			map[string]interface{}{"setProps": []interface{}{"val1", "val2"}},
			map[string]interface{}{"setProps": []interface{}{"val1", "val2", "val3"}},
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ setProps: [
          + [2]: "val3"
        ]
`),
		},
		{
			"set element added middle",
			map[string]interface{}{"setProps": []interface{}{"val1", "val3"}},
			map[string]interface{}{"setProps": []interface{}{"val1", "val2", "val3"}},
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ setProps: [
          + [1]: "val2"
        ]
`),
		},
		{
			"set element removed front",
			map[string]interface{}{"setProps": []interface{}{"val1", "val2", "val3"}},
			map[string]interface{}{"setProps": []interface{}{"val2", "val3"}},
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ setProps: [
          - [0]: "val1"
        ]
`),
		},
		{
			"set element removed back",
			map[string]interface{}{"setProps": []interface{}{"val1", "val2", "val3"}},
			map[string]interface{}{"setProps": []interface{}{"val1", "val2"}},
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ setProps: [
          - [2]: "val3"
        ]
`),
		},
		{
			"set element removed middle",
			map[string]interface{}{"setProps": []interface{}{"val1", "val2", "val3"}},
			map[string]interface{}{"setProps": []interface{}{"val1", "val3"}},
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ setProps: [
          - [1]: "val2"
        ]
`),
		},
		{
			"set element changed",
			map[string]interface{}{"setProps": []interface{}{"val1"}},
			map[string]interface{}{"setProps": []interface{}{"val2"}},
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ setProps: [
          ~ [0]: "val1" => "val2"
        ]
`),
		},
		{
			"map unchanged",
			map[string]interface{}{"mapProp": map[string]interface{}{"key": "val"}},
			map[string]interface{}{"mapProp": map[string]interface{}{"key": "val"}},
			autogold.Expect("\n"),
		},
		{
			"map added",
			map[string]interface{}{},
			map[string]interface{}{"mapProp": map[string]interface{}{"key": "val"}},
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      + mapProp: {
          + key: "val"
        }
`),
		},
		// pulumi/pulumi-terraform-bridge#2233: This is the intended behavior
		{
			"map added empty",
			map[string]interface{}{},
			map[string]interface{}{"mapProp": map[string]interface{}{}},
			autogold.Expect("\n"),
		},
		{
			"map removed",
			map[string]interface{}{"mapProp": map[string]interface{}{"key": "val"}},
			map[string]interface{}{},
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      - mapProp: {
          - key: "val"
        }
`),
		},
		// pulumi/pulumi-terraform-bridge#2233: This is the intended behavior
		{
			"map removed empty",
			map[string]interface{}{"mapProp": map[string]interface{}{}},
			map[string]interface{}{},
			autogold.Expect("\n"),
		},
		{
			"map element added",
			map[string]interface{}{"mapProp": map[string]interface{}{}},
			map[string]interface{}{"mapProp": map[string]interface{}{"key": "val"}},
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      + mapProp: {
          + key: "val"
        }
`),
		},
		{
			"map element removed",
			map[string]interface{}{"mapProp": map[string]interface{}{"key": "val"}},
			map[string]interface{}{"mapProp": map[string]interface{}{}},
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ mapProp: {
          - key: "val"
        }
`),
		},
		{
			"map value changed",
			map[string]interface{}{"mapProp": map[string]interface{}{"key": "val1"}},
			map[string]interface{}{"mapProp": map[string]interface{}{"key": "val2"}},
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ mapProp: {
          ~ key: "val1" => "val2"
        }
`),
		},
		{
			"map key changed",
			map[string]interface{}{"mapProp": map[string]interface{}{"key1": "val"}},
			map[string]interface{}{"mapProp": map[string]interface{}{"key2": "val"}},
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ mapProp: {
          - key1: "val"
          + key2: "val"
        }
`),
		},
		{
			"list block unchanged",
			map[string]interface{}{"listBlocks": []interface{}{map[string]interface{}{"prop": "val"}}},
			map[string]interface{}{"listBlocks": []interface{}{map[string]interface{}{"prop": "val"}}},
			autogold.Expect("\n"),
		},
		{
			"list block added",
			map[string]interface{}{},
			map[string]interface{}{"listBlocks": []interface{}{map[string]interface{}{"prop": "val"}}},
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      + listBlocks: [
      +     [0]: {
              + prop      : "val"
            }
        ]
`),
		},
		// This is expected to be a no-op because blocks can not be nil in TF
		{
			"list block added empty",
			map[string]interface{}{},
			map[string]interface{}{"listBlocks": []interface{}{}},
			autogold.Expect("\n"),
		},
		{
			"list block added empty object",
			map[string]interface{}{},
			map[string]interface{}{"listBlocks": []interface{}{map[string]interface{}{}}},
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      + listBlocks: [
      +     [0]: {
            }
        ]
`),
		},
		{
			"list block removed",
			map[string]interface{}{"listBlocks": []interface{}{map[string]interface{}{"prop": "val"}}},
			map[string]interface{}{},
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      - listBlocks: [
      -     [0]: {
              - prop: "val"
            }
        ]
`),
		},
		// This is expected to be a no-op because blocks can not be nil in TF
		{
			"list block removed empty",
			map[string]interface{}{"listBlocks": []interface{}{}},
			map[string]interface{}{},
			autogold.Expect("\n"),
		},
		{
			"list block removed empty object",
			map[string]interface{}{"listBlocks": []interface{}{map[string]interface{}{}}},
			map[string]interface{}{},
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      - listBlocks: [
      -     [0]: {
              - prop: <null>
            }
        ]
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
			autogold.Expect(`
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
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ listBlocks: [
          + [2]: {
                  + prop      : "val3"
                }
        ]
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
			autogold.Expect(`
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
			autogold.Expect(`
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
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ listBlocks: [
          - [2]: {
                  - prop: "val3"
                }
        ]
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
			autogold.Expect(`
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
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ listBlocks: [
          ~ [0]: {
                  ~ prop: "val1" => "val2"
                }
        ]
`),
		},
		{
			"set block unchanged",
			map[string]interface{}{"setBlocks": []interface{}{map[string]interface{}{"prop": "val"}}},
			map[string]interface{}{"setBlocks": []interface{}{map[string]interface{}{"prop": "val"}}},
			autogold.Expect("\n"),
		},
		{
			"set block added",
			map[string]interface{}{},
			map[string]interface{}{"setBlocks": []interface{}{map[string]interface{}{"prop": "val"}}},
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      + setBlocks: [
      +     [0]: {
              + prop      : "val"
            }
        ]
`),
		},
		// This is expected to be a no-op because blocks can not be nil in TF
		{
			"set block added empty",
			map[string]interface{}{},
			map[string]interface{}{"setBlocks": []interface{}{}},
			autogold.Expect("\n"),
		},
		{
			"set block added empty object",
			map[string]interface{}{},
			map[string]interface{}{"setBlocks": []interface{}{map[string]interface{}{}}},
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      + setBlocks: [
      +     [0]: {
            }
        ]
`),
		},
		{
			"set block removed",
			map[string]interface{}{"setBlocks": []interface{}{map[string]interface{}{"prop": "val"}}},
			map[string]interface{}{},
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      - setBlocks: [
      -     [0]: {
              - prop: "val"
            }
        ]
`),
		},
		// This is expected to be a no-op because blocks can not be nil in TF
		{
			"set block removed empty",
			map[string]interface{}{"setBlocks": []interface{}{}},
			map[string]interface{}{},
			autogold.Expect("\n"),
		},
		// TODO[pulumi/pulumi-terraform-bridge#2399] nested prop diff
		{
			"set block removed empty object",
			map[string]interface{}{"setBlocks": []interface{}{map[string]interface{}{}}},
			map[string]interface{}{},
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      - setBlocks: [
      -     [0]: {
              - prop: ""
            }
        ]
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
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ setBlocks: [
          + [0]: {
                  + prop      : "val1"
                }
        ]
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
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ setBlocks: [
          + [2]: {
                  + prop      : "val3"
                }
        ]
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
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ setBlocks: [
          + [1]: {
                  + prop      : "val2"
                }
        ]
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
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ setBlocks: [
          - [0]: {
                  - prop: "val1"
                }
        ]
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
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ setBlocks: [
          - [2]: {
                  - prop: "val3"
                }
        ]
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
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ setBlocks: [
          - [1]: {
                  - prop: "val2"
                }
        ]
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
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ setBlocks: [
          ~ [0]: {
                  ~ prop: "val1" => "val2"
                }
        ]
`),
		},
		{
			"maxItemsOne block unchanged",
			map[string]interface{}{"maxItemsOneBlock": map[string]interface{}{"prop": "val"}},
			map[string]interface{}{"maxItemsOneBlock": map[string]interface{}{"prop": "val"}},
			autogold.Expect("\n"),
		},
		{
			"maxItemsOne block added",
			map[string]interface{}{},
			map[string]interface{}{"maxItemsOneBlock": map[string]interface{}{"prop": "val"}},
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      + maxItemsOneBlock: {
          + prop      : "val"
        }
`),
		},
		{
			"maxItemsOne block added empty",
			map[string]interface{}{},
			map[string]interface{}{"maxItemsOneBlock": map[string]interface{}{}},
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      + maxItemsOneBlock: {
        }
`),
		},
		{
			"maxItemsOne block removed",
			map[string]interface{}{"maxItemsOneBlock": map[string]interface{}{"prop": "val"}},
			map[string]interface{}{},
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      - maxItemsOneBlock: {
          - prop: "val"
        }
`),
		},
		// TODO[pulumi/pulumi-terraform-bridge#2399] nested prop diff
		{
			"maxItemsOne block removed empty",
			map[string]interface{}{"maxItemsOneBlock": map[string]interface{}{}},
			map[string]interface{}{},
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      - maxItemsOneBlock: {
          - prop: <null>
        }
`),
		},
		{
			"maxItemsOne block changed",
			map[string]interface{}{"maxItemsOneBlock": map[string]interface{}{"prop": "val1"}},
			map[string]interface{}{"maxItemsOneBlock": map[string]interface{}{"prop": "val2"}},
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ maxItemsOneBlock: {
          ~ prop: "val1" => "val2"
        }
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
			tc.expected.Equal(t, trimDiff(t, res.StdOut))
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
	// TODO[pulumi/pulumi-terraform-bridge#2517]: Remove this once accurate bridge previews are rolled out
	t.Setenv("PULUMI_TF_BRIDGE_ACCURATE_BRIDGE_PREVIEW", "true")
	runTest := func(t *testing.T, resMap map[string]*schema.Resource, props1, props2 interface{},
		expected, expectedDetailedDiff autogold.Value,
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

		expected.Equal(t, trimDiff(t, out))
		expectedDetailedDiff.Equal(t, detailedDiff)
	}

	// The following test cases use the same inputs (props1 and props2) to test a few variations:
	// - The diff when the schema is a set of strings
	// - The diff when the schema is a set of strings with ForceNew
	// - The diff when the schema is a set of structs with a nested string
	// - The diff when the schema is a set of structs with a nested string and ForceNew
	// For each of these variations, we record both the detailed diff output sent to the engine
	// and the output that we expect to see in the Pulumi console.
	type setDetailedDiffTestCase struct {
		name                              string
		props1                            []string
		props2                            []string
		expectedAttrDetailedDiff          autogold.Value
		expectedAttr                      autogold.Value
		expectedAttrForceNewDetailedDiff  autogold.Value
		expectedAttrForceNew              autogold.Value
		expectedBlockDetailedDiff         autogold.Value
		expectedBlock                     autogold.Value
		expectedBlockForceNewDetailedDiff autogold.Value
		expectedBlockForceNew             autogold.Value
	}

	testCases := []setDetailedDiffTestCase{
		{
			name:                              "unchanged",
			props1:                            []string{"val1"},
			props2:                            []string{"val1"},
			expectedAttrDetailedDiff:          autogold.Expect(map[string]interface{}{}),
			expectedAttr:                      autogold.Expect("\n"),
			expectedAttrForceNewDetailedDiff:  autogold.Expect(map[string]interface{}{}),
			expectedAttrForceNew:              autogold.Expect("\n"),
			expectedBlockDetailedDiff:         autogold.Expect(map[string]interface{}{}),
			expectedBlock:                     autogold.Expect("\n"),
			expectedBlockForceNewDetailedDiff: autogold.Expect(map[string]interface{}{}),
			expectedBlockForceNew:             autogold.Expect("\n"),
		},
		{
			name:                     "changed non-empty",
			props1:                   []string{"val1"},
			props2:                   []string{"val2"},
			expectedAttrDetailedDiff: autogold.Expect(map[string]interface{}{"tests[0]": map[string]interface{}{"kind": "UPDATE"}}),
			expectedAttr: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          ~ [0]: "val1" => "val2"
        ]
`),
			expectedAttrForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests[0]": map[string]interface{}{"kind": "UPDATE_REPLACE"}}),
			expectedAttrForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          ~ [0]: "val1" => "val2"
        ]
`),
			expectedBlockDetailedDiff: autogold.Expect(map[string]interface{}{"tests[0].nested": map[string]interface{}{"kind": "UPDATE"}}),
			expectedBlock: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          ~ [0]: {
                  ~ nested: "val1" => "val2"
                }
        ]
`),
			expectedBlockForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests[0].nested": map[string]interface{}{"kind": "UPDATE_REPLACE"}}),
			expectedBlockForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          ~ [0]: {
                  ~ nested: "val1" => "val2"
                }
        ]
`),
		},
		{
			name:                     "changed from empty",
			props1:                   []string{},
			props2:                   []string{"val1"},
			expectedAttrDetailedDiff: autogold.Expect(map[string]interface{}{"tests": map[string]interface{}{}}),
			expectedAttr: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      + tests: [
      +     [0]: "val1"
        ]
`),
			expectedAttrForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests": map[string]interface{}{"kind": "ADD_REPLACE"}}),
			expectedAttrForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      + tests: [
      +     [0]: "val1"
        ]
`),
			expectedBlockDetailedDiff: autogold.Expect(map[string]interface{}{"tests": map[string]interface{}{}}),
			expectedBlock: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      + tests: [
      +     [0]: {
              + nested    : "val1"
            }
        ]
`),
			expectedBlockForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests": map[string]interface{}{"kind": "ADD_REPLACE"}}),
			expectedBlockForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      + tests: [
      +     [0]: {
              + nested    : "val1"
            }
        ]
`),
		},
		{
			name:                     "changed to empty",
			props1:                   []string{"val1"},
			props2:                   []string{},
			expectedAttrDetailedDiff: autogold.Expect(map[string]interface{}{"tests": map[string]interface{}{"kind": "DELETE"}}),
			expectedAttr: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      - tests: [
      -     [0]: "val1"
        ]
`),
			expectedAttrForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests": map[string]interface{}{"kind": "DELETE_REPLACE"}}),
			expectedAttrForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      - tests: [
      -     [0]: "val1"
        ]
`),
			expectedBlockDetailedDiff: autogold.Expect(map[string]interface{}{"tests": map[string]interface{}{"kind": "DELETE"}}),
			expectedBlock: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      - tests: [
      -     [0]: {
              - nested: "val1"
            }
        ]
`),
			expectedBlockForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests": map[string]interface{}{"kind": "DELETE_REPLACE"}}),
			expectedBlockForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      - tests: [
      -     [0]: {
              - nested: "val1"
            }
        ]
`),
		},
		{
			name:                     "removed front",
			props1:                   []string{"val1", "val2", "val3"},
			props2:                   []string{"val2", "val3"},
			expectedAttrDetailedDiff: autogold.Expect(map[string]interface{}{"tests[0]": map[string]interface{}{"kind": "DELETE"}}),
			expectedAttr: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [0]: "val1"
        ]
`),
			expectedAttrForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests[0]": map[string]interface{}{"kind": "DELETE_REPLACE"}}),
			expectedAttrForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [0]: "val1"
        ]
`),
			expectedBlockDetailedDiff: autogold.Expect(map[string]interface{}{"tests[0]": map[string]interface{}{"kind": "DELETE"}}),
			expectedBlock: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [0]: {
                  - nested: "val1"
                }
        ]
`),
			expectedBlockForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests[0]": map[string]interface{}{"kind": "DELETE_REPLACE"}}),
			expectedBlockForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [0]: {
                  - nested: "val1"
                }
        ]
`),
		},
		{
			name:                     "removed front unordered",
			props1:                   []string{"val2", "val1", "val3"},
			props2:                   []string{"val1", "val3"},
			expectedAttrDetailedDiff: autogold.Expect(map[string]interface{}{"tests[1]": map[string]interface{}{"kind": "DELETE"}}),
			expectedAttr: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [1]: "val2"
        ]
`),
			expectedAttrForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests[1]": map[string]interface{}{"kind": "DELETE_REPLACE"}}),
			expectedAttrForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [1]: "val2"
        ]
`),
			expectedBlockDetailedDiff: autogold.Expect(map[string]interface{}{"tests[1]": map[string]interface{}{"kind": "DELETE"}}),
			expectedBlock: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [1]: {
                  - nested: "val2"
                }
        ]
`),
			expectedBlockForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests[1]": map[string]interface{}{"kind": "DELETE_REPLACE"}}),
			expectedBlockForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [1]: {
                  - nested: "val2"
                }
        ]
`),
		},
		{
			name:                     "removed middle",
			props1:                   []string{"val1", "val2", "val3"},
			props2:                   []string{"val1", "val3"},
			expectedAttrDetailedDiff: autogold.Expect(map[string]interface{}{"tests[1]": map[string]interface{}{"kind": "DELETE"}}),
			expectedAttr: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [1]: "val2"
        ]
`),
			expectedAttrForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests[1]": map[string]interface{}{"kind": "DELETE_REPLACE"}}),
			expectedAttrForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [1]: "val2"
        ]
`),
			expectedBlockDetailedDiff: autogold.Expect(map[string]interface{}{"tests[1]": map[string]interface{}{"kind": "DELETE"}}),
			expectedBlock: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [1]: {
                  - nested: "val2"
                }
        ]
`),
			expectedBlockForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests[1]": map[string]interface{}{"kind": "DELETE_REPLACE"}}),
			expectedBlockForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [1]: {
                  - nested: "val2"
                }
        ]
`),
		},
		{
			name:                     "removed middle unordered",
			props1:                   []string{"val2", "val3", "val1"},
			props2:                   []string{"val2", "val1"},
			expectedAttrDetailedDiff: autogold.Expect(map[string]interface{}{"tests[2]": map[string]interface{}{"kind": "DELETE"}}),
			expectedAttr: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [2]: "val3"
        ]
`),
			expectedAttrForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests[2]": map[string]interface{}{"kind": "DELETE_REPLACE"}}),
			expectedAttrForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [2]: "val3"
        ]
`),
			expectedBlockDetailedDiff: autogold.Expect(map[string]interface{}{"tests[2]": map[string]interface{}{"kind": "DELETE"}}),
			expectedBlock: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [2]: {
                  - nested: "val3"
                }
        ]
`),
			expectedBlockForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests[2]": map[string]interface{}{"kind": "DELETE_REPLACE"}}),
			expectedBlockForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [2]: {
                  - nested: "val3"
                }
        ]
`),
		},
		{
			name:                     "removed end",
			props1:                   []string{"val1", "val2", "val3"},
			props2:                   []string{"val1", "val2"},
			expectedAttrDetailedDiff: autogold.Expect(map[string]interface{}{"tests[2]": map[string]interface{}{"kind": "DELETE"}}),
			expectedAttr: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [2]: "val3"
        ]
`),
			expectedAttrForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests[2]": map[string]interface{}{"kind": "DELETE_REPLACE"}}),
			expectedAttrForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [2]: "val3"
        ]
`),
			expectedBlockDetailedDiff: autogold.Expect(map[string]interface{}{"tests[2]": map[string]interface{}{"kind": "DELETE"}}),
			expectedBlock: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [2]: {
                  - nested: "val3"
                }
        ]
`),
			expectedBlockForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests[2]": map[string]interface{}{"kind": "DELETE_REPLACE"}}),
			expectedBlockForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [2]: {
                  - nested: "val3"
                }
        ]
`),
		},
		{
			name:                     "removed end unordered",
			props1:                   []string{"val2", "val3", "val1"},
			props2:                   []string{"val2", "val3"},
			expectedAttrDetailedDiff: autogold.Expect(map[string]interface{}{"tests[0]": map[string]interface{}{"kind": "DELETE"}}),
			expectedAttr: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [0]: "val1"
        ]
`),
			expectedAttrForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests[0]": map[string]interface{}{"kind": "DELETE_REPLACE"}}),
			expectedAttrForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [0]: "val1"
        ]
`),
			expectedBlockDetailedDiff: autogold.Expect(map[string]interface{}{"tests[0]": map[string]interface{}{"kind": "DELETE"}}),
			expectedBlock: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [0]: {
                  - nested: "val1"
                }
        ]
`),
			expectedBlockForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests[0]": map[string]interface{}{"kind": "DELETE_REPLACE"}}),
			expectedBlockForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [0]: {
                  - nested: "val1"
                }
        ]
`),
		},
		{
			name:                     "added front",
			props1:                   []string{"val2", "val3"},
			props2:                   []string{"val1", "val2", "val3"},
			expectedAttrDetailedDiff: autogold.Expect(map[string]interface{}{"tests[0]": map[string]interface{}{}}),
			expectedAttr: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [0]: "val1"
        ]
`),
			expectedAttrForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests[0]": map[string]interface{}{"kind": "ADD_REPLACE"}}),
			expectedAttrForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [0]: "val1"
        ]
`),
			expectedBlockDetailedDiff: autogold.Expect(map[string]interface{}{"tests[0]": map[string]interface{}{}}),
			expectedBlock: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [0]: {
                  + nested    : "val1"
                }
        ]
`),
			expectedBlockForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests[0]": map[string]interface{}{"kind": "ADD_REPLACE"}}),
			expectedBlockForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [0]: {
                  + nested    : "val1"
                }
        ]
`),
		},
		{
			name:                     "added front unordered",
			props1:                   []string{"val3", "val1"},
			props2:                   []string{"val2", "val2", "val1"},
			expectedAttrDetailedDiff: autogold.Expect(map[string]interface{}{"tests[1]": map[string]interface{}{"kind": "UPDATE"}}),
			expectedAttr: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          ~ [1]: "val3" => "val2"
        ]
`),
			expectedAttrForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests[1]": map[string]interface{}{"kind": "UPDATE_REPLACE"}}),
			expectedAttrForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          ~ [1]: "val3" => "val2"
        ]
`),
			expectedBlockDetailedDiff: autogold.Expect(map[string]interface{}{"tests[1].nested": map[string]interface{}{"kind": "UPDATE"}}),
			expectedBlock: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          ~ [1]: {
                  ~ nested: "val3" => "val2"
                }
        ]
`),
			expectedBlockForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests[1].nested": map[string]interface{}{"kind": "UPDATE_REPLACE"}}),
			expectedBlockForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          ~ [1]: {
                  ~ nested: "val3" => "val2"
                }
        ]
`),
		},
		{
			name:                     "added middle",
			props1:                   []string{"val1", "val3"},
			props2:                   []string{"val1", "val2", "val3"},
			expectedAttrDetailedDiff: autogold.Expect(map[string]interface{}{"tests[1]": map[string]interface{}{}}),
			expectedAttr: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [1]: "val2"
        ]
`),
			expectedAttrForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests[1]": map[string]interface{}{"kind": "ADD_REPLACE"}}),
			expectedAttrForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [1]: "val2"
        ]
`),
			expectedBlockDetailedDiff: autogold.Expect(map[string]interface{}{"tests[1]": map[string]interface{}{}}),
			expectedBlock: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [1]: {
                  + nested    : "val2"
                }
        ]
`),
			expectedBlockForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests[1]": map[string]interface{}{"kind": "ADD_REPLACE"}}),
			expectedBlockForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [1]: {
                  + nested    : "val2"
                }
        ]
`),
		},
		{
			name:                     "added middle unordered",
			props1:                   []string{"val2", "val1"},
			props2:                   []string{"val2", "val3", "val1"},
			expectedAttrDetailedDiff: autogold.Expect(map[string]interface{}{"tests[1]": map[string]interface{}{}}),
			expectedAttr: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [1]: "val3"
        ]
`),
			expectedAttrForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests[1]": map[string]interface{}{"kind": "ADD_REPLACE"}}),
			expectedAttrForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [1]: "val3"
        ]
`),
			expectedBlockDetailedDiff: autogold.Expect(map[string]interface{}{"tests[1]": map[string]interface{}{}}),
			expectedBlock: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [1]: {
                  + nested    : "val3"
                }
        ]
`),
			expectedBlockForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests[1]": map[string]interface{}{"kind": "ADD_REPLACE"}}),
			expectedBlockForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [1]: {
                  + nested    : "val3"
                }
        ]
`),
		},
		{
			name:                     "added end",
			props1:                   []string{"val1", "val2"},
			props2:                   []string{"val1", "val2", "val3"},
			expectedAttrDetailedDiff: autogold.Expect(map[string]interface{}{"tests[2]": map[string]interface{}{}}),
			expectedAttr: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [2]: "val3"
        ]
`),
			expectedAttrForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests[2]": map[string]interface{}{"kind": "ADD_REPLACE"}}),
			expectedAttrForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [2]: "val3"
        ]
`),
			expectedBlockDetailedDiff: autogold.Expect(map[string]interface{}{"tests[2]": map[string]interface{}{}}),
			expectedBlock: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [2]: {
                  + nested    : "val3"
                }
        ]
`),
			expectedBlockForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests[2]": map[string]interface{}{"kind": "ADD_REPLACE"}}),
			expectedBlockForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [2]: {
                  + nested    : "val3"
                }
        ]
`),
		},
		{
			name:                     "added end unordered",
			props1:                   []string{"val2", "val3"},
			props2:                   []string{"val2", "val3", "val1"},
			expectedAttrDetailedDiff: autogold.Expect(map[string]interface{}{"tests[2]": map[string]interface{}{}}),
			expectedAttr: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [2]: "val1"
        ]
`),
			expectedAttrForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests[2]": map[string]interface{}{"kind": "ADD_REPLACE"}}),
			expectedAttrForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [2]: "val1"
        ]
`),
			expectedBlockDetailedDiff: autogold.Expect(map[string]interface{}{"tests[2]": map[string]interface{}{}}),
			expectedBlock: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [2]: {
                  + nested    : "val1"
                }
        ]
`),
			expectedBlockForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests[2]": map[string]interface{}{"kind": "ADD_REPLACE"}}),
			expectedBlockForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [2]: {
                  + nested    : "val1"
                }
        ]
`),
		},
		{
			name:                     "same element updated",
			props1:                   []string{"val1", "val2", "val3"},
			props2:                   []string{"val1", "val4", "val3"},
			expectedAttrDetailedDiff: autogold.Expect(map[string]interface{}{"tests[1]": map[string]interface{}{"kind": "UPDATE"}}),
			expectedAttr: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          ~ [1]: "val2" => "val4"
        ]
`),
			expectedAttrForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests[1]": map[string]interface{}{"kind": "UPDATE_REPLACE"}}),
			expectedAttrForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          ~ [1]: "val2" => "val4"
        ]
`),
			expectedBlockDetailedDiff: autogold.Expect(map[string]interface{}{"tests[1].nested": map[string]interface{}{"kind": "UPDATE"}}),
			expectedBlock: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          ~ [1]: {
                  ~ nested: "val2" => "val4"
                }
        ]
`),
			expectedBlockForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests[1].nested": map[string]interface{}{"kind": "UPDATE_REPLACE"}}),
			expectedBlockForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          ~ [1]: {
                  ~ nested: "val2" => "val4"
                }
        ]
`),
		},
		{
			name:   "same element updated unordered",
			props1: []string{"val2", "val3", "val1"},
			props2: []string{"val2", "val4", "val1"},
			expectedAttrDetailedDiff: autogold.Expect(map[string]interface{}{
				"tests[1]": map[string]interface{}{},
				"tests[2]": map[string]interface{}{"kind": "DELETE"},
			}),
			expectedAttr: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [1]: "val4"
          - [2]: "val3"
        ]
`),
			expectedAttrForceNewDetailedDiff: autogold.Expect(map[string]interface{}{
				"tests[1]": map[string]interface{}{"kind": "ADD_REPLACE"},
				"tests[2]": map[string]interface{}{"kind": "DELETE_REPLACE"},
			}),
			expectedAttrForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [1]: "val4"
          - [2]: "val3"
        ]
`),
			expectedBlockDetailedDiff: autogold.Expect(map[string]interface{}{
				"tests[1]": map[string]interface{}{},
				"tests[2]": map[string]interface{}{"kind": "DELETE"},
			}),
			expectedBlock: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [1]: {
                  + nested    : "val4"
                }
          - [2]: {
                  - nested: "val3"
                }
        ]
`),
			expectedBlockForceNewDetailedDiff: autogold.Expect(map[string]interface{}{
				"tests[1]": map[string]interface{}{"kind": "ADD_REPLACE"},
				"tests[2]": map[string]interface{}{"kind": "DELETE_REPLACE"},
			}),
			expectedBlockForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [1]: {
                  + nested    : "val4"
                }
          - [2]: {
                  - nested: "val3"
                }
        ]
`),
		},
		{
			name:                              "shuffled",
			props1:                            []string{"val1", "val2", "val3"},
			props2:                            []string{"val3", "val1", "val2"},
			expectedAttrDetailedDiff:          autogold.Expect(map[string]interface{}{}),
			expectedAttr:                      autogold.Expect("\n"),
			expectedAttrForceNewDetailedDiff:  autogold.Expect(map[string]interface{}{}),
			expectedAttrForceNew:              autogold.Expect("\n"),
			expectedBlockDetailedDiff:         autogold.Expect(map[string]interface{}{}),
			expectedBlock:                     autogold.Expect("\n"),
			expectedBlockForceNewDetailedDiff: autogold.Expect(map[string]interface{}{}),
			expectedBlockForceNew:             autogold.Expect("\n"),
		},
		{
			name:                              "shuffled unordered",
			props1:                            []string{"val2", "val3", "val1"},
			props2:                            []string{"val3", "val1", "val2"},
			expectedAttrDetailedDiff:          autogold.Expect(map[string]interface{}{}),
			expectedAttr:                      autogold.Expect("\n"),
			expectedAttrForceNewDetailedDiff:  autogold.Expect(map[string]interface{}{}),
			expectedAttrForceNew:              autogold.Expect("\n"),
			expectedBlockDetailedDiff:         autogold.Expect(map[string]interface{}{}),
			expectedBlock:                     autogold.Expect("\n"),
			expectedBlockForceNewDetailedDiff: autogold.Expect(map[string]interface{}{}),
			expectedBlockForceNew:             autogold.Expect("\n"),
		},
		{
			name:                              "shuffled with duplicates",
			props1:                            []string{"val1", "val2", "val3"},
			props2:                            []string{"val3", "val1", "val2", "val3"},
			expectedAttrDetailedDiff:          autogold.Expect(map[string]interface{}{}),
			expectedAttr:                      autogold.Expect("\n"),
			expectedAttrForceNewDetailedDiff:  autogold.Expect(map[string]interface{}{}),
			expectedAttrForceNew:              autogold.Expect("\n"),
			expectedBlockDetailedDiff:         autogold.Expect(map[string]interface{}{}),
			expectedBlock:                     autogold.Expect("\n"),
			expectedBlockForceNewDetailedDiff: autogold.Expect(map[string]interface{}{}),
			expectedBlockForceNew:             autogold.Expect("\n"),
		},
		{
			name:                              "shuffled with duplicates unordered",
			props1:                            []string{"val2", "val3", "val1"},
			props2:                            []string{"val3", "val1", "val2", "val3"},
			expectedAttrDetailedDiff:          autogold.Expect(map[string]interface{}{}),
			expectedAttr:                      autogold.Expect("\n"),
			expectedAttrForceNewDetailedDiff:  autogold.Expect(map[string]interface{}{}),
			expectedAttrForceNew:              autogold.Expect("\n"),
			expectedBlockDetailedDiff:         autogold.Expect(map[string]interface{}{}),
			expectedBlock:                     autogold.Expect("\n"),
			expectedBlockForceNewDetailedDiff: autogold.Expect(map[string]interface{}{}),
			expectedBlockForceNew:             autogold.Expect("\n"),
		},
		{
			name:                     "shuffled added front",
			props1:                   []string{"val2", "val3"},
			props2:                   []string{"val1", "val3", "val2"},
			expectedAttrDetailedDiff: autogold.Expect(map[string]interface{}{"tests[0]": map[string]interface{}{}}),
			expectedAttr: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [0]: "val1"
        ]
`),
			expectedAttrForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests[0]": map[string]interface{}{"kind": "ADD_REPLACE"}}),
			expectedAttrForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [0]: "val1"
        ]
`),
			expectedBlockDetailedDiff: autogold.Expect(map[string]interface{}{"tests[0]": map[string]interface{}{}}),
			expectedBlock: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [0]: {
                  + nested    : "val1"
                }
        ]
`),
			expectedBlockForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests[0]": map[string]interface{}{"kind": "ADD_REPLACE"}}),
			expectedBlockForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [0]: {
                  + nested    : "val1"
                }
        ]
`),
		},
		{
			name:                     "shuffled added front unordered",
			props1:                   []string{"val3", "val1"},
			props2:                   []string{"val2", "val1", "val3"},
			expectedAttrDetailedDiff: autogold.Expect(map[string]interface{}{"tests[0]": map[string]interface{}{}}),
			expectedAttr: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [0]: "val2"
        ]
`),
			expectedAttrForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests[0]": map[string]interface{}{"kind": "ADD_REPLACE"}}),
			expectedAttrForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [0]: "val2"
        ]
`),
			expectedBlockDetailedDiff: autogold.Expect(map[string]interface{}{"tests[0]": map[string]interface{}{}}),
			expectedBlock: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [0]: {
                  + nested    : "val2"
                }
        ]
`),
			expectedBlockForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests[0]": map[string]interface{}{"kind": "ADD_REPLACE"}}),
			expectedBlockForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [0]: {
                  + nested    : "val2"
                }
        ]
`),
		},
		{
			name:                     "shuffled added middle",
			props1:                   []string{"val1", "val3"},
			props2:                   []string{"val3", "val2", "val1"},
			expectedAttrDetailedDiff: autogold.Expect(map[string]interface{}{"tests[1]": map[string]interface{}{}}),
			expectedAttr: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [1]: "val2"
        ]
`),
			expectedAttrForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests[1]": map[string]interface{}{"kind": "ADD_REPLACE"}}),
			expectedAttrForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [1]: "val2"
        ]
`),
			expectedBlockDetailedDiff: autogold.Expect(map[string]interface{}{"tests[1]": map[string]interface{}{}}),
			expectedBlock: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [1]: {
                  + nested    : "val2"
                }
        ]
`),
			expectedBlockForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests[1]": map[string]interface{}{"kind": "ADD_REPLACE"}}),
			expectedBlockForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [1]: {
                  + nested    : "val2"
                }
        ]
`),
		},
		{
			name:                     "shuffled added middle unordered",
			props1:                   []string{"val2", "val1"},
			props2:                   []string{"val1", "val3", "val2"},
			expectedAttrDetailedDiff: autogold.Expect(map[string]interface{}{"tests[1]": map[string]interface{}{}}),
			expectedAttr: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [1]: "val3"
        ]
`),
			expectedAttrForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests[1]": map[string]interface{}{"kind": "ADD_REPLACE"}}),
			expectedAttrForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [1]: "val3"
        ]
`),
			expectedBlockDetailedDiff: autogold.Expect(map[string]interface{}{"tests[1]": map[string]interface{}{}}),
			expectedBlock: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [1]: {
                  + nested    : "val3"
                }
        ]
`),
			expectedBlockForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests[1]": map[string]interface{}{"kind": "ADD_REPLACE"}}),
			expectedBlockForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [1]: {
                  + nested    : "val3"
                }
        ]
`),
		},
		{
			name:                     "shuffled added end",
			props1:                   []string{"val1", "val2"},
			props2:                   []string{"val2", "val1", "val3"},
			expectedAttrDetailedDiff: autogold.Expect(map[string]interface{}{"tests[2]": map[string]interface{}{}}),
			expectedAttr: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [2]: "val3"
        ]
`),
			expectedAttrForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests[2]": map[string]interface{}{"kind": "ADD_REPLACE"}}),
			expectedAttrForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [2]: "val3"
        ]
`),
			expectedBlockDetailedDiff: autogold.Expect(map[string]interface{}{"tests[2]": map[string]interface{}{}}),
			expectedBlock: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [2]: {
                  + nested    : "val3"
                }
        ]
`),
			expectedBlockForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests[2]": map[string]interface{}{"kind": "ADD_REPLACE"}}),
			expectedBlockForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [2]: {
                  + nested    : "val3"
                }
        ]
`),
		},
		{
			name:                     "shuffled removed front",
			props1:                   []string{"val1", "val2", "val3"},
			props2:                   []string{"val3", "val2"},
			expectedAttrDetailedDiff: autogold.Expect(map[string]interface{}{"tests[0]": map[string]interface{}{"kind": "DELETE"}}),
			expectedAttr: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [0]: "val1"
        ]
`),
			expectedAttrForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests[0]": map[string]interface{}{"kind": "DELETE_REPLACE"}}),
			expectedAttrForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [0]: "val1"
        ]
`),
			expectedBlockDetailedDiff: autogold.Expect(map[string]interface{}{"tests[0]": map[string]interface{}{"kind": "DELETE"}}),
			expectedBlock: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [0]: {
                  - nested: "val1"
                }
        ]
`),
			expectedBlockForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests[0]": map[string]interface{}{"kind": "DELETE_REPLACE"}}),
			expectedBlockForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [0]: {
                  - nested: "val1"
                }
        ]
`),
		},
		{
			name:                     "shuffled removed middle",
			props1:                   []string{"val1", "val2", "val3"},
			props2:                   []string{"val3", "val1"},
			expectedAttrDetailedDiff: autogold.Expect(map[string]interface{}{"tests[1]": map[string]interface{}{"kind": "DELETE"}}),
			expectedAttr: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [1]: "val2"
        ]
`),
			expectedAttrForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests[1]": map[string]interface{}{"kind": "DELETE_REPLACE"}}),
			expectedAttrForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [1]: "val2"
        ]
`),
			expectedBlockDetailedDiff: autogold.Expect(map[string]interface{}{"tests[1]": map[string]interface{}{"kind": "DELETE"}}),
			expectedBlock: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [1]: {
                  - nested: "val2"
                }
        ]
`),
			expectedBlockForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests[1]": map[string]interface{}{"kind": "DELETE_REPLACE"}}),
			expectedBlockForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [1]: {
                  - nested: "val2"
                }
        ]
`),
		},
		{
			name:                     "shuffled removed end",
			props1:                   []string{"val1", "val2", "val3"},
			props2:                   []string{"val2", "val1"},
			expectedAttrDetailedDiff: autogold.Expect(map[string]interface{}{"tests[2]": map[string]interface{}{"kind": "DELETE"}}),
			expectedAttr: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [2]: "val3"
        ]
`),
			expectedAttrForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests[2]": map[string]interface{}{"kind": "DELETE_REPLACE"}}),
			expectedAttrForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [2]: "val3"
        ]
`),
			expectedBlockDetailedDiff: autogold.Expect(map[string]interface{}{"tests[2]": map[string]interface{}{"kind": "DELETE"}}),
			expectedBlock: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [2]: {
                  - nested: "val3"
                }
        ]
`),
			expectedBlockForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests[2]": map[string]interface{}{"kind": "DELETE_REPLACE"}}),
			expectedBlockForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [2]: {
                  - nested: "val3"
                }
        ]
`),
		},
		{
			name:                     "two added",
			props1:                   []string{"val1", "val2"},
			props2:                   []string{"val1", "val2", "val3", "val4"},
			expectedAttrDetailedDiff: autogold.Expect(map[string]interface{}{"tests[2]": map[string]interface{}{}, "tests[3]": map[string]interface{}{}}),
			expectedAttr: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [2]: "val3"
          + [3]: "val4"
        ]
`),
			expectedAttrForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests[2]": map[string]interface{}{"kind": "ADD_REPLACE"}, "tests[3]": map[string]interface{}{"kind": "ADD_REPLACE"}}),
			expectedAttrForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [2]: "val3"
          + [3]: "val4"
        ]
`),
			expectedBlockDetailedDiff: autogold.Expect(map[string]interface{}{"tests[2]": map[string]interface{}{}, "tests[3]": map[string]interface{}{}}),
			expectedBlock: autogold.Expect(`
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
`),
			expectedBlockForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests[2]": map[string]interface{}{"kind": "ADD_REPLACE"}, "tests[3]": map[string]interface{}{"kind": "ADD_REPLACE"}}),
			expectedBlockForceNew: autogold.Expect(`
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
`),
		},
		{
			name:                     "two removed",
			props1:                   []string{"val1", "val2", "val3", "val4"},
			props2:                   []string{"val1", "val2"},
			expectedAttrDetailedDiff: autogold.Expect(map[string]interface{}{"tests[2]": map[string]interface{}{"kind": "DELETE"}, "tests[3]": map[string]interface{}{"kind": "DELETE"}}),
			expectedAttr: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [2]: "val3"
          - [3]: "val4"
        ]
`),
			expectedAttrForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests[2]": map[string]interface{}{"kind": "DELETE_REPLACE"}, "tests[3]": map[string]interface{}{"kind": "DELETE_REPLACE"}}),
			expectedAttrForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [2]: "val3"
          - [3]: "val4"
        ]
`),
			expectedBlockDetailedDiff: autogold.Expect(map[string]interface{}{"tests[2]": map[string]interface{}{"kind": "DELETE"}, "tests[3]": map[string]interface{}{"kind": "DELETE"}}),
			expectedBlock: autogold.Expect(`
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
`),
			expectedBlockForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests[2]": map[string]interface{}{"kind": "DELETE_REPLACE"}, "tests[3]": map[string]interface{}{"kind": "DELETE_REPLACE"}}),
			expectedBlockForceNew: autogold.Expect(`
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
`),
		},
		{
			name:                     "two added and two removed",
			props1:                   []string{"val1", "val2", "val3", "val4"},
			props2:                   []string{"val1", "val2", "val5", "val6"},
			expectedAttrDetailedDiff: autogold.Expect(map[string]interface{}{"tests[2]": map[string]interface{}{"kind": "UPDATE"}, "tests[3]": map[string]interface{}{"kind": "UPDATE"}}),
			expectedAttr: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          ~ [2]: "val3" => "val5"
          ~ [3]: "val4" => "val6"
        ]
`),
			expectedAttrForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests[2]": map[string]interface{}{"kind": "UPDATE_REPLACE"}, "tests[3]": map[string]interface{}{"kind": "UPDATE_REPLACE"}}),
			expectedAttrForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          ~ [2]: "val3" => "val5"
          ~ [3]: "val4" => "val6"
        ]
`),
			expectedBlockDetailedDiff: autogold.Expect(map[string]interface{}{"tests[2].nested": map[string]interface{}{"kind": "UPDATE"}, "tests[3].nested": map[string]interface{}{"kind": "UPDATE"}}),
			expectedBlock: autogold.Expect(`
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
`),
			expectedBlockForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests[2].nested": map[string]interface{}{"kind": "UPDATE_REPLACE"}, "tests[3].nested": map[string]interface{}{"kind": "UPDATE_REPLACE"}}),
			expectedBlockForceNew: autogold.Expect(`
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
`),
		},
		{
			name:   "two added and two removed shuffled, one overlaps",
			props1: []string{"val1", "val2", "val3", "val4"},
			props2: []string{"val1", "val5", "val6", "val2"},
			expectedAttrDetailedDiff: autogold.Expect(map[string]interface{}{
				"tests[1]": map[string]interface{}{},
				"tests[2]": map[string]interface{}{"kind": "UPDATE"},
				"tests[3]": map[string]interface{}{"kind": "DELETE"},
			}),
			expectedAttr: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [1]: "val5"
          ~ [2]: "val3" => "val6"
          - [3]: "val4"
        ]
`),
			expectedAttrForceNewDetailedDiff: autogold.Expect(map[string]interface{}{
				"tests[1]": map[string]interface{}{"kind": "ADD_REPLACE"},
				"tests[2]": map[string]interface{}{"kind": "UPDATE_REPLACE"},
				"tests[3]": map[string]interface{}{"kind": "DELETE_REPLACE"},
			}),
			expectedAttrForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [1]: "val5"
          ~ [2]: "val3" => "val6"
          - [3]: "val4"
        ]
`),
			expectedBlockDetailedDiff: autogold.Expect(map[string]interface{}{
				"tests[1]":        map[string]interface{}{},
				"tests[2].nested": map[string]interface{}{"kind": "UPDATE"},
				"tests[3]":        map[string]interface{}{"kind": "DELETE"},
			}),
			expectedBlock: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [1]: {
                  + nested    : "val5"
                }
          ~ [2]: {
                  ~ nested: "val3" => "val6"
                }
          - [3]: {
                  - nested: "val4"
                }
        ]
`),
			expectedBlockForceNewDetailedDiff: autogold.Expect(map[string]interface{}{
				"tests[1]":        map[string]interface{}{"kind": "ADD_REPLACE"},
				"tests[2].nested": map[string]interface{}{"kind": "UPDATE_REPLACE"},
				"tests[3]":        map[string]interface{}{"kind": "DELETE_REPLACE"},
			}),
			expectedBlockForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [1]: {
                  + nested    : "val5"
                }
          ~ [2]: {
                  ~ nested: "val3" => "val6"
                }
          - [3]: {
                  - nested: "val4"
                }
        ]
`),
		},
		{
			name:   "two added and two removed shuffled, no overlaps",
			props1: []string{"val1", "val2", "val3", "val4"},
			props2: []string{"val5", "val6", "val1", "val2"},
			expectedAttrDetailedDiff: autogold.Expect(map[string]interface{}{
				"tests[0]": map[string]interface{}{},
				"tests[1]": map[string]interface{}{},
				"tests[2]": map[string]interface{}{"kind": "DELETE"},
				"tests[3]": map[string]interface{}{"kind": "DELETE"},
			}),
			expectedAttr: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [0]: "val5"
          + [1]: "val6"
          - [2]: "val3"
          - [3]: "val4"
        ]
`),
			expectedAttrForceNewDetailedDiff: autogold.Expect(map[string]interface{}{
				"tests[0]": map[string]interface{}{"kind": "ADD_REPLACE"},
				"tests[1]": map[string]interface{}{"kind": "ADD_REPLACE"},
				"tests[2]": map[string]interface{}{"kind": "DELETE_REPLACE"},
				"tests[3]": map[string]interface{}{"kind": "DELETE_REPLACE"},
			}),
			expectedAttrForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [0]: "val5"
          + [1]: "val6"
          - [2]: "val3"
          - [3]: "val4"
        ]
`),
			expectedBlockDetailedDiff: autogold.Expect(map[string]interface{}{
				"tests[0]": map[string]interface{}{},
				"tests[1]": map[string]interface{}{},
				"tests[2]": map[string]interface{}{"kind": "DELETE"},
				"tests[3]": map[string]interface{}{"kind": "DELETE"},
			}),
			expectedBlock: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [0]: {
                  + nested    : "val5"
                }
          + [1]: {
                  + nested    : "val6"
                }
          - [2]: {
                  - nested: "val3"
                }
          - [3]: {
                  - nested: "val4"
                }
        ]
`),
			expectedBlockForceNewDetailedDiff: autogold.Expect(map[string]interface{}{
				"tests[0]": map[string]interface{}{"kind": "ADD_REPLACE"},
				"tests[1]": map[string]interface{}{"kind": "ADD_REPLACE"},
				"tests[2]": map[string]interface{}{"kind": "DELETE_REPLACE"},
				"tests[3]": map[string]interface{}{"kind": "DELETE_REPLACE"},
			}),
			expectedBlockForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [0]: {
                  + nested    : "val5"
                }
          + [1]: {
                  + nested    : "val6"
                }
          - [2]: {
                  - nested: "val3"
                }
          - [3]: {
                  - nested: "val4"
                }
        ]
`),
		},
		{
			name:   "two added and two removed shuffled, with duplicates",
			props1: []string{"val1", "val2", "val3", "val4"},
			props2: []string{"val1", "val5", "val6", "val2", "val1", "val2"},
			expectedAttrDetailedDiff: autogold.Expect(map[string]interface{}{
				"tests[1]": map[string]interface{}{},
				"tests[2]": map[string]interface{}{"kind": "UPDATE"},
				"tests[3]": map[string]interface{}{"kind": "DELETE"},
			}),
			expectedAttr: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [1]: "val5"
          ~ [2]: "val3" => "val6"
          - [3]: "val4"
        ]
`),
			expectedAttrForceNewDetailedDiff: autogold.Expect(map[string]interface{}{
				"tests[1]": map[string]interface{}{"kind": "ADD_REPLACE"},
				"tests[2]": map[string]interface{}{"kind": "UPDATE_REPLACE"},
				"tests[3]": map[string]interface{}{"kind": "DELETE_REPLACE"},
			}),
			expectedAttrForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [1]: "val5"
          ~ [2]: "val3" => "val6"
          - [3]: "val4"
        ]
`),
			expectedBlockDetailedDiff: autogold.Expect(map[string]interface{}{
				"tests[1]":        map[string]interface{}{},
				"tests[2].nested": map[string]interface{}{"kind": "UPDATE"},
				"tests[3]":        map[string]interface{}{"kind": "DELETE"},
			}),
			expectedBlock: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [1]: {
                  + nested    : "val5"
                }
          ~ [2]: {
                  ~ nested: "val3" => "val6"
                }
          - [3]: {
                  - nested: "val4"
                }
        ]
`),
			expectedBlockForceNewDetailedDiff: autogold.Expect(map[string]interface{}{
				"tests[1]":        map[string]interface{}{"kind": "ADD_REPLACE"},
				"tests[2].nested": map[string]interface{}{"kind": "UPDATE_REPLACE"},
				"tests[3]":        map[string]interface{}{"kind": "DELETE_REPLACE"},
			}),
			expectedBlockForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [1]: {
                  + nested    : "val5"
                }
          ~ [2]: {
                  ~ nested: "val3" => "val6"
                }
          - [3]: {
                  - nested: "val4"
                }
        ]
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
	expected.Equal(t, trimDiff(t, out))
	expectedDetailedDiff.Equal(t, detailedDiff)
}

func TestDetailedDiffUnknownSetAttributeElement(t *testing.T) {
	// TODO[pulumi/pulumi-terraform-bridge#2517]: Remove this once accurate bridge previews are rolled out
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
			autogold.Expect(`
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
`),
			autogold.Expect(map[string]interface{}{"tests": map[string]interface{}{}}))
	})

	t.Run("non-empty to unknown element", func(t *testing.T) {
		testDetailedDiffWithUnknowns(t, resMap, "${auxRes.aux}",
			[]interface{}{"val1"},
			[]interface{}{"UNKNOWN"},
			autogold.Expect(`
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          ~ [0]: "val1" => output<string>
        ]
`),
			autogold.Expect(map[string]interface{}{"tests": map[string]interface{}{"kind": "UPDATE"}}))
	})

	t.Run("unknown element added front", func(t *testing.T) {
		testDetailedDiffWithUnknowns(t, resMap, "${auxRes.aux}",
			[]interface{}{"val2", "val3"},
			[]interface{}{"UNKNOWN", "val2", "val3"},
			autogold.Expect(`
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
`),
			autogold.Expect(map[string]interface{}{"tests": map[string]interface{}{"kind": "UPDATE"}}),
		)
	})

	t.Run("unknown element added middle", func(t *testing.T) {
		testDetailedDiffWithUnknowns(t, resMap, "${auxRes.aux}",
			[]interface{}{"val1", "val3"},
			[]interface{}{"val1", "UNKNOWN", "val3"},
			autogold.Expect(`
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
`),
			autogold.Expect(map[string]interface{}{"tests": map[string]interface{}{"kind": "UPDATE"}}),
		)
	})

	t.Run("unknown element added end", func(t *testing.T) {
		testDetailedDiffWithUnknowns(t, resMap, "${auxRes.aux}",
			[]interface{}{"val1", "val2"},
			[]interface{}{"val1", "val2", "UNKNOWN"},
			autogold.Expect(`
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
`),
			autogold.Expect(map[string]interface{}{"tests": map[string]interface{}{"kind": "UPDATE"}}),
		)
	})

	t.Run("element updated to unknown", func(t *testing.T) {
		testDetailedDiffWithUnknowns(t, resMap, "${auxRes.aux}",
			[]interface{}{"val1", "val2", "val3"},
			[]interface{}{"val1", "UNKNOWN", "val3"},
			autogold.Expect(`
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
`),
			autogold.Expect(map[string]interface{}{"tests": map[string]interface{}{"kind": "UPDATE"}}),
		)
	})

	t.Run("shuffled unknown added front", func(t *testing.T) {
		testDetailedDiffWithUnknowns(t, resMap, "${auxRes.aux}",
			[]interface{}{"val2", "val3"},
			[]interface{}{"UNKNOWN", "val3", "val2"},
			autogold.Expect(`
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
`),
			autogold.Expect(map[string]interface{}{"tests": map[string]interface{}{"kind": "UPDATE"}}),
		)
	})

	t.Run("shuffled unknown added middle", func(t *testing.T) {
		testDetailedDiffWithUnknowns(t, resMap, "${auxRes.aux}",
			[]interface{}{"val1", "val3"},
			[]interface{}{"val3", "UNKNOWN", "val1"},
			autogold.Expect(`
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
`),
			autogold.Expect(map[string]interface{}{"tests": map[string]interface{}{"kind": "UPDATE"}}),
		)
	})

	t.Run("shuffled unknown added end", func(t *testing.T) {
		testDetailedDiffWithUnknowns(t, resMap, "${auxRes.aux}",
			[]interface{}{"val1", "val2"},
			[]interface{}{"val2", "val1", "UNKNOWN"},
			autogold.Expect(`
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
`),
			autogold.Expect(map[string]interface{}{"tests": map[string]interface{}{"kind": "UPDATE"}}),
		)
	})
}

func TestUnknownSetAttributeDiff(t *testing.T) {
	// TODO[pulumi/pulumi-terraform-bridge#2517]: Remove this once accurate bridge previews are rolled out
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
			autogold.Expect(`
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      + tests: output<string>
    --outputs:--
  + testOut: output<string>
`),
			autogold.Expect(map[string]interface{}{"tests": map[string]interface{}{}}),
		)
	})

	t.Run("non-empty to unknown set", func(t *testing.T) {
		testDetailedDiffWithUnknowns(t, resMap, "${auxRes.auxes}",
			[]interface{}{"val"},
			"UNKNOWN",
			autogold.Expect(`
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      - tests: [
      -     [0]: "val"
        ]
      + tests: output<string>
`),
			autogold.Expect(map[string]interface{}{"tests": map[string]interface{}{"kind": "UPDATE"}}),
		)
	})
}

func TestDetailedDiffSetDuplicates(t *testing.T) {
	// TODO[pulumi/pulumi-terraform-bridge#2517]: Remove this once accurate bridge previews are rolled out
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
      tests: %s`

	t.Run("pulumi", func(t *testing.T) {
		pt := pulcheck.PulCheck(t, bridgedProvider, fmt.Sprintf(program, `["a", "a"]`))
		pt.Up(t)

		pt.WritePulumiYaml(t, fmt.Sprintf(program, `["b", "b", "a", "a", "c"]`))

		res := pt.Preview(t, optpreview.Diff())

		autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [1]: "b"
          + [4]: "c"
        ]
`).Equal(t, trimDiff(t, res.StdOut))
	})

	t.Run("terraform", func(t *testing.T) {
		tfdriver := tfcheck.NewTfDriver(t, t.TempDir(), "prov", tfp)
		tfdriver.Write(t, `
resource "prov_test" "mainRes" {
  test = ["a", "a"]
}`)

		plan, err := tfdriver.Plan(t)
		require.NoError(t, err)
		err = tfdriver.Apply(t, plan)
		require.NoError(t, err)

		tfdriver.Write(t, `
resource "prov_test" "mainRes" {
  test = ["b", "b", "a", "a", "c"]
}`)

		plan, err = tfdriver.Plan(t)
		require.NoError(t, err)

		autogold.Expect(`
Terraform used the selected providers to generate the following execution
plan. Resource actions are indicated with the following symbols:
  ~ update in-place

Terraform will perform the following actions:

  # prov_test.mainRes will be updated in-place
  ~ resource "prov_test" "mainRes" {
        id   = "newid"
      ~ test = [
          + "b",
          + "c",
            # (1 unchanged element hidden)
        ]
    }

Plan: 0 to add, 1 to change, 0 to destroy.

`).Equal(t, plan.StdOut)
	})
}

func TestDetailedDiffSetNestedAttributeUpdated(t *testing.T) {
	// TODO[pulumi/pulumi-terraform-bridge#2517]: Remove this once accurate bridge previews are rolled out
	t.Setenv("PULUMI_TF_BRIDGE_ACCURATE_BRIDGE_PREVIEW", "true")

	resMap := map[string]*schema.Resource{
		"prov_test": {
			Schema: map[string]*schema.Schema{
				"test": {
					Type:     schema.TypeSet,
					Optional: true,
					Elem: &schema.Resource{
						Schema: map[string]*schema.Schema{
							"nested": {
								Type:     schema.TypeString,
								Optional: true,
							},
							"nested2": {
								Type:     schema.TypeString,
								Optional: true,
							},
							"nested3": {
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
    properties:
      tests: %s`

	t.Run("pulumi", func(t *testing.T) {
		props1 := []map[string]string{
			{"nested": "b", "nested2": "b", "nested3": "b"},
			{"nested": "a", "nested2": "a", "nested3": "a"},
			{"nested": "c", "nested2": "c", "nested3": "c"},
		}
		props2 := []map[string]string{
			{"nested": "b", "nested2": "b", "nested3": "b"},
			{"nested": "d", "nested2": "a", "nested3": "a"},
			{"nested": "c", "nested2": "c", "nested3": "c"},
		}

		props1JSON, err := json.Marshal(props1)
		require.NoError(t, err)

		pt := pulcheck.PulCheck(t, bridgedProvider, fmt.Sprintf(program, string(props1JSON)))
		pt.Up(t)

		props2JSON, err := json.Marshal(props2)
		require.NoError(t, err)

		pt.WritePulumiYaml(t, fmt.Sprintf(program, string(props2JSON)))

		res := pt.Preview(t, optpreview.Diff())

		autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [0]: {
                  - nested : "a"
                  - nested2: "a"
                  - nested3: "a"
                }
          + [1]: {
                  + nested    : "d"
                  + nested2   : "a"
                  + nested3   : "a"
                }
        ]
`).Equal(t, trimDiff(t, res.StdOut))
	})

	t.Run("terraform", func(t *testing.T) {
		tfdriver := tfcheck.NewTfDriver(t, t.TempDir(), "prov", tfp)
		tfdriver.Write(t, `
resource "prov_test" "mainRes" {
  test {
	  nested = "b"
	  nested2 = "b"
	  nested3 = "b"
	}
 test {
	  nested = "a"	
	  nested2 = "a"
	  nested3 = "a"
	}
 test {
	  nested = "c"
	  nested2 = "c"
	  nested3 = "c"
	}
}`)

		plan, err := tfdriver.Plan(t)
		require.NoError(t, err)
		err = tfdriver.Apply(t, plan)
		require.NoError(t, err)

		tfdriver.Write(t, `
resource "prov_test" "mainRes" {
  test {
	  nested = "b"
	  nested2 = "b"
	  nested3 = "b"
	}
 test {
	  nested = "d"	
	  nested2 = "a"
	  nested3 = "a"
	}
 test {
	  nested = "c"
	  nested2 = "c"
	  nested3 = "c"
	}
}`)

		plan, err = tfdriver.Plan(t)
		require.NoError(t, err)

		autogold.Expect(`
Terraform used the selected providers to generate the following execution
plan. Resource actions are indicated with the following symbols:
  ~ update in-place

Terraform will perform the following actions:

  # prov_test.mainRes will be updated in-place
  ~ resource "prov_test" "mainRes" {
        id = "newid"

      - test {
          - nested  = "a" -> null
          - nested2 = "a" -> null
          - nested3 = "a" -> null
        }
      + test {
          + nested  = "d"
          + nested2 = "a"
          + nested3 = "a"
        }

        # (2 unchanged blocks hidden)
    }

Plan: 0 to add, 1 to change, 0 to destroy.

`).Equal(t, plan.StdOut)
	})
}

func TestDetailedDiffSetComputedNestedAttribute(t *testing.T) {
	// TODO[pulumi/pulumi-terraform-bridge#2517]: Remove this once accurate bridge previews are rolled out
	t.Setenv("PULUMI_TF_BRIDGE_ACCURATE_BRIDGE_PREVIEW", "true")

	resCount := 0
	setComputedProp := func(t *testing.T, d *schema.ResourceData, _ interface{}) diag.Diagnostics {
		testSet := d.Get("test").(*schema.Set)
		testVals := testSet.List()
		newTestVals := make([]interface{}, len(testVals))
		for i, v := range testVals {
			val := v.(map[string]interface{})
			if val["computed"] == nil || val["computed"] == "" {
				val["computed"] = fmt.Sprint(resCount)
				resCount++
			}
			newTestVals[i] = val
		}

		err := d.Set("test", schema.NewSet(testSet.F, newTestVals))
		require.NoError(t, err)
		return nil
	}

	resMap := map[string]*schema.Resource{
		"prov_test": {
			Schema: map[string]*schema.Schema{
				"test": {
					Type:     schema.TypeSet,
					Optional: true,
					Elem: &schema.Resource{
						Schema: map[string]*schema.Schema{
							"nested": {
								Type:     schema.TypeString,
								Optional: true,
							},
							"computed": {
								Type:     schema.TypeString,
								Optional: true,
								Computed: true,
							},
						},
					},
				},
			},
			CreateContext: func(ctx context.Context, d *schema.ResourceData, i interface{}) diag.Diagnostics {
				d.SetId("id")
				return setComputedProp(t, d, i)
			},
			UpdateContext: func(ctx context.Context, d *schema.ResourceData, i interface{}) diag.Diagnostics {
				return setComputedProp(t, d, i)
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
      tests: %s`

	t.Run("pulumi", func(t *testing.T) {
		props1 := []map[string]string{
			{"nested": "a", "computed": "b"},
		}
		props1JSON, err := json.Marshal(props1)
		require.NoError(t, err)

		pt := pulcheck.PulCheck(t, bridgedProvider, fmt.Sprintf(program, string(props1JSON)))
		pt.Up(t)

		props2 := []map[string]string{
			{"nested": "a"},
			{"nested": "a", "computed": "b"},
		}
		props2JSON, err := json.Marshal(props2)
		require.NoError(t, err)

		pt.WritePulumiYaml(t, fmt.Sprintf(program, string(props2JSON)))
		res := pt.Preview(t, optpreview.Diff())

		// TODO[pulumi/pulumi-terraform-bridge#2528]: The preview is wrong here because of the computed property
		autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=id]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [1]: {
                  + computed  : "b"
                  + nested    : "a"
                }
        ]
`).Equal(t, trimDiff(t, res.StdOut))
	})

	t.Run("terraform", func(t *testing.T) {
		resCount = 0
		tfdriver := tfcheck.NewTfDriver(t, t.TempDir(), "prov", tfp)
		tfdriver.Write(t, `
resource "prov_test" "mainRes" {
  test {
	  nested = "a"
	  computed = "b"
	}
}`)

		plan, err := tfdriver.Plan(t)
		require.NoError(t, err)
		err = tfdriver.Apply(t, plan)
		require.NoError(t, err)

		tfdriver.Write(t, `
resource "prov_test" "mainRes" {
  test {
	  nested = "a"
	  computed = "b"
	}
  test {
	  nested = "a"
	}
}`)
		plan, err = tfdriver.Plan(t)
		require.NoError(t, err)

		autogold.Expect(`
Terraform used the selected providers to generate the following execution
plan. Resource actions are indicated with the following symbols:
  ~ update in-place

Terraform will perform the following actions:

  # prov_test.mainRes will be updated in-place
  ~ resource "prov_test" "mainRes" {
        id = "id"

      + test {
          + computed = (known after apply)
          + nested   = "a"
        }

        # (1 unchanged block hidden)
    }

Plan: 0 to add, 1 to change, 0 to destroy.

`).Equal(t, plan.StdOut)
	})
}

func TestUnknownCollectionForceNewDetailedDiff(t *testing.T) {
	// TODO[pulumi/pulumi-terraform-bridge#2517]: Remove this once accurate bridge previews are rolled out
	t.Setenv("PULUMI_TF_BRIDGE_ACCURATE_BRIDGE_PREVIEW", "true")

	collectionForceNewResource := func(typ schema.ValueType) *schema.Resource {
		return &schema.Resource{
			Schema: map[string]*schema.Schema{
				"test": {
					Type:     typ,
					Optional: true,
					ForceNew: true,
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
		}
	}

	propertyForceNewResource := func(typ schema.ValueType) *schema.Resource {
		return &schema.Resource{
			Schema: map[string]*schema.Schema{
				"test": {
					Type:     typ,
					Optional: true,
					Elem: &schema.Resource{
						Schema: map[string]*schema.Schema{
							"prop": {
								Type:     schema.TypeString,
								Optional: true,
								ForceNew: true,
							},
						},
					},
				},
			},
		}
	}

	auxResource := func(typ schema.ValueType) *schema.Resource {
		return &schema.Resource{
			Schema: map[string]*schema.Schema{
				"aux": {
					Type:     typ,
					Computed: true,
					Elem: &schema.Resource{
						Schema: map[string]*schema.Schema{
							"prop": {
								Type:     schema.TypeString,
								Computed: true,
							},
						},
					},
				},
			},
			CreateContext: func(_ context.Context, d *schema.ResourceData, _ interface{}) diag.Diagnostics {
				d.SetId("aux")
				err := d.Set("aux", []map[string]interface{}{{"prop": "aux"}})
				require.NoError(t, err)
				return nil
			},
		}
	}

	initialProgram := `
    name: test
    runtime: yaml
    resources:
      mainRes:
        type: prov:index:Test
        properties:
          tests: [{prop: 'value'}]
`

	program := `
    name: test
    runtime: yaml
    resources:
      auxRes:
        type: prov:index:Aux
      mainRes:
        type: prov:index:Test
        properties:
          tests: %s
`

	runTest := func(t *testing.T, program2 string, bridgedProvider info.Provider, expectedOutput autogold.Value) {
		pt := pulcheck.PulCheck(t, bridgedProvider, initialProgram)
		pt.Up(t)
		pt.WritePulumiYaml(t, program2)

		res := pt.Preview(t, optpreview.Diff())

		expectedOutput.Equal(t, trimDiff(t, res.StdOut))
	}

	t.Run("list force new", func(t *testing.T) {
		resMap := map[string]*schema.Resource{
			"prov_test": collectionForceNewResource(schema.TypeList),
			"prov_aux":  auxResource(schema.TypeList),
		}

		tfp := &schema.Provider{ResourcesMap: resMap}
		bridgedProvider := pulcheck.BridgedProvider(t, "prov", tfp)
		runTest := func(t *testing.T, program2 string, expectedOutput autogold.Value) {
			runTest(t, program2, bridgedProvider, expectedOutput)
		}

		t.Run("unknown plain property", func(t *testing.T) {
			program2 := fmt.Sprintf(program, "[{prop: \"${auxRes.auxes[0].prop}\"}]")
			runTest(t, program2, autogold.Expect(`
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          ~ [0]: {
                  ~ prop: "value" => output<string>
                }
        ]
`))
		})

		t.Run("unknown object", func(t *testing.T) {
			program2 := fmt.Sprintf(program, "[\"${auxRes.auxes[0]}\"]")
			runTest(t, program2, autogold.Expect(`
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [0]: {
                  - prop: "value"
                }
          + [0]: output<string>
        ]
`))
		})

		t.Run("unknown collection", func(t *testing.T) {
			program2 := fmt.Sprintf(program, "\"${auxRes.auxes}\"")
			runTest(t, program2, autogold.Expect(`
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      - tests: [
      -     [0]: {
              - prop: "value"
            }
        ]
      + tests: output<string>
`))
		})
	})

	t.Run("list property force new", func(t *testing.T) {
		resMap := map[string]*schema.Resource{
			"prov_test": propertyForceNewResource(schema.TypeList),
			"prov_aux":  auxResource(schema.TypeList),
		}

		tfp := &schema.Provider{ResourcesMap: resMap}
		bridgedProvider := pulcheck.BridgedProvider(t, "prov", tfp)
		runTest := func(t *testing.T, program2 string, expectedOutput autogold.Value) {
			runTest(t, program2, bridgedProvider, expectedOutput)
		}

		t.Run("unknown plain property", func(t *testing.T) {
			program2 := fmt.Sprintf(program, "[{prop: \"${auxRes.auxes[0].prop}\"}]")
			runTest(t, program2, autogold.Expect(`
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          ~ [0]: {
                  ~ prop: "value" => output<string>
                }
        ]
`))
		})

		t.Run("unknown object", func(t *testing.T) {
			program2 := fmt.Sprintf(program, "[\"${auxRes.auxes[0]}\"]")
			runTest(t, program2, autogold.Expect(`
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [0]: {
                  - prop: "value"
                }
          + [0]: output<string>
        ]
`))
		})

		t.Run("unknown collection", func(t *testing.T) {
			program2 := fmt.Sprintf(program, "\"${auxRes.auxes}\"")
			runTest(t, program2, autogold.Expect(`
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      - tests: [
      -     [0]: {
              - prop: "value"
            }
        ]
      + tests: output<string>
`))
		})
	})

	t.Run("set force new", func(t *testing.T) {
		resMap := map[string]*schema.Resource{
			"prov_test": collectionForceNewResource(schema.TypeSet),
			"prov_aux":  auxResource(schema.TypeSet),
		}

		tfp := &schema.Provider{ResourcesMap: resMap}
		bridgedProvider := pulcheck.BridgedProvider(t, "prov", tfp)
		runTest := func(t *testing.T, program2 string, expectedOutput autogold.Value) {
			runTest(t, program2, bridgedProvider, expectedOutput)
		}

		t.Run("unknown plain property", func(t *testing.T) {
			program2 := fmt.Sprintf(program, "[{prop: \"${auxRes.auxes[0].prop}\"}]")
			runTest(t, program2, autogold.Expect(`
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          ~ [0]: {
                  ~ prop: "value" => output<string>
                }
        ]
`))
		})

		t.Run("unknown object", func(t *testing.T) {
			program2 := fmt.Sprintf(program, "[\"${auxRes.auxes[0]}\"]")
			runTest(t, program2, autogold.Expect(`
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [0]: {
                  - prop: "value"
                }
          + [0]: output<string>
        ]
`))
		})

		t.Run("unknown collection", func(t *testing.T) {
			program2 := fmt.Sprintf(program, "\"${auxRes.auxes}\"")
			runTest(t, program2, autogold.Expect(`
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      - tests: [
      -     [0]: {
              - prop: "value"
            }
        ]
      + tests: output<string>
`))
		})
	})

	t.Run("set property force new", func(t *testing.T) {
		resMap := map[string]*schema.Resource{
			"prov_test": propertyForceNewResource(schema.TypeSet),
			"prov_aux":  auxResource(schema.TypeSet),
		}

		tfp := &schema.Provider{ResourcesMap: resMap}
		bridgedProvider := pulcheck.BridgedProvider(t, "prov", tfp)
		runTest := func(t *testing.T, program2 string, expectedOutput autogold.Value) {
			runTest(t, program2, bridgedProvider, expectedOutput)
		}

		t.Run("unknown plain property", func(t *testing.T) {
			program2 := fmt.Sprintf(program, "[{prop: \"${auxRes.auxes[0].prop}\"}]")
			runTest(t, program2, autogold.Expect(`
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          ~ [0]: {
                  ~ prop: "value" => output<string>
                }
        ]
`))
		})

		t.Run("unknown object", func(t *testing.T) {
			program2 := fmt.Sprintf(program, "[\"${auxRes.auxes[0]}\"]")
			runTest(t, program2, autogold.Expect(`
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [0]: {
                  - prop: "value"
                }
          + [0]: output<string>
        ]
`))
		})

		t.Run("unknown collection", func(t *testing.T) {
			program2 := fmt.Sprintf(program, "\"${auxRes.auxes}\"")
			runTest(t, program2, autogold.Expect(`
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      - tests: [
      -     [0]: {
              - prop: "value"
            }
        ]
      + tests: output<string>
`))
		})
	})
}

func TestTypeChecker(t *testing.T) {
	t.Setenv("PULUMI_DEBUG_YAML_DISABLE_TYPE_CHECKING", "true")
	makeResMap := func(sch map[string]*schema.Schema) map[string]*schema.Resource {
		return map[string]*schema.Resource{
			"prov_test": {Schema: sch},
		}
	}

	runTest := func(t *testing.T, resMap map[string]*schema.Resource, props interface{}, expectedError string) {
		propsJSON, err := json.Marshal(props)
		require.NoError(t, err)
		program := fmt.Sprintf(`
name: test
runtime: yaml
resources:
  mainRes:
    type: prov:index:Test
	properties: %s`, propsJSON)

		bridgedProvider := pulcheck.BridgedProvider(t, "prov", &schema.Provider{ResourcesMap: resMap})
		pt := pulcheck.PulCheck(t, bridgedProvider, program)
		_, err = pt.CurrentStack().Up(pt.Context())

		require.ErrorContains(t, err, "Unexpected type at field")
		require.ErrorContains(t, err, expectedError)
	}

	t.Run("flat type instead of array", func(t *testing.T) {
		resMap := makeResMap(map[string]*schema.Schema{
			"network_configuration": {
				Type:     schema.TypeList,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"subnets": {
							Type: schema.TypeSet,
							Elem: &schema.Schema{Type: schema.TypeString},
						},
					},
				},
			},
		})
		runTest(t, resMap, map[string]interface{}{"networkConfiguration": map[string]any{"subnets": "subnet"}}, "expected array type, got")
	})

	t.Run("flat type instead of map", func(t *testing.T) {
		resMap := makeResMap(map[string]*schema.Schema{
			"tags": {
				Type:     schema.TypeMap,
				Elem:     &schema.Schema{Type: schema.TypeString},
				Optional: true,
			},
		})
		runTest(t, resMap, map[string]interface{}{"tags": "tag"}, "expected object type, got")
	})

	t.Run("flat type instead of object", func(t *testing.T) {
		t.Skip("This is caught by the YAML runtime, not the type checker")
		resMap := makeResMap(map[string]*schema.Schema{
			"network_configuration": {
				Type:     schema.TypeList,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"subnets": {
							Type: schema.TypeSet,
							Optional: true,
							Elem: &schema.Schema{Type: schema.TypeString},
						},
					},
				},
			},
		})
		runTest(t, resMap, map[string]interface{}{"network_configuration": "config"}, "expected object type, got")
	})

	t.Run("array instead of object", func(t *testing.T) {
		t.Skip("This is caught by the YAML runtime, not the type checker")
		resMap := makeResMap(map[string]*schema.Schema{
			"network_configuration": {
				Type:     schema.TypeList,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"subnets": {
							Type: schema.TypeSet,
							Elem: &schema.Schema{Type: schema.TypeString},
						},
					},
				},
			},
		})
		runTest(t, resMap, map[string]interface{}{"network_configuration": []string{"config"}}, "expected object type, got")
	})

	t.Run("array instead of map", func(t *testing.T) {
		resMap := makeResMap(map[string]*schema.Schema{
			"tags": {
				Type:     schema.TypeMap,
				Elem:     &schema.Schema{Type: schema.TypeString},
				Optional: true,
			},
		})
		runTest(t, resMap, map[string]interface{}{"tags": []string{"tag"}}, "expected object type, got")
	})

	t.Run("array instead of flat type", func(t *testing.T) {
		t.Skip("This is caught by the YAML runtime, not the type checker")
		resMap := makeResMap(map[string]*schema.Schema{
			"network_configuration": {
				Type:     schema.TypeList,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"assign_public_ip": {
							Type: schema.TypeBool,
						},
					},
				},
			},
		})
		runTest(t, resMap, map[string]interface{}{"network_configuration": map[string]interface{}{"assign_public_ip": []any{true}}}, "expected array type, got")
	})

	t.Run("map instead of array", func(t *testing.T) {
		resMap := makeResMap(map[string]*schema.Schema{
			"network_configuration": {
				Type:     schema.TypeList,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"subnets": {
							Type: schema.TypeSet,
							Optional: true,
							Elem: &schema.Schema{Type: schema.TypeString},
						},
					},
				},
			},
		})
		runTest(t, resMap,
			map[string]interface{}{"networkConfiguration": map[string]any{"subnets": map[string]any{"sub": "sub"}}},
			"expected array type, got")
	})

	t.Run("map instead of flat type", func(t *testing.T) {
		t.Skip("This is caught by the YAML runtime, not the type checker")
		resMap := makeResMap(map[string]*schema.Schema{
			"network_configuration": {
				Type:     schema.TypeList,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"assign_public_ip": {
							Type: schema.TypeBool,
						},
					},
				},
			},
		})
		runTest(t, resMap, map[string]interface{}{"network_configuration": map[string]interface{}{"assign_public_ip": map[string]any{"val": true}}}, "expected array type, got")
	})
}
