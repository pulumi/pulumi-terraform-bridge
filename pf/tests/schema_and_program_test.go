package tfbridgetests

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	pschema "github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/pulumi/pulumi-terraform-bridge/pf/tests/internal/providerbuilder"
	pb "github.com/pulumi/pulumi-terraform-bridge/pf/tests/internal/providerbuilder"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optpreview"
	presource "github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

	pt, err := pulCheck(t, prov, program)
	require.NoError(t, err)

	pt.Up()
}

func TestComputedSetNoDiffWhenElementRemoved(t *testing.T) {
	// Regression test for [pulumi/pulumi-terraform-bridge#2192]
	provBuilder := pb.NewProvider(pb.NewProviderArgs{
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
	})

	prov := bridgedProvider(provBuilder)

	program1 := `
name: test
runtime: yaml
resources:
    mainRes:
        type: testprovider:index:Test
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
        type: testprovider:index:Test
        properties:
            vlanNames:
                - name: "vlan1"
                  vlanId: "1"`

	pt, err := pulCheck(t, prov, program1)
	require.NoError(t, err)

	pt.Up()

	pulumiYamlPath := filepath.Join(pt.CurrentStack().Workspace().WorkDir(), "Pulumi.yaml")

	err = os.WriteFile(pulumiYamlPath, []byte(program2), 0o600)
	require.NoError(t, err)

	res := pt.Preview(optpreview.Diff())
	t.Log(res.StdOut)

	for _, entry := range pt.GrpcLog().Entries {
		if entry.Method == "/pulumirpc.ResourceProvider/Diff" {
			var diff map[string]interface{}
			err := json.Unmarshal(entry.Response, &diff)
			require.NoError(t, err)
			assert.Equal(t, "DIFF_SOME", diff["changes"])
		}
	}
}

func TestIDAttribute(t *testing.T) {
	tests := []struct {
		name                     string
		attribute                rschema.Attribute
		computeIDField           string
		schemaName               string
		expectErrorContains      string
		expectTFGenErrorContains string
		expectedIDOutput         string
	}{
		{
			name:             "Valid - Optional + Computed",
			attribute:        rschema.StringAttribute{Optional: true, Computed: true},
			computeIDField:   "otherId",
			schemaName:       "otherId",
			expectedIDOutput: "test-id",
		},
		{
			name:             "Valid - Optional",
			attribute:        rschema.StringAttribute{Optional: true},
			computeIDField:   "otherId",
			schemaName:       "otherId",
			expectedIDOutput: "test-id",
		},
		{
			name:             "Valid - Computed (with computeID & Name)",
			attribute:        rschema.StringAttribute{Computed: true},
			computeIDField:   "otherId",
			schemaName:       "otherId",
			expectedIDOutput: "test-id",
		},
		{
			name:             "Valid - Computed (without computeID & Name)",
			attribute:        rschema.StringAttribute{Computed: true},
			expectedIDOutput: "test-id",
		},
		{
			// ComputeID must point to an existing field
			// or have StringAttribute.Name provided as well
			name:                "Invalid - Computed error",
			attribute:           rschema.StringAttribute{Computed: true},
			computeIDField:      "otherId",
			expectErrorContains: "Could not find required property 'otherId' in state",
		},
		{
			// ComputeID with no "Name"
			name:             "Valid - Computed id points to a different field",
			attribute:        rschema.StringAttribute{Computed: true},
			computeIDField:   "s",
			expectedIDOutput: "hello",
		},
		// This one fails on checks during tfgen
		{
			// without the check failure the runtime error would be:
			// `Resource state did not contain an id property`
			name: "Optional Name error",
			// this would also fail for a Computed only "id" property
			attribute: rschema.StringAttribute{Optional: true, Computed: true},
			// it would also fail if mapped to either an input property or a purely computed property
			schemaName:               "s",
			expectedIDOutput:         "hello",
			expectTFGenErrorContains: "There were 1 unresolved ID mapping errors",
		},

		// While this is technically possible it should probably become an error in the future
		// Current this can cause a race condition where the output of `s` could either be from `s` or `id`
		// https://github.com/pulumi/pulumi-terraform-bridge/issues/2283
		// {
		// 	// delegate to an existing field
		// 	name:             "Optional id points to a different field",
		// 	attribute:        rschema.StringAttribute{Optional: true},
		// 	computeIDField:   "s",
		// 	schemaName:       "s",
		// 	expectedIDOutput: "hello",
		// },
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
								"x":  rschema.StringAttribute{Computed: true},
							},
						},
						CreateFunc: func(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
							resp.State = tfsdk.State(req.Config)
							resp.State.SetAttribute(ctx, path.Root("id"), "test-id")
							resp.State.SetAttribute(ctx, path.Root("x"), "x-id")
						},
					},
				},
			}

			var computeIDField tfbridge.ComputeID
			var idSchema info.Schema
			if tc.schemaName != "" {
				idSchema = info.Schema{Name: tc.schemaName}
			}
			if tc.computeIDField != "" {
				computeIDField = tfbridge.DelegateIDField(presource.PropertyKey(tc.computeIDField), "prov", "")
			}
			prov := bridgedProvider(&provBuilder)
			prov.Resources = map[string]*info.Resource{
				"prov_test": {
					Tok:       "prov:index/test:Test",
					ComputeID: computeIDField,
					Fields: map[string]*info.Schema{
						"id": &idSchema,
					},
				},
			}

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

			pt, err := pulCheck(t, prov, program)
			if tc.expectTFGenErrorContains != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectTFGenErrorContains)
				return
			}
			require.NoError(t, err)

			upres, err := pt.CurrentStack().Up(pt.Context())
			if tc.expectErrorContains != "" {
				assert.Contains(t, err.Error(), tc.expectErrorContains)
			} else {
				assert.NoError(t, err)
			}

			if val, ok := upres.Outputs["id"].Value.(string); ok {
				assert.Equal(t, tc.expectedIDOutput, val)
			}
		})
	}
}

func TestDefaults(t *testing.T) {
	provBuilder := pb.NewProvider(pb.NewProviderArgs{
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
							Computed: true,
							Default:  stringdefault.StaticString("Default val"),
						},
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
            otherProp: "val"
outputs:
    changeReason: ${mainRes.changeReason}`

	pt, err := pulCheck(t, prov, program)
	require.NoError(t, err)
	upRes := pt.Up()
	t.Log(upRes.StdOut)

	require.Equal(t, "Default val", upRes.Outputs["changeReason"].Value)

	pt.Preview(optpreview.Diff(), optpreview.ExpectNoChanges())
}

type changeReasonPlanModifier struct {
	planmodifier.String
}

func (c changeReasonPlanModifier) PlanModifyString(ctx context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	resp.PlanValue = basetypes.NewStringValue("Default val")
}

func (c changeReasonPlanModifier) Description(context.Context) string {
	return "Change reason plan modifier"
}

func (c changeReasonPlanModifier) MarkdownDescription(context.Context) string {
	return "Change reason plan modifier"
}

func TestPlanModifiers(t *testing.T) {
	provBuilder := pb.NewProvider(pb.NewProviderArgs{
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
							Computed: true,
							PlanModifiers: []planmodifier.String{
								changeReasonPlanModifier{},
							},
						},
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
            otherProp: "val"
outputs:
    changeReason: ${mainRes.changeReason}`

	pt, err := pulCheck(t, prov, program)
	require.NoError(t, err)
	upRes := pt.Up()
	t.Log(upRes.StdOut)

	require.Equal(t, "Default val", upRes.Outputs["changeReason"].Value)

	pt.Preview(optpreview.Diff(), optpreview.ExpectNoChanges())
}

func TestImportSingleNested(t *testing.T) {
	// https://github.com/pulumi/pulumi-terraform-bridge/issues/2219

	type StringPropModel struct {
		Title       types.String `tfsdk:"title"`
		Description types.String `tfsdk:"description"`
	}

	type PropertiesModel struct {
		StringProps map[string]StringPropModel `tfsdk:"string_props"`
	}

	type BlueprintModel struct {
		ID         string           `tfsdk:"id"`
		Properties *PropertiesModel `tfsdk:"properties"`
	}

	provBuilder := pb.NewProvider(pb.NewProviderArgs{
		AllResources: []providerbuilder.Resource{
			{
				Name: "blueprint",
				ResourceSchema: rschema.Schema{
					Attributes: map[string]rschema.Attribute{
						"id": rschema.StringAttribute{
							Computed: true,
						},
						"properties": rschema.SingleNestedAttribute{
							Optional: true,
							Attributes: map[string]rschema.Attribute{
								"string_props": rschema.MapNestedAttribute{
									MarkdownDescription: "The string property of the blueprint",
									Optional:            true,
									NestedObject: rschema.NestedAttributeObject{
										Attributes: map[string]rschema.Attribute{
											"description": schema.StringAttribute{
												MarkdownDescription: "The description of the property",
												Optional:            true,
											},
											"title": schema.StringAttribute{
												MarkdownDescription: "The title of the property",
												Optional:            true,
											},
										},
									},
								},
							},
						},
					},
				},
				ImportStateFunc: func(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
					resp.State.SetAttribute(ctx, path.Root("id"), types.StringValue("test-id"))
				},
				ReadFunc: func(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
					resp.State.Set(ctx, BlueprintModel{
						ID: "test-id",
						Properties: &PropertiesModel{
							StringProps: map[string]StringPropModel{
								"description": {
									Title: types.StringValue("Description"),
								},
							},
						},
					})
				},
			},
		},
	})

	prov := bridgedProvider(provBuilder)

	program := `
name: test
runtime: yaml`

	pt, err := pulCheck(t, prov, program)
	require.NoError(t, err)

	out := pt.Import("testprovider:index/blueprint:Blueprint", "testres", "test-id", "")
	t.Log(out.Stdout)
	t.Log(out.Stderr)

	require.Equal(t, 0, out.ReturnCode)

	t.FailNow()
}
