package tests

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tests/pulcheck"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optpreview"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optrefresh"
	"github.com/stretchr/testify/require"
)


func TestCollectionsNullEmptyRefreshClean(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name               string
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
			name:                   "map null",
			schemaType:             schema.TypeMap,
			cloudVal:               map[string]interface{}{},
			programVal:             "null",
			expectedOutputTopLevel: nil,
			expectedOutputNested:   nil,
		},
		{
			name:                   "map null with nil cloud value",
			schemaType:             schema.TypeMap,
			cloudVal:               nil,
			programVal:             "null",
			expectedOutputTopLevel: nil,
			expectedOutputNested:   nil,
		},
		{
			name:                   "map null with cloud override",
			schemaType:             schema.TypeMap,
			cloudVal:               map[string]interface{}{},
			programVal:             "null",
			createCloudValOverride: true,
			expectedOutputTopLevel: nil,
			expectedOutputNested:   nil,
		},
		{
			name:                   "map null with nil cloud value and cloud override",
			schemaType:             schema.TypeMap,
			cloudVal:               nil,
			programVal:             "null",
			createCloudValOverride: true,
			expectedOutputTopLevel: nil,
			expectedOutputNested:   nil,
		},
		{
			name:                   "map empty",
			schemaType:             schema.TypeMap,
			cloudVal:               map[string]interface{}{},
			programVal:             "{}",
			expectedOutputTopLevel: nil,
			expectedOutputNested:   nil,
		},
		{
			name:                   "map empty with cloud override",
			schemaType:             schema.TypeMap,
			cloudVal:               map[string]interface{}{},
			programVal:             "{}",
			createCloudValOverride: true,
			expectedOutputTopLevel: nil,
			expectedOutputNested:   nil,
		},
		{
			name:                   "map nonempty",
			schemaType:             schema.TypeMap,
			cloudVal:               map[string]interface{}{"val": "test"},
			programVal:             `{"val": "test"}`,
			expectedOutputTopLevel: map[string]interface{}{"val": "test"},
			expectedOutputNested:   map[string]interface{}{"val": "test"},
		},
		{
			name:                   "map nonempty with cloud override",
			schemaType:             schema.TypeMap,
			cloudVal:               map[string]interface{}{"val": "test"},
			programVal:             `{"val": "test"}`,
			createCloudValOverride: true,
			expectedOutputTopLevel: map[string]interface{}{"val": "test"},
			expectedOutputNested:   map[string]interface{}{"val": "test"},
		},
		{
			name:                   "list null",
			schemaType:             schema.TypeList,
			cloudVal:               []interface{}{},
			programVal:             "null",
			expectedOutputTopLevel: nil,
			expectedOutputNested:   nil,
		},
		{
			name:                   "list null with nil cloud value",
			schemaType:             schema.TypeList,
			cloudVal:               nil,
			programVal:             "null",
			expectedOutputTopLevel: nil,
			expectedOutputNested:   nil,
		},
		{
			name:                   "list null with cloud override",
			schemaType:             schema.TypeList,
			cloudVal:               []interface{}{},
			programVal:             "null",
			createCloudValOverride: true,
			expectedOutputTopLevel: nil,
			expectedOutputNested:   nil,
		},
		{
			name:                   "list null with nil cloud value and cloud override",
			schemaType:             schema.TypeList,
			cloudVal:               nil,
			programVal:             "null",
			createCloudValOverride: true,
			expectedOutputTopLevel: nil,
			expectedOutputNested:   nil,
		},
		{
			name:                   "list empty",
			schemaType:             schema.TypeList,
			cloudVal:               []string{},
			programVal:             "[]",
			expectedOutputTopLevel: []interface{}{},
			expectedOutputNested:   []interface{}{},
		},
		{
			name:                   "list empty with cloud override",
			schemaType:             schema.TypeList,
			cloudVal:               []string{},
			programVal:             "[]",
			createCloudValOverride: true,
			expectedOutputTopLevel: []interface{}{},
			expectedOutputNested:   []interface{}{},
		},
		{
			name:                   "list nonempty",
			schemaType:             schema.TypeList,
			cloudVal:               []interface{}{"val"},
			programVal:             `["val"]`,
			expectedOutputTopLevel: []interface{}{"val"},
			expectedOutputNested:   []interface{}{"val"},
		},
		{
			name:                   "list nonempty with cloud override",
			schemaType:             schema.TypeList,
			cloudVal:               []interface{}{"val"},
			programVal:             `["val"]`,
			createCloudValOverride: true,
			expectedOutputTopLevel: []interface{}{"val"},
			expectedOutputNested:   []interface{}{"val"},
		},
		{
			name:                   "set null",
			schemaType:             schema.TypeSet,
			cloudVal:               []interface{}{},
			programVal:             "null",
			expectedOutputTopLevel: nil,
			expectedOutputNested:   nil,
		},
		{
			name:                   "set null with nil cloud value",
			schemaType:             schema.TypeSet,
			cloudVal:               nil,
			programVal:             "null",
			expectedOutputTopLevel: nil,
			expectedOutputNested:   nil,
		},
		{
			name:                   "set null with cloud override",
			schemaType:             schema.TypeSet,
			cloudVal:               []interface{}{},
			programVal:             "null",
			createCloudValOverride: true,
			expectedOutputTopLevel: nil,
			expectedOutputNested:   nil,
		},
		{
			name:                   "set null with nil cloud value and cloud override",
			schemaType:             schema.TypeSet,
			cloudVal:               nil,
			programVal:             "null",
			createCloudValOverride: true,
			expectedOutputTopLevel: nil,
			expectedOutputNested:   nil,
		},
		{
			name:                   "set empty",
			schemaType:             schema.TypeSet,
			cloudVal:               []interface{}{},
			programVal:             "[]",
			expectedOutputTopLevel: nil,
			expectedOutputNested:   nil,
		},
		{
			name:                   "set empty with cloud override",
			schemaType:             schema.TypeSet,
			cloudVal:               []interface{}{},
			programVal:             "[]",
			createCloudValOverride: true,
			expectedOutputTopLevel: nil,
			expectedOutputNested:   nil,
		},
		{
			name:                   "set nonempty",
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
	} {
		collectionPropPlural := ""
		pluralized := tc.schemaType == schema.TypeList || tc.schemaType == schema.TypeSet
		if pluralized {
			collectionPropPlural += "s"
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
				bridgedProvider := pulcheck.BridgedProvider(t, "prov", tfp)
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
				bridgedProvider := pulcheck.BridgedProvider(t, "prov", tfp)
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