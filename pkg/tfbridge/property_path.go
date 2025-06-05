package tfbridge

import (
	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/reservedkeys"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/unstable/propertyvalue"
)

// propertyPath is a wrapper around resource.PropertyPath that adds some convenience methods.
// If the path is constructed using the propertyPath methods, it will only have string and int elements.
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

func (k propertyPath) Subkey(subkey resource.PropertyKey) propertyPath {
	return k.append(string(subkey))
}

func (k propertyPath) Index(i int) propertyPath {
	return k.append(i)
}

func (k propertyPath) IsReservedKey() bool {
	leaf := k[len(k)-1]
	if s, ok := leaf.(string); ok {
		return reservedkeys.IsBridgeReservedKey(s)
	}
	return false
}

func (k propertyPath) GetFromMap(v resource.PropertyMap) (resource.PropertyValue, bool) {
	path := resource.PropertyPath(k)
	return path.Get(resource.NewProperty(v))
}

func lookupSchemas(
	path propertyPath, tfs shim.SchemaMap, ps map[string]*info.Schema,
) (shim.Schema, *info.Schema, error) {
	schemaPath := PropertyPathToSchemaPath(resource.PropertyPath(path), tfs, ps)
	return LookupSchemas(schemaPath, tfs, ps)
}

func combinePropertyMapKeys(
	object1, object2 resource.PropertyMap,
) map[resource.PropertyKey]struct{} {
	combined := make(map[resource.PropertyKey]struct{})
	for k := range object1 {
		combined[k] = struct{}{}
	}
	for k := range object2 {
		combined[k] = struct{}{}
	}
	return combined
}

// SkipChildrenError is an error that can be returned by the visitor to skip the children of the current step.
type SkipChildrenError struct{}

func (SkipChildrenError) Error() string {
	return "skip children"
}

type TypeMismatchError struct{}

func (TypeMismatchError) Error() string {
	return "type mismatch"
}

type twoPropertyValueVisitor func(path propertyPath, val1, val2 resource.PropertyValue) error

// walkTwoPropertyValues walks the two property values and calls the visitor for each step.
// It returns an error if the visitor returns an error other than SkipChildrenError.
//
// The visitor can return SkipChildrenError to skip the children of the current step.
// In case the two values have different types, we walk both values, starting with val1.
//
// Note that elements inside a Secret or Output value will not get visited.
//
// Can return a TypeMismatchError in case the two values' types do not match.
func walkTwoPropertyValues(
	path propertyPath,
	val1, val2 resource.PropertyValue,
	visitor twoPropertyValueVisitor,
) error {
	err := visitor(path, val1, val2)
	if err != nil {
		if errors.Is(err, SkipChildrenError{}) {
			return nil
		}
		return err
	}

	if val1.IsNull() || val2.IsNull() {
		return nil
	}

	if val1.IsArray() && val2.IsArray() {
		arr1 := val1.ArrayValue()
		arr2 := val2.ArrayValue()
		for i := range max(len(arr1), len(arr2)) {
			childPath := path.Index(i)
			var childVal1, childVal2 resource.PropertyValue
			if i >= len(arr1) {
				childVal1 = resource.NewNullProperty()
			} else {
				childVal1 = arr1[i]
			}
			if i >= len(arr2) {
				childVal2 = resource.NewNullProperty()
			} else {
				childVal2 = arr2[i]
			}
			err := walkTwoPropertyValues(childPath, childVal1, childVal2, visitor)
			if err != nil {
				return err
			}
		}
	} else if val1.IsObject() && val2.IsObject() {
		obj1 := val1.ObjectValue()
		obj2 := val2.ObjectValue()
		combined := combinePropertyMapKeys(obj1, obj2)
		for k := range combined {
			err := walkTwoPropertyValues(path.Subkey(k), obj1[k], obj2[k], visitor)
			if err != nil {
				return err
			}
		}
	} else if val1.IsArray() || val2.IsArray() || val1.IsObject() || val2.IsObject() {
		return TypeMismatchError{}
	}
	return nil
}

func propertyPathTriggersReplacement(
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

func propertyValueTriggersReplacement(
	path propertyPath, value resource.PropertyValue, tfs shim.SchemaMap, ps map[string]*info.Schema,
) bool {
	replacement := false
	visitor := func(subpath resource.PropertyPath, val resource.PropertyValue) (resource.PropertyValue, error) {
		if propertyPathTriggersReplacement(propertyPath(subpath), tfs, ps) {
			replacement = true
		}
		return val, nil
	}

	_, err := propertyvalue.TransformPropertyValue(
		resource.PropertyPath(path),
		visitor,
		value,
	)
	contract.AssertNoErrorf(err, "TransformPropertyValue should not return an error")

	return replacement
}
