package tfbridgetests

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hexops/autogold/v2"
	crosstests "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tests/internal/cross-tests"
	"github.com/zclconf/go-cty/cty"
)

func TestDetailedDiffObject(t *testing.T) {
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

	schemas := []struct {
		name   string
		schema rschema.Schema
	}{
		{"attribute no replace", attributeSchema},
		{"attribute requires replace", attributeReplaceSchema},
		{"nested block no replace", nestedBlockSchema},
		{"nested block requires replace", nestedBlockReplaceSchema},
		{"nested block nested requires replace", nestedBlockNestedReplaceSchema},
	}

	scenarios := []struct {
		name         string
		initialValue *map[string]string
		changeValue  *map[string]string
	}{
		{"unchanged null", nil, nil},
		{"unchanged empty", &map[string]string{}, &map[string]string{}},
		{"unchanged non-empty", &map[string]string{"nested": "value"}, &map[string]string{"nested": "value"}},
		{"changed value non-null", &map[string]string{"nested": "value"}, &map[string]string{"nested": "changed"}},
		{"changed value null to non-null", nil, &map[string]string{"nested": "value"}},
		{"changed value non-null to null", &map[string]string{"nested": "value"}, nil},
		{"changed null to empty", nil, &map[string]string{}},
		{"changed empty to null", &map[string]string{}, nil},
		{"added", nil, &map[string]string{"nested": "value"}},
		{"removed", &map[string]string{"nested": "value"}, nil},
	}

	type testOutput struct {
		initialValue *map[string]string
		changeValue  *map[string]string
		tfOut        string
		pulumiOut    string
	}

	for _, schema := range schemas {
		t.Run(schema.name, func(t *testing.T) {
			t.Parallel()
			for _, scenario := range scenarios {
				t.Run(scenario.name, func(t *testing.T) {
					t.Parallel()
					initialValue := map[string]cty.Value{"key": makeValue(scenario.initialValue)}
					changeValue := map[string]cty.Value{"key": makeValue(scenario.changeValue)}
					diff := crosstests.Diff(t, schema.schema, initialValue, changeValue)
					autogold.ExpectFile(t, testOutput{
						initialValue: scenario.initialValue,
						changeValue:  scenario.changeValue,
						tfOut:        diff.TFOut,
						pulumiOut:    diff.PulumiOut,
					})
				})
			}
		})
	}
}
