package tests

import (
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hexops/autogold/v2"
	"github.com/zclconf/go-cty/cty"

	crosstests "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/internal/tests/cross-tests"
)

func TestSDKv2DetailedDiffString(t *testing.T) {
	t.Parallel()

	var nilVal string
	schemaValueMakerPairs, scenarios := generateBaseTests(
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
	schemaValueMakerPairs, scenarios := generateBaseTests(
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
	schemaValueMakerPairs, scenarios := generateBaseTests(
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
	schemaValueMakerPairs, scenarios := generateBaseTests(
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
