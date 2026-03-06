package tfbridgetests

import (
	"encoding/json"
	"testing"

	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hexops/autogold/v2"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optpreview"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"

	crosstestsimpl "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/internal/tests/cross-tests/impl"
	pb "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/internal/providerbuilder"
	crosstests "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tests/internal/cross-tests"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tests/pulcheck"
	tfbridge "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
)

func TestPFDetailedDiffMap(t *testing.T) {
	t.Parallel()

	attributeSchema := pb.NewResource(pb.NewResourceArgs{
		ResourceSchema: rschema.Schema{
			Attributes: map[string]rschema.Attribute{
				"key": rschema.MapAttribute{
					Optional:    true,
					ElementType: types.StringType,
				},
			},
		},
	})

	attributeReplaceSchema := pb.NewResource(pb.NewResourceArgs{
		ResourceSchema: rschema.Schema{
			Attributes: map[string]rschema.Attribute{
				"key": rschema.MapAttribute{
					Optional:    true,
					ElementType: types.StringType,
					PlanModifiers: []planmodifier.Map{
						mapplanmodifier.RequiresReplace(),
					},
				},
			},
		},
	})

	nestedAttributeSchema := pb.NewResource(pb.NewResourceArgs{
		ResourceSchema: rschema.Schema{
			Attributes: map[string]rschema.Attribute{
				"key": rschema.MapNestedAttribute{
					Optional: true,
					NestedObject: rschema.NestedAttributeObject{
						Attributes: map[string]rschema.Attribute{
							"nested": rschema.StringAttribute{Optional: true},
						},
					},
				},
			},
		},
	})

	nestedAttributeReplaceSchema := pb.NewResource(pb.NewResourceArgs{
		ResourceSchema: rschema.Schema{
			Attributes: map[string]rschema.Attribute{
				"key": rschema.MapNestedAttribute{
					Optional: true,
					NestedObject: rschema.NestedAttributeObject{
						Attributes: map[string]rschema.Attribute{
							"nested": rschema.StringAttribute{Optional: true},
						},
					},
					PlanModifiers: []planmodifier.Map{
						mapplanmodifier.RequiresReplace(),
					},
				},
			},
		},
	})

	nestedAttributeNestedReplaceSchema := pb.NewResource(pb.NewResourceArgs{
		ResourceSchema: rschema.Schema{
			Attributes: map[string]rschema.Attribute{
				"key": rschema.MapNestedAttribute{
					Optional: true,
					NestedObject: rschema.NestedAttributeObject{
						Attributes: map[string]rschema.Attribute{
							"nested": rschema.StringAttribute{
								Optional: true,
								PlanModifiers: []planmodifier.String{
									stringplanmodifier.RequiresReplace(),
								},
							},
						},
					},
				},
			},
		},
	})

	attrMap := func(m *map[string]*string) cty.Value {
		if m == nil {
			return cty.NullVal(cty.DynamicPseudoType)
		}
		if len(*m) == 0 {
			return cty.MapValEmpty(cty.String)
		}
		values := make(map[string]cty.Value, len(*m))
		for k, v := range *m {
			if v == nil {
				values[k] = cty.NullVal(cty.String)
			} else {
				values[k] = cty.StringVal(*v)
			}
		}
		return cty.MapVal(values)
	}

	nestedAttrMap := func(m *map[string]*string) cty.Value {
		if m == nil {
			return cty.NullVal(cty.DynamicPseudoType)
		}
		if len(*m) == 0 {
			return cty.MapValEmpty(cty.Object(map[string]cty.Type{"nested": cty.String}))
		}
		values := make(map[string]cty.Value, len(*m))
		for k, v := range *m {
			if v == nil {
				values[k] = cty.NullVal(cty.DynamicPseudoType)
			} else {
				values[k] = cty.MapVal(map[string]cty.Value{
					"nested": cty.StringVal(*v),
				})
			}
		}
		return cty.MapVal(values)
	}

	schemaValueMakerPairs := []struct {
		name       string
		res        pb.Resource
		valueMaker func(*map[string]*string) cty.Value
	}{
		{"attribute no replace", attributeSchema, attrMap},
		{"attribute requires replace", attributeReplaceSchema, attrMap},
		{"nested attribute no replace", nestedAttributeSchema, nestedAttrMap},
		{"nested attribute requires replace", nestedAttributeReplaceSchema, nestedAttrMap},
		{"nested attribute nested requires replace", nestedAttributeNestedReplaceSchema, nestedAttrMap},
	}

	scenarios := []struct {
		name         string
		initialValue *map[string]*string
		changeValue  *map[string]*string
	}{
		{"unchanged null", nil, nil},
		{"unchanged empty", &map[string]*string{}, &map[string]*string{}},
		{"unchanged non-empty", &map[string]*string{"k": ref("value")}, &map[string]*string{"k": ref("value")}},
		{"added empty", nil, &map[string]*string{}},
		{"removed empty", &map[string]*string{}, nil},
		{"added non-empty", nil, &map[string]*string{"k": ref("value")}},
		{"removed non-empty", &map[string]*string{"k": ref("value")}, nil},
		{"unchanged null value", &map[string]*string{"k": nil}, &map[string]*string{"k": nil}},
		{"changed value non-null", &map[string]*string{"k": ref("value")}, &map[string]*string{"k": ref("value1")}},

		// TODO[pulumi/pulumi-terraform-bridge#752]: Non-deterministic output
		// {"changed value null to non-null", &map[string]*string{"k": nil}, &map[string]*string{"k": ref("value")}},
		// TODO[pulumi/pulumi-terraform-bridge#752]: We do not correctly identify the replace here.
		// see pkg/pf/tests/testdata/TestDetailedDiffMap/nested_attribute_nested_requires_replace/changed_value_non-null_to_null.golden
		// {"changed value non-null to null", &map[string]*string{"k": ref("value")}, &map[string]*string{"k": nil}},
	}

	for _, schemaValueMakerPair := range schemaValueMakerPairs {
		t.Run(schemaValueMakerPair.name, func(t *testing.T) {
			t.Parallel()
			for _, scenario := range scenarios {
				t.Run(scenario.name, func(t *testing.T) {
					t.Parallel()
					initialValue := schemaValueMakerPair.valueMaker(scenario.initialValue)
					changeValue := schemaValueMakerPair.valueMaker(scenario.changeValue)

					diff := crosstests.Diff(
						t, schemaValueMakerPair.res, map[string]cty.Value{"key": initialValue}, map[string]cty.Value{"key": changeValue},
					)

					autogold.ExpectFile(t, testOutput{
						initialValue: scenario.initialValue,
						changeValue:  scenario.changeValue,
						tfOut:        diff.TFOut,
						pulumiOut:    diff.PulumiOut,
						detailedDiff: diff.PulumiDiff.DetailedDiff,
					})
				})
			}
		})
	}
}

func TestPFDetailedDiffNestedBlockMapOfMapsAccuratePreviewFlag(t *testing.T) {
	t.Parallel()

	res := pb.NewResource(pb.NewResourceArgs{
		ResourceSchema: rschema.Schema{
			Blocks: map[string]rschema.Block{
				"export": rschema.SingleNestedBlock{
					Attributes: map[string]rschema.Attribute{
						"table_configurations": rschema.MapAttribute{
							Optional: true,
							Computed: true,
							ElementType: types.MapType{
								ElemType: types.StringType,
							},
							PlanModifiers: []planmodifier.Map{
								mapplanmodifier.UseStateForUnknown(),
								mapplanmodifier.RequiresReplace(),
							},
						},
					},
				},
			},
		},
	})

	makeProgram := func(includeBillingViewArn bool) string {
		billingViewLine := ""
		if includeBillingViewArn {
			billingViewLine = "                        BILLING_VIEW_ARN: arn:aws:billing::123456789012:billingview/primary\n"
		}

		return `name: project
runtime: yaml
resources:
    p:
        type: testprovider:index:Test
        properties:
            export:
                tableConfigurations:
                    COST_AND_USAGE_REPORT:
` + billingViewLine + `                        INCLUDE_RESOURCES: "TRUE"
                        TIME_GRANULARITY: HOURLY
`
	}

	previewDiff := func(t *testing.T, providerInfo tfbridge.ProviderInfo) (string, crosstestsimpl.PulumiDiffResp) {
		t.Helper()

		pt1, err := pulcheck.PulCheck(t, providerInfo, makeProgram(false))
		require.NoError(t, err)
		pt1.Up(t)
		state := pt1.ExportStack(t)

		pt2, err := pulcheck.PulCheck(t, providerInfo, makeProgram(true))
		require.NoError(t, err)
		pt2.ImportStack(t, state)

		previewRes := pt2.Preview(t, optpreview.Diff())
		diffResponse := crosstestsimpl.GetPulumiDiffResponse(t, pt2.GrpcLog(t).Entries)
		return previewRes.StdOut, diffResponse
	}

	t.Run("disabled by default", func(t *testing.T) {
		providerInfo := pb.NewProvider(pb.NewProviderArgs{
			AllResources: []pb.Resource{res},
		}).ToProviderInfo()

		previewOut, diffResponse := previewDiff(t, providerInfo)
		require.Contains(t, previewOut, "BILLING_VIEW_ARN")
		require.Nil(t, diffResponse.DetailedDiff)
	})

	t.Run("enabled shows nested detailed diff", func(t *testing.T) {
		providerInfo := pb.NewProvider(pb.NewProviderArgs{
			AllResources: []pb.Resource{res},
		}).ToProviderInfo()
		providerInfo.EnableAccuratePFBridgePreview = true

		previewOut, diffResponse := previewDiff(t, providerInfo)
		require.Contains(t, previewOut, "BILLING_VIEW_ARN")

		detailedDiffJSON, err := json.Marshal(diffResponse.DetailedDiff)
		require.NoError(t, err)
		require.Contains(t, string(detailedDiffJSON), "BILLING_VIEW_ARN")

		leaf, ok := diffResponse.DetailedDiff["export.tableConfigurations.COST_AND_USAGE_REPORT.BILLING_VIEW_ARN"]
		require.True(t, ok)
		leafMap, ok := leaf.(map[string]interface{})
		require.True(t, ok)
		require.Equal(t, "ADD_REPLACE", leafMap["kind"])

		_, hasMeta := diffResponse.DetailedDiff["__meta"]
		require.False(t, hasMeta)
	})
}
