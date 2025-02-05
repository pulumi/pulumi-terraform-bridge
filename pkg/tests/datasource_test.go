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
outputs:
	prov_test: test.prop
`

	pt := pulcheck.PulCheck(t, bridgedProvider, program)
	res := pt.Up(t)

	require.Equal(t, true, res.Outputs["prov_test"].Secret)
}

func TestDataSourceOverlaySecretOutput(t *testing.T) {
	t.Parallel()

	prov := &schema.Provider{
		DataSourcesMap: map[string]*schema.Resource{
			"prov_test": {
				Schema: map[string]*schema.Schema{
					"prop": {
						Type:     schema.TypeString,
						Computed: true,
					},
				},
				ReadContext: func(ctx context.Context, d *schema.ResourceData, i interface{}) diag.Diagnostics {
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
outputs:
	prov_test: test.prop
`

	pt := pulcheck.PulCheck(t, bridgedProvider, program)
	res := pt.Up(t)

	require.Equal(t, true, res.Outputs["prov_test"].Secret)
}
