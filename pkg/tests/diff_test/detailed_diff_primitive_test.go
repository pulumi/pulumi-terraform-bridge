package tests

import (
	"context"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hexops/autogold/v2"
	crosstests "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/internal/tests/cross-tests"
	"github.com/zclconf/go-cty/cty"
)

func TestSDKv2DetailedDiffString(t *testing.T) {
	t.Parallel()

	optionalSchema := schema.Resource{
		Schema: map[string]*schema.Schema{
			"string_prop": {
				Type:     schema.TypeString,
				Optional: true,
			},
		},
	}

	optionalForceNewSchema := schema.Resource{
		Schema: map[string]*schema.Schema{
			"string_prop": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},
		},
	}

	requiredSchema := schema.Resource{
		Schema: map[string]*schema.Schema{
			"string_prop": {
				Type:     schema.TypeString,
				Required: true,
			},
		},
	}

	requiredForceNewSchema := schema.Resource{
		Schema: map[string]*schema.Schema{
			"string_prop": {
				Type:     schema.TypeString,
				ForceNew: true,
				Required: true,
			},
		},
	}

	setComputedFunc := func(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
		if d.Get("string_prop") == nil {
			err := d.Set("string_prop", "computed")
			if err != nil {
				return diag.FromErr(err)
			}
		}
		return nil
	}

	optionalComputedSchema := schema.Resource{
		Schema: map[string]*schema.Schema{
			"string_prop": {
				Type:     schema.TypeString,
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
			"string_prop": {
				Type:     schema.TypeString,
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
			"string_prop": {
				Type:     schema.TypeString,
				Optional: true,
				Default:  "default",
			},
		},
	}

	optionalDefaultForceNewSchema := schema.Resource{
		Schema: map[string]*schema.Schema{
			"string_prop": {
				Type:     schema.TypeString,
				Optional: true,
				Default:  "default",
				ForceNew: true,
			},
		},
	}

	valueOne := ref("val1")
	valueTwo := ref("val2")
	var noValue *string

	ctyVal := func(v *string) map[string]cty.Value {
		if v == nil {
			return map[string]cty.Value{}
		}
		return map[string]cty.Value{
			"string_prop": cty.StringVal(*v),
		}
	}

	schemaValueMakerPairs := []struct {
		name       string
		schema     schema.Resource
		valueMaker func(v *string) map[string]cty.Value
	}{
		{"optional", optionalSchema, ctyVal},
		{"optionalForceNew", optionalForceNewSchema, ctyVal},
		{"required", requiredSchema, ctyVal},
		{"requiredForceNew", requiredForceNewSchema, ctyVal},
		{"optionalComputed", optionalComputedSchema, ctyVal},
		{"optionalComputedForceNew", optionalComputedForceNewSchema, ctyVal},
		{"optionalDefault", optionalDefaultSchema, ctyVal},
		{"optionalDefaultForceNew", optionalDefaultForceNewSchema, ctyVal},
	}

	scenarios := []struct {
		name         string
		initialValue *string
		changeValue  *string
	}{
		{"unchanged empty", noValue, noValue},
		{"unchanged non-empty", valueOne, valueOne},
		{"added", noValue, valueOne},
		{"removed", valueOne, noValue},
		{"changed", valueOne, valueTwo},
	}

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
