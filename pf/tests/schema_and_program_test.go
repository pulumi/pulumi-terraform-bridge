package tfbridgetests

import (
	"testing"

	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/pulumi/pulumi-terraform-bridge/pf/tests/internal/providerbuilder"
)

func TestBasic(t *testing.T) {
	provBuilder := providerbuilder.NewProvider(
		providerbuilder.NewProviderArgs{
			AllResources: []providerbuilder.Resource{
				{
					Name: "test",
					ResourceSchema: rschema.Schema{
						Attributes: map[string]rschema.Attribute{
							"s": rschema.StringAttribute{Optional: true},
						},
					},
				},
			},
		})

	prov := bridgedProvider(provBuilder)

	program := `
name: test
runtime: yaml
resources:
    mainRes:
        type: testprovider:index:Test
        properties:
            s: "hello"`

	pt := pulCheck(t, prov, program)

	pt.Up()
}
