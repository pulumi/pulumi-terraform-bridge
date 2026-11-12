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

package main

import (
	"strings"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/util"
)

// fixUnderscorePrefixedFields walks every resource and data source schema in prov and
// assigns an explicit SchemaInfo.Name to every property (at any nesting depth) whose
// Terraform name begins with an underscore.
//
// tfgen's propertyVariable requires an explicit Name mapping for such properties: a
// required underscore-prefixed property with no mapping is a hard error that aborts
// schema generation for the whole provider, and an optional one is silently dropped.
// That design assumes a maintainer can hand-author a SchemaInfo.Fields entry to fix the
// handful of cases that come up in a given provider. A dynamically bridged provider has
// no such hand-authored Go code: it is reparameterized for an arbitrary upstream
// Terraform provider and version at runtime. Without this step, any upstream provider
// that ships a required underscore-prefixed attribute (for example
// elastic/elasticstack's `_id`/`_index` fields on its query-ruleset action docs, which
// mirror the Elasticsearch API's own field names) breaks `pulumi package add`/`pulumi
// install` entirely, with no way for a user to work around it short of pinning an older
// provider version.
//
// This mirrors what a maintainer would do by hand: give the property a valid Pulumi
// name by stripping its leading underscore(s). It only touches properties that tfgen
// would otherwise refuse to generate or silently drop; it never overrides a Name the
// provider parameterization already specified.
func fixUnderscorePrefixedFields(prov *tfbridge.ProviderInfo) {
	for key, res := range prov.Resources {
		if r, ok := prov.P.ResourcesMap().GetOk(key); ok {
			res.Fields = mergeUnderscoreFields(res.Fields, r.Schema())
		}
	}
	for key, ds := range prov.DataSources {
		if r, ok := prov.P.DataSourcesMap().GetOk(key); ok {
			ds.Fields = mergeUnderscoreFields(ds.Fields, r.Schema())
		}
	}
}

// mergeUnderscoreFields returns fields augmented with an entry for every
// underscore-prefixed property directly in schemaMap, recursing into nested object
// types (single-nested blocks, and list/set-nested blocks). Pre-existing entries and
// any Name they already carry are left untouched.
func mergeUnderscoreFields(
	fields map[string]*tfbridge.SchemaInfo, schemaMap shim.SchemaMap,
) map[string]*tfbridge.SchemaInfo {
	if schemaMap == nil {
		return fields
	}

	result := make(map[string]*tfbridge.SchemaInfo, len(fields))
	for k, v := range fields {
		result[k] = v
	}

	schemaMap.Range(func(key string, sch shim.Schema) bool {
		fieldInfo := result[key]

		// tfgen always looks up nested object properties (whether from a
		// single-nested block or a list/set-nested block) via SchemaInfo.Elem.Fields,
		// never SchemaInfo.Fields directly; see makeObjectPropertyType /
		// makePropertyType in pkg/tfgen/generate.go.
		var childFields map[string]*tfbridge.SchemaInfo
		if nested := nestedObjectSchema(sch); nested != nil {
			var base map[string]*tfbridge.SchemaInfo
			if fieldInfo != nil && fieldInfo.Elem != nil {
				base = fieldInfo.Elem.Fields
			}
			childFields = mergeUnderscoreFields(base, nested)
		}

		needsName := strings.HasPrefix(key, "_") && (fieldInfo == nil || fieldInfo.Name == "")
		if !needsName && childFields == nil {
			return true
		}

		updated := &tfbridge.SchemaInfo{}
		if fieldInfo != nil {
			cp := *fieldInfo
			updated = &cp
		}
		if needsName {
			updated.Name = underscoreFieldName(key, schemaMap)
		}
		if childFields != nil {
			elem := &tfbridge.SchemaInfo{}
			if updated.Elem != nil {
				cp := *updated.Elem
				elem = &cp
			}
			elem.Fields = childFields
			updated.Elem = elem
		}
		result[key] = updated
		return true
	})

	if len(result) == 0 {
		return nil
	}
	return result
}

// nestedObjectSchema returns the schema of the object type nested inside sch, if any
// (covering both single-nested blocks and list/set-nested blocks), or nil if sch does
// not describe an object type.
func nestedObjectSchema(sch shim.Schema) shim.SchemaMap {
	if sch == nil {
		return nil
	}
	if m, ok := util.CastToTypeObject(sch); ok {
		return m
	}
	switch sch.Type() {
	case shim.TypeList, shim.TypeSet:
		if res, ok := sch.Elem().(shim.Resource); ok {
			return res.Schema()
		}
	}
	return nil
}

// underscoreFieldName derives a valid Pulumi property name for a Terraform attribute
// whose name begins with an underscore, by stripping the leading underscore(s) and
// applying the bridge's usual snake_case-to-camelCase conversion. If stripping would
// collide with a sibling attribute's Terraform name, the underscore is instead spelled
// out as the word "underscore" so the two properties don't map to the same Pulumi name.
func underscoreFieldName(key string, siblings shim.SchemaMap) string {
	trimmed := strings.TrimLeft(key, "_")
	if trimmed == "" {
		trimmed = "field"
	}
	if _, collides := siblings.GetOk(trimmed); collides {
		trimmed = "underscore_" + trimmed
	}
	return tfbridge.TerraformToPulumiNameV2(trimmed, nil, nil)
}
