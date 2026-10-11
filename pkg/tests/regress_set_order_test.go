package tests

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/pulumi/providertest/pulumitest"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/internal/tests/pulcheck"
)

// Refresh must not reorder the recorded inputs of a scalar TypeSet. A set is unordered, so a
// provider returns its elements in an implementation-defined order that generally differs from the
// order the values appear in the program. Rewriting the inputs into that order leaves state
// permanently different from what the program produces. The difference is invisible to Diff(), but
// it defeats the engine's checkpoint write elision and forces a full state write for the resource
// on every subsequent update.
//
// The program writes ["zeta", "alpha"] and the provider returns ["alpha", "zeta"].
func TestRegressScalarSetOrderRefreshPreservesStateInputs(t *testing.T) {
	t.Parallel()

	resMap := map[string]*schema.Resource{
		"prov_test": {
			Schema: map[string]*schema.Schema{
				"administrators": {
					Type:     schema.TypeSet,
					Optional: true,
					Computed: true,
					Elem:     &schema.Schema{Type: schema.TypeString},
					Set:      schema.HashString,
				},
			},
			ReadContext: func(_ context.Context, rd *schema.ResourceData, _ interface{}) diag.Diagnostics {
				require.NoError(t, rd.Set("administrators", []interface{}{"zeta", "alpha"}))
				return nil
			},
			CreateContext: func(_ context.Context, rd *schema.ResourceData, _ interface{}) diag.Diagnostics {
				rd.SetId("id0")
				return nil
			},
		},
	}

	bridgedProvider := pulcheck.BridgedProvider(t, "prov", &schema.Provider{ResourcesMap: resMap})
	pt := pulcheck.PulCheck(t, bridgedProvider, `
name: test
runtime: yaml
resources:
  mainRes:
    type: prov:index:Test
    properties:
      administrators: ["zeta", "alpha"]
`)
	pt.Up(t)

	before := setResInputs(t, pt)
	require.Equal(t, []interface{}{"zeta", "alpha"}, before["administrators"],
		"precondition: state should record the order the program wrote")

	pt.Refresh(t)

	// Assert on the set alone rather than the whole input bag: the empty __defaults marker is
	// dropped by refresh through an unrelated bug, tracked separately as #3567.
	require.Equal(t, before["administrators"], setResInputs(t, pt)["administrators"],
		"refresh must not reorder the inputs recorded in state")
}

func setResInputs(t *testing.T, pt *pulumitest.PulumiTest) map[string]interface{} {
	t.Helper()

	data, err := pt.ExportStack(t).Deployment.MarshalJSON()
	require.NoError(t, err)

	var deployment struct {
		Resources []struct {
			Type   string                 `json:"type"`
			Inputs map[string]interface{} `json:"inputs"`
		} `json:"resources"`
	}
	require.NoError(t, json.Unmarshal(data, &deployment))

	for _, r := range deployment.Resources {
		if r.Type == "prov:index/test:Test" {
			return r.Inputs
		}
	}

	require.Fail(t, "did not find the test resource in the exported stack")
	return nil
}
