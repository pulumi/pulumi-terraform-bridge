package schema

import (
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/zclconf/go-cty/cty"
)

// ImpliedType returns the cty.Type corresponding to the given schema map.
// If timeouts is true, the resulting type will include a "timeouts" attribute.
// It will also include an "id" attribute if it is not present in the schema map.
// This is modelled after https://github.com/hashicorp/terraform-plugin-sdk/blob/main/internal/configs/configschema/implied_type.go
// and https://github.com/pulumi/pulumi-terraform-bridge/blob/5ff2fb1f2983b7260534ad139d3281deeb8efee6/pkg/tfshim/shim.go#L169
func ImpliedType(schemaMap shim.SchemaMap, timeouts bool) cty.Type {
	if schemaMap.Len() == 0 {
		return cty.EmptyObject
	}

	atys := make(map[string]cty.Type)
	schemaMap.Range(func(name string, schema shim.Schema) bool {
		atys[name] = impliedTypeForSchema(schema)
		return true
	})

	if _, ok := atys["id"]; !ok {
		atys["id"] = cty.String
	}

	if _, ok := atys["timeouts"]; !ok && timeouts {
		atys["timeouts"] = cty.Object(map[string]cty.Type{
			"create":  cty.String,
			"read":    cty.String,
			"update":  cty.String,
			"delete":  cty.String,
			"default": cty.String,
		})
	}

	return cty.Object(atys)
}

func impliedTypeForSchemaMap(schemaMap shim.SchemaMap) cty.Type {
	if schemaMap.Len() == 0 {
		return cty.EmptyObject
	}

	atys := make(map[string]cty.Type)
	schemaMap.Range(func(name string, schema shim.Schema) bool {
		atys[name] = impliedTypeForSchema(schema)
		return true
	})

	return cty.Object(atys)
}

func impliedTypeForSchema(schema shim.Schema) cty.Type {
	if schema.Type() == shim.TypeBool {
		return cty.Bool
	}

	if schema.Type() == shim.TypeInt || schema.Type() == shim.TypeFloat {
		return cty.Number
	}

	if schema.Type() == shim.TypeString {
		return cty.String
	}

	if schema.Type() == shim.TypeDynamic {
		return cty.DynamicPseudoType
	}

	// must be a collection type
	contract.Assertf(schema.Type() == shim.TypeList || schema.Type() == shim.TypeSet || schema.Type() == shim.TypeMap, "Unexpected collection type: %v", schema.Type())

	if schema.Elem() == nil {
		// unknown collection element type - best we can do is dynamic
		return cty.DynamicPseudoType
	}

	_, isSchemaElem := schema.Elem().(shim.Schema)
	if isSchemaElem {
		schemaElem := schema.Elem().(shim.SchemaWithHasDynamicTypes)
		if schemaElem.HasDynamicTypes() {
			return cty.DynamicPseudoType
		}

		if schema.Type() == shim.TypeList {
			return cty.List(impliedTypeForSchema(schemaElem))
		}

		if schema.Type() == shim.TypeSet {
			return cty.Set(impliedTypeForSchema(schemaElem))
		}

		if schema.Type() == shim.TypeMap {
			return cty.Map(impliedTypeForSchema(schemaElem))
		}
		contract.Assertf(false, "Invalid collection type: %v", schema.Type())
	}

	_, isResElem := schema.Elem().(shim.Resource)
	if isResElem {
		resElem := schema.Elem().(shim.ResourceWithHasDynamicTypes)
		if resElem.HasDynamicTypes() {
			return cty.DynamicPseudoType
		}

		if schema.Type() == shim.TypeMap {
			// This one is special - it is how single-nested blocks are encoded in the shim layer.
			return impliedTypeForSchemaMap(resElem.Schema())
		}

		if schema.Type() == shim.TypeList {
			return cty.List(impliedTypeForSchemaMap(resElem.Schema()))
		}

		if schema.Type() == shim.TypeSet {
			return cty.Set(impliedTypeForSchemaMap(resElem.Schema()))
		}

		contract.Assertf(false, "Invalid collection type: %v", schema.Type())
	}

	contract.Assertf(false, "Elem must be a Schema or Resource, found a %T", schema.Elem())
	return cty.EmptyObject
}
