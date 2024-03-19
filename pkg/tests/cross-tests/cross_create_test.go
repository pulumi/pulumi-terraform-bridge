package crosstests

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/pulumi/providertest/providers"
	"github.com/pulumi/providertest/pulumitest"
	"github.com/pulumi/providertest/pulumitest/opttest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/stretchr/testify/require"
)

func runCreate(t *testing.T, tc diffTestCase) {
	// TODO: rename diffTestCase
	tfwd := t.TempDir()

	reattachConfig := startTFProvider(t, tc)

	tfWriteJson(t, tfwd, tc.Config1)
	p1 := runTFPlan(t, tfwd, reattachConfig)
	runTFApply(t, tfwd, reattachConfig, p1)

	{
		_, err := json.MarshalIndent(p1.RawPlan, "", "  ")
		contract.AssertNoErrorf(err, "failed to marshal terraform plan")
	}

	if tc.SkipPulumi {
		return
	}

	puwd := t.TempDir()
	pulumiWriteYaml(t, tc, puwd, tc.Config1)

	pt := pulumitest.NewPulumiTest(t, puwd,
		// Needed while using Nix-built pulumi.
		opttest.Env("PULUMI_AUTOMATION_API_SKIP_VERSION_CHECK", "true"),
		opttest.TestInPlace(),
		opttest.SkipInstall(),
		opttest.AttachProvider(
			providerShortName,
			func(ctx context.Context, pt providers.PulumiTest) (providers.Port, error) {
				handle, err := startPulumiProvider(ctx, tc)
				require.NoError(t, err)
				return providers.Port(handle.Port), nil
			},
		),
	)

	pt.Up()
}

func TestMaxItemsOnePropCreateValue(t *testing.T) {
	vals := make([]cty.Value, 0, 2)
	runCreate(t, diffTestCase{
		Resource: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"default_action": {
					Type:     schema.TypeList,
					Required: true,
					Elem: &schema.Resource{
						Schema: map[string]*schema.Schema{
							"forward": {
								Type:     schema.TypeList,
								Optional: true,
								MaxItems: 1,
								Elem: &schema.Schema{
									Type: schema.TypeString,
								},
							},
							"type": {
								Type:     schema.TypeString,
								Required: true,
							},
						},
					},
				},
			},
			CreateContext: func(ctx context.Context, rd *schema.ResourceData, i interface{}) diag.Diagnostics {
				actions := rd.GetRawConfig().GetAttr("default_action")
				val := actions.Index(cty.NumberIntVal(0)).GetAttr("forward")

				vals = append(vals, val)
				t.Logf("Get: %s", val)
				rd.SetId("newid")
				return nil
			},
		},
		Config1: map[string]any{
			"default_action": []map[string]any{
				{
					"type": "forw",
				},
			},
		},
	})
	t.Logf("vals: %v", vals)
	panic("here!")
}
