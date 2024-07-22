package tfbridgetests

import (
	"testing"

	pschema "github.com/hashicorp/terraform-plugin-framework/provider/schema"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/pulumi/pulumi-terraform-bridge/pf/tests/internal/providerbuilder"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optpreview"
	"github.com/stretchr/testify/require"
)

func TestBasic(t *testing.T) {
	provBuilder := providerbuilder.Provider{
		TypeName:       "prov",
		Version:        "0.0.1",
		ProviderSchema: pschema.Schema{},
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
	}

	prov := bridgedProvider(&provBuilder)

	program := `
name: test
runtime: yaml
resources:
    mainRes:
        type: prov:index:Test
        properties:
            s: "hello"`

	pt := pulCheck(t, prov, program)

	pt.Up()
}

func TestDefaults(t *testing.T) {
	provBuilder := providerbuilder.Provider{
		TypeName:       "prov",
		Version:        "0.0.1",
		ProviderSchema: pschema.Schema{},
		AllResources: []providerbuilder.Resource{
			{
				Name: "test",
				ResourceSchema: rschema.Schema{
					Attributes: map[string]rschema.Attribute{
						"other_prop": rschema.StringAttribute{
							Optional: true,
						},
						"change_reason": rschema.StringAttribute{
							Optional: true,
							Default:  stringdefault.StaticString("Default val"),
						},
					},
				},
			},
		},
	}

	prov := bridgedProvider(&provBuilder)

	program := `
name: test
runtime: yaml
resources:
    mainRes:
        type: prov:index:Test
        properties:
            otherProp: "val"
outputs:
    changeReason: ${mainRes.changeReason}`

	pt := pulCheck(t, prov, program)
	upRes := pt.Up()
	t.Logf(upRes.StdOut)

	require.Equal(t, "Default val", upRes.Outputs["changeReason"].Value)

	pt.Preview(optpreview.Diff(), optpreview.ExpectNoChanges())
}
