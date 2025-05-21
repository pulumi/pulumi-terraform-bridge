package pftfcheck

import (
	"testing"

	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/stretchr/testify/require"

	pb "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/internal/providerbuilder"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tests/tfcheck"
)

func TestTFBasic(t *testing.T) {
	t.Parallel()
	prov := pb.NewProvider(pb.NewProviderArgs{
		AllResources: []pb.Resource{
			pb.NewResource(pb.NewResourceArgs{
				Name: "res",
				ResourceSchema: rschema.Schema{
					Attributes: map[string]rschema.Attribute{
						"s": rschema.StringAttribute{Optional: true},
					},
				},
			}),
		},
	})

	driver := tfcheck.NewTfDriver(t, t.TempDir(), prov.TypeName, tfcheck.NewTFDriverOpts{
		V6Provider: prov,
	})

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
	err = driver.ApplyPlan(t, plan)
	require.NoError(t, err)

	require.Equal(t, "hello", driver.GetOutput(t, "s_val"))
}

func TestTFDefaults(t *testing.T) {
	t.Parallel()
	prov := pb.NewProvider(pb.NewProviderArgs{
		AllResources: []pb.Resource{
			pb.NewResource(pb.NewResourceArgs{
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
			}),
		},
	})

	driver := tfcheck.NewTfDriver(t, t.TempDir(), prov.TypeName, tfcheck.NewTFDriverOpts{
		V6Provider: prov,
	})

	driver.Write(t, `
resource "testprovider_res" "test" {}
output "s_val" {
	value = testprovider_res.test.s
}
`)

	plan, err := driver.Plan(t)
	require.NoError(t, err)
	err = driver.ApplyPlan(t, plan)
	require.NoError(t, err)

	require.Equal(t, "Default val", driver.GetOutput(t, "s_val"))
}
