package tests

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/pulumi/providertest/pulumitest/opttest"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/internal/tests/pulcheck"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
)

func TestAutonaming(t *testing.T) {
	t.Parallel()
	resMap := map[string]*schema.Resource{
		"prov_test": {
			Schema: map[string]*schema.Schema{
				"name": {
					Type:     schema.TypeString,
					Optional: true,
				},
			},
		},
	}
	tfp := &schema.Provider{ResourcesMap: resMap}
	bridgedProvider := pulcheck.BridgedProvider(t, "prov", tfp)
	bridgedProvider.Resources["prov_test"] = &tfbridge.ResourceInfo{
		Tok: "prov:index:Test",
		Fields: map[string]*tfbridge.SchemaInfo{
			"name": tfbridge.AutoName("name", 50, "-"),
		},
	}
	program := `
name: test
runtime: yaml
config:
  pulumi:autonaming:
    value:
	  pattern: ${name}-world
resources:
  hello:
    type: prov:index:Test
outputs:
  testOut: ${hello.name}
`
	opts := []opttest.Option{
		opttest.Env("PULUMI_EXPERIMENTAL", "true"),
	}
	pt := pulcheck.PulCheck(t, bridgedProvider, program, opts...)
	res := pt.Up(t)
	require.Equal(t, "hello-world", res.Outputs["testOut"].Value)
}
