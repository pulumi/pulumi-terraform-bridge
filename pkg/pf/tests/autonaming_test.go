package tfbridgetests

import (
	"testing"

	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/pulumi/providertest/pulumitest/opttest"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/internal/providerbuilder"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tests/pulcheck"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
)

func TestAutonaming(t *testing.T) {
	t.Parallel()
	provBuilder := providerbuilder.NewProvider(
		providerbuilder.NewProviderArgs{
			AllResources: []providerbuilder.Resource{
				providerbuilder.NewResource(providerbuilder.NewResourceArgs{
					ResourceSchema: rschema.Schema{
						Attributes: map[string]rschema.Attribute{
							"name": rschema.StringAttribute{Optional: true},
						},
					},
				}),
			},
		})

	prov := provBuilder.ToProviderInfo()
	prov.Resources["testprovider_test"] = &tfbridge.ResourceInfo{
		Tok: "testprovider:index:Test",
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
      pattern: ${project}-${name}
resources:
  hello:
    type: testprovider:index:Test
outputs:
  testOut: ${hello.name}
`
	opts := []opttest.Option{
		opttest.Env("PULUMI_EXPERIMENTAL", "true"),
	}
	pt, err := pulcheck.PulCheck(t, prov, program, opts...)
	require.NoError(t, err)
	res := pt.Up(t)
	require.Equal(t, "test-hello", res.Outputs["testOut"].Value)
}
