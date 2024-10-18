package tfbridge

import (
	"cmp"
	"context"
	"slices"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/walk"
)

func isPresent(val resource.PropertyValue) bool {
	return !val.IsNull() &&
		!(val.IsArray() && val.ArrayValue() == nil) &&
		!(val.IsObject() && val.ObjectValue() == nil)
}

func isForceNew(tfs shim.Schema, ps *SchemaInfo) bool {
	return (tfs != nil && tfs.ForceNew()) ||
		(ps != nil && ps.ForceNew != nil && *ps.ForceNew)
}

func sortedMergedKeys[K cmp.Ordered, V any, M ~map[K]V](a, b M) []K {
	keys := make(map[K]struct{})
	for k := range a {
		keys[k] = struct{}{}
	}
	for k := range b {
		keys[k] = struct{}{}
	}
	keysSlice := make([]K, 0, len(keys))
	for k := range keys {
		keysSlice = append(keysSlice, k)
	}
	slices.Sort(keysSlice)
	return keysSlice
}

func isTypeShapeMismatched(val resource.PropertyValue, propType shim.ValueType) bool {
	if !isPresent(val) {
		return false
	}
	switch propType {
	case shim.TypeList:
		return !val.IsArray()
	case shim.TypeSet:
		return !val.IsArray()
	case shim.TypeMap:
		return !val.IsObject()
	default:
		return false
	}
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

func promoteToReplace(diff *pulumirpc.PropertyDiff) *pulumirpc.PropertyDiff {
	if diff == nil {
		return nil
	}

	kind := diff.GetKind()
	switch kind {
	case pulumirpc.PropertyDiff_ADD:
		return &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_ADD_REPLACE}
	case pulumirpc.PropertyDiff_DELETE:
		return &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_DELETE_REPLACE}
	case pulumirpc.PropertyDiff_UPDATE:
		return &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_UPDATE_REPLACE}
	default:
		return diff
	}
}

type baseDiff string

const (
	undecidedDiff baseDiff = ""
	noDiff        baseDiff = "NoDiff"
	addDiff       baseDiff = "Add"
	deleteDiff    baseDiff = "Delete"
	updateDiff    baseDiff = "Update"
)

func (b baseDiff) ToPropertyDiff() *pulumirpc.PropertyDiff {
	switch b {
	case noDiff:
		return nil
	case addDiff:
		return &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_ADD}
	case deleteDiff:
		return &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_DELETE}
	case updateDiff:
		return &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_UPDATE}
	case undecidedDiff:
		contract.Failf("diff should not be undecided")
	default:
		contract.Failf("unexpected base diff %s", b)
	}
	contract.Failf("unreachable")
	return nil
}

func makeBaseDiff(old, new resource.PropertyValue) baseDiff {
	oldPresent := isPresent(old)
	newPresent := isPresent(new)
	if !oldPresent {
		if !newPresent {
			return noDiff
		}

		return addDiff
	}
	if !newPresent {
		return deleteDiff
	}

	if new.IsComputed() {
		return updateDiff
	}

	return undecidedDiff
}

type (
	detailedDiffKey string
	propertyPath    resource.PropertyPath
)

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

type detailedDiffer struct {
	tfs shim.SchemaMap
	ps  map[string]*SchemaInfo
}

func (differ detailedDiffer) propertyPathToSchemaPath(path propertyPath) walk.SchemaPath {
	return PropertyPathToSchemaPath(resource.PropertyPath(path), differ.tfs, differ.ps)
}

// getEffectiveType returns the pulumi-visible type of the property at the given path.
// It takes into account any MaxItemsOne flattening which might have occurred.
// Specifically:
// - If the property is a list/set with MaxItemsOne, it returns the type of the element.
// - Otherwise it returns the type of the property.
func (differ detailedDiffer) getEffectiveType(path walk.SchemaPath) shim.ValueType {
	tfs, ps, err := LookupSchemas(path, differ.tfs, differ.ps)

	if err != nil || tfs == nil {
		return shim.TypeInvalid
	}

	if IsMaxItemsOne(tfs, ps) {
		return differ.getEffectiveType(path.Element())
	}

	return tfs.Type()
}

// makePlainPropDiff is used for plain properties and ones with an unknown schema.
// It does not access the TF schema, so it does not know about the type of the property.
func (differ detailedDiffer) makePlainPropDiff(
	path propertyPath, old, new resource.PropertyValue,
) map[detailedDiffKey]*pulumirpc.PropertyDiff {
	baseDiff := makeBaseDiff(old, new)
	isReplacement := willTriggerReplacement(path, differ.tfs, differ.ps)
	var propDiff *pulumirpc.PropertyDiff
	if baseDiff != undecidedDiff {
		propDiff = baseDiff.ToPropertyDiff()
	} else if !old.DeepEquals(new) {
		propDiff = &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_UPDATE}
	}

	if isReplacement {
		propDiff = promoteToReplace(propDiff)
	}

	if propDiff != nil {
		return map[detailedDiffKey]*pulumirpc.PropertyDiff{path.Key(): propDiff}
	}
	return nil
}

// makeShortCircuitDiff is used for properties that are nil or computed in either the old or new state.
// It makes sure to check recursively if the property will trigger a replacement.
func (differ detailedDiffer) makeShortCircuitDiff(
	path propertyPath, old, new resource.PropertyValue,
) map[detailedDiffKey]*pulumirpc.PropertyDiff {
	contract.Assertf(old.IsNull() || new.IsNull() || new.IsComputed(),
		"short-circuit diff should only be used for nil properties")
	if old.IsNull() && new.IsNull() {
		return nil
	}

	baseDiff := makeBaseDiff(old, new)
	contract.Assertf(baseDiff != undecidedDiff, "short-circuit diff could not determine diff kind")

	propDiff := baseDiff.ToPropertyDiff()
	if new.IsComputed() && willTriggerReplacement(path, differ.tfs, differ.ps) {
		propDiff = promoteToReplace(propDiff)
	} else if !new.IsNull() && !new.IsComputed() && willTriggerReplacementRecursive(path, new, differ.tfs, differ.ps) {
		propDiff = promoteToReplace(propDiff)
	} else if !old.IsNull() && willTriggerReplacementRecursive(path, old, differ.tfs, differ.ps) {
		propDiff = promoteToReplace(propDiff)
	}

	return map[detailedDiffKey]*pulumirpc.PropertyDiff{path.Key(): propDiff}
}

func (differ detailedDiffer) makePropDiff(
	path propertyPath, old, new resource.PropertyValue,
) map[detailedDiffKey]*pulumirpc.PropertyDiff {
	if path.IsReservedKey() {
		return nil
	}
	propType := differ.getEffectiveType(differ.propertyPathToSchemaPath(path))
	if !isPresent(old) || isTypeShapeMismatched(old, propType) {
		old = resource.NewNullProperty()
	}
	if !isPresent(new) || isTypeShapeMismatched(new, propType) && !new.IsComputed() {
		new = resource.NewNullProperty()
	}
	if old.IsNull() || new.IsNull() || new.IsComputed() {
		return differ.makeShortCircuitDiff(path, old, new)
	}

	switch propType {
	case shim.TypeList:
		return differ.makeListDiff(path, old, new)
	case shim.TypeSet:
		// TODO[pulumi/pulumi-terraform-bridge#2200]: Implement set diffing
		return differ.makeListDiff(path, old, new)
	case shim.TypeMap:
		// Note that TF objects are represented as maps when returned by LookupSchemas
		return differ.makeMapDiff(path, old, new)
	default:
		return differ.makePlainPropDiff(path, old, new)
	}
}

func (differ detailedDiffer) makeListDiff(
	path propertyPath, old, new resource.PropertyValue,
) map[detailedDiffKey]*pulumirpc.PropertyDiff {
	diff := make(map[detailedDiffKey]*pulumirpc.PropertyDiff)
	oldList := old.ArrayValue()
	newList := new.ArrayValue()

	// naive diffing of lists
	// TODO[pulumi/pulumi-terraform-bridge#2295]: implement a more sophisticated diffing algorithm
	// investigate how this interacts with force new - is identity preserved or just order
	longerLen := max(len(oldList), len(newList))
	for i := 0; i < longerLen; i++ {
		elem := func(l []resource.PropertyValue) resource.PropertyValue {
			if i < len(l) {
				return l[i]
			}
			return resource.NewNullProperty()
		}
		elemKey := path.Index(i)

		d := differ.makePropDiff(elemKey, elem(oldList), elem(newList))
		for subKey, subDiff := range d {
			diff[subKey] = subDiff
		}
	}

	return diff
}

func (differ detailedDiffer) makeMapDiff(
	path propertyPath, old, new resource.PropertyValue,
) map[detailedDiffKey]*pulumirpc.PropertyDiff {
	oldMap := old.ObjectValue()
	newMap := new.ObjectValue()
	diff := make(map[detailedDiffKey]*pulumirpc.PropertyDiff)
	for _, k := range sortedMergedKeys(oldMap, newMap) {
		subindex := path.Subpath(string(k))
		oldVal := oldMap[k]
		newVal := newMap[k]

		elemDiff := differ.makePropDiff(subindex, oldVal, newVal)

		for subKey, subDiff := range elemDiff {
			diff[subKey] = subDiff
		}
	}

	return diff
}

func (differ detailedDiffer) makeDetailedDiffPropertyMap(
	oldState, plannedState resource.PropertyMap,
) map[string]*pulumirpc.PropertyDiff {
	diff := make(map[detailedDiffKey]*pulumirpc.PropertyDiff)
	for _, k := range sortedMergedKeys(oldState, plannedState) {
		old := oldState[k]
		new := plannedState[k]

		path := newPropertyPath(k)
		propDiff := differ.makePropDiff(path, old, new)

		for subKey, subDiff := range propDiff {
			diff[subKey] = subDiff
		}
	}

	result := make(map[string]*pulumirpc.PropertyDiff)
	for k, v := range diff {
		result[string(k)] = v
	}

	return result
}

func makeDetailedDiffV2(
	ctx context.Context,
	tfs shim.SchemaMap,
	ps map[string]*SchemaInfo,
	res shim.Resource,
	prov shim.Provider,
	state shim.InstanceState,
	diff shim.InstanceDiff,
	assets AssetTable,
	supportsSecrets bool,
) (map[string]*pulumirpc.PropertyDiff, error) {
	// We need to compare the new and olds after all transformations have been applied.
	// ex. state upgrades, implementation-specific normalizations etc.
	proposedState, err := diff.ProposedState(res, state)
	if err != nil {
		return nil, err
	}
	props, err := MakeTerraformResult(ctx, prov, proposedState, tfs, ps, assets, supportsSecrets)
	if err != nil {
		return nil, err
	}

	prior, err := diff.PriorState()
	if err != nil {
		return nil, err
	}
	priorProps, err := MakeTerraformResult(ctx, prov, prior, tfs, ps, assets, supportsSecrets)
	if err != nil {
		return nil, err
	}

	differ := detailedDiffer{tfs: tfs, ps: ps}
	return differ.makeDetailedDiffPropertyMap(priorProps, props), nil
}
