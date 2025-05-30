package tfbridgetests

import (
	"fmt"
	"testing"

	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hexops/autogold/v2"
	"github.com/zclconf/go-cty/cty"

	pb "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/internal/providerbuilder"
	crosstests "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tests/internal/cross-tests"
)

func TestPFDetailedDiffList(t *testing.T) {
	t.Parallel()

	attributeSchema := rschema.Schema{
		Attributes: map[string]rschema.Attribute{
			"key": rschema.ListAttribute{
				Optional:    true,
				ElementType: types.StringType,
			},
		},
	}

	attributeReplaceSchema := rschema.Schema{
		Attributes: map[string]rschema.Attribute{
			"key": rschema.ListAttribute{
				Optional:    true,
				ElementType: types.StringType,
				PlanModifiers: []planmodifier.List{
					listplanmodifier.RequiresReplace(),
				},
			},
		},
	}

	nestedAttributeSchema := rschema.Schema{
		Attributes: map[string]rschema.Attribute{
			"key": rschema.ListNestedAttribute{
				Optional: true,
				NestedObject: rschema.NestedAttributeObject{
					Attributes: map[string]rschema.Attribute{
						"nested": rschema.StringAttribute{Optional: true},
					},
				},
			},
		},
	}

	nestedAttributeReplaceSchema := rschema.Schema{
		Attributes: map[string]rschema.Attribute{
			"key": rschema.ListNestedAttribute{
				Optional: true,
				NestedObject: rschema.NestedAttributeObject{
					Attributes: map[string]rschema.Attribute{
						"nested": rschema.StringAttribute{
							Optional: true,
						},
					},
				},
				PlanModifiers: []planmodifier.List{
					listplanmodifier.RequiresReplace(),
				},
			},
		},
	}

	nestedAttributeNestedReplaceSchema := rschema.Schema{
		Attributes: map[string]rschema.Attribute{
			"key": rschema.ListNestedAttribute{
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
	}

	blockSchema := rschema.Schema{
		Blocks: map[string]rschema.Block{
			"key": rschema.ListNestedBlock{
				NestedObject: rschema.NestedBlockObject{
					Attributes: map[string]rschema.Attribute{
						"nested": rschema.StringAttribute{Optional: true},
					},
				},
			},
		},
	}

	blockReplaceSchema := rschema.Schema{
		Blocks: map[string]rschema.Block{
			"key": rschema.ListNestedBlock{
				NestedObject: rschema.NestedBlockObject{
					Attributes: map[string]rschema.Attribute{
						"nested": rschema.StringAttribute{Optional: true},
					},
				},
				PlanModifiers: []planmodifier.List{
					listplanmodifier.RequiresReplace(),
				},
			},
		},
	}

	blockNestedReplaceSchema := rschema.Schema{
		Blocks: map[string]rschema.Block{
			"key": rschema.ListNestedBlock{
				NestedObject: rschema.NestedBlockObject{
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
	}

	schemaValueMakerPairs := []struct {
		name       string
		schema     rschema.Schema
		valueMaker func(*[]string) cty.Value
	}{
		{"attribute no replace", attributeSchema, listValueMaker},
		{"attribute requires replace", attributeReplaceSchema, listValueMaker},
		{"nested attribute no replace", nestedAttributeSchema, nestedListValueMaker},
		{"nested attribute requires replace", nestedAttributeReplaceSchema, nestedListValueMaker},
		{"nested attribute nested requires replace", nestedAttributeNestedReplaceSchema, nestedListValueMaker},
		{"block no replace", blockSchema, nestedListValueMaker},
		{"block requires replace", blockReplaceSchema, nestedListValueMaker},
		{"block nested requires replace", blockNestedReplaceSchema, nestedListValueMaker},
	}

	longList := &[]string{}
	for i := 0; i < 20; i++ {
		*longList = append(*longList, fmt.Sprintf("value%d", i))
	}
	longListAddedBack := append([]string{}, *longList...)
	longListAddedBack = append(longListAddedBack, "value20")
	longListAddedFront := append([]string{"value20"}, *longList...)

	scenarios := []struct {
		name         string
		initialValue *[]string
		changeValue  *[]string
	}{
		{"unchanged non-empty", &[]string{"value"}, &[]string{"value"}},
		{"changed non-empty", &[]string{"value"}, &[]string{"value1"}},
		{"added", &[]string{}, &[]string{"value"}},
		{"removed", &[]string{"value"}, &[]string{}},
		{"null unchanged", nil, nil},
		{"null to non-null", nil, &[]string{"value"}},
		{"non-null to null", &[]string{"value"}, nil},
		{"changed null to empty", nil, &[]string{}},
		{"changed empty to null", &[]string{}, nil},
		{"element added", &[]string{"value"}, &[]string{"value", "value1"}},
		{"element removed", &[]string{"value", "value1"}, &[]string{"value"}},
		{"removed front", &[]string{"val1", "val2", "val3"}, &[]string{"val2", "val3"}},
		{"removed middle", &[]string{"val1", "val2", "val3"}, &[]string{"val1", "val3"}},
		{"removed end", &[]string{"val1", "val2", "val3"}, &[]string{"val1", "val2"}},
		{"added front", &[]string{"val2", "val3"}, &[]string{"val1", "val2", "val3"}},
		{"added middle", &[]string{"val1", "val3"}, &[]string{"val1", "val2", "val3"}},
		{"added end", &[]string{"val1", "val2"}, &[]string{"val1", "val2", "val3"}},
		{"long list added", longList, &longListAddedBack},
		{"long list added front", longList, &longListAddedFront},
	}

	for _, schemaValueMakerPair := range schemaValueMakerPairs {
		t.Run(schemaValueMakerPair.name, func(t *testing.T) {
			t.Parallel()
			for _, scenario := range scenarios {
				t.Run(scenario.name, func(t *testing.T) {
					t.Parallel()
					initialValue := schemaValueMakerPair.valueMaker(scenario.initialValue)
					changeValue := schemaValueMakerPair.valueMaker(scenario.changeValue)

					res := pb.NewResource(pb.NewResourceArgs{
						ResourceSchema: schemaValueMakerPair.schema,
					})

					diff := crosstests.Diff(
						t, res, map[string]cty.Value{"key": initialValue}, map[string]cty.Value{"key": changeValue},
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
