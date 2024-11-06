package tfbridgetests

import (
	"testing"

	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hexops/autogold/v2"
	crosstests "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tests/internal/cross-tests"
	"github.com/zclconf/go-cty/cty"
)

func TestDetailedDiffMap(t *testing.T) {
	t.Parallel()

	attributeSchema := rschema.Schema{
		Attributes: map[string]rschema.Attribute{
			"key": rschema.MapAttribute{
				Optional: true,
			},
		},
	}

	attributeReplaceSchema := rschema.Schema{
		Attributes: map[string]rschema.Attribute{
			"key": rschema.MapAttribute{
				Optional: true,
				PlanModifiers: []planmodifier.Map{
					mapplanmodifier.RequiresReplace(),
				},
			},
		},
	}

	nestedAttributeSchema := rschema.Schema{
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
	}

	nestedAttributeReplaceSchema := rschema.Schema{
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
	}

	nestedAttributeNestedReplaceSchema := rschema.Schema{
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
	}

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
		schema     rschema.Schema
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

		{"changed value null to non-null", &map[string]*string{"k": nil}, &map[string]*string{"k": ref("value")}},
		{"changed value non-null to null", &map[string]*string{"k": ref("value")}, &map[string]*string{"k": nil}},
	}

	type testOutput struct {
		initialValue *map[string]*string
		changeValue  *map[string]*string
		tfOut        string
		pulumiOut    string
	}

	for _, schemaValueMakerPair := range schemaValueMakerPairs {
		t.Run(schemaValueMakerPair.name, func(t *testing.T) {
			t.Parallel()
			for _, scenario := range scenarios {
				t.Run(scenario.name, func(t *testing.T) {
					t.Parallel()
					initialValue := schemaValueMakerPair.valueMaker(scenario.initialValue)
					changeValue := schemaValueMakerPair.valueMaker(scenario.changeValue)

					diff := crosstests.Diff(t, schemaValueMakerPair.schema, map[string]cty.Value{"key": initialValue}, map[string]cty.Value{"key": changeValue})

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
