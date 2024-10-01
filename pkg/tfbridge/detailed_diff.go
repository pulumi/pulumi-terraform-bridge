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

func mapHasReplacements(m map[detailedDiffKey]*pulumirpc.PropertyDiff) bool {
	for _, diff := range m {
		if diff.GetKind() == pulumirpc.PropertyDiff_ADD_REPLACE ||
			diff.GetKind() == pulumirpc.PropertyDiff_DELETE_REPLACE ||
			diff.GetKind() == pulumirpc.PropertyDiff_UPDATE_REPLACE {
			return true
		}
	}
	return false
}

type detailedDiffer struct {
	tfs shim.SchemaMap
	ps  map[string]*SchemaInfo
}

func (differ detailedDiffer) propertyPathToSchemaPath(path propertyPath) walk.SchemaPath {
	return PropertyPathToSchemaPath(resource.PropertyPath(path), differ.tfs, differ.ps)
}

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

func (differ detailedDiffer) lookupSchemas(path propertyPath) (shim.Schema, *info.Schema, error) {
	schemaPath := PropertyPathToSchemaPath(resource.PropertyPath(path), differ.tfs, differ.ps)
	return LookupSchemas(schemaPath, differ.tfs, differ.ps)
}

func (differ detailedDiffer) isForceNew(pair propertyPath) bool {
	// A change on a property might trigger a replacement if:
	// - The property itself is marked as ForceNew
	// - The direct parent property is a collection (list, set, map) and is marked as ForceNew
	// See pkg/cross-tests/diff_cross_test.go
	// TestAttributeCollectionForceNew, TestBlockCollectionForceNew, TestBlockCollectionElementForceNew
	// for a full case study of replacements in TF
	tfs, ps, err := differ.lookupSchemas(pair)
	if err != nil {
		return false
	}
	if isForceNew(tfs, ps) {
		return true
	}

	if len(pair) == 1 {
		return false
	}

	parent := pair[:len(pair)-1]
	tfs, ps, err = differ.lookupSchemas(parent)
	if err != nil {
		return false
	}
	// Note this is mimicking the TF behaviour, so the effective type is not considered here.
	if tfs.Type() != shim.TypeList && tfs.Type() != shim.TypeSet && tfs.Type() != shim.TypeMap {
		return false
	}
	return isForceNew(tfs, ps)
}

// We do not short-circuit detailed diffs when comparing non-nil properties against nil ones. The reason for that is
// that a replace might be triggered by a ForceNew inside a nested property of a non-ForceNew property. We instead
// always walk the full tree even when comparing against a nil property. We then later do a simplification step for
// the detailed diff in simplifyDiff in order to reduce the diff to what the user expects to see.
// See [pulumi/pulumi-terraform-bridge#2405] for more details.
func (differ detailedDiffer) simplifyDiff(
	diff map[detailedDiffKey]*pulumirpc.PropertyDiff, path propertyPath, old, new resource.PropertyValue,
) (map[detailedDiffKey]*pulumirpc.PropertyDiff, bool) {
	baseDiff := makeBaseDiff(old, new)
	if baseDiff == undecidedDiff {
		return nil, false
	}
	propDiff := baseDiff.ToPropertyDiff()
	if propDiff == nil {
		return nil, true
	}
	if differ.isForceNew(path) || mapHasReplacements(diff) {
		propDiff = promoteToReplace(propDiff)
	}
	return map[detailedDiffKey]*pulumirpc.PropertyDiff{path.Key(): propDiff}, true
}

func (differ detailedDiffer) makeTopPropDiff(
	path propertyPath, old, new resource.PropertyValue,
) map[detailedDiffKey]*pulumirpc.PropertyDiff {
	baseDiff := makeBaseDiff(old, new)
	isForceNew := differ.isForceNew(path)
	var propDiff *pulumirpc.PropertyDiff
	if baseDiff != undecidedDiff {
		propDiff = baseDiff.ToPropertyDiff()
	} else if !old.DeepEquals(new) {
		propDiff = &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_UPDATE}
	}

	if isForceNew {
		propDiff = promoteToReplace(propDiff)
	}

	if propDiff != nil {
		return map[detailedDiffKey]*pulumirpc.PropertyDiff{path.Key(): propDiff}
	}
	return nil
}

func (differ detailedDiffer) makePropDiff(
	path propertyPath, old, new resource.PropertyValue,
) map[detailedDiffKey]*pulumirpc.PropertyDiff {
	if path.IsReservedKey() {
		return nil
	}
	propType := differ.getEffectiveType(differ.propertyPathToSchemaPath(path))

	switch propType {
	case shim.TypeList:
		return differ.makeListDiff(path, old, new)
	case shim.TypeSet:
		// TODO[pulumi/pulumi-terraform-bridge#2200]: Implement set diffing
		return differ.makeListDiff(path, old, new)
	case shim.TypeMap:
		// Note that TF objects are represented as maps in the shim layer.
		return differ.makeMapDiff(path, old, new)
	default:
		return differ.makeTopPropDiff(path, old, new)
	}
}

func (differ detailedDiffer) makeListDiff(
	path propertyPath, old, new resource.PropertyValue,
) map[detailedDiffKey]*pulumirpc.PropertyDiff {
	diff := make(map[detailedDiffKey]*pulumirpc.PropertyDiff)
	oldList := []resource.PropertyValue{}
	newList := []resource.PropertyValue{}
	if isPresent(old) && old.IsArray() {
		oldList = old.ArrayValue()
	}
	if isPresent(new) && new.IsArray() {
		newList = new.ArrayValue()
	}

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

	simplerDiff, isSimplified := differ.simplifyDiff(diff, path, old, new)
	if isSimplified {
		return simplerDiff
	}

	return diff
}

func (differ detailedDiffer) makeMapDiff(
	path propertyPath, old, new resource.PropertyValue,
) map[detailedDiffKey]*pulumirpc.PropertyDiff {
	diff := make(map[detailedDiffKey]*pulumirpc.PropertyDiff)
	oldMap := resource.PropertyMap{}
	newMap := resource.PropertyMap{}
	if isPresent(old) && old.IsObject() {
		oldMap = old.ObjectValue()
	}
	if isPresent(new) && new.IsObject() {
		newMap = new.ObjectValue()
	}

	for _, k := range sortedMergedKeys(oldMap, newMap) {
		subindex := path.Subpath(string(k))
		oldVal := oldMap[k]
		newVal := newMap[k]

		elemDiff := differ.makePropDiff(subindex, oldVal, newVal)

		for subKey, subDiff := range elemDiff {
			diff[subKey] = subDiff
		}
	}

	simplerDiff, isSimplified := differ.simplifyDiff(diff, path, old, new)
	if isSimplified {
		return simplerDiff
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
