package tests

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optpreview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"

	crosstests "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/internal/tests/cross-tests"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/internal/tests/pulcheck"
)

// TestStateFunc ensures that resources with a StateFunc set on their schema are correctly
// handled. This includes ensuring that the PlannedPrivate blob is passed from
// PlanResourceChange to ApplyResourceChange. If this is passed correctly, the provider
// will see the original value of the field, rather than the value that was produced by
// the StateFunc.
func TestStateFuncProviderNotAffected(t *testing.T) {
	t.Parallel()
	crosstests.Create(t,
		map[string]*schema.Schema{
			"test": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
				StateFunc: func(v interface{}) string {
					return v.(string) + " world"
				},
			},
		},
		cty.ObjectVal(map[string]cty.Value{
			"test": cty.StringVal("hello"),
		}),
	)
}

func TestStateFuncBasic(t *testing.T) {
	t.Parallel()
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
