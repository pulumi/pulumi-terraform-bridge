package tfbridge

import (
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

type propertyPath resource.PropertyPath

func isForceNew(tfs shim.Schema, ps *SchemaInfo) bool {
	return (tfs != nil && tfs.ForceNew()) ||
		(ps != nil && ps.ForceNew != nil && *ps.ForceNew)
}

func newPropertyPath(root resource.PropertyKey) propertyPath {
	return propertyPath{string(root)}
}

func (k propertyPath) String() string {
	return resource.PropertyPath(k).String()
}

func (k propertyPath) Key() detailedDiffKey {
	return detailedDiffKey(k.String())
}

func (k propertyPath) append(subkey interface{}) propertyPath {
	return append(k, subkey)
}

func (k propertyPath) Subpath(subkey string) propertyPath {
	return k.append(subkey)
}

func (k propertyPath) Index(i int) propertyPath {
	return k.append(i)
}

func (k propertyPath) IsReservedKey() bool {
	leaf := k[len(k)-1]
	return leaf == "__meta" || leaf == "__defaults"
}

func lookupSchemas(
	path propertyPath, tfs shim.SchemaMap, ps map[string]*info.Schema,
) (shim.Schema, *info.Schema, error) {
	schemaPath := PropertyPathToSchemaPath(resource.PropertyPath(path), tfs, ps)
	return LookupSchemas(schemaPath, tfs, ps)
}

// walkPropertyValue walks a property value and calls the visitor function for each path in the property value.
func walkPropertyValue(
	val resource.PropertyValue, path propertyPath, visitor func(propertyPath, resource.PropertyValue) bool,
) bool {
	if !visitor(path, val) {
		return false
	}

	switch {
	case val.IsArray():
		for i, elVal := range val.ArrayValue() {
			if !walkPropertyValue(elVal, path.Index(i), visitor) {
				return false
			}
		}

	case val.IsObject():
		for k, elVal := range val.ObjectValue() {
			if !walkPropertyValue(elVal, path.Subpath(string(k)), visitor) {
				return false
			}
		}
	}
	return true
}

func willTriggerReplacement(
	path propertyPath, rootTFSchema shim.SchemaMap, rootPulumiSchema map[string]*info.Schema,
) bool {
	// A change on a property might trigger a replacement if:
	// - The property itself is marked as ForceNew
	// - The direct parent property is a collection (list, set, map) and is marked as ForceNew
	// See pkg/cross-tests/diff_cross_test.go
	// TestAttributeCollectionForceNew, TestBlockCollectionForceNew, TestBlockCollectionElementForceNew
	// for a full case study of replacements in TF
	tfs, ps, err := lookupSchemas(path, rootTFSchema, rootPulumiSchema)
	if err != nil {
		return false
	}
	if isForceNew(tfs, ps) {
		return true
	}

	if len(path) == 1 {
		return false
	}

	parent := path[:len(path)-1]
	tfs, ps, err = lookupSchemas(parent, rootTFSchema, rootPulumiSchema)
	if err != nil {
		return false
	}
	// Note this is mimicking the TF behaviour, so the effective type is not considered here.
	if tfs.Type() != shim.TypeList && tfs.Type() != shim.TypeSet && tfs.Type() != shim.TypeMap {
		return false
	}
	return isForceNew(tfs, ps)
}

func willTriggerReplacementRecursive(
	path propertyPath, value resource.PropertyValue, tfs shim.SchemaMap, ps map[string]*info.Schema,
) bool {
	replacement := false
	visitor := func(subpath propertyPath, val resource.PropertyValue) bool {
		if willTriggerReplacement(subpath, tfs, ps) {
			replacement = true
			return false
		}
		return true
	}

	walkPropertyValue(value, path, visitor)

	return replacement
}
