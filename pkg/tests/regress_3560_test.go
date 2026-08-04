package tests

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/internal/tests/pulcheck"
)

// Regression test for https://github.com/pulumi/pulumi-terraform-bridge/issues/3560:
// ignoring a list index absent from the prior state used to materialize an empty
// element (["a", ""]) instead of leaving the append in place.
func TestRegress3560(t *testing.T) {
	t.Parallel()

	resMap := map[string]*schema.Resource{
		"prov_test": {
			Schema: map[string]*schema.Schema{
				"zones": {
					Type:     schema.TypeList,
					Optional: true,
					Elem:     &schema.Schema{Type: schema.TypeString},
				},
			},
		},
	}
	prov := &schema.Provider{ResourcesMap: resMap}
	bridgedProvider := pulcheck.BridgedProvider(t, "prov", prov)

	program := `
name: test
runtime: yaml
resources:
  mainRes:
    type: prov:index:Test
    properties:
      zones: %s
    options:
      ignoreChanges: ["zones[1]"]
outputs:
  zones: ${mainRes.zones}
`

	pt := pulcheck.PulCheck(t, bridgedProvider, fmt.Sprintf(program, `["a"]`))
	pt.Up(t)

	pt.WritePulumiYaml(t, fmt.Sprintf(program, `["a", "z"]`))
	res := pt.Up(t)

	require.Equal(t, []interface{}{"a", "z"}, res.Outputs["zones"].Value)
}
