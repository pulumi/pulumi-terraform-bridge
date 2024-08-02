package tfbridgetests

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	pschema "github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/pulumi/pulumi-terraform-bridge/pf/tests/internal/providerbuilder"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optpreview"
	presource "github.com/pulumi/pulumi/sdk/v3/go/common/resource"
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

	prov := bridgedProvider(&provBuilder, nil)

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

	prov := bridgedProvider(&provBuilder, nil)

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

func TestIDAttribute(t *testing.T) {
	tests := []struct {
		name                string
		attribute           rschema.Attribute
		computeIdField      string
		schemaName          string
		expectErrorContains string
		expectedIdOutput    string
	}{
		{
			name:             "Valid - Optional + Computed",
			attribute:        rschema.StringAttribute{Optional: true, Computed: true},
			computeIdField:   "otherId",
			schemaName:       "otherId",
			expectedIdOutput: "test-id",
		},
		{
			name:             "Valid - Optional",
			attribute:        rschema.StringAttribute{Optional: true},
			computeIdField:   "otherId",
			schemaName:       "otherId",
			expectedIdOutput: "test-id",
		},
		{
			name:             "Valid - Computed (with computeID & Name)",
			attribute:        rschema.StringAttribute{Computed: true},
			computeIdField:   "otherId",
			schemaName:       "otherId",
			expectedIdOutput: "test-id",
		},
		{
			name:             "Valid - Computed (without computeID & Name)",
			attribute:        rschema.StringAttribute{Computed: true},
			expectedIdOutput: "test-id",
		},
		{
			// ComputeID must point to an existing field
			name:                "Computed error",
			attribute:           rschema.StringAttribute{Computed: true},
			computeIdField:      "otherId",
			expectErrorContains: "Could not find required property 'otherId' in state",
		},
		{
			name:             "Computed id points to a different field",
			attribute:        rschema.StringAttribute{Computed: true},
			computeIdField:   "s",
			expectedIdOutput: "hello",
		},
		{
			// delegate to an existing field
			name:             "Optional id points to a different field",
			attribute:        rschema.StringAttribute{Optional: true},
			computeIdField:   "s",
			schemaName:       "s",
			expectedIdOutput: "hello",
		},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			provBuilder := providerbuilder.Provider{
				TypeName:       "prov",
				Version:        "0.0.1",
				ProviderSchema: pschema.Schema{},
				AllResources: []providerbuilder.Resource{
					{
						Name: "test",
						ResourceSchema: rschema.Schema{
							Attributes: map[string]rschema.Attribute{
								"id": tc.attribute,
								"s":  rschema.StringAttribute{Optional: true},
							},
						},
						CreateFunc: func(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
							resp.State = tfsdk.State(req.Config)
							resp.State.SetAttribute(ctx, path.Root("id"), "test-id")
						},
					},
				},
			}

			var computeIdField tfbridge.ComputeID
			var idSchema info.Schema
			if tc.schemaName != "" {
				idSchema = info.Schema{Name: tc.schemaName}
			}
			if tc.computeIdField != "" {
				computeIdField = tfbridge.DelegateIDField(presource.PropertyKey(tc.computeIdField), "prov", "")
			}
			prov := bridgedProvider(&provBuilder, map[string]*info.Resource{
				"prov_test": {
					Tok:       "prov:index/test:Test",
					ComputeID: computeIdField,
					Fields: map[string]*info.Schema{
						"id": &idSchema,
					},
				},
			})

			program := `
name: test
runtime: yaml
outputs:
  id: ${mainRes.id}
  s: ${mainRes.s}
resources:
    mainRes:
        type: prov:index:Test
        properties:
            s: "hello"
`

			pt := pulCheck(t, prov, program)

			upres, err := pt.CurrentStack().Up(pt.Context())
			if tc.expectErrorContains != "" {
				assert.Contains(t, err.Error(), tc.expectErrorContains)
			} else {
				assert.NoError(t, err)
			}

			if val, ok := upres.Outputs["id"].Value.(string); ok {
				assert.Equal(t, tc.expectedIdOutput, val)
			}
			t.Logf("stderr: %s", upres.StdErr)
			t.Logf("stdout: %s", upres.StdOut)

		})
	}
}
