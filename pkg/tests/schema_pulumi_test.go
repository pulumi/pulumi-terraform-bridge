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
	bridgedProvider := pulcheck.BridgedProvider(t, "prov", resMap)
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

// The clean refresh on empty/nil collections is an intentional divergence from TF behaviour.
func TestCollectionsRefreshClean(t *testing.T) {
	for _, tc := range []struct {
		name               string
		planResourceChange bool
		schemaType         schema.ValueType
		readVal            interface{}
		// Note maps are not pluralized in the program while lists and sets are.
		programVal     string
		outputString   string
		expectedOutput interface{}
	}{
		{
			name:               "map null with planResourceChange",
			planResourceChange: true,
			schemaType:         schema.TypeMap,
			readVal:            map[string]interface{}{},
			programVal:         "collectionProp: null",
			outputString:       "${mainRes.collectionProp}",
			expectedOutput:     nil,
		},
		{
			name:               "map null without planResourceChange",
			planResourceChange: false,
			schemaType:         schema.TypeMap,
			readVal:            map[string]interface{}{},
			programVal:         "collectionProp: null",
			outputString:       "${mainRes.collectionProp}",
			expectedOutput:     map[string]interface{}{},
		},
		{
			name:               "map empty with planResourceChange",
			planResourceChange: true,
			schemaType:         schema.TypeMap,
			readVal:            map[string]interface{}{},
			programVal:         "collectionProp: {}",
			outputString:       "${mainRes.collectionProp}",
			expectedOutput:     nil,
		},
		{
			name:               "map empty without planResourceChange",
			planResourceChange: false,
			schemaType:         schema.TypeMap,
			readVal:            map[string]interface{}{},
			programVal:         "collectionProp: {}",
			outputString:       "${mainRes.collectionProp}",
			expectedOutput:     map[string]interface{}{},
		},
		{
			name:               "map nonempty with planResourceChange",
			planResourceChange: true,
			schemaType:         schema.TypeMap,
			readVal:            map[string]interface{}{"val": "test"},
			programVal:         `collectionProp: {"val": "test"}`,
			outputString:       "${mainRes.collectionProp}",
			expectedOutput:     map[string]interface{}{"val": "test"},
		},
		{
			name:               "map nonempty without planResourceChange",
			planResourceChange: false,
			schemaType:         schema.TypeMap,
			readVal:            map[string]interface{}{"val": "test"},
			programVal:         `collectionProp: {"val": "test"}`,
			outputString:       "${mainRes.collectionProp}",
			expectedOutput:     map[string]interface{}{"val": "test"},
		},
		{
			name:               "list null with planResourceChange",
			planResourceChange: true,
			schemaType:         schema.TypeList,
			readVal:            []interface{}{},
			programVal:         "collectionProps: null",
			outputString:       "${mainRes.collectionProps}",
			expectedOutput:     nil,
		},
		{
			name:               "list null without planResourceChange",
			planResourceChange: false,
			schemaType:         schema.TypeList,
			readVal:            []interface{}{},
			programVal:         "collectionProps: null",
			outputString:       "${mainRes.collectionProps}",
			expectedOutput:     []interface{}{},
		},
		{
			name:               "list empty with planResourceChange",
			planResourceChange: true,
			schemaType:         schema.TypeList,
			readVal:            []string{},
			programVal:         "collectionProps: []",
			outputString:       "${mainRes.collectionProps}",
			expectedOutput:     []interface{}{},
		},
		{
			name:               "list empty without planResourceChange",
			planResourceChange: false,
			schemaType:         schema.TypeList,
			readVal:            []string{},
			programVal:         "collectionProps: []",
			outputString:       "${mainRes.collectionProps}",
			expectedOutput:     []interface{}{},
		},
		{
			name:               "list nonempty with planResourceChange",
			planResourceChange: true,
			schemaType:         schema.TypeList,
			readVal:            []interface{}{"val"},
			programVal:         `collectionProps: ["val"]`,
			outputString:       "${mainRes.collectionProps}",
			expectedOutput:     []interface{}{"val"},
		},
		{
			name:               "list nonempty without planResourceChange",
			planResourceChange: false,
			schemaType:         schema.TypeList,
			readVal:            []interface{}{"val"},
			programVal:         `collectionProps: ["val"]`,
			outputString:       "${mainRes.collectionProps}",
			expectedOutput:     []interface{}{"val"},
		},
		{
			name:               "set null with planResourceChange",
			planResourceChange: true,
			schemaType:         schema.TypeSet,
			readVal:            []interface{}{},
			programVal:         "collectionProps: null",
			outputString:       "${mainRes.collectionProps}",
			expectedOutput:     nil,
		},
		{
			name:               "set null without planResourceChange",
			planResourceChange: false,
			schemaType:         schema.TypeSet,
			readVal:            []interface{}{},
			programVal:         "collectionProps: null",
			outputString:       "${mainRes.collectionProps}",
			expectedOutput:     []interface{}{},
		},
		{
			name:               "set empty with planResourceChange",
			planResourceChange: true,
			schemaType:         schema.TypeSet,
			readVal:            []interface{}{},
			programVal:         "collectionProps: []",
			outputString:       "${mainRes.collectionProps}",
			expectedOutput:     nil,
		},
		{
			name:               "set empty without planResourceChange",
			planResourceChange: false,
			schemaType:         schema.TypeSet,
			readVal:            []interface{}{},
			programVal:         "collectionProps: []",
			outputString:       "${mainRes.collectionProps}",
			expectedOutput:     []interface{}{},
		},
		{
			name:           "set nonempty with planResourceChange",
			schemaType:     schema.TypeSet,
			readVal:        []interface{}{"val"},
			programVal:     `collectionProps: ["val"]`,
			outputString:   "${mainRes.collectionProps}",
			expectedOutput: []interface{}{"val"},
		},
		{
			name:               "set nonempty without planResourceChange",
			planResourceChange: false,
			schemaType:         schema.TypeSet,
			readVal:            []interface{}{"val"},
			programVal:         `collectionProps: ["val"]`,
			outputString:       "${mainRes.collectionProps}",
			expectedOutput:     []interface{}{"val"},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			resource := &schema.Resource{
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
				CreateContext: func(ctx context.Context, rd *schema.ResourceData, i interface{}) diag.Diagnostics {
					err := rd.Set("collection_prop", tc.readVal)
					require.NoError(t, err)
					rd.SetId("id0")
					return nil
				},
				ReadContext: func(_ context.Context, d *schema.ResourceData, _ interface{}) diag.Diagnostics {
					err := d.Set("collection_prop", tc.readVal)
					require.NoError(t, err)
					err = d.Set("other_prop", "test")
					require.NoError(t, err)
					return nil
				},
				UpdateContext: func(ctx context.Context, rd *schema.ResourceData, i interface{}) diag.Diagnostics {
					return nil
				},
				DeleteContext: func(ctx context.Context, rd *schema.ResourceData, i interface{}) diag.Diagnostics {
					return nil
				},
			}
			require.NoError(t, resource.InternalValidate(nil, true))

			resMap := map[string]*schema.Resource{
				"prov_test": resource,
			}
			opts := []pulcheck.BridgedProviderOpt{}
			if !tc.planResourceChange {
				opts = append(opts, pulcheck.DisablePlanResourceChange())
			}
			bridgedProvider := pulcheck.BridgedProvider(t, "prov", resMap, opts...)
			program := fmt.Sprintf(`
name: test
runtime: yaml
resources:
  mainRes:
    type: prov:index:Test
    properties:
      otherProp: "test"
      %s
outputs:
  collectionOutput: %s
`, tc.programVal, tc.outputString)
			pt := pulcheck.PulCheck(t, bridgedProvider, program)
			upRes := pt.Up()
			require.Equal(t, tc.expectedOutput, upRes.Outputs["collectionOutput"].Value)
			res := pt.Refresh(optrefresh.ExpectNoChanges())
			t.Logf(res.StdOut)
		})
	}
}

func TestNestedEmptyMapRefreshClean(t *testing.T) {
	resMap := map[string]*schema.Resource{
		"prov_test": {
			Schema: map[string]*schema.Schema{
				"prop": {
					Type:     schema.TypeList,
					Optional: true,
					Elem: &schema.Resource{
						Schema: map[string]*schema.Schema{
							"map_prop": {
								Type:     schema.TypeMap,
								Optional: true,
								Elem: &schema.Schema{
									Type: schema.TypeString,
								},
							},
						},
					},
				},
				"other_prop": {
					Type:     schema.TypeString,
					Optional: true,
				},
			},
			ReadContext: func(_ context.Context, d *schema.ResourceData, _ interface{}) diag.Diagnostics {
				err := d.Set("prop", []map[string]interface{}{{"map_prop": map[string]interface{}{}}})
				require.NoError(t, err)
				err = d.Set("other_prop", "test")
				require.NoError(t, err)
				return nil
			},
		},
	}
	bridgedProvider := pulcheck.BridgedProvider(t, "prov", resMap)
	program := `
name: test
runtime: yaml
resources:
  mainRes:
    type: prov:index:Test
    properties:
      otherProp: "test"
      props:
        - mapProp: {}
`
	pt := pulcheck.PulCheck(t, bridgedProvider, program)
	pt.Up()

	res := pt.Refresh(optrefresh.ExpectNoChanges())
	t.Logf(res.StdOut)
}
