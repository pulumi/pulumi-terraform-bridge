package tests

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hexops/autogold/v2"
	"github.com/zclconf/go-cty/cty"

	crosstests "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/internal/tests/cross-tests"
)

func TestDetailedDiffList(t *testing.T) {
	t.Parallel()

	listAttrSchema := schema.Resource{
		Schema: map[string]*schema.Schema{
			"list_attr": {
				Type:     schema.TypeList,
				Optional: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},
		},
	}

	maxItemsOneAttrSchema := schema.Resource{
		Schema: map[string]*schema.Schema{
			"list_attr": {
				Type:     schema.TypeList,
				Optional: true,
				MaxItems: 1,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},
		},
	}

	listBlockSchema := schema.Resource{
		Schema: map[string]*schema.Schema{
			"list_block": {
				Type:     schema.TypeList,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"prop": {
							Type:     schema.TypeString,
							Optional: true,
						},
					},
				},
			},
		},
	}

	maxItemsOneBlockSchema := schema.Resource{
		Schema: map[string]*schema.Schema{
			"list_block": {
				Type:     schema.TypeList,
				Optional: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"nested_prop": {
							Type:     schema.TypeString,
							Optional: true,
						},
					},
				},
			},
		},
	}

	attrList := func(arr *[]string) map[string]cty.Value {
		if arr == nil {
			return map[string]cty.Value{}
		}

		if len(*arr) == 0 {
			return map[string]cty.Value{
				"list_attr": cty.ListValEmpty(cty.String),
			}
		}

		slice := make([]cty.Value, len(*arr))
		for i, v := range *arr {
			slice[i] = cty.StringVal(v)
		}
		return map[string]cty.Value{
			"list_attr": cty.ListVal(slice),
		}
	}

	blockList := func(arr *[]string) map[string]cty.Value {
		if arr == nil {
			return map[string]cty.Value{}
		}

		if len(*arr) == 0 {
			return map[string]cty.Value{
				"list_block": cty.ListValEmpty(cty.DynamicPseudoType),
			}
		}

		slice := make([]cty.Value, len(*arr))
		for i, v := range *arr {
			slice[i] = cty.ObjectVal(map[string]cty.Value{"prop": cty.StringVal(v)})
		}
		return map[string]cty.Value{
			"list_block": cty.ListVal(slice),
		}
	}

	nestedBlockList := func(arr *[]string) map[string]cty.Value {
		if arr == nil {
			return map[string]cty.Value{}
		}

		if len(*arr) == 0 {
			return map[string]cty.Value{
				"list_block": cty.ListValEmpty(cty.DynamicPseudoType),
			}
		}

		slice := make([]cty.Value, len(*arr))
		for i, v := range *arr {
			slice[i] = cty.ObjectVal(map[string]cty.Value{"nested_prop": cty.StringVal(v)})
		}
		return map[string]cty.Value{
			"list_block": cty.ListVal(slice),
		}
	}

	listPairs := []struct {
		name       string
		schema     schema.Resource
		valueMaker func(*[]string) map[string]cty.Value
	}{
		{"list attribute", listAttrSchema, attrList},
		{"list block", listBlockSchema, blockList},
	}

	maxItemsOnePairs := []struct {
		name       string
		schema     schema.Resource
		valueMaker func(*[]string) map[string]cty.Value
	}{
		{"max items one attribute", maxItemsOneAttrSchema, attrList},
		{"max items one block", maxItemsOneBlockSchema, nestedBlockList},
	}

	oneElementScenarios := []struct {
		name         string
		initialValue *[]string
		changeValue  *[]string
	}{
		{"unchanged empty", nil, nil},
		{"unchanged non-empty", ref([]string{"val1"}), ref([]string{"val1"})},
		{"added non-empty", nil, ref([]string{"val1"})},
		{"added empty", nil, ref([]string{})},
		{"removed non-empty", ref([]string{"val1"}), nil},
		{"removed empty", ref([]string{}), nil},
		{"changed", ref([]string{"val1"}), ref([]string{"val2"})},
	}

	multiElementScenarios := []struct {
		name         string
		initialValue *[]string
		changeValue  *[]string
	}{
		{"list element added front", ref([]string{"val2", "val3"}), ref([]string{"val1", "val2", "val3"})},
		{"list element added back", ref([]string{"val1", "val2"}), ref([]string{"val1", "val2", "val3"})},
		{"list element added middle", ref([]string{"val1", "val3"}), ref([]string{"val1", "val2", "val3"})},
		{"list element removed front", ref([]string{"val1", "val2", "val3"}), ref([]string{"val3", "val2"})},
		{"list element removed middle", ref([]string{"val1", "val2", "val3"}), ref([]string{"val3", "val1"})},
		{"list element removed end", ref([]string{"val1", "val2", "val3"}), ref([]string{"val2", "val1"})},
	}

	scenarios := append(oneElementScenarios, multiElementScenarios...)

	type testOutput struct {
		initialValue *[]string
		changeValue  *[]string
		tfOut        string
		pulumiOut    string
		detailedDiff map[string]any
	}

	runTest := func(t *testing.T, schema schema.Resource, valueMaker func(*[]string) map[string]cty.Value, initialValue *[]string, changeValue *[]string) {
		diff := crosstests.Diff(t, &schema, valueMaker(initialValue), valueMaker(changeValue))
		autogold.ExpectFile(t, testOutput{
			initialValue: initialValue,
			changeValue:  changeValue,
			tfOut:        diff.TFOut,
			pulumiOut:    diff.PulumiOut,
			detailedDiff: diff.PulumiDiff.DetailedDiff,
		})
	}

	for _, schemaValueMakerPair := range listPairs {
		t.Run(schemaValueMakerPair.name, func(t *testing.T) {
			t.Parallel()
			for _, scenario := range scenarios {
				t.Run(scenario.name, func(t *testing.T) {
					t.Parallel()
					runTest(t, schemaValueMakerPair.schema, schemaValueMakerPair.valueMaker, scenario.initialValue, scenario.changeValue)
				})
			}
		})
	}

	for _, schemaValueMakerPair := range maxItemsOnePairs {
		t.Run(schemaValueMakerPair.name, func(t *testing.T) {
			t.Parallel()
			for _, scenario := range oneElementScenarios {
				t.Run(scenario.name, func(t *testing.T) {
					t.Parallel()
					runTest(t, schemaValueMakerPair.schema, schemaValueMakerPair.valueMaker, scenario.initialValue, scenario.changeValue)
				})
			}
		})
	}
}
