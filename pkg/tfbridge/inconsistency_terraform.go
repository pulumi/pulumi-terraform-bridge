// Copyright 2016-2024, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tfbridge

import (
	"context"
	"fmt"

	"github.com/hashicorp/go-multierror"
	"github.com/zclconf/go-cty/cty"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/vendored/opentofu/configs/configschema"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/vendored/opentofu/plans/objchange"
)

// detectInconsistentApplyWithTerraform uses OpenTofu's vendored AssertObjectCompatible
// to detect inconsistencies between planned and actual states for SDKv2 providers.
//
// This function replaces the custom detection logic and ensures we match Terraform's
// behavior exactly.
func detectInconsistentApplyWithTerraform(
	ctx context.Context,
	resourceType string,
	plannedState shim.InstanceState,
	actualState shim.InstanceState,
	schema shim.SchemaMap,
) error {
	// Skip if inconsistency detection is disabled for this resource
	if !ShouldDetectInconsistentApply(resourceType) {
		return nil
	}

	if plannedState == nil || actualState == nil || schema == nil {
		return nil
	}

	// Convert shim types to cty.Value and configschema.Block
	plannedValue, err := shimStateToCtyValue(plannedState, schema)
	if err != nil {
		return fmt.Errorf("error converting planned state: %w", err)
	}

	actualValue, err := shimStateToCtyValue(actualState, schema)
	if err != nil {
		return fmt.Errorf("error converting actual state: %w", err)
	}

	schemaBlock := shimSchemaToBlock(schema)

	// Use OpenTofu's vendored consistency check
	errs := objchange.AssertObjectCompatible(
		schemaBlock,
		plannedValue,
		actualValue,
	)

	if len(errs) > 0 {
		// Format and log the inconsistency
		msg := FormatTerraformStyleInconsistency(resourceType, errs)
		logger := GetLogger(ctx)
		logger.Warn(msg)
	}

	return nil
}

// shimStateToCtyValue converts a shim.InstanceState to cty.Value
func shimStateToCtyValue(state shim.InstanceState, schema shim.SchemaMap) (cty.Value, error) {
	if state == nil {
		return cty.NullVal(cty.DynamicPseudoType), nil
	}

	// Get the object representation from state
	obj, err := state.Object(schema)
	if err != nil {
		return cty.NilVal, fmt.Errorf("error getting object from state: %w", err)
	}

	// Build cty.Type from schema
	ctyType := schemaMapToCtyType(schema)

	// Convert object map to cty.Value
	return objectMapToCtyValue(obj, ctyType, schema)
}

// shimSchemaToBlock converts shim.SchemaMap to configschema.Block
func shimSchemaToBlock(schemaMap shim.SchemaMap) *configschema.Block {
	block := &configschema.Block{
		Attributes: make(map[string]*configschema.Attribute),
		BlockTypes: make(map[string]*configschema.NestedBlock),
	}

	schemaMap.Range(func(key string, elem shim.Schema) bool {
		if elem == nil {
			return true // continue
		}

		// Check if this is a nested block (List/Set/Map of Resource)
		if isNestedBlock(elem) {
			block.BlockTypes[key] = shimSchemaToNestedBlock(elem)
		} else {
			// Regular attribute
			block.Attributes[key] = shimSchemaToAttribute(elem)
		}
		return true // continue
	})

	return block
}

// isNestedBlock checks if a schema element represents a nested block
func isNestedBlock(elem shim.Schema) bool {
	// Nested blocks are List, Set, or Map types with a nested Resource
	schemaType := elem.Type()
	if schemaType != shim.TypeList && schemaType != shim.TypeSet && schemaType != shim.TypeMap {
		return false
	}

	elemValue := elem.Elem()
	if elemValue == nil {
		return false
	}

	// Check if Elem is a Resource (SchemaMap)
	_, isResource := elemValue.(shim.SchemaMap)
	return isResource
}

// shimSchemaToAttribute converts a shim.Schema to configschema.Attribute
func shimSchemaToAttribute(elem shim.Schema) *configschema.Attribute {
	return &configschema.Attribute{
		Type:      shimTypeToCtyType(elem),
		Optional:  elem.Optional(),
		Required:  elem.Required(),
		Computed:  elem.Computed(),
		Sensitive: elem.Sensitive(),
	}
}

// shimSchemaToNestedBlock converts a shim.Schema to configschema.NestedBlock
func shimSchemaToNestedBlock(elem shim.Schema) *configschema.NestedBlock {
	elemValue := elem.Elem()
	nestedSchemaMap, _ := elemValue.(shim.SchemaMap)

	nb := &configschema.NestedBlock{
		Block:    *shimSchemaToBlock(nestedSchemaMap),
		MinItems: elem.MinItems(),
		MaxItems: elem.MaxItems(),
	}

	// Determine nesting mode based on type
	switch elem.Type() {
	case shim.TypeList:
		nb.Nesting = configschema.NestingList
	case shim.TypeSet:
		nb.Nesting = configschema.NestingSet
	case shim.TypeMap:
		nb.Nesting = configschema.NestingMap
	default:
		nb.Nesting = configschema.NestingSingle
	}

	return nb
}

// shimTypeToCtyType converts a shim.Schema to its cty.Type
func shimTypeToCtyType(elem shim.Schema) cty.Type {
	switch elem.Type() {
	case shim.TypeBool:
		return cty.Bool
	case shim.TypeInt:
		return cty.Number
	case shim.TypeFloat:
		return cty.Number
	case shim.TypeString:
		return cty.String
	case shim.TypeList:
		elemType := cty.DynamicPseudoType
		if elemValue := elem.Elem(); elemValue != nil {
			if elemSchema, ok := elemValue.(shim.Schema); ok {
				elemType = shimTypeToCtyType(elemSchema)
			} else if nestedSchemaMap, ok := elemValue.(shim.SchemaMap); ok {
				elemType = schemaMapToCtyType(nestedSchemaMap)
			}
		}
		return cty.List(elemType)
	case shim.TypeSet:
		elemType := cty.DynamicPseudoType
		if elemValue := elem.Elem(); elemValue != nil {
			if elemSchema, ok := elemValue.(shim.Schema); ok {
				elemType = shimTypeToCtyType(elemSchema)
			} else if nestedSchemaMap, ok := elemValue.(shim.SchemaMap); ok {
				elemType = schemaMapToCtyType(nestedSchemaMap)
			}
		}
		return cty.Set(elemType)
	case shim.TypeMap:
		elemType := cty.DynamicPseudoType
		if elemValue := elem.Elem(); elemValue != nil {
			if elemSchema, ok := elemValue.(shim.Schema); ok {
				elemType = shimTypeToCtyType(elemSchema)
			} else if nestedSchemaMap, ok := elemValue.(shim.SchemaMap); ok {
				elemType = schemaMapToCtyType(nestedSchemaMap)
			}
		}
		return cty.Map(elemType)
	default:
		return cty.DynamicPseudoType
	}
}

// schemaMapToCtyType converts a shim.SchemaMap to cty.Type (object type)
func schemaMapToCtyType(schemaMap shim.SchemaMap) cty.Type {
	attrTypes := make(map[string]cty.Type)

	schemaMap.Range(func(key string, elem shim.Schema) bool {
		if elem == nil {
			return true // continue
		}

		if isNestedBlock(elem) {
			// Nested blocks are handled as nested objects
			elemValue := elem.Elem()
			if nestedSchemaMap, ok := elemValue.(shim.SchemaMap); ok {
				nestedType := schemaMapToCtyType(nestedSchemaMap)
				switch elem.Type() {
				case shim.TypeList:
					attrTypes[key] = cty.List(nestedType)
				case shim.TypeSet:
					attrTypes[key] = cty.Set(nestedType)
				case shim.TypeMap:
					attrTypes[key] = cty.Map(nestedType)
				default:
					attrTypes[key] = nestedType
				}
			}
		} else {
			attrTypes[key] = shimTypeToCtyType(elem)
		}
		return true // continue
	})

	return cty.Object(attrTypes)
}

// objectMapToCtyValue converts a map[string]interface{} to cty.Value
func objectMapToCtyValue(obj map[string]interface{}, typ cty.Type, schema shim.SchemaMap) (cty.Value, error) {
	if obj == nil {
		return cty.NullVal(typ), nil
	}

	attrValues := make(map[string]cty.Value)
	var errs error

	schema.Range(func(key string, elem shim.Schema) bool {
		if elem == nil {
			return true // continue
		}

		value, exists := obj[key]
		if !exists || value == nil {
			// Use null value with the correct type
			attrValues[key] = cty.NullVal(shimTypeToCtyType(elem))
			return true // continue
		}

		ctyVal, err := interfaceToCtyValue(value, elem)
		if err != nil {
			// Accumulate errors but continue processing
			errs = multierror.Append(errs, fmt.Errorf("converting attribute %q: %w", key, err))
			attrValues[key] = cty.NullVal(shimTypeToCtyType(elem))
			return true // continue
		}
		attrValues[key] = ctyVal
		return true // continue
	})

	if errs != nil {
		return cty.NilVal, errs
	}

	return cty.ObjectVal(attrValues), nil
}

// interfaceToCtyValue converts an interface{} to cty.Value based on schema
func interfaceToCtyValue(value interface{}, elem shim.Schema) (cty.Value, error) {
	if value == nil {
		return cty.NullVal(shimTypeToCtyType(elem)), nil
	}

	switch elem.Type() {
	case shim.TypeBool:
		if v, ok := value.(bool); ok {
			return cty.BoolVal(v), nil
		}
		return cty.NilVal, fmt.Errorf("expected bool, got %T", value)

	case shim.TypeInt:
		switch v := value.(type) {
		case int:
			return cty.NumberIntVal(int64(v)), nil
		case int64:
			return cty.NumberIntVal(v), nil
		case float64:
			return cty.NumberFloatVal(v), nil
		default:
			return cty.NilVal, fmt.Errorf("expected number, got %T", value)
		}

	case shim.TypeFloat:
		if v, ok := value.(float64); ok {
			return cty.NumberFloatVal(v), nil
		}
		return cty.NilVal, fmt.Errorf("expected float, got %T", value)

	case shim.TypeString:
		if v, ok := value.(string); ok {
			return cty.StringVal(v), nil
		}
		return cty.NilVal, fmt.Errorf("expected string, got %T", value)

	case shim.TypeList, shim.TypeSet:
		arr, ok := value.([]interface{})
		if !ok {
			return cty.NilVal, fmt.Errorf("expected array, got %T", value)
		}

		elemSchema := elem.Elem()
		if elemSchema == nil {
			return cty.NilVal, fmt.Errorf("missing elem schema for list/set")
		}

		var elemValues []cty.Value
		for _, item := range arr {
			if itemSchema, ok := elemSchema.(shim.Schema); ok {
				ctyVal, err := interfaceToCtyValue(item, itemSchema)
				if err != nil {
					return cty.NilVal, err
				}
				elemValues = append(elemValues, ctyVal)
			} else if nestedSchemaMap, ok := elemSchema.(shim.SchemaMap); ok {
				// Handle nested objects
				itemMap, ok := item.(map[string]interface{})
				if !ok {
					return cty.NilVal, fmt.Errorf("expected map for nested object, got %T", item)
				}
				nestedType := schemaMapToCtyType(nestedSchemaMap)
				ctyVal, err := objectMapToCtyValue(itemMap, nestedType, nestedSchemaMap)
				if err != nil {
					return cty.NilVal, err
				}
				elemValues = append(elemValues, ctyVal)
			}
		}

		if elem.Type() == shim.TypeList {
			return cty.ListVal(elemValues), nil
		}
		return cty.SetVal(elemValues), nil

	case shim.TypeMap:
		m, ok := value.(map[string]interface{})
		if !ok {
			return cty.NilVal, fmt.Errorf("expected map, got %T", value)
		}

		elemSchema := elem.Elem()
		if elemSchema == nil {
			return cty.NilVal, fmt.Errorf("missing elem schema for map")
		}

		mapValues := make(map[string]cty.Value)
		for k, v := range m {
			if itemSchema, ok := elemSchema.(shim.Schema); ok {
				ctyVal, err := interfaceToCtyValue(v, itemSchema)
				if err != nil {
					return cty.NilVal, err
				}
				mapValues[k] = ctyVal
			}
		}

		return cty.MapVal(mapValues), nil

	default:
		return cty.NilVal, fmt.Errorf("unsupported type: %v", elem.Type())
	}
}
