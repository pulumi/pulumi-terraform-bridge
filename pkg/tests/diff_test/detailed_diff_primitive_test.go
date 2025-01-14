package tests

import (
	"context"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hexops/autogold/v2"
	"github.com/zclconf/go-cty/cty"

	crosstests "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/internal/tests/cross-tests"
)

func generatePrimitiveSchemaValueMakerPairs[T any](
	typ schema.ValueType, ctyMaker func(v T) cty.Value, val1, val2, computedVal, defaultVal, nilVal T,
) ([]diffSchemaValueMakerPair[T], []diffScenario[T]) {
	valueOne := ref(val1)
	valueTwo := ref(val2)
	noValue := ref(nilVal)

	ctyVal := func(v *T) map[string]cty.Value {
		if v == nil {
			return map[string]cty.Value{}
		}
		return map[string]cty.Value{
			"prop": ctyMaker(*v),
		}
	}

	optionalSchema := schema.Resource{
		Schema: map[string]*schema.Schema{
			"prop": {
				Type:     typ,
				Optional: true,
			},
		},
	}

	optionalForceNewSchema := schema.Resource{
		Schema: map[string]*schema.Schema{
			"prop": {
				Type:     typ,
				Optional: true,
				ForceNew: true,
			},
		},
	}

	requiredSchema := schema.Resource{
		Schema: map[string]*schema.Schema{
			"prop": {
				Type:     typ,
				Required: true,
			},
		},
	}

	requiredForceNewSchema := schema.Resource{
		Schema: map[string]*schema.Schema{
			"prop": {
				Type:     typ,
				ForceNew: true,
				Required: true,
			},
		},
	}

	setComputedFunc := func(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
		if _, ok := d.GetOk("prop"); !ok {
			err := d.Set("prop", computedVal)
			if err != nil {
				return diag.FromErr(err)
			}
		}
		return nil
	}

	optionalComputedSchema := schema.Resource{
		Schema: map[string]*schema.Schema{
			"prop": {
				Type:     typ,
				Optional: true,
				Computed: true,
			},
		},
		CreateContext: func(ctx context.Context, rd *schema.ResourceData, i interface{}) diag.Diagnostics {
			rd.SetId("id")
			return setComputedFunc(ctx, rd, i)
		},
		UpdateContext: setComputedFunc,
	}

	optionalComputedForceNewSchema := schema.Resource{
		Schema: map[string]*schema.Schema{
			"prop": {
				Type:     typ,
				Optional: true,
				Computed: true,
				ForceNew: true,
			},
		},
		CreateContext: func(ctx context.Context, rd *schema.ResourceData, i interface{}) diag.Diagnostics {
			rd.SetId("id")
			return setComputedFunc(ctx, rd, i)
		},
		UpdateContext: setComputedFunc,
	}

	optionalDefaultSchema := schema.Resource{
		Schema: map[string]*schema.Schema{
			"prop": {
				Type:     typ,
				Optional: true,
				Default:  defaultVal,
			},
		},
	}

	optionalDefaultForceNewSchema := schema.Resource{
		Schema: map[string]*schema.Schema{
			"prop": {
				Type:     typ,
				Optional: true,
				Default:  defaultVal,
				ForceNew: true,
			},
		},
	}

	return []diffSchemaValueMakerPair[T]{
			{"optional", optionalSchema, ctyVal},
			{"optionalForceNew", optionalForceNewSchema, ctyVal},
			{"required", requiredSchema, ctyVal},
			{"requiredForceNew", requiredForceNewSchema, ctyVal},
			{"optionalComputed", optionalComputedSchema, ctyVal},
			{"optionalComputedForceNew", optionalComputedForceNewSchema, ctyVal},
			{"optionalDefault", optionalDefaultSchema, ctyVal},
			{"optionalDefaultForceNew", optionalDefaultForceNewSchema, ctyVal},
		}, []diffScenario[T]{
			{"unchanged empty", noValue, noValue},
			{"unchanged non-empty", valueOne, valueOne},
			{"added", noValue, valueOne},
			{"removed", valueOne, noValue},
			{"changed", valueOne, valueTwo},
		}
}

func TestSDKv2DetailedDiffString(t *testing.T) {
	t.Parallel()

	var nilVal string
	schemaValueMakerPairs, scenarios := generatePrimitiveSchemaValueMakerPairs(
		schema.TypeString, cty.StringVal, "val1", "val2", "computed", "default", nilVal)

	for _, schemaValueMakerPair := range schemaValueMakerPairs {
		t.Run(schemaValueMakerPair.name, func(t *testing.T) {
			t.Parallel()
			for _, scenario := range scenarios {
				t.Run(scenario.name, func(t *testing.T) {
					if strings.Contains(schemaValueMakerPair.name, "required") &&
						(scenario.initialValue == nil || scenario.changeValue == nil) {
						t.Skip("Required fields cannot be unset")
					}
					t.Parallel()
					diff := crosstests.Diff(t, &schemaValueMakerPair.schema, schemaValueMakerPair.valueMaker(scenario.initialValue), schemaValueMakerPair.valueMaker(scenario.changeValue))
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

func TestSDKv2DetailedDiffBool(t *testing.T) {
	t.Parallel()

	var nilVal bool
	schemaValueMakerPairs, scenarios := generatePrimitiveSchemaValueMakerPairs(
		schema.TypeBool, cty.BoolVal, true, false, true, false, nilVal)

	for _, schemaValueMakerPair := range schemaValueMakerPairs {
		t.Run(schemaValueMakerPair.name, func(t *testing.T) {
			t.Parallel()
			for _, scenario := range scenarios {
				t.Run(scenario.name, func(t *testing.T) {
					if strings.Contains(schemaValueMakerPair.name, "required") &&
						(scenario.initialValue == nil || scenario.changeValue == nil) {
						t.Skip("Required fields cannot be unset")
					}
					t.Parallel()
					diff := crosstests.Diff(t, &schemaValueMakerPair.schema, schemaValueMakerPair.valueMaker(scenario.initialValue), schemaValueMakerPair.valueMaker(scenario.changeValue))
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

func TestSDKv2DetailedDiffInt(t *testing.T) {
	t.Parallel()

	var nilVal int64
	schemaValueMakerPairs, scenarios := generatePrimitiveSchemaValueMakerPairs(
		schema.TypeInt, cty.NumberIntVal, 1, 2, 3, 4, nilVal)

	for _, schemaValueMakerPair := range schemaValueMakerPairs {
		t.Run(schemaValueMakerPair.name, func(t *testing.T) {
			t.Parallel()
			for _, scenario := range scenarios {
				t.Run(scenario.name, func(t *testing.T) {
					if strings.Contains(schemaValueMakerPair.name, "required") &&
						(scenario.initialValue == nil || scenario.changeValue == nil) {
						t.Skip("Required fields cannot be unset")
					}
					t.Parallel()
					diff := crosstests.Diff(t, &schemaValueMakerPair.schema, schemaValueMakerPair.valueMaker(scenario.initialValue), schemaValueMakerPair.valueMaker(scenario.changeValue))
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

func TestSDKv2DetailedDiffFloat(t *testing.T) {
	t.Parallel()

	var nilVal float64
	schemaValueMakerPairs, scenarios := generatePrimitiveSchemaValueMakerPairs(
		schema.TypeFloat, cty.NumberFloatVal, 1.0, 2.0, 3.0, 4.0, nilVal)

	for _, schemaValueMakerPair := range schemaValueMakerPairs {
		t.Run(schemaValueMakerPair.name, func(t *testing.T) {
			t.Parallel()
			for _, scenario := range scenarios {
				t.Run(scenario.name, func(t *testing.T) {
					if strings.Contains(schemaValueMakerPair.name, "required") &&
						(scenario.initialValue == nil || scenario.changeValue == nil) {
						t.Skip("Required fields cannot be unset")
					}
					t.Parallel()
					diff := crosstests.Diff(t, &schemaValueMakerPair.schema, schemaValueMakerPair.valueMaker(scenario.initialValue), schemaValueMakerPair.valueMaker(scenario.changeValue))
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
