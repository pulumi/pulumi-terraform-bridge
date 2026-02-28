package tests

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/internal/tests/pulcheck"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
)

func TestDataSourceSensitiveOutput(t *testing.T) {
	t.Parallel()

	prov := &schema.Provider{
		DataSourcesMap: map[string]*schema.Resource{
			"prov_test": {
				Schema: map[string]*schema.Schema{
					"prop": {
						Type:      schema.TypeString,
						Computed:  true,
						Sensitive: true,
					},
				},
				ReadContext: func(ctx context.Context, d *schema.ResourceData, i interface{}) diag.Diagnostics {
					d.SetId("id")
					err := d.Set("prop", "value")
					require.NoError(t, err)
					return diag.Diagnostics{}
				},
			},
		},
	}

	bridgedProvider := pulcheck.BridgedProvider(t, "prov", prov)

	program := `
name: test
runtime: yaml
variables:
  test:
    fn::invoke:
      function: prov:index:getTest
      return: prop
outputs:
	test: ${test}
`

	pt := pulcheck.PulCheck(t, bridgedProvider, program)
	res := pt.Up(t)

	require.Equal(t, "value", res.Outputs["test"].Value)
	require.Equal(t, true, res.Outputs["test"].Secret)
}

func TestDataSourceNestedSensitiveOutput(t *testing.T) {
	t.Parallel()

	prov := &schema.Provider{
		DataSourcesMap: map[string]*schema.Resource{
			"prov_test": {
				Schema: map[string]*schema.Schema{
					"prop": {
						Type:     schema.TypeList,
						Computed: true,
						Elem: &schema.Resource{
							Schema: map[string]*schema.Schema{
								"nested": {
									Type:      schema.TypeString,
									Computed:  true,
									Sensitive: true,
								},
							},
						},
					},
				},
				ReadContext: func(ctx context.Context, d *schema.ResourceData, i interface{}) diag.Diagnostics {
					d.SetId("id")
					err := d.Set("prop", []interface{}{map[string]interface{}{"nested": "value"}})
					require.NoError(t, err)
					return diag.Diagnostics{}
				},
			},
		},
	}

	bridgedProvider := pulcheck.BridgedProvider(t, "prov", prov)

	program := `
name: test
runtime: yaml
variables:
  test:
    fn::invoke:
      function: prov:index:getTest
      return: props
outputs:
	test: ${test[0].nested}
`

	pt := pulcheck.PulCheck(t, bridgedProvider, program)
	res := pt.Up(t)

	require.Equal(t, "value", res.Outputs["test"].Value)
	require.Equal(t, true, res.Outputs["test"].Secret)
}

func TestDataSourceOverlaySecretOutput(t *testing.T) {
	t.Parallel()

	prov := &schema.Provider{
		DataSourcesMap: map[string]*schema.Resource{
			"prov_test": {
				Schema: map[string]*schema.Schema{
					"prop": {
						Type:      schema.TypeString,
						Computed:  true,
						Sensitive: true,
					},
				},
				ReadContext: func(ctx context.Context, d *schema.ResourceData, i interface{}) diag.Diagnostics {
					d.SetId("id")
					err := d.Set("prop", "value")
					require.NoError(t, err)
					return diag.Diagnostics{}
				},
			},
		},
	}

	bridgedProvider := pulcheck.BridgedProvider(t, "prov", prov)
	bridgedProvider.DataSources = map[string]*info.DataSource{
		"prov_test": {
			Tok: "prov:index:getTest",
			Fields: map[string]*info.Schema{
				"prop": {Secret: tfbridge.True()},
			},
		},
	}

	program := `
name: test
runtime: yaml
variables:
  test:
    fn::invoke:
      function: prov:index:getTest
      return: prop
outputs:
	test: ${test}
`

	pt := pulcheck.PulCheck(t, bridgedProvider, program)
	res := pt.Up(t)

	require.Equal(t, "value", res.Outputs["test"].Value)
	require.Equal(t, true, res.Outputs["test"].Secret)
}
