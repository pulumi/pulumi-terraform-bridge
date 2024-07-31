package tfbridgetests

import (
	"os"
	"path/filepath"
	"testing"

	pschema "github.com/hashicorp/terraform-plugin-framework/provider/schema"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/pulumi/pulumi-terraform-bridge/pf/tests/internal/providerbuilder"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optpreview"
	"github.com/stretchr/testify/assert"
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

func TestComputedSetNoDiffWhenElementRemoved(t *testing.T) {
	// Regression test for [pulumi/pulumi-terraform-bridge#2192]
	provBuilder := providerbuilder.Provider{
		TypeName:       "prov",
		Version:        "0.0.1",
		ProviderSchema: pschema.Schema{},
		AllResources: []providerbuilder.Resource{
			{
				Name: "test",
				ResourceSchema: rschema.Schema{
					Attributes: map[string]rschema.Attribute{
						"vlan_names": rschema.SetNestedAttribute{
							MarkdownDescription: `An array of named VLANs`,
							Computed:            true,
							Optional:            true,
							PlanModifiers: []planmodifier.Set{
								setplanmodifier.UseStateForUnknown(),
							},
							NestedObject: rschema.NestedAttributeObject{
								Attributes: map[string]rschema.Attribute{
									"name": rschema.StringAttribute{
										MarkdownDescription: `Name of the VLAN, string length must be from 1 to 32 characters`,
										Optional:            true,
										PlanModifiers: []planmodifier.String{
											stringplanmodifier.UseStateForUnknown(),
										},
									},
									"vlan_id": rschema.StringAttribute{
										MarkdownDescription: `VLAN ID`,
										Computed:            true,
										Optional:            true,
										PlanModifiers: []planmodifier.String{
											stringplanmodifier.UseStateForUnknown(),
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	prov := bridgedProvider(&provBuilder)

	program1 := `
name: test
runtime: yaml
resources:
    mainRes:
        type: prov:index:Test
        properties:
            vlanNames:
                - name: "vlan1"
                  vlanId: "1"
                - name: "vlan2"
                  vlanId: "2"`

	program2 := `
name: test
runtime: yaml
resources:
    mainRes:
        type: prov:index:Test
        properties:
            vlanNames:
                - name: "vlan1"
                  vlanId: "1"`

	pt := pulCheck(t, prov, program1)

	pt.Up()

	pulumiYamlPath := filepath.Join(pt.CurrentStack().Workspace().WorkDir(), "Pulumi.yaml")

	err := os.WriteFile(pulumiYamlPath, []byte(program2), 0o600)
	require.NoError(t, err)

	res := pt.Preview(optpreview.Diff())
	t.Logf(res.StdOut)

	diffs, err := pt.GrpcLog().Diffs()
	require.NoError(t, err)

	assert.Len(t, diffs, 1)
	assert.Equal(t, "DIFF_SOME", diffs[0].Response.Changes.String())
}
