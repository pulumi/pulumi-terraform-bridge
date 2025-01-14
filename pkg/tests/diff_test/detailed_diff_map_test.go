package tests

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/zclconf/go-cty/cty"
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
	schemaValueMakerPairs, scenarios := generateBaseTests(
		schema.TypeMap, &schema.Schema{Type: schema.TypeString}, ctyMaker,
		map[string]string{"key": "val1"}, map[string]string{"key": "val2"},
		map[string]string{"key": "computedVal"}, map[string]string{"key": "defaultVal"}, nilVal)

	scenarios = append(scenarios, diffScenario[map[string]string]{
		name:         "key changed",
		initialValue: ref(map[string]string{"key": "val1"}),
		changeValue:  ref(map[string]string{"key2": "val1"}),
	})

	runSDKv2TestMatrix(t, schemaValueMakerPairs, scenarios)
}
