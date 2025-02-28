package tfbridgetests

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	pschema "github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optpreview"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	presource "github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/internal/providerbuilder"
	pb "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/internal/providerbuilder"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tests/pulcheck"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
)

func TestBasic(t *testing.T) {
	t.Parallel()
	provBuilder := providerbuilder.NewProvider(
		providerbuilder.NewProviderArgs{
			AllResources: []providerbuilder.Resource{
				providerbuilder.NewResource(providerbuilder.NewResourceArgs{
					ResourceSchema: rschema.Schema{
						Attributes: map[string]rschema.Attribute{
							"s": rschema.StringAttribute{Optional: true},
						},
					},
				}),
			},
		})

	prov := provBuilder.ToProviderInfo()

	program := `
name: test
runtime: yaml
resources:
    mainRes:
        type: testprovider:index:Test
        properties:
            s: "hello"`

	pt, err := pulcheck.PulCheck(t, prov, program)
	require.NoError(t, err)

	pt.Up(t)
}

func TestComputedSetNoDiffWhenElementRemoved(t *testing.T) {
	t.Parallel()
	// Regression test for [pulumi/pulumi-terraform-bridge#2192]
	provBuilder := pb.NewProvider(pb.NewProviderArgs{
		AllResources: []providerbuilder.Resource{
			providerbuilder.NewResource(providerbuilder.NewResourceArgs{
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
			}),
		},
	})

	prov := provBuilder.ToProviderInfo()

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

	pt, err := pulcheck.PulCheck(t, prov, program1)
	require.NoError(t, err)

	pt.Up(t)

	pulumiYamlPath := filepath.Join(pt.CurrentStack().Workspace().WorkDir(), "Pulumi.yaml")

	err = os.WriteFile(pulumiYamlPath, []byte(program2), 0o600)
	require.NoError(t, err)

	res := pt.Preview(t, optpreview.Diff())
	t.Log(res.StdOut)

	for _, entry := range pt.GrpcLog(t).Entries {
		if entry.Method == "/pulumirpc.ResourceProvider/Diff" {
			var diff map[string]interface{}
			err := json.Unmarshal(entry.Response, &diff)
			require.NoError(t, err)
			assert.Equal(t, "DIFF_SOME", diff["changes"])
		}
	}
}

func TestIDAttribute(t *testing.T) {
	t.Parallel()
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
			t.Parallel()
			provBuilder := providerbuilder.Provider{
				TypeName:       "prov",
				Version:        "0.0.1",
				ProviderSchema: pschema.Schema{},
				AllResources: []providerbuilder.Resource{
					providerbuilder.NewResource(providerbuilder.NewResourceArgs{
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
					}),
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
			prov := provBuilder.ToProviderInfo()
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

			pt, err := pulcheck.PulCheck(t, prov, program)
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
	t.Parallel()
	provBuilder := pb.NewProvider(pb.NewProviderArgs{
		AllResources: []providerbuilder.Resource{
			providerbuilder.NewResource(providerbuilder.NewResourceArgs{
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
			}),
		},
	})

	prov := provBuilder.ToProviderInfo()

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

	pt, err := pulcheck.PulCheck(t, prov, program)
	require.NoError(t, err)
	upRes := pt.Up(t)
	t.Log(upRes.StdOut)

	require.Equal(t, "Default val", upRes.Outputs["changeReason"].Value)

	pt.Preview(t, optpreview.Diff(), optpreview.ExpectNoChanges())
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
	t.Parallel()
	provBuilder := pb.NewProvider(pb.NewProviderArgs{
		AllResources: []providerbuilder.Resource{
			providerbuilder.NewResource(providerbuilder.NewResourceArgs{
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
			}),
		},
	})

	prov := provBuilder.ToProviderInfo()

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

	pt, err := pulcheck.PulCheck(t, prov, program)
	require.NoError(t, err)
	upRes := pt.Up(t)
	t.Log(upRes.StdOut)

	require.Equal(t, "Default val", upRes.Outputs["changeReason"].Value)

	pt.Preview(t, optpreview.Diff(), optpreview.ExpectNoChanges())
}

type lifecycleRuleFilterModel struct {
	ObjectSizeGreaterThan types.Int64 `tfsdk:"object_size_greater_than"`
}
type lifecycleRuleFilterModelV0 struct {
	ObjectSizeGreaterThan types.String `tfsdk:"object_size_greater_than"`
}

func stringToInt64Legacy(_ context.Context, s types.String, diags *diag.Diagnostics) types.Int64 {
	if s.ValueString() == "" {
		return types.Int64Null()
	}

	v, err := strconv.ParseInt(s.ValueString(), 10, 64)
	if err != nil {
		diags.AddError(
			"Conversion Error",
			fmt.Sprintf("When upgrading state, failed to read a string as an integer value.\n"+
				"Value: %q\nError: %s",
				s.ValueString(),
				err.Error(),
			),
		)
		return types.Int64Unknown()
	}
	return types.Int64Value(v)
}

func TestStateUpgrade(t *testing.T) {
	t.Parallel()
	provBuilder := providerbuilder.NewProvider(
		providerbuilder.NewProviderArgs{
			AllResources: []providerbuilder.Resource{
				providerbuilder.NewResource(providerbuilder.NewResourceArgs{
					UpgradeStateFunc: func(ctx context.Context) map[int64]resource.StateUpgrader {
						return map[int64]resource.StateUpgrader{
							0: {
								StateUpgrader: func(ctx context.Context, usr1 resource.UpgradeStateRequest, usr2 *resource.UpgradeStateResponse) {
									var usr1V0 lifecycleRuleFilterModelV0
									usr2.Diagnostics.Append(usr1.State.Get(ctx, &usr1V0)...)
									usr2.Diagnostics.Append(usr2.State.Set(ctx, lifecycleRuleFilterModel{
										ObjectSizeGreaterThan: stringToInt64Legacy(ctx, usr1V0.ObjectSizeGreaterThan, &usr2.Diagnostics),
									})...)
								},
								PriorSchema: &rschema.Schema{
									Version: 0,
									Attributes: map[string]rschema.Attribute{
										"object_size_greater_than": rschema.StringAttribute{
											Optional: true,
										},
									},
								},
							},
						}
					},
					ResourceSchema: rschema.Schema{
						Attributes: map[string]rschema.Attribute{
							"object_size_greater_than": rschema.Int64Attribute{
								Optional: true,
								Computed: true, // Because of Legacy value handling
								PlanModifiers: []planmodifier.Int64{
									int64planmodifier.UseStateForUnknown(),
								},
							},
						},
					},
				}),
			},
		})

	prov := provBuilder.ToProviderInfo()

	program := `
name: test
runtime: yaml
resources:
    mainRes:
        type: testprovider:index:Test`

	pt, err := pulcheck.PulCheck(t, prov, program)
	require.NoError(t, err)
	pt.ImportStack(t, apitype.UntypedDeployment{
		Version: 3,
		Deployment: []byte(`{
		"manifest": {
				"time": "2025-02-20T14:09:00.155613-05:00",
				"magic": "0cfd49ecb2b79ab5c815533dd5e24026f84295ad05c68df3861ea07ea846919a",
				"version": "v3.150.0"
		},
		"resources": [
				{
						"urn": "urn:pulumi:test::test::pulumi:pulumi:Stack::test-test",
						"custom": false,
						"type": "pulumi:pulumi:Stack",
						"created": "2025-02-20T19:09:00.146543Z",
						"modified": "2025-02-20T19:09:00.146543Z"
				},
				{
						"urn": "urn:pulumi:test::test::pulumi:providers:testprovider::default",
						"custom": true,
						"id": "127dc091-46cd-41f1-a3a9-ccdeca036b02",
						"type": "pulumi:providers:testprovider",
						"created": "2025-02-20T19:09:00.151776Z",
						"modified": "2025-02-20T19:09:00.151776Z"
				},
				{
						"urn": "urn:pulumi:test::test::testprovider:index/test:Test::mainRes",
						"custom": true,
						"id": "test-id",
						"type": "testprovider:index/test:Test",
						"inputs": {
						},
						"outputs": {
								"id": "test-id",
								"objectSizeGreaterThan": ""
						},
						"parent": "urn:pulumi:test::test::pulumi:pulumi:Stack::test-test",
						"provider": "urn:pulumi:test::test::pulumi:providers:testprovider::default::127dc091-46cd-41f1-a3a9-ccdeca036b02",
						"propertyDependencies": {
								"objectSizeGreaterThan": []
						},
						"created": "2025-02-20T19:09:00.154504Z",
						"modified": "2025-02-20T19:09:00.154504Z"
				}
		],
		"metadata": {}
}`),
	})

	pt.Up(t)
}
