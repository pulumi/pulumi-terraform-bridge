package crosstests

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/pulumi/providertest/providers"
	"github.com/pulumi/providertest/pulumitest"
	"github.com/pulumi/providertest/pulumitest/opttest"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/tokens"
	shimv2 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v2"
	"github.com/stretchr/testify/require"
)

func pulTest(t *testing.T, providerName string, resMap map[string]*schema.Resource, program string) *pulumitest.PulumiTest {
	tfp := &schema.Provider{ResourcesMap: resMap}
	ensureProviderValid(t, tfp)

	shimProvider := shimv2.NewProvider(tfp, shimv2.WithPlanResourceChange(
		func(tfResourceType string) bool { return true },
	))

	provider := tfbridge.ProviderInfo{
		P:    shimProvider,
		Name: providerName,
	}
	makeToken := func(module, name string) (string, error) {
		return tokens.MakeStandard(providerName)(module, name)
	}
	provider.MustComputeTokens(tokens.SingleModule("prov", "index", makeToken))
	provider.MustApplyAutoAliases()

	puwd := t.TempDir()
	p := filepath.Join(puwd, "Pulumi.yaml")

	err := os.WriteFile(p, []byte(program), 0o600)
	require.NoError(t, err)

	opts := []opttest.Option{
		opttest.TestInPlace(),
		opttest.SkipInstall(),
		opttest.AttachProvider(
			defProviderShortName,
			func(ctx context.Context, pt providers.PulumiTest) (providers.Port, error) {
				handle, err := startPulumiProvider(ctx, providerName, "0.0.1", provider)
				require.NoError(t, err)
				return providers.Port(handle.Port), nil
			},
		),
	}

	return pulumitest.NewPulumiTest(t, puwd, opts...)
}

func TestUnknownHandling(t *testing.T) {
	resMap := map[string]*schema.Resource{
		"test": {
			Schema: map[string]*schema.Schema{
				"test": {
					Type:     schema.TypeString,
					Optional: true,
				},
			},
		},
		"aux": {
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
  test: ${mainRes.test}
`
	pt := pulTest(t, "prov", resMap, program)
	res := pt.Up()
	require.Equal(t, "aux", res.Outputs["test"])
}
