package crosstests

import (
	"fmt"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"pgregory.net/rapid"
)

var auxiliaryResourceSchema = map[string]*schema.Schema{
	"true_bool": {
		Type:     schema.TypeBool,
		Optional: true,
	},
	"false_bool": {
		Type:     schema.TypeBool,
		Optional: true,
	},
}

var auxiliaryResourceConfig = tftypes.NewValue(
	tftypes.Object{
		AttributeTypes: map[string]tftypes.Type{
			"true_bool":  tftypes.Bool,
			"false_bool": tftypes.Bool,
		},
	},
	map[string]tftypes.Value{
		"true_bool":  tftypes.NewValue(tftypes.Bool, true),
		"false_bool": tftypes.NewValue(tftypes.Bool, false),
	},
)

// None of the functions here should call Draw
func GenValue(mainRes schema.Resource) *rapid.Generator[tftypes.Value] {
	_, ok := mainRes.SchemaMap()["id"]
	if !ok {
		contract.Failf("Expected an id property.")
	}
	return rapid.Custom(func(t *rapid.T) tftypes.Value {
		return GenBlock(t, mainRes)
	})
}

func GenBlock(t *rapid.T, blockRes schema.Resource) tftypes.Value {
	values := make(map[string]tftypes.Value, 0)

	blockSchemaMap := blockRes.SchemaMap()
	for key := range blockRes.CoreConfigSchema().Attributes {
		if key == "id" {
			continue
		}
		// gen attr
		attrSchema, ok := blockSchemaMap[key]
		if !ok {
			panic(fmt.Sprintf("failed to find %s", key))
		}
		attrGenerator := GenAttr(t, attrSchema, key)
		values[key] = attrGenerator
	}

	for range blockRes.CoreConfigSchema().BlockTypes {
		// gen blocks
		// TODO
	}

	types := make(map[string]tftypes.Type, len(values))
	for key, val := range values {
		types[key] = val.Type()
	}

	return tftypes.NewValue(tftypes.Object{AttributeTypes: types}, values)
}

func GenAttr(t *rapid.T, attrSchema *schema.Schema, attrName string) tftypes.Value {
	switch attrSchema.Type {
	case schema.TypeBool:
		return genBool().Draw(t, attrName)
	default:
		panic("TODO")
	}
}

func genBool() *rapid.Generator[tftypes.Value] {
	// TODO Unknowns, computed, secret
	return genBoolPlain()
}

func genBoolPlain() *rapid.Generator[tftypes.Value] {
	return rapid.SampledFrom([]tftypes.Value{
		tftypes.NewValue(tftypes.Bool, true),
		tftypes.NewValue(tftypes.Bool, false),
	})
}

// func genListNestedBlock(maxDepth int, parentName string) *rapid.Generator[tftypes.Value] {
// 	// TODO Unknowns, computed, secret
// 	return genListNestedBlockPlain(maxDepth, parentName)
// }

// func genListNestedBlockPlain(maxDepth int, parentName string) *rapid.Generator[tftypes.Value] {
// 	opts := []*rapid.Generator[tftypes.Value]{
// 	}
// 	return rapid.OneOf[](opts)
// }
