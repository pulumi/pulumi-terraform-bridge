package tests

import (
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
