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

func ref[T any](v T) *T {
	return &v
}

type valueMaker[T any] func(v *T) map[string]cty.Value

type diffSchemaValueMakerPair[T any] struct {
	name       string
	schema     schema.Resource
	valueMaker valueMaker[T]
}

type diffScenario[T any] struct {
	name         string
	initialValue *T
	changeValue  *T
}

func runSDKv2TestMatrix[T any](
	t *testing.T, schemaValueMakerPairs []diffSchemaValueMakerPair[T], scenarios []diffScenario[T],
) {
	for _, schemaValueMakerPair := range schemaValueMakerPairs {
		t.Run(schemaValueMakerPair.name, func(t *testing.T) {
			t.Parallel()
			for _, scenario := range scenarios {
				t.Run(scenario.name, func(t *testing.T) {
					t.Parallel()
					if strings.Contains(schemaValueMakerPair.name, "required") &&
						(scenario.initialValue == nil || scenario.changeValue == nil) {
						t.Skip("Required fields cannot be unset")
					}
					diff := crosstests.Diff(
						t, &schemaValueMakerPair.schema, schemaValueMakerPair.valueMaker(scenario.initialValue),
						schemaValueMakerPair.valueMaker(scenario.changeValue))
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

func generateBaseTests[T any](
	typ schema.ValueType, elem any, ctyMaker func(v T) cty.Value, val1, val2, computedVal, defaultVal, nilVal T,
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
				Elem:     elem,
				Optional: true,
			},
		},
	}

	optionalForceNewSchema := schema.Resource{
		Schema: map[string]*schema.Schema{
			"prop": {
				Type:     typ,
				Elem:     elem,
				Optional: true,
				ForceNew: true,
			},
		},
	}

	requiredSchema := schema.Resource{
		Schema: map[string]*schema.Schema{
			"prop": {
				Type:     typ,
				Elem:     elem,
				Required: true,
			},
		},
	}

	requiredForceNewSchema := schema.Resource{
		Schema: map[string]*schema.Schema{
			"prop": {
				Type:     typ,
				Elem:     elem,
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
				Elem:     elem,
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
				Elem:     elem,
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
				Elem:     elem,
				Optional: true,
				Default:  defaultVal,
			},
		},
	}

	optionalDefaultForceNewSchema := schema.Resource{
		Schema: map[string]*schema.Schema{
			"prop": {
				Type:     typ,
				Elem:     elem,
				Optional: true,
				Default:  defaultVal,
				ForceNew: true,
			},
		},
	}

	sensitiveSchema := schema.Resource{
		Schema: map[string]*schema.Schema{
			"prop": {
				Type:      typ,
				Elem:      elem,
				Optional:  true,
				Sensitive: true,
			},
		},
	}

	sensitiveForceNewSchema := schema.Resource{
		Schema: map[string]*schema.Schema{
			"prop": {
				Type:      typ,
				Elem:      elem,
				Optional:  true,
				Sensitive: true,
				ForceNew:  true,
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
			{"sensitive", sensitiveSchema, ctyVal},
			{"sensitiveForceNew", sensitiveForceNewSchema, ctyVal},
		}, []diffScenario[T]{
			{"unchanged empty", noValue, noValue},
			{"unchanged non-empty", valueOne, valueOne},
			{"added", noValue, valueOne},
			{"removed", valueOne, noValue},
			{"changed", valueOne, valueTwo},
		}
}

func listValueMaker(arr *[]string) map[string]cty.Value {
	if arr == nil {
		return map[string]cty.Value{}
	}

	if len(*arr) == 0 {
		return map[string]cty.Value{
			"prop": cty.ListValEmpty(cty.String),
		}
	}

	slice := make([]cty.Value, len(*arr))
	for i, v := range *arr {
		slice[i] = cty.StringVal(v)
	}
	return map[string]cty.Value{
		"prop": cty.ListVal(slice),
	}
}

var _ valueMaker[[]string] = listValueMaker

func nestedListValueMaker(arr *[]string) map[string]cty.Value {
	if arr == nil {
		return map[string]cty.Value{}
	}

	if len(*arr) == 0 {
		return map[string]cty.Value{
			"prop": cty.ListValEmpty(cty.DynamicPseudoType),
		}
	}

	slice := make([]cty.Value, len(*arr))
	for i, v := range *arr {
		slice[i] = cty.ObjectVal(map[string]cty.Value{"nested_prop": cty.StringVal(v)})
	}
	return map[string]cty.Value{
		"prop": cty.ListVal(slice),
	}
}

var _ valueMaker[[]string] = nestedListValueMaker

func nestedListValueMakerWithComputedSpecified(arr *[]string) map[string]cty.Value {
	if arr == nil {
		return map[string]cty.Value{}
	}

	if len(*arr) == 0 {
		return map[string]cty.Value{
			"prop": cty.ListValEmpty(cty.DynamicPseudoType),
		}
	}

	slice := make([]cty.Value, len(*arr))
	for i, v := range *arr {
		slice[i] = cty.ObjectVal(map[string]cty.Value{
			"nested_prop": cty.StringVal(v),
			"computed":    cty.StringVal("non-computed-" + v),
		})
	}
	return map[string]cty.Value{
		"prop": cty.ListVal(slice),
	}
}

func nestedListValueMakerWithDefaultSpecified(arr *[]string) map[string]cty.Value {
	if arr == nil {
		return map[string]cty.Value{}
	}

	if len(*arr) == 0 {
		return map[string]cty.Value{
			"prop": cty.ListValEmpty(cty.DynamicPseudoType),
		}
	}

	slice := make([]cty.Value, len(*arr))
	for i, v := range *arr {
		slice[i] = cty.ObjectVal(map[string]cty.Value{
			"nested_prop": cty.StringVal(v),
			"default":     cty.StringVal("non-default-" + v),
		})
	}
	return map[string]cty.Value{
		"prop": cty.ListVal(slice),
	}
}

var _ valueMaker[[]string] = nestedListValueMakerWithComputedSpecified

type testOutput struct {
	initialValue any
	changeValue  any
	tfOut        string
	pulumiOut    string
	detailedDiff map[string]any
}
