package tests

import (
	"context"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/zclconf/go-cty/cty"
)

func ref[T any](v T) *T {
	return &v
}

type diffSchemaValueMakerPair[T any] struct {
	name       string
	schema     schema.Resource
	valueMaker func(v *T) map[string]cty.Value
}

type diffScenario[T any] struct {
	name         string
	initialValue *T
	changeValue  *T
}

func generateBaseTests[T any](
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

func listValueMaker(arr *[]string) cty.Value {
	if arr == nil {
		return cty.NullVal(cty.DynamicPseudoType)
	}
	slice := make([]cty.Value, len(*arr))
	for i, v := range *arr {
		slice[i] = cty.StringVal(v)
	}
	if len(slice) == 0 {
		return cty.ListValEmpty(cty.String)
	}
	return cty.ListVal(slice)
}

func nestedListValueMaker(arr *[]string) cty.Value {
	if arr == nil {
		return cty.NullVal(cty.DynamicPseudoType)
	}
	slice := make([]cty.Value, len(*arr))
	for i, v := range *arr {
		slice[i] = cty.ObjectVal(
			map[string]cty.Value{
				"nested": cty.StringVal(v),
			},
		)
	}
	if len(slice) == 0 {
		return cty.ListValEmpty(cty.Object(map[string]cty.Type{"nested": cty.String}))
	}
	return cty.ListVal(slice)
}

func nestedListValueMakerWithComputedSpecified(arr *[]string) cty.Value {
	if arr == nil {
		return cty.NullVal(cty.DynamicPseudoType)
	}

	slice := make([]cty.Value, len(*arr))
	for i, v := range *arr {
		slice[i] = cty.ObjectVal(
			map[string]cty.Value{
				"nested":   cty.StringVal(v),
				"computed": cty.StringVal("non-computed-" + v),
			},
		)
	}
	if len(slice) == 0 {
		return cty.ListValEmpty(cty.Object(map[string]cty.Type{
			"nested":   cty.String,
			"computed": cty.String,
		}))
	}
	return cty.ListVal(slice)
}

type testOutput struct {
	initialValue any
	changeValue  any
	tfOut        string
	pulumiOut    string
	detailedDiff map[string]any
}
