package tests

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hexops/autogold/v2"
	"github.com/zclconf/go-cty/cty"

	crosstests "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/internal/tests/cross-tests"
)

func TestSDKv2DetailedDiffString(t *testing.T) {
	t.Parallel()

	res := schema.Resource{
		Schema: map[string]*schema.Schema{
			"string_prop": {
				Type:     schema.TypeString,
				Optional: true,
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

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			t.Parallel()
			diff := crosstests.Diff(t, &res, ctyVal(scenario.initialValue), ctyVal(scenario.changeValue))
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

func TestSDKv2DetailedDiffMap(t *testing.T) {
	t.Parallel()

	res := schema.Resource{
		Schema: map[string]*schema.Schema{
			"map_prop": {
				Type:     schema.TypeMap,
				Optional: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},
		},
	}

	ctyVal := func(v map[string]string) map[string]cty.Value {
		ctyMap := make(map[string]cty.Value)

		if len(v) == 0 {
			return map[string]cty.Value{
				"map_prop": cty.MapValEmpty(cty.String),
			}
		}

		for k, v := range v {
			ctyMap[k] = cty.StringVal(v)
		}
		return map[string]cty.Value{
			"map_prop": cty.MapVal(ctyMap),
		}
	}

	scenarios := []struct {
		name         string
		initialValue map[string]string
		changeValue  map[string]string
	}{
		{"unchanged empty", map[string]string{}, map[string]string{}},
		{"unchanged non-empty", map[string]string{"key": "val"}, map[string]string{"key": "val"}},
		{"added", map[string]string{}, map[string]string{"key": "val"}},
		{"removed", map[string]string{"key": "val"}, map[string]string{}},
		{"value changed", map[string]string{"key": "val"}, map[string]string{"key": "val2"}},
		{"key changed", map[string]string{"key": "val"}, map[string]string{"key2": "val"}},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			t.Parallel()
			diff := crosstests.Diff(t, &res, ctyVal(scenario.initialValue), ctyVal(scenario.changeValue))
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
