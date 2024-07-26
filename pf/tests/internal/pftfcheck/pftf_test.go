package pftfcheck

import (
	"testing"

	pschema "github.com/hashicorp/terraform-plugin-framework/provider/schema"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/pulumi/pulumi-terraform-bridge/pf/tests/internal/providerbuilder"
	"github.com/stretchr/testify/require"
)

func TestBasic(t *testing.T) {
	provBuilder := providerbuilder.Provider{
		TypeName:       "prov",
		Version:        "0.0.1",
		ProviderSchema: pschema.Schema{},
		AllResources: []providerbuilder.Resource{
			{
				Name: "res",
				ResourceSchema: rschema.Schema{
					Attributes: map[string]rschema.Attribute{
						"s": rschema.StringAttribute{Optional: true},
					},
				},
			},
		},
	}

	driver := NewTfDriverPF(t, t.TempDir(), &provBuilder)

	driver.Write(t, `
resource "prov_res" "test" {
    s = "hello"
}
output "s_val" {
	value = prov_res.test.s
}
`)

	plan := driver.Plan(t)
	driver.Apply(t, plan)

	require.Equal(t, "hello", driver.GetOutput(t, "s_val"))
}

func TestDefaults(t *testing.T) {
	provBuilder := providerbuilder.Provider{
		TypeName:       "prov",
		Version:        "0.0.1",
		ProviderSchema: pschema.Schema{},
		AllResources: []providerbuilder.Resource{
			{
				Name: "res",
				ResourceSchema: rschema.Schema{
					Attributes: map[string]rschema.Attribute{
						"s": rschema.StringAttribute{
							Optional: true,
							Computed: true,
							Default:  stringdefault.StaticString("Default val"),
						},
					},
				},
			},
		},
	}

	driver := NewTfDriverPF(t, t.TempDir(), &provBuilder)

	driver.Write(t, `
resource "prov_res" "test" {}
output "s_val" {
	value = prov_res.test.s
}
`)

	plan := driver.Plan(t)
	driver.Apply(t, plan)

	require.Equal(t, "Default val", driver.GetOutput(t, "s_val"))
}
