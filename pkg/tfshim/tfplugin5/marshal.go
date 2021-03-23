package tfplugin5

import (
	"encoding/json"
	fmt "fmt"

	"github.com/hashicorp/go-cty/cty"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"

	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/schema"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/tfplugin5/proto"
)

func deprecationMessage(propertyName string, isDeprecated bool) string {
	if isDeprecated {
		return fmt.Sprintf("%v is deprecated", propertyName)
	}
	return ""
}

func unmarshalNestedType(elementType cty.Type) (interface{}, error) {
	valueType, elem, err := unmarshalType(elementType)
	if err != nil {
		return nil, err
	}

	switch valueType {
	case shim.TypeBool, shim.TypeInt, shim.TypeFloat, shim.TypeString:
		return &attributeSchema{
			ctyType:   elementType,
			valueType: valueType,
		}, nil
	case shim.TypeList, shim.TypeSet:
		return &attributeSchema{
			ctyType:   elementType,
			valueType: valueType,
			elem:      elem,
		}, nil
	case shim.TypeMap:
		if r, ok := elem.(*resource); ok {
			return r, nil
		}
		return &attributeSchema{
			ctyType:   elementType,
			valueType: valueType,
			elem:      elem,
		}, nil
	default:
		return nil, fmt.Errorf("unexpected value type %v", valueType)
	}
}

func unmarshalCompositeType(ty cty.Type) (shim.ValueType, interface{}, error) {
	switch {
	case ty.IsListType():
		elementType, err := unmarshalNestedType(ty.ElementType())
		if err != nil {
			return shim.TypeInvalid, nil, err
		}
		return shim.TypeList, elementType, nil
	case ty.IsMapType():
		elementType, err := unmarshalNestedType(ty.ElementType())
		if err != nil {
			return shim.TypeInvalid, nil, err
		}
		return shim.TypeMap, elementType, nil
	case ty.IsSetType():
		elementType, err := unmarshalNestedType(ty.ElementType())
		if err != nil {
			return shim.TypeInvalid, nil, err
		}
		return shim.TypeSet, elementType, nil
	case ty.IsObjectType():
		properties := schema.SchemaMap{}
		for name, ty := range ty.AttributeTypes() {
			property, err := unmarshalNestedType(ty)
			if err != nil {
				return shim.TypeInvalid, nil, err
			}
			if s, ok := property.(*attributeSchema); ok {
				properties[name] = s
			} else {
				r, isResource := property.(*resource)
				contract.Assert(isResource)
				properties[name] = &attributeSchema{
					ctyType:   r.ctyType,
					valueType: shim.TypeMap,
					elem:      s,
				}
			}
		}
		return shim.TypeMap, &resource{ctyType: ty, schema: properties}, nil
	default:
		return shim.TypeInvalid, nil, fmt.Errorf("unexpected composite type %v", ty)
	}
}

func unmarshalType(ty cty.Type) (shim.ValueType, interface{}, error) {
	switch ty {
	case cty.String:
		return shim.TypeString, nil, nil
	case cty.Bool:
		return shim.TypeBool, nil, nil
	case cty.Number:
		return shim.TypeFloat, nil, nil
	default:
		return unmarshalCompositeType(ty)
	}
}

func unmarshalAttribute(attribute *proto.Schema_Attribute) (*attributeSchema, error) {
	var ty cty.Type
	if err := json.Unmarshal(attribute.Type, &ty); err != nil {
		return nil, fmt.Errorf("failed to unmarshal type: %w", err)
	}

	valueType, elem, err := unmarshalType(ty)
	if err != nil {
		return nil, err
	}

	optional := attribute.Optional
	if !attribute.Computed && !attribute.Required {
		optional = true
	}

	return &attributeSchema{
		ctyType:     ty,
		valueType:   valueType,
		elem:        elem,
		description: attribute.Description,
		required:    attribute.Required,
		optional:    optional,
		computed:    attribute.Computed,
		sensitive:   attribute.Sensitive,
		deprecated:  deprecationMessage(attribute.Name, attribute.Deprecated),
	}, nil
}

func unmarshalBlock(block *proto.Schema_Block) (cty.Type, schema.SchemaMap, bool, error) {
	attributes, properties, allComputed := map[string]cty.Type{}, schema.SchemaMap{}, true
	for _, attribute := range block.Attributes {
		property, err := unmarshalAttribute(attribute)
		if err != nil {
			return cty.Type{}, nil, false, err
		}
		attributes[attribute.Name], properties[attribute.Name] = property.ctyType, property
		if !property.Computed() {
			allComputed = false
		}
	}
	for _, nestedBlock := range block.BlockTypes {
		property, err := unmarshalNestedBlock(nestedBlock)
		if err != nil {
			return cty.Type{}, nil, false, err
		}
		attributes[nestedBlock.TypeName], properties[nestedBlock.TypeName] = property.ctyType, property
		if !property.Computed() {
			allComputed = false
		}
	}
	return cty.Object(attributes), properties, allComputed, nil
}

func unmarshalNestedBlock(nestedBlock *proto.Schema_NestedBlock) (*attributeSchema, error) {
	objectType, properties, computed, err := unmarshalBlock(nestedBlock.Block)
	if err != nil {
		return nil, err
	}

	ctyType, valueType := objectType, shim.TypeMap
	switch nestedBlock.Nesting {
	case proto.Schema_NestedBlock_LIST:
		ctyType, valueType = cty.List(objectType), shim.TypeList
	case proto.Schema_NestedBlock_SET:
		ctyType, valueType = cty.Set(objectType), shim.TypeSet
	case proto.Schema_NestedBlock_MAP:
		ctyType = cty.Map(objectType)
	}

	required, optional := false, false
	if !computed {
		if nestedBlock.Nesting == proto.Schema_NestedBlock_SINGLE {
			required, optional = true, false
		} else {
			required, optional = nestedBlock.MinItems > 0, nestedBlock.MinItems == 0
		}
	}
	return &attributeSchema{
		ctyType:     ctyType,
		valueType:   valueType,
		elem:        &resource{ctyType: objectType, schema: properties},
		description: nestedBlock.Block.Description,
		required:    required,
		optional:    optional,
		computed:    computed,
		deprecated:  deprecationMessage(nestedBlock.TypeName, nestedBlock.Block.Deprecated),
		minItems:    int(nestedBlock.MinItems),
		maxItems:    int(nestedBlock.MaxItems),
	}, nil
}

func unmarshalResource(p *provider, typeName string, resourceSchema *proto.Schema) (*resource, error) {
	ctyType, properties, _, err := unmarshalBlock(resourceSchema.Block)
	if err != nil {
		return nil, err
	}

	// Ensure that `id` is treated as a pure output property.
	if id, ok := properties["id"]; ok {
		schema := id.(*attributeSchema)
		schema.optional = false
		schema.required = false
		schema.computed = true
	}

	return &resource{
		provider:      p,
		resourceType:  typeName,
		ctyType:       ctyType,
		schema:        properties,
		schemaVersion: int(resourceSchema.Version),
	}, nil
}

func unmarshalResourceMap(p *provider, resources map[string]*proto.Schema) (resourceMap, error) {
	resourceMap := resourceMap{}
	for name, schema := range resources {
		r, err := unmarshalResource(p, name, schema)
		if err != nil {
			return nil, err
		}
		resourceMap[name] = r
	}
	return resourceMap, nil
}
