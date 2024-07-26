package pftfcheck

import (
	"testing"

	pschema "github.com/hashicorp/terraform-plugin-framework/provider/schema"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/pulumi/pulumi-terraform-bridge/pf/tests/internal/providerbuilder"
)

func TestBasic(t *testing.T) {
	provBuilder := providerbuilder.Provider{
		TypeName:       "test",
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

	driver := NewTfDriverPF(t, t.TempDir(), "test", &provBuilder)

	driver.Write(t, `
resource "test_test" "test" {
    s = "hello"
}`)

	plan := driver.Plan(t)
	t.Logf(driver.Show(t, plan.PlanFile))
	driver.Apply(t, plan)

	t.Logf(driver.GetState(t))
}

func TestDefaults(t *testing.T) {
	provBuilder := providerbuilder.Provider{
		TypeName:       "test",
		Version:        "0.0.1",
		ProviderSchema: pschema.Schema{},
		AllResources: []providerbuilder.Resource{
			{
				Name: "test",
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

	driver := NewTfDriverPF(t, t.TempDir(), "test", &provBuilder)

	driver.Write(t, `
resource "test_test" "test" {}`)

	plan := driver.Plan(t)
	t.Logf(driver.Show(t, plan.PlanFile))

	driver.Apply(t, plan)
	t.Logf(driver.GetState(t))
}
