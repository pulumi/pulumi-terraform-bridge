package tests

import (
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hexops/autogold/v2"
	"github.com/zclconf/go-cty/cty"

	crosstests "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/internal/tests/cross-tests"
)

func TestSDKv2DetailedDiffMap(t *testing.T) {
	t.Parallel()

	ctyMaker := func(v map[string]string) cty.Value {
		if len(v) == 0 {
			return cty.MapValEmpty(cty.String)
		}
		ctyMap := make(map[string]cty.Value)
		for k, v := range v {
			ctyMap[k] = cty.StringVal(v)
		}
		return cty.MapVal(ctyMap)
	}

	var nilVal map[string]string
	schemaValueMakerPairs, scenarios := generatePrimitiveSchemaValueMakerPairs(
		schema.TypeMap, &schema.Schema{Type: schema.TypeString}, ctyMaker,
		map[string]string{"key": "val1"}, map[string]string{"key": "val2"},
		map[string]string{"key": "computedVal"}, map[string]string{"key": "defaultVal"}, nilVal)

	scenarios = append(scenarios, diffScenario[map[string]string]{
		name:         "key changed",
		initialValue: ref(map[string]string{"key": "val1"}),
		changeValue:  ref(map[string]string{"key2": "val1"}),
	})

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
