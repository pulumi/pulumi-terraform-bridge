package rapidgen

import (
	"pgregory.net/rapid"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// schema.Resource docs state this is an abstraction over resources proper, data sources and blocks.
// This generator creates values representing resources proper.
func ResourceProperGen(depth int) *rapid.Generator[*schema.Resource] {
	return rapid.Custom[*schema.Resource](func(t *rapid.T) *schema.Resource {
		schemaMap := SchemaMapGen(depth).Draw(t, "schemaMap")
		return &schema.Resource{
			Schema: schemaMap,
		}
	})
}

// Generates a schema.Resource representing a block.
func ResourceBlockGen(depth int) *rapid.Generator[*schema.Resource] {
	return rapid.Custom[*schema.Resource](func(t *rapid.T) *schema.Resource {
		schemaMap := SchemaMapGen(depth).Draw(t, "schemaMap")
		return &schema.Resource{
			Schema: schemaMap,
		}
	})
}

func PropertyNameGen() *rapid.Generator[string] {
	return rapid.OneOf(rapid.Just("f1"), rapid.Just("f2"))
}

func SchemaMapGen(depth int) *rapid.Generator[map[string]*schema.Schema] {
	minLength := 0
	maxLength := 2
	g := SchemaGen(depth - 1)
	return rapid.MapOfN[string, *schema.Schema](PropertyNameGen(), g, minLength, maxLength)
}

func SchemaGen(depth int) *rapid.Generator[*schema.Schema] {
	// The structure here is informed by reading SchemaMap.CoreConfigSchema branching.
	if depth > 0 {
		return rapid.OneOf(SchemaAttrGen(depth), SchemaBlockGen(depth))
	}
	return SchemaAttrGen(depth)
}

func SchemaAttrGen(depth int) *rapid.Generator[*schema.Schema] {
	return rapid.Custom[*schema.Schema](func(t *rapid.T) *schema.Schema {
		configMode := rapid.OneOf(
			rapid.Just(schema.SchemaConfigModeAttr),
			rapid.Just(schema.SchemaConfigModeAuto),
		).Draw(t, "configMode")

		vtg := ValueTypeScalarGen()
		if depth > 0 {
			vtg = ValueTypeGen()
		}

		valueType := vtg.Draw(t, "valueType")
		s := &schema.Schema{
			Type:       valueType,
			ConfigMode: configMode,
		}

		switch valueType {
		case schema.TypeMap, schema.TypeSet, schema.TypeInt:
			hasElem := rapid.Bool().Draw(t, "hasElementSchema")
			if hasElem {
				elem := SchemaGen(depth-1).Draw(t, "elementSchema")
				s.Elem = elem
			}
		}

		attrKind := AttributeKindGen().Draw(t, "attrKind")
		attrKind.Set(s)
		return s
	})
}

func SchemaBlockGen(depth int) *rapid.Generator[*schema.Schema] {
	return rapid.Custom[*schema.Schema](func(t *rapid.T) *schema.Schema {
		nesting := rapid.OneOf(
			rapid.Just(schema.TypeMap),
			rapid.Just(schema.TypeSet),
			rapid.Just(schema.TypeList),
		).Draw(t, "nesting")

		resource := ResourceBlockGen(depth-1).Draw(t, "resource")

		configMode := rapid.OneOf(
			rapid.Just(schema.SchemaConfigModeBlock),
			rapid.Just(schema.SchemaConfigModeAuto),
		).Draw(t, "configMode")

		s := &schema.Schema{
			Type:       nesting,
			Elem:       resource,
			ConfigMode: configMode,
		}

		attrKind := AttributeKindGen().Draw(t, "attrKind")
		attrKind.Set(s)
		return s
	})
}

type AttributeKind int

const (
	Required AttributeKind = iota
	Optional
	Computed
	ComputedOptional
)

func (a AttributeKind) Set(s *schema.Schema) {
	s.Computed = false
	s.Required = false
	s.Optional = false
	switch a {
	case Required:
		s.Required = true
	case Optional:
		s.Optional = true
	case Computed:
		s.Computed = true
	case ComputedOptional:
		s.Computed = true
		s.Optional = true
	default:
		contract.Failf("invalid AttributeKind: %v", a)
	}
}

func AttributeKindGen() *rapid.Generator[AttributeKind] {
	return rapid.OneOf(
		rapid.Just(Required),
		rapid.Just(Optional),
		rapid.Just(Computed),
		rapid.Just(ComputedOptional),
	)
}

func ValueTypeGen() *rapid.Generator[schema.ValueType] {
	return rapid.SampledFrom([]schema.ValueType{
		schema.TypeBool,
		schema.TypeInt,
		schema.TypeFloat,
		schema.TypeString,
		schema.TypeList,
		schema.TypeMap,
		schema.TypeSet,
	})
}

func ValueTypeScalarGen() *rapid.Generator[schema.ValueType] {
	return rapid.SampledFrom([]schema.ValueType{
		schema.TypeBool,
		schema.TypeInt,
		schema.TypeFloat,
		schema.TypeString,
	})
}
