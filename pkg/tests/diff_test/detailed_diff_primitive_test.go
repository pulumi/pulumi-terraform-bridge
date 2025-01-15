package tests

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/zclconf/go-cty/cty"
)

func TestSDKv2DetailedDiffString(t *testing.T) {
	t.Parallel()

	var nilVal string
	schemaValueMakerPairs, scenarios := generateBaseTests(
		schema.TypeString, cty.StringVal, "val1", "val2", "computed", "default", nilVal)

	runSDKv2TestMatrix(t, schemaValueMakerPairs, scenarios)
}

func TestSDKv2DetailedDiffBool(t *testing.T) {
	t.Parallel()

	var nilVal bool
	schemaValueMakerPairs, scenarios := generateBaseTests(
		schema.TypeBool, cty.BoolVal, true, false, true, false, nilVal)

	runSDKv2TestMatrix(t, schemaValueMakerPairs, scenarios)
}

func TestSDKv2DetailedDiffInt(t *testing.T) {
	t.Parallel()

	var nilVal int64
	schemaValueMakerPairs, scenarios := generateBaseTests(
		schema.TypeInt, cty.NumberIntVal, 1, 2, 3, 4, nilVal)

	runSDKv2TestMatrix(t, schemaValueMakerPairs, scenarios)
}

func TestSDKv2DetailedDiffFloat(t *testing.T) {
	t.Parallel()

	var nilVal float64
	schemaValueMakerPairs, scenarios := generateBaseTests(
		schema.TypeFloat, cty.NumberFloatVal, 1.0, 2.0, 3.0, 4.0, nilVal)

	runSDKv2TestMatrix(t, schemaValueMakerPairs, scenarios)
}
