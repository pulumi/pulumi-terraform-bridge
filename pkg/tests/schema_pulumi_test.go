package tests

import (
	"context"
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

func TestUnspecifiedMapsRefreshClean(t *testing.T) {
	resMap := map[string]*schema.Resource{
		"prov_test": {
			Schema: map[string]*schema.Schema{
				"map_prop": {
					Type:     schema.TypeMap,
					Optional: true,
					Elem: &schema.Schema{
						Type: schema.TypeString,
					},
				},
				"other_prop": {
					Type:     schema.TypeString,
					Optional: true,
				},
			},
			ReadContext: func(_ context.Context, d *schema.ResourceData, _ interface{}) diag.Diagnostics {
				err := d.Set("map_prop", map[string]interface{}{})
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
outputs:
  mapPropOut: ${mainRes.mapProp}
`
	pt := pulcheck.PulCheck(t, bridgedProvider, program)

	upRes := pt.Up()
	require.Equal(t, nil, upRes.Outputs["mapPropOut"].Value)

	res := pt.Refresh(optrefresh.ExpectNoChanges())
	t.Logf(res.StdOut)
}

func TestEmptyMapsRefreshClean(t *testing.T) {
	resMap := map[string]*schema.Resource{
		"prov_test": {
			Schema: map[string]*schema.Schema{
				"map_prop": {
					Type:     schema.TypeMap,
					Optional: true,
					Elem: &schema.Schema{
						Type: schema.TypeString,
					},
				},
				"other_prop": {
					Type:     schema.TypeString,
					Optional: true,
				},
			},
			ReadContext: func(_ context.Context, d *schema.ResourceData, _ interface{}) diag.Diagnostics {
				err := d.Set("map_prop", map[string]interface{}{})
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
      mapProp: {}
outputs:
  mapPropOut: ${mainRes.mapProp}
`
	pt := pulcheck.PulCheck(t, bridgedProvider, program)

	upRes := pt.Up()
	require.Equal(t, nil, upRes.Outputs["mapPropOut"].Value)

	res := pt.Refresh(optrefresh.ExpectNoChanges())
	t.Logf(res.StdOut)
}
