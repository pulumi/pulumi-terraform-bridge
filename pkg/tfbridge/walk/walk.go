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

package walk

import (
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/walk"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
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
	schemaInfos map[string]*tfbridge.SchemaInfo) SchemaPath {
	return propertyPathToSchemaPath(walk.NewSchemaPath(), propertyPath, schemaMap, schemaInfos)
}

func propertyPathToSchemaPath(
	basePath SchemaPath,
	propertyPath resource.PropertyPath,
	schemaMap shim.SchemaMap,
	schemaInfos map[string]*tfbridge.SchemaInfo,
) SchemaPath {

	if len(propertyPath) == 0 {
		return basePath
	}

	if schemaInfos == nil {
		schemaInfos = make(map[string]*tfbridge.SchemaInfo)
	}

	firstStep, ok := propertyPath[0].(string)
	if !ok {
		return nil
	}

	firstStepTF := tfbridge.PulumiToTerraformName(firstStep, schemaMap, schemaInfos)

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
	schemaInfo *tfbridge.SchemaInfo,
) SchemaPath {

	if len(propertyPath) == 0 {
		return basePath
	}

	if schemaInfo == nil {
		schemaInfo = &tfbridge.SchemaInfo{}
	}

	// Detect single-nested blocks (object types).
	if res, isRes := schema.Elem().(shim.Resource); schema.Type() == shim.TypeMap && isRes {
		return propertyPathToSchemaPath(basePath, propertyPath, res.Schema(), schemaInfo.Fields)
	}

	// Detect collections.
	switch schema.Type() {
	case shim.TypeList, shim.TypeMap, shim.TypeSet:
		var elemPP resource.PropertyPath
		if tfbridge.IsMaxItemsOne(schema, schemaInfo) {
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

// Convenience method to lookup both a Schema and a SchemaInfo by path.
func LookupSchemas(schemaPath walk.SchemaPath,
	schemaMap shim.SchemaMap,
	schemaInfos map[string]*tfbridge.SchemaInfo) (shim.Schema, *tfbridge.SchemaInfo, error) {

	s, err := walk.LookupSchemaMapPath(schemaPath, schemaMap)
	if err != nil {
		return nil, nil, err
	}

	return s, LookupSchemaInfoMapPath(schemaPath, schemaInfos), nil
}

// Drill down a path from a map of SchemaInfo objects and find a matching SchemaInfo if any.
func LookupSchemaInfoMapPath(
	schemaPath SchemaPath,
	schemaInfos map[string]*tfbridge.SchemaInfo,
) *tfbridge.SchemaInfo {

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
func LookupSchemaInfoPath(schemaPath SchemaPath, schemaInfo *tfbridge.SchemaInfo) *tfbridge.SchemaInfo {
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
