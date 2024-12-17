package tfbridge

import (
	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/unstable/propertyvalue"
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

func (k propertyPath) Get(v resource.PropertyValue) (resource.PropertyValue, bool) {
	path := resource.PropertyPath(k)
	return path.Get(v)
}

func (k propertyPath) IsReservedKey() bool {
	leaf := k[len(k)-1]
	return leaf == "__meta" || leaf == "__defaults"
}

func (k propertyPath) GetFromMap(v resource.PropertyMap) (resource.PropertyValue, bool) {
	path := resource.PropertyPath(k)
	return path.Get(resource.NewProperty(v))
}

func (k propertyPath) GetPathRelativeTo(other propertyPath) (propertyPath, error) {
	contains := resource.PropertyPath(other).Contains(resource.PropertyPath(k))
	if !contains {
		return propertyPath{}, errors.New("other path is not a subpath of k")
	}

	relativePath := resource.PropertyPath(k)[len(resource.PropertyPath(other)):]
	return propertyPath(relativePath), nil
}

func lookupSchemas(
	path propertyPath, tfs shim.SchemaMap, ps map[string]*info.Schema,
) (shim.Schema, *info.Schema, error) {
	schemaPath := PropertyPathToSchemaPath(resource.PropertyPath(path), tfs, ps)
	return LookupSchemas(schemaPath, tfs, ps)
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

// propertyValueIsSubsetBarComputed returns true if all values in the walkedValue are also in the comparedValue,
// bar any computed properties.
func propertyValueIsSubsetBarComputed(
	path propertyPath,
	comparedValue resource.PropertyValue,
	walkedValue resource.PropertyValue,
	tfs shim.SchemaMap,
	ps map[string]*info.Schema,
) bool {
	abortErr := errors.New("abort")
	visitor := func(subpath resource.PropertyPath, walkedSubVal resource.PropertyValue) (resource.PropertyValue, error) {
		tfs, _, err := lookupSchemas(propertyPath(subpath), tfs, ps)
		if err != nil {
			// TODO log
			return resource.NewNullProperty(), abortErr
		}

		relativePath, err := propertyPath(subpath).GetPathRelativeTo(path)
		if err != nil {
			return resource.NewNullProperty(), abortErr
		}

		comparedSubVal, ok := relativePath.Get(comparedValue)
		if !ok {
			if tfs.Computed() {
				return walkedSubVal, nil
			}
			return resource.NewNullProperty(), abortErr
		}

		if tfs.Type() == shim.TypeList || tfs.Type() == shim.TypeMap {
			// We only need to check the leaf values, so we skip any collection types.
			return walkedSubVal, nil
		}

		// We can not descend into nested sets as planning re-orders the elements
		if tfs.Type() == shim.TypeSet {
			// TODO: more sophisticated comparison of nested sets
			if walkedSubVal.DeepEquals(comparedSubVal) {
				return walkedSubVal, propertyvalue.LimitDescentError{}
			}
			return resource.NewNullProperty(), abortErr
		}

		if walkedSubVal.DeepEquals(comparedSubVal) {
			return walkedSubVal, nil
		}

		return resource.NewNullProperty(), abortErr
	}
	_, err := propertyvalue.TransformPropertyValueLimitDescent(
		resource.PropertyPath(path),
		visitor,
		walkedValue,
	)
	if err == abortErr {
		return false
	}
	contract.AssertNoErrorf(err, "TransformPropertyValue should only return an abort error")
	return true
}

// validInputsFromPlan returns true if the given plan property value could originate from the given inputs.
// Under the hood, it walks the plan and the inputs and checks that all differences stem from computed properties.
// Any differences coming from properties which are not computed will be rejected.
// Note that we are relying on the fact that the inputs will have defaults already applied.
func validInputsFromPlan(
	path propertyPath,
	inputs resource.PropertyValue,
	plan resource.PropertyValue,
	tfs shim.SchemaMap,
	ps map[string]*info.Schema,
) bool {
	// We walk both the plan and the inputs and check that all differences stem from computed properties.
	if !propertyValueIsSubsetBarComputed(path, inputs, plan, tfs, ps) {
		return false
	}

	return propertyValueIsSubsetBarComputed(path, plan, inputs, tfs, ps)
}
