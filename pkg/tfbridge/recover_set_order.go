package tfbridge

import (
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"

	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/walk"
)

// a variant of PropertyPath.Get which works on PropertyMaps
func getPathFromPropertyMap(
	path resource.PropertyPath, propertyMap resource.PropertyMap,
) (resource.PropertyValue, bool) {
	if len(path) == 0 {
		return resource.NewNullProperty(), false
	}

	rootKeyStr, ok := path[0].(string)
	contract.Assertf(ok && rootKeyStr != "", "root key must be a non-empty string")
	rootKey := resource.PropertyKey(rootKeyStr)
	restPath := path[1:]

	if len(restPath) == 0 {
		return propertyMap[rootKey], true
	}

	if !propertyMap.HasValue(rootKey) {
		return resource.NewNullProperty(), false
	}

	return restPath.Get(propertyMap[rootKey])
}

// a variant of PropertyPath.Set which works on PropertyMaps
func setPathInPropertyMap(
	path resource.PropertyPath, propertyMap resource.PropertyMap, value resource.PropertyValue,
) bool {
	if len(path) == 0 {
		return false
	}

	rootKeyStr, ok := path[0].(string)
	contract.Assertf(ok && rootKeyStr != "", "root key must be a non-empty string")
	rootKey := resource.PropertyKey(rootKeyStr)
	restPath := path[1:]

	if len(restPath) == 0 {
		propertyMap[rootKey] = value
		return true
	}

	if !propertyMap.HasValue(rootKey) {
		return false
	}

	return restPath.Set(propertyMap[rootKey], value)
}

// Sets in Terraform are unordered but are represented as ordered lists in the Pulumi world, and detailed diff must
// return indices into this list to indicate the elements that have changed.
func recoverSetOrderSingle(
	target resource.PropertyMap, origin resource.PropertyMap, setPath resource.PropertyPath,
) {
	targetSet, ok := getPathFromPropertyMap(setPath, target)
	if !ok || !targetSet.IsArray() || targetSet.IsComputed() {
		return
	}
	originSet, ok := getPathFromPropertyMap(setPath, origin)
	if !ok || !originSet.IsArray() {
		return
	}

	// No need to recover the order if the set is empty or has only one element
	if len(targetSet.ArrayValue()) == 0 || len(originSet.ArrayValue()) == 0 || len(targetSet.ArrayValue()) == 1 {
		return
	}

	ok = setPathInPropertyMap(setPath, target, originSet)
	contract.Assertf(ok, "failed to set the origin set but it should exist!")
}

// TODO: Should we cache this?
func getSetPaths(schema shim.SchemaMap) []SchemaPath {
	paths := make([]SchemaPath, 0)

	walk.VisitSchemaMap(schema, func(path SchemaPath, schema shim.Schema) {
		if schema.Type() == shim.TypeSet {
			paths = append(paths, path)
		}
	})

	return paths
}

func recoverSetOrder(
	target resource.PropertyMap, origin resource.PropertyMap, tfs shim.SchemaMap, ps map[string]*SchemaInfo,
) {
	setPaths := getSetPaths(tfs)
	for _, setPath := range setPaths {
		propertyPath := SchemaPathToPropertyPath(setPath, tfs, ps)
		recoverSetOrderSingle(target, origin, propertyPath)
	}
}

// TODO: unit tests
