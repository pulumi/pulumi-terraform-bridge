package pftfcheck

import (
	"testing"

	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	pb "github.com/pulumi/pulumi-terraform-bridge/v3/pf/tests/internal/providerbuilder"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tests/tfcheck"
	"github.com/stretchr/testify/require"
)

func TestBasic(t *testing.T) {
	prov := pb.NewProvider(pb.NewProviderArgs{
		AllResources: []pb.Resource{
			{
				Name: "res",
				ResourceSchema: rschema.Schema{
					Attributes: map[string]rschema.Attribute{
						"s": rschema.StringAttribute{Optional: true},
					},
				},
			},
		},
	})

	driver := tfcheck.NewTfDriver(t, t.TempDir(), prov.TypeName, prov)

	driver.Write(t, `
resource "testprovider_res" "test" {
    s = "hello"
}
output "s_val" {
	value = testprovider_res.test.s
}
`)

	plan, err := driver.Plan(t)
	require.NoError(t, err)
	err = driver.Apply(t, plan)
	require.NoError(t, err)

	require.Equal(t, "hello", driver.GetOutput(t, "s_val"))
}

func TestDefaults(t *testing.T) {
	prov := pb.NewProvider(pb.NewProviderArgs{
		AllResources: []pb.Resource{
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
	})

	driver := tfcheck.NewTfDriver(t, t.TempDir(), prov.TypeName, prov)

	driver.Write(t, `
resource "testprovider_res" "test" {}
output "s_val" {
	value = testprovider_res.test.s
}
`)

	plan, err := driver.Plan(t)
	require.NoError(t, err)
	err = driver.Apply(t, plan)
	require.NoError(t, err)

	require.Equal(t, "Default val", driver.GetOutput(t, "s_val"))
}
