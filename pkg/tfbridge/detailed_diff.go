package tfbridge

import (
	"context"
	"errors"
	"slices"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

func isPresent(val resource.PropertyValue) bool {
	return !val.IsNull() &&
		!(val.IsArray() && val.ArrayValue() == nil) &&
		!(val.IsObject() && val.ObjectValue() == nil)
}

func isForceNew(tfs shim.Schema, ps *SchemaInfo) bool {
	if tfs != nil && tfs.ForceNew() {
		return true
	}
	if ps != nil && ps.ForceNew != nil && *ps.ForceNew {
		return true
	}
	return false
}

func isObject(tfs shim.Schema, ps *SchemaInfo) bool {
	if tfs.Type() == shim.TypeMap {
		return true
	}

	if tfs.Type() != shim.TypeList && tfs.Type() != shim.TypeSet {
		return false
	}

	if ps != nil && ps.MaxItemsOne != nil {
		return *ps.MaxItemsOne
	}

	return tfs.MaxItems() == 1
}

func sortedMergedKeys(a resource.PropertyMap, b resource.PropertyMap) []resource.PropertyKey {
	keys := make(map[resource.PropertyKey]struct{})
	for k := range a {
		keys[k] = struct{}{}
	}
	for k := range b {
		keys[k] = struct{}{}
	}
	keysSlice := make([]resource.PropertyKey, 0, len(keys))
	for k := range keys {
		keysSlice = append(keysSlice, k)
	}
	slices.Sort(keysSlice)
	return keysSlice
}

func promoteToReplace(diff *pulumirpc.PropertyDiff) *pulumirpc.PropertyDiff {
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
	NoDiff    baseDiff = "NoDiff"
	Add       baseDiff = "Add"
	Delete    baseDiff = "Delete"
	Update    baseDiff = "Update"
	Undecided baseDiff = "Undecided"
)

func (b baseDiff) ToPropertyDiff() *pulumirpc.PropertyDiff {
	contract.Assertf(b != Undecided, "diff should not be undecided")
	switch b {
	case Add:
		return &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_ADD}
	case Delete:
		return &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_DELETE}
	case Update:
		return &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_UPDATE}
	default:
		return nil
	}
}

func makeBaseDiff(old, new resource.PropertyValue) baseDiff {
	oldPresent := isPresent(old)
	newPresent := isPresent(new)
	if !oldPresent {
		if !newPresent {
			return NoDiff
		}

		return Add
	}
	if !newPresent {
		return Delete
	}

	if new.IsComputed() {
		return Update
	}

	return Undecided
}

type (
	detailedDiffKey string
	propertyPath    resource.PropertyPath
)

func (k propertyPath) String() string {
	return resource.PropertyPath(k).String()
}

func (k propertyPath) Key() detailedDiffKey {
	return detailedDiffKey(k.String())
}

func (k propertyPath) append(subkey interface{}) propertyPath {
	return append(k, subkey)
}

func (k propertyPath) SubKey(subkey string) propertyPath {
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

func (differ detailedDiffer) lookupSchemas(path propertyPath) (shim.Schema, *info.Schema, error) {
	schemaPath := PropertyPathToSchemaPath(resource.PropertyPath(path), differ.tfs, differ.ps)
	return LookupSchemas(schemaPath, differ.tfs, differ.ps)
}

func (differ detailedDiffer) isForceNew(pair propertyPath) bool {
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
) (map[detailedDiffKey]*pulumirpc.PropertyDiff, error) {
	baseDiff := makeBaseDiff(old, new)
	if baseDiff != Undecided {
		propDiff := baseDiff.ToPropertyDiff()
		if propDiff == nil {
			return nil, nil
		}
		if differ.isForceNew(path) || mapHasReplacements(diff) {
			propDiff = promoteToReplace(propDiff)
		}
		return map[detailedDiffKey]*pulumirpc.PropertyDiff{path.Key(): propDiff}, nil
	}
	return nil, errors.New("diff is not simplified")
}

func (differ detailedDiffer) makeTopPropDiff(
	old, new resource.PropertyValue, path propertyPath,
) *pulumirpc.PropertyDiff {
	baseDiff := makeBaseDiff(old, new)
	isForceNew := differ.isForceNew(path)
	if baseDiff != Undecided {
		propDiff := baseDiff.ToPropertyDiff()
		if isForceNew {
			propDiff = promoteToReplace(propDiff)
		}
		return propDiff
	}

	if !old.DeepEquals(new) {
		diff := &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_UPDATE}
		if isForceNew {
			return promoteToReplace(diff)
		}
		return diff
	}
	return nil
}

func (differ detailedDiffer) makePropDiff(
	path propertyPath, old, new resource.PropertyValue,
) map[detailedDiffKey]*pulumirpc.PropertyDiff {
	if path.IsReservedKey() {
		return nil
	}

	result := make(map[detailedDiffKey]*pulumirpc.PropertyDiff)
	tfs, ps, err := differ.lookupSchemas(path)
	if err != nil || tfs == nil {
		// If the schema is nil, we just return the top-level diff
		topDiff := differ.makeTopPropDiff(old, new, path)
		if topDiff == nil {
			return nil
		}
		result[path.Key()] = topDiff
		return result
	}

	if isObject(tfs, ps) {
		diff := differ.makeObjectDiff(path, old, new)
		for subKey, subDiff := range diff {
			result[subKey] = subDiff
		}
	} else if tfs.Type() == shim.TypeList {
		diff := differ.makeListDiff(path, old, new)
		for subKey, subDiff := range diff {
			result[subKey] = subDiff
		}
	} else if tfs.Type() == shim.TypeSet {
		// TODO[pulumi/pulumi-terraform-bridge#2200]: Implement set diffing
		diff := differ.makeListDiff(path, old, new)
		for subKey, subDiff := range diff {
			result[subKey] = subDiff
		}
	} else {
		topDiff := differ.makeTopPropDiff(old, new, path)
		if topDiff == nil {
			return nil
		}
		result[path.Key()] = topDiff
	}

	return result
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
		elemKey := path.Index(i)
		oldOk := i < len(oldList)
		oldVal := resource.NewNullProperty()
		if oldOk {
			oldVal = oldList[i]
		}
		newOk := i < len(newList)
		newVal := resource.NewNullProperty()
		if newOk {
			newVal = newList[i]
		}

		d := differ.makePropDiff(elemKey, oldVal, newVal)
		for subKey, subDiff := range d {
			diff[subKey] = subDiff
		}
	}

	simplerDiff, err := differ.simplifyDiff(diff, path, old, new)
	if err == nil {
		return simplerDiff
	}

	return diff
}

func (differ detailedDiffer) makeObjectDiff(
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
		subindex := path.SubKey(string(k))
		oldVal := oldMap[k]
		newVal := newMap[k]

		elemDiff := differ.makePropDiff(subindex, oldVal, newVal)

		for subKey, subDiff := range elemDiff {
			diff[subKey] = subDiff
		}
	}

	simplerDiff, err := differ.simplifyDiff(diff, path, old, new)
	if err == nil {
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

		path := propertyPath{k}
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
