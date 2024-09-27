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

func isObject(tfs shim.Schema, ps *SchemaInfo) bool {
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

type (
	detailedDiffKey  string
	detailedDiffPair struct {
		key  detailedDiffKey
		path resource.PropertyPath
	}
)

func newDetailedDiffPair(root resource.PropertyKey) detailedDiffPair {
	rootString := string(root)
	return detailedDiffPair{
		key:  detailedDiffKey(rootString),
		path: resource.PropertyPath{rootString},
	}
}

func (k detailedDiffPair) String() string {
	return string(k.key)
}

func (k detailedDiffPair) append(subkey interface{}) detailedDiffPair {
	subpath := append(k.path, subkey)
	return detailedDiffPair{
		key:  detailedDiffKey(subpath.String()),
		path: subpath,
	}
}

func (k detailedDiffPair) SubKey(subkey string) detailedDiffPair {
	return k.append(subkey)
}

func (k detailedDiffPair) Index(i int) detailedDiffPair {
	return k.append(i)
}

type baseDiff string

const (
	NoDiff    baseDiff = "NoDiff"
	Add       baseDiff = "Add"
	Delete    baseDiff = "Delete"
	Update    baseDiff = "Update"
	Undecided baseDiff = "Undecided"
)

func isPresent(val resource.PropertyValue) bool {
	return !val.IsNull() &&
		!(val.IsArray() && val.ArrayValue() == nil) &&
		!(val.IsObject() && val.ObjectValue() == nil)
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

func baseDiffToPropertyDiff(diff baseDiff) *pulumirpc.PropertyDiff {
	contract.Assertf(diff != Undecided, "diff should not be undecided")
	switch diff {
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

func isForceNew(tfs shim.Schema, ps *SchemaInfo) bool {
	if tfs != nil && tfs.ForceNew() {
		return true
	}
	if ps != nil && ps.ForceNew != nil && *ps.ForceNew {
		return true
	}
	return false
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

func (differ detailedDiffer) makeTopPropDiff(
	old, new resource.PropertyValue,
	ddIndex detailedDiffPair,
) *pulumirpc.PropertyDiff {
	baseDiff := makeBaseDiff(old, new)
	isForceNew := differ.isForceNew(ddIndex)
	if baseDiff != Undecided {
		propDiff := baseDiffToPropertyDiff(baseDiff)
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

func (differ detailedDiffer) lookupSchemas(path resource.PropertyPath) (shim.Schema, *info.Schema, error) {
	schemaPath := PropertyPathToSchemaPath(path, differ.tfs, differ.ps)
	return LookupSchemas(schemaPath, differ.tfs, differ.ps)
}

func (differ detailedDiffer) isForceNew(pair detailedDiffPair) bool {
	// See pkg/cross-tests/diff_cross_test.go
	// TestAttributeCollectionForceNew, TestBlockCollectionForceNew, TestBlockCollectionElementForceNew
	// for a full case study of replacements in TF
	tfs, ps, err := differ.lookupSchemas(pair.path)
	if err != nil {
		return false
	}
	if isForceNew(tfs, ps) {
		return true
	}

	if len(pair.path) == 1 {
		return false
	}

	parent := pair.path[:len(pair.path)-1]
	tfs, ps, err = differ.lookupSchemas(parent)
	if err != nil {
		return false
	}
	if tfs.Type() != shim.TypeList && tfs.Type() != shim.TypeSet {
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
	diff map[detailedDiffKey]*pulumirpc.PropertyDiff, ddIndex detailedDiffPair, old, new resource.PropertyValue,
) (map[detailedDiffKey]*pulumirpc.PropertyDiff, error) {
	baseDiff := makeBaseDiff(old, new)
	if baseDiff != Undecided {
		propDiff := baseDiffToPropertyDiff(baseDiff)
		if propDiff == nil {
			return nil, nil
		}
		if differ.isForceNew(ddIndex) || mapHasReplacements(diff) {
			propDiff = promoteToReplace(propDiff)
		}
		return map[detailedDiffKey]*pulumirpc.PropertyDiff{ddIndex.key: propDiff}, nil
	}
	return nil, errors.New("diff is not simplified")
}

func (differ detailedDiffer) makePropDiff(
	ctx context.Context,
	ddIndex detailedDiffPair,
	old, new resource.PropertyValue,
) map[detailedDiffKey]*pulumirpc.PropertyDiff {
	result := make(map[detailedDiffKey]*pulumirpc.PropertyDiff)
	tfs, ps, err := differ.lookupSchemas(ddIndex.path)
	if err != nil || tfs == nil {
		// If the schema is nil, we just return the top-level diff
		topDiff := differ.makeTopPropDiff(old, new, ddIndex)
		if topDiff == nil {
			return nil
		}
		result[ddIndex.key] = topDiff
		return result
	}

	if isObject(tfs, ps) {
		diff := differ.makeObjectDiff(ctx, ddIndex, old, new)
		for subKey, subDiff := range diff {
			result[subKey] = subDiff
		}
	} else if tfs.Type() == shim.TypeMap {
		diff := differ.makeMapDiff(ctx, ddIndex, old, new)
		for subKey, subDiff := range diff {
			result[subKey] = subDiff
		}
	} else if tfs.Type() == shim.TypeList {
		diff := differ.makeListDiff(ctx, ddIndex, old, new)
		for subKey, subDiff := range diff {
			result[subKey] = subDiff
		}
	} else if tfs.Type() == shim.TypeSet {
		// TODO[pulumi/pulumi-terraform-bridge#2200]: Implement set diffing
		diff := differ.makeListDiff(ctx, ddIndex, old, new)
		for subKey, subDiff := range diff {
			result[subKey] = subDiff
		}
	} else {
		topDiff := differ.makeTopPropDiff(old, new, ddIndex)
		if topDiff == nil {
			return nil
		}
		result[ddIndex.key] = topDiff
	}

	return result
}

func (differ detailedDiffer) makeObjectDiff(
	ctx context.Context,
	ddIndex detailedDiffPair,
	old, new resource.PropertyValue,
) map[detailedDiffKey]*pulumirpc.PropertyDiff {
	diff := make(map[detailedDiffKey]*pulumirpc.PropertyDiff)
	oldObj := resource.PropertyMap{}
	newObj := resource.PropertyMap{}

	if isPresent(old) && old.IsObject() {
		oldObj = old.ObjectValue()
	}
	if isPresent(new) && new.IsObject() {
		newObj = new.ObjectValue()
	}

	for _, k := range sortedMergedKeys(oldObj, newObj) {
		subindex := ddIndex.SubKey(string(k))
		oldVal := oldObj[k]
		newVal := newObj[k]
		propDiff := differ.makePropDiff(ctx, subindex, oldVal, newVal)

		for subKey, subDiff := range propDiff {
			diff[subKey] = subDiff
		}
	}

	simplerDiff, err := differ.simplifyDiff(diff, ddIndex, old, new)
	if err == nil {
		return simplerDiff
	}

	return diff
}

func (differ detailedDiffer) makeListDiff(
	ctx context.Context,
	ddIndex detailedDiffPair,
	old, new resource.PropertyValue,
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
		elemKey := ddIndex.Index(i)
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

		d := differ.makePropDiff(
			ctx, elemKey, oldVal, newVal)
		for subKey, subDiff := range d {
			diff[subKey] = subDiff
		}
	}

	simplerDiff, err := differ.simplifyDiff(diff, ddIndex, old, new)
	if err == nil {
		return simplerDiff
	}

	return diff
}

func (differ detailedDiffer) makeMapDiff(
	ctx context.Context,
	ddIndex detailedDiffPair,
	old, new resource.PropertyValue,
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
		subindex := ddIndex.SubKey(string(k))
		oldVal := oldMap[k]
		newVal := newMap[k]

		elemDiff := differ.makePropDiff(ctx, subindex, oldVal, newVal)

		for subKey, subDiff := range elemDiff {
			diff[subKey] = subDiff
		}
	}

	simplerDiff, err := differ.simplifyDiff(diff, ddIndex, old, new)
	if err == nil {
		return simplerDiff
	}

	return diff
}

func (differ detailedDiffer) makeDetailedDiffPropertyMap(
	ctx context.Context, oldState, plannedState resource.PropertyMap,
) map[string]*pulumirpc.PropertyDiff {
	diff := make(map[detailedDiffKey]*pulumirpc.PropertyDiff)
	for _, k := range sortedMergedKeys(oldState, plannedState) {
		old := oldState[k]
		new := plannedState[k]

		ddIndex := newDetailedDiffPair(k)
		propDiff := differ.makePropDiff(ctx, ddIndex, old, new)

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
	props, err := MakeTerraformResult(
		ctx, prov, proposedState, tfs, ps, assets, supportsSecrets)
	if err != nil {
		return nil, err
	}

	prior, err := diff.PriorState()
	if err != nil {
		return nil, err
	}
	priorProps, err := MakeTerraformResult(
		ctx, prov, prior, tfs, ps, assets, supportsSecrets)
	if err != nil {
		return nil, err
	}

	differ := detailedDiffer{tfs: tfs, ps: ps}
	return differ.makeDetailedDiffPropertyMap(ctx, priorProps, props), nil
}
