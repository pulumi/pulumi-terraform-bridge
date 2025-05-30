package tfbridgetests

import (
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hexops/autogold/v2"
	"github.com/zclconf/go-cty/cty"

	pb "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/internal/providerbuilder"
	crosstests "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tests/internal/cross-tests"
)

func TestPFDetailedDiffObject(t *testing.T) {
	t.Parallel()

	attributeSchema := rschema.Schema{
		Attributes: map[string]rschema.Attribute{
			"key": rschema.ObjectAttribute{
				Optional: true,
				AttributeTypes: map[string]attr.Type{
					"nested": types.StringType,
				},
			},
		},
	}

	attributeReplaceSchema := rschema.Schema{
		Attributes: map[string]rschema.Attribute{
			"key": rschema.ObjectAttribute{
				Optional: true,
				PlanModifiers: []planmodifier.Object{
					objectplanmodifier.RequiresReplace(),
				},
				AttributeTypes: map[string]attr.Type{
					"nested": types.StringType,
				},
			},
		},
	}

	objDef := basetypes.NewObjectValueMust(map[string]attr.Type{
		"nested": types.StringType,
	}, map[string]attr.Value{
		"nested": basetypes.NewStringValue("default"),
	})

	attributeSchemaWithDefault := rschema.Schema{
		Attributes: map[string]rschema.Attribute{
			"key": rschema.ObjectAttribute{
				Optional: true,
				Computed: true,
				AttributeTypes: map[string]attr.Type{
					"nested": types.StringType,
				},
				Default: objectDefault(objDef),
			},
		},
	}

	attributeSchemaWithDefaultReplace := rschema.Schema{
		Attributes: map[string]rschema.Attribute{
			"key": rschema.ObjectAttribute{
				Optional: true,
				Computed: true,
				Default:  objectDefault(objDef),
				PlanModifiers: []planmodifier.Object{
					objectplanmodifier.RequiresReplace(),
				},
				AttributeTypes: map[string]attr.Type{
					"nested": types.StringType,
				},
			},
		},
	}

	attributeSchemaWithPlanModifier := rschema.Schema{
		Attributes: map[string]rschema.Attribute{
			"key": rschema.ObjectAttribute{
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.Object{
					objectDefault(objDef),
				},
				AttributeTypes: map[string]attr.Type{
					"nested": types.StringType,
				},
			},
		},
	}

	attributeSchemaWithPlanModifierReplace := rschema.Schema{
		Attributes: map[string]rschema.Attribute{
			"key": rschema.ObjectAttribute{
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.Object{
					objectplanmodifier.RequiresReplace(),
					objectDefault(objDef),
				},
				AttributeTypes: map[string]attr.Type{
					"nested": types.StringType,
				},
			},
		},
	}

	nestedBlockSchema := rschema.Schema{
		Blocks: map[string]rschema.Block{
			"key": rschema.SingleNestedBlock{
				Attributes: map[string]rschema.Attribute{
					"nested": rschema.StringAttribute{Optional: true},
				},
			},
		},
	}

	nestedBlockReplaceSchema := rschema.Schema{
		Blocks: map[string]rschema.Block{
			"key": rschema.SingleNestedBlock{
				Attributes: map[string]rschema.Attribute{
					"nested": rschema.StringAttribute{
						Optional: true,
					},
				},
				PlanModifiers: []planmodifier.Object{
					objectplanmodifier.RequiresReplace(),
				},
			},
		},
	}

	nestedBlockNestedReplaceSchema := rschema.Schema{
		Blocks: map[string]rschema.Block{
			"key": rschema.SingleNestedBlock{
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
	}

	nestedBlockWithDefaultSchema := rschema.Schema{
		Blocks: map[string]rschema.Block{
			"key": rschema.SingleNestedBlock{
				Attributes: map[string]rschema.Attribute{
					"nested": rschema.StringAttribute{
						Optional: true,
						Computed: true,
						Default:  stringDefault("default"),
					},
				},
			},
		},
	}

	nestedBlockWithDefaultReplaceSchema := rschema.Schema{
		Blocks: map[string]rschema.Block{
			"key": rschema.SingleNestedBlock{
				Attributes: map[string]rschema.Attribute{
					"nested": rschema.StringAttribute{
						Optional: true,
						Computed: true,
						Default:  stringDefault("default"),
					},
				},
				PlanModifiers: []planmodifier.Object{
					objectplanmodifier.RequiresReplace(),
				},
			},
		},
	}

	nestedBlockWithDefaultNestedReplaceSchema := rschema.Schema{
		Blocks: map[string]rschema.Block{
			"key": rschema.SingleNestedBlock{
				Attributes: map[string]rschema.Attribute{
					"nested": rschema.StringAttribute{
						Optional: true,
						Computed: true,
						Default:  stringDefault("default"),
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.RequiresReplace(),
						},
					},
				},
			},
		},
	}

	nestedBlockWithPlanModifierSchema := rschema.Schema{
		Blocks: map[string]rschema.Block{
			"key": rschema.SingleNestedBlock{
				Attributes: map[string]rschema.Attribute{
					"nested": rschema.StringAttribute{
						Optional: true,
						Computed: true,
						PlanModifiers: []planmodifier.String{
							stringDefault("default"),
						},
					},
				},
			},
		},
	}

	nestedBlockWithPlanModifierReplaceSchema := rschema.Schema{
		Blocks: map[string]rschema.Block{
			"key": rschema.SingleNestedBlock{
				Attributes: map[string]rschema.Attribute{
					"nested": rschema.StringAttribute{
						Optional: true,
						Computed: true,
						PlanModifiers: []planmodifier.String{
							stringDefault("default"),
							stringplanmodifier.RequiresReplace(),
						},
					},
				},
			},
		},
	}

	makeValue := func(v *map[string]string) cty.Value {
		if v == nil {
			return cty.NullVal(cty.DynamicPseudoType)
		}
		values := make(map[string]cty.Value, len(*v))
		for k, v := range *v {
			values[k] = cty.StringVal(v)
		}
		return cty.ObjectVal(values)
	}

	type namedSchema struct {
		name   string
		schema rschema.Schema
	}

	attrSchemas := []namedSchema{
		{"attribute no replace", attributeSchema},
		{"attribute requires replace", attributeReplaceSchema},
		{"attribute default no replace", attributeSchemaWithDefault},
		{"attribute default replace", attributeSchemaWithDefaultReplace},
		{"attribute plan modifier default", attributeSchemaWithPlanModifier},
		{"attribute plan modifier default replace", attributeSchemaWithPlanModifierReplace},
	}

	blockSchemas := []namedSchema{
		{"nested block no replace", nestedBlockSchema},
		{"nested block requires replace", nestedBlockReplaceSchema},
		{"nested block nested requires replace", nestedBlockNestedReplaceSchema},
		{"nested block default no replace", nestedBlockWithDefaultSchema},
		{"nested block default replace", nestedBlockWithDefaultReplaceSchema},
		{"nested block default nested replace", nestedBlockWithDefaultNestedReplaceSchema},
		{"nested block plan modifier no replace", nestedBlockWithPlanModifierSchema},
		{"nested block plan modifier replace", nestedBlockWithPlanModifierReplaceSchema},
	}

	schemas := append(attrSchemas, blockSchemas...)

	scenarios := []struct {
		name         string
		initialValue *map[string]string
		changeValue  *map[string]string
	}{
		{"unchanged null", nil, nil},
		{"unchanged non-empty", &map[string]string{"nested": "value"}, &map[string]string{"nested": "value"}},
		{"changed value non-null", &map[string]string{"nested": "value"}, &map[string]string{"nested": "changed"}},
		{"added", nil, &map[string]string{"nested": "value"}},
		{"removed", &map[string]string{"nested": "value"}, nil},
	}

	// Attribute objects can't be empty, but block objects can.
	emptyBlockScenarios := []struct {
		name         string
		initialValue *map[string]string
		changeValue  *map[string]string
	}{
		{"unchanged empty", &map[string]string{}, &map[string]string{}},
		{"changed empty to non-empty", &map[string]string{}, &map[string]string{"nested": "value"}},
		{"changed non-empty to empty", &map[string]string{"nested": "value"}, &map[string]string{}},
	}

	for _, schema := range schemas {
		t.Run(schema.name, func(t *testing.T) {
			t.Parallel()
			for _, scenario := range scenarios {
				t.Run(scenario.name, func(t *testing.T) {
					t.Parallel()
					initialValue := map[string]cty.Value{"key": makeValue(scenario.initialValue)}
					changeValue := map[string]cty.Value{"key": makeValue(scenario.changeValue)}
					res := pb.NewResource(pb.NewResourceArgs{
						ResourceSchema: schema.schema,
					})
					diff := crosstests.Diff(t, res, initialValue, changeValue)
					autogold.ExpectFile(t, testOutput{
						initialValue: scenario.initialValue,
						changeValue:  scenario.changeValue,
						tfOut:        diff.TFOut,
						pulumiOut:    diff.PulumiOut,
						detailedDiff: diff.PulumiDiff.DetailedDiff,
					})
				})
			}

			if strings.Contains(schema.name, "block") {
				for _, scenario := range emptyBlockScenarios {
					t.Run(scenario.name, func(t *testing.T) {
						t.Parallel()
						initialValue := map[string]cty.Value{"key": makeValue(scenario.initialValue)}
						changeValue := map[string]cty.Value{"key": makeValue(scenario.changeValue)}
						res := pb.NewResource(pb.NewResourceArgs{
							ResourceSchema: schema.schema,
						})
						diff := crosstests.Diff(t, res, initialValue, changeValue)
						autogold.ExpectFile(t, testOutput{
							initialValue: scenario.initialValue,
							changeValue:  scenario.changeValue,
							tfOut:        diff.TFOut,
							pulumiOut:    diff.PulumiOut,
							detailedDiff: diff.PulumiDiff.DetailedDiff,
						})
					})
				}
			}
		})
	}
}
