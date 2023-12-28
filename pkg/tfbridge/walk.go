// Copyright 2016-2023, Pulumi Corporation.
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
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/util"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/walk"
)

type SchemaPath = walk.SchemaPath

// Translate a Pulumi property path to a bridged provider schema path.
//
// The function hides the complexity of mapping Pulumi property names to Terraform names, joining Schema with user
// overrides in SchemaInfos, and accounting for MaxItems=1 situations where Pulumi flattens collections to plain values.
// and therefore SchemaPath values are longer than the PropertyPath values.
//
// PropertyPathToSchemaPath may return nil if there is no matching schema found. This may happen when drilling down to
// values of unknown type, attributes not tracked in schema, or when there is a type mismatch between the path and the
// schema.
func PropertyPathToSchemaPath(
	propertyPath resource.PropertyPath,
	schemaMap shim.SchemaMap,
	schemaInfos map[string]*SchemaInfo) SchemaPath {
	return propertyPathToSchemaPath(walk.NewSchemaPath(), propertyPath, schemaMap, schemaInfos)
}

func propertyPathToSchemaPath(
	basePath SchemaPath,
	propertyPath resource.PropertyPath,
	schemaMap shim.SchemaMap,
	schemaInfos map[string]*SchemaInfo,
) SchemaPath {

	if len(propertyPath) == 0 {
		return basePath
	}

	if schemaInfos == nil {
		schemaInfos = make(map[string]*SchemaInfo)
	}

	firstStep, ok := propertyPath[0].(string)
	if !ok {
		return nil
	}

	firstStepTF := PulumiToTerraformName(firstStep, schemaMap, schemaInfos)

	fieldSchema, found := schemaMap.GetOk(firstStepTF)
	if !found {
		return nil
	}
	fieldInfo := schemaInfos[firstStepTF]
	return propertyPathToSchemaPathInner(basePath.GetAttr(firstStepTF), propertyPath[1:], fieldSchema, fieldInfo)
}

func propertyPathToSchemaPathInner(
	basePath SchemaPath,
	propertyPath resource.PropertyPath,
	schema shim.Schema,
	schemaInfo *SchemaInfo,
) SchemaPath {

	if len(propertyPath) == 0 {
		return basePath
	}

	if schemaInfo == nil {
		schemaInfo = &SchemaInfo{}
	}

	// Detect single-nested blocks (object types).
	if res, isRes := schema.Elem().(shim.Resource); schema.Type() == shim.TypeMap && isRes {
		return propertyPathToSchemaPath(basePath, propertyPath, res.Schema(), schemaInfo.Fields)
	}

	// Detect collections.
	switch schema.Type() {
	case shim.TypeList, shim.TypeMap, shim.TypeSet:
		var elemPP resource.PropertyPath
		if IsMaxItemsOne(schema, schemaInfo) {
			// Pulumi flattens MaxItemsOne values, so the element path is the same as the current path.
			elemPP = propertyPath
		} else {
			// For normal collections the first element drills down into the collection, skip it.
			elemPP = propertyPath[1:]
		}
		switch e := schema.Elem().(type) {
		case shim.Resource: // object element type
			return propertyPathToSchemaPath(basePath.Element(), elemPP, e.Schema(), schemaInfo.Fields)
		case shim.Schema: // non-object element type
			return propertyPathToSchemaPathInner(basePath.Element(), elemPP, e, schemaInfo.Elem)
		case nil: // unknown element type
			// Cannot drill down further, but len(propertyPath)>0.
			return nil
		}
	}

	// Cannot drill down further, but len(propertyPath)>0.
	return nil
}

// Translate a a bridged provider schema path into a Pulumi property path.
//
// The function hides the complexity of mapping Terraform names to Pulumi property names,
// joining Schema with user overrides in [SchemaInfo]s, and accounting for MaxItems=1
// situations where Pulumi flattens collections to plain values.
//
// [SchemaPathToPropertyPath] may return nil if there is no matching schema found. This
// may happen when drilling down to values of unknown type, attributes not tracked in
// schema, or when there is a type mismatch between the path and the schema.
//
// ## Element handling
//
// [SchemaPath]s can be either attributes or existential elements. .Element() segments are
// existential because they represent some element access, but not a specific element
// access. For example:
//
//	NewSchemaPath().GetAttr("x").Element().GetAttr("y")
//
// [resource.PropertyPath]s have attributes or instantiated elements. Elements are
// instantiated because they represent a specific element access. For example:
//
//	x[3].y
//
// [SchemaPathToPropertyPath] translates all existential elements into the "*"
// (_universal_) element. For example:
//
//	NewSchemaPath().GetAttr("x").Element().GetAttr("y") => x["*"].y
//
// This is information preserving, but "*"s are not usable in all functions that
// accept [resource.PropertyPath].
func SchemaPathToPropertyPath(
	schemaPath SchemaPath,
	schemaMap shim.SchemaMap,
	schemaInfos map[string]*SchemaInfo) resource.PropertyPath {
	return schemaPathToPropertyPath(resource.PropertyPath{}, schemaPath, schemaMap, schemaInfos)
}

func schemaPathToPropertyPath(
	basePath resource.PropertyPath,
	schemaPath SchemaPath,
	schemaMap shim.SchemaMap,
	schemaInfos map[string]*SchemaInfo) resource.PropertyPath {

	if len(schemaPath) == 0 {
		return basePath
	}

	if schemaInfos == nil {
		schemaInfos = make(map[string]*SchemaInfo)
	}

	firstStep, ok := schemaPath[0].(walk.GetAttrStep)
	if !ok {
		return nil
	}

	firstStepPu := TerraformToPulumiNameV2(firstStep.Name, schemaMap, schemaInfos)

	fieldSchema, found := schemaMap.GetOk(firstStep.Name)
	if !found {
		return nil
	}
	fieldInfo := schemaInfos[firstStep.Name]
	return schemaPathToPropertyPathInner(append(basePath, firstStepPu), schemaPath[1:], fieldSchema, fieldInfo)
}

func schemaPathToPropertyPathInner(
	basePath resource.PropertyPath,
	schemaPath SchemaPath,
	schema shim.Schema,
	schemaInfo *SchemaInfo,
) resource.PropertyPath {

	if len(schemaPath) == 0 {
		return basePath
	}

	if schemaInfo == nil {
		schemaInfo = &SchemaInfo{}
	}

	// Detect single-nested blocks (object types).
	if obj, isObject := util.CastToTypeObject(schema); isObject {
		return schemaPathToPropertyPath(basePath, schemaPath, obj, schemaInfo.Fields)
	}

	// Detect collections.
	switch schema.Type() {
	case shim.TypeList, shim.TypeMap, shim.TypeSet:
		// If a element is MaxItemsOne, it doesn't appear in the resource.PropertyPath at all.
		//
		// Otherwise we represent Elem relationships with a "*", since the schema
		// change applies to all nested items.
		if !IsMaxItemsOne(schema, schemaInfo) {
			basePath = append(basePath, "*")
		}
		switch e := schema.Elem().(type) {
		case shim.Resource: // object element type
			return schemaPathToPropertyPath(basePath, schemaPath[1:], e.Schema(), schemaInfo.Fields)
		case shim.Schema: // non-object element type
			return schemaPathToPropertyPathInner(basePath, schemaPath[1:], e, schemaInfo.Elem)
		case nil: // unknown element type
			// Cannot drill down further, but len(propertyPath)>0.
			return nil
		}
	}

	// Cannot drill down further, but len(propertyPath)>0.
	return nil
}

// Convenience method to lookup both a Schema and a SchemaInfo by path.
func LookupSchemas(schemaPath SchemaPath,
	schemaMap shim.SchemaMap,
	schemaInfos map[string]*SchemaInfo) (shim.Schema, *SchemaInfo, error) {

	s, err := walk.LookupSchemaMapPath(schemaPath, schemaMap)
	if err != nil {
		return nil, nil, err
	}

	return s, LookupSchemaInfoMapPath(schemaPath, schemaInfos), nil
}

// Drill down a path from a map of SchemaInfo objects and find a matching SchemaInfo if any.
func LookupSchemaInfoMapPath(
	schemaPath SchemaPath,
	schemaInfos map[string]*SchemaInfo,
) *SchemaInfo {

	if len(schemaPath) == 0 {
		return nil
	}

	if schemaInfos == nil {
		return nil
	}

	switch step := schemaPath[0].(type) {
	case walk.ElementStep:
		return nil
	case walk.GetAttrStep:
		return LookupSchemaInfoPath(schemaPath[1:], schemaInfos[step.Name])
	}

	return nil
}

// Drill down a path from a  SchemaInfo object and find a matching SchemaInfo if any.
func LookupSchemaInfoPath(schemaPath SchemaPath, schemaInfo *SchemaInfo) *SchemaInfo {
	if len(schemaPath) == 0 {
		return schemaInfo
	}
	if schemaInfo == nil {
		return nil
	}
	switch schemaPath[0].(type) {
	case walk.ElementStep:
		return LookupSchemaInfoPath(schemaPath[1:], schemaInfo.Elem)
	case walk.GetAttrStep:
		return LookupSchemaInfoMapPath(schemaPath, schemaInfo.Fields)
	default:
		return nil
	}
}
