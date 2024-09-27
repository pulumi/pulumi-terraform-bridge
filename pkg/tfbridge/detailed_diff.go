package tfbridge

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

func isFlattened(s shim.Schema, ps *SchemaInfo) bool {
	if s.Type() != shim.TypeList && s.Type() != shim.TypeSet {
		return false
	}

	if ps != nil && ps.MaxItemsOne != nil {
		return *ps.MaxItemsOne
	}

	return s.MaxItems() == 1
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

type detailedDiffKey string

func (k detailedDiffKey) String() string {
	return string(k)
}

func (k detailedDiffKey) SubKey(subkey string) detailedDiffKey {
	if k == "" {
		return detailedDiffKey(subkey)
	}
	if strings.ContainsAny(subkey, `."[]`) {
		return detailedDiffKey(fmt.Sprintf(`%s["%s"]`, k, strings.ReplaceAll(subkey, `"`, `\"`)))
	}
	return detailedDiffKey(fmt.Sprintf(`%s.%s`, k, subkey))
}

func (k detailedDiffKey) Index(i int) detailedDiffKey {
	return detailedDiffKey(fmt.Sprintf("%s[%d]", k, i))
}

type baseDiff string

const (
	NoDiff    baseDiff = "NoDiff"
	Add       baseDiff = "Add"
	Delete    baseDiff = "Delete"
	Update    baseDiff = "Update"
	Undecided baseDiff = "Undecided"
)

func isPresent(val resource.PropertyValue, valOk bool) bool {
	return valOk &&
		!val.IsNull() &&
		!(val.IsArray() && val.ArrayValue() == nil) &&
		!(val.IsObject() && val.ObjectValue() == nil)
}

func makeBaseDiff(old, new resource.PropertyValue, oldOk, newOk bool) baseDiff {
	oldPresent := isPresent(old, oldOk)
	newPresent := isPresent(new, newOk)
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

func baseDiffToPropertyDiff(diff baseDiff, tfs shim.Schema, ps *SchemaInfo) *pulumirpc.PropertyDiff {
	contract.Assertf(diff != Undecided, "diff should not be undecided")
	switch diff {
	case Add:
		result := &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_ADD}
		return propertyDiffResult(tfs, ps, result)
	case Delete:
		result := &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_DELETE}
		return propertyDiffResult(tfs, ps, result)
	case Update:
		result := &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_UPDATE}
		return propertyDiffResult(tfs, ps, result)
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

func mapHasReplacements(m map[detailedDiffKey]*pulumirpc.PropertyDiff ) bool {
	for _, diff := range m {
		if diff.GetKind() == pulumirpc.PropertyDiff_ADD_REPLACE ||
			diff.GetKind() == pulumirpc.PropertyDiff_DELETE_REPLACE ||
			diff.GetKind() == pulumirpc.PropertyDiff_UPDATE_REPLACE {
			return true
		}
	}
	return false
}

// We do not short-circuit detailed diffs when comparing non-nil properties against nil ones. The reason for that is
// that a replace might be triggered by a ForceNew inside a nested property of a non-ForceNew property. We instead
// always walk the full tree even when comparing against a nil property. We then later do a simplification step for
// the detailed diff in simplifyDiff in order to reduce the diff to what the user expects to see.
// See [pulumi/pulumi-terraform-bridge#2405] for more details.
func simplifyDiff(
	diff map[detailedDiffKey]*pulumirpc.PropertyDiff , key detailedDiffKey, old, new resource.PropertyValue,
	oldOk, newOk bool, tfs interface{}, ps *SchemaInfo,
) (map[detailedDiffKey]*pulumirpc.PropertyDiff , error) {
	baseDiff := makeBaseDiff(old, new, oldOk, newOk)
	if baseDiff != Undecided {
		tfs, ok := tfs.(shim.Schema)
		if !ok {
			tfs = nil
		}
		propDiff := baseDiffToPropertyDiff(baseDiff, tfs, ps)
		if propDiff == nil {
			return nil, nil
		}
		if mapHasReplacements(diff) {
			propDiff = promoteToReplace(propDiff)
		}
		return map[detailedDiffKey]*pulumirpc.PropertyDiff {key: propDiff}, nil
	}
	return nil, errors.New("diff is not simplified")
}

func propertyDiffResult(tfs shim.Schema, ps *SchemaInfo, diff *pulumirpc.PropertyDiff) *pulumirpc.PropertyDiff {
	// See pkg/cross-tests/diff_cross_test.go
	// TestAttributeCollectionForceNew, TestBlockCollectionForceNew, TestBlockCollectionElementForceNew
	// for a full case study of replacements in TF
	if isForceNew(tfs, ps) {
		return promoteToReplace(diff)
	}
	return diff
}

func makeTopPropDiff(
	old, new resource.PropertyValue,
	oldOk, newOk bool,
	tfs shim.Schema,
	ps *SchemaInfo,
) *pulumirpc.PropertyDiff {
	baseDiff := makeBaseDiff(old, new, oldOk, newOk)
	if baseDiff != Undecided {
		return baseDiffToPropertyDiff(baseDiff, tfs, ps)
	}

	if !old.DeepEquals(new) {
		return propertyDiffResult(tfs, ps, &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_UPDATE})
	}
	return nil
}

func makePropDiff(
	ctx context.Context,
	key detailedDiffKey,
	tfs shim.Schema,
	ps *SchemaInfo,
	old, new resource.PropertyValue,
	oldOk, newOk bool,
) map[detailedDiffKey]*pulumirpc.PropertyDiff {
	res := make(map[detailedDiffKey]*pulumirpc.PropertyDiff)

	if tfs == nil {
		// If the schema is nil, we just return the top-level diff
		topDiff := makeTopPropDiff(old, new, oldOk, newOk, tfs, ps)
		if topDiff == nil {
			return nil
		}
		res[key] = topDiff
		return res
	}

	if isFlattened(tfs, ps) {
		pelem := &info.Schema{}
		if ps != nil {
			pelem = ps.Elem
		}
		collectionForceNew := isForceNew(tfs, ps)
		diff := makeElemDiff(ctx, key, tfs.Elem(), pelem, collectionForceNew, old, new, oldOk, newOk)
		for subKey, subDiff := range diff {
			res[subKey] = subDiff
		}
	} else if tfs.Type() == shim.TypeList {
		diff := makeListDiff(ctx, key, tfs, ps, old, new, oldOk, newOk)
		for subKey, subDiff := range diff {
			res[subKey] = subDiff
		}
	} else if tfs.Type() == shim.TypeMap {
		diff := makeMapDiff(ctx, key, tfs, ps, old, new, oldOk, newOk)
		for subKey, subDiff := range diff {
			res[subKey] = subDiff
		}
	} else if tfs.Type() == shim.TypeSet {
		// TODO[pulumi/pulumi-terraform-bridge#2200]: Implement set diffing
		diff := makeListDiff(ctx, key, tfs, ps, old, new, oldOk, newOk)
		for subKey, subDiff := range diff {
			res[subKey] = subDiff
		}
	} else {
		topDiff := makeTopPropDiff(old, new, oldOk, newOk, tfs, ps)
		if topDiff == nil {
			return nil
		}
		res[key] = topDiff
	}

	return res
}

func makeObjectDiff(
	ctx context.Context,
	key detailedDiffKey,
	tfs shim.SchemaMap,
	ps map[string]*SchemaInfo,
	old, new resource.PropertyValue,
	oldOk, newOk bool,
) map[detailedDiffKey]*pulumirpc.PropertyDiff {
	diff := make(map[detailedDiffKey]*pulumirpc.PropertyDiff )
	oldObj := resource.PropertyMap{}
	newObj := resource.PropertyMap{}
	if isPresent(old, oldOk) && old.IsObject() {
		oldObj = old.ObjectValue()
	}
	if isPresent(new, newOk) && new.IsObject() {
		newObj = new.ObjectValue()
	}

	for _, k := range sortedMergedKeys(oldObj, newObj) {
		subkey := key.SubKey(string(k))
		oldVal, oldOk := oldObj[k]
		newVal, newOk := newObj[k]
		_, etfs, eps := getInfoFromPulumiName(k, tfs, ps)

		propDiff := makePropDiff(ctx, subkey, etfs, eps, oldVal, newVal, oldOk, newOk)

		for subKey, subDiff := range propDiff {
			diff[subKey] = subDiff
		}
	}

	simplerDiff, err := simplifyDiff(diff, key, old, new, oldOk, newOk, tfs, nil)
	if err == nil {
		return simplerDiff
	}

	return diff
}

func makeElemDiff(
	ctx context.Context,
	key detailedDiffKey,
	tfs interface{},
	ps *SchemaInfo,
	parentForceNew bool,
	old, new resource.PropertyValue,
	oldOk, newOk bool,
) map[detailedDiffKey]*pulumirpc.PropertyDiff {
	diff := make(map[detailedDiffKey]*pulumirpc.PropertyDiff )
	if _, ok := tfs.(shim.Resource); ok {
		eps := map[string]*SchemaInfo{}
		if ps != nil {
			eps = ps.Fields
		}
		etfs := tfs.(shim.Resource).Schema()
		d := makeObjectDiff(
			ctx,
			key,
			etfs,
			eps,
			old,
			new,
			oldOk,
			newOk,
		)
		for subKey, subDiff := range d {
			diff[subKey] = subDiff
		}
	} else if _, ok := tfs.(shim.Schema); ok {
		d := makePropDiff(
			ctx,
			key,
			tfs.(shim.Schema),
			ps,
			old,
			new,
			oldOk,
			newOk,
		)
		for subKey, subDiff := range d {
			if parentForceNew {
				subDiff = promoteToReplace(subDiff)
			}
			diff[subKey] = subDiff
		}
	} else {
		d := makePropDiff(ctx, key, nil, ps.Elem, old, new, oldOk, newOk)
		for subKey, subDiff := range d {
			if parentForceNew {
				subDiff = promoteToReplace(subDiff)
			}
			diff[subKey] = subDiff
		}
	}

	simplerDiff, err := simplifyDiff(diff, key, old, new, oldOk, newOk, tfs, ps)
	if err == nil {
		return simplerDiff
	}

	return diff
}

func makeListDiff(
	ctx context.Context,
	key detailedDiffKey,
	tfs shim.Schema,
	ps *info.Schema,
	old, new resource.PropertyValue,
	oldOk, newOk bool,
) map[detailedDiffKey]*pulumirpc.PropertyDiff {
	diff := make(map[detailedDiffKey]*pulumirpc.PropertyDiff )
	oldList := []resource.PropertyValue{}
	newList := []resource.PropertyValue{}
	if isPresent(old, oldOk) && old.IsArray() {
		oldList = old.ArrayValue()
	}
	if isPresent(new, newOk) && new.IsArray() {
		newList = new.ArrayValue()
	}

	// naive diffing of lists
	// TODO[pulumi/pulumi-terraform-bridge#2295]: implement a more sophisticated diffing algorithm
	// investigate how this interacts with force new - is identity preserved or just order
	collectionForceNew := isForceNew(tfs, ps)
	longerLen := max(len(oldList), len(newList))
	for i := 0; i < longerLen; i++ {
		elemKey := key.Index(i)
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

		d := makeElemDiff(
			ctx, elemKey, tfs.Elem(), ps, collectionForceNew, oldVal, newVal, oldOk, newOk)
		for subKey, subDiff := range d {
			diff[subKey] = subDiff
		}
	}

	simplerDiff, err := simplifyDiff(diff, key, old, new, oldOk, newOk, tfs, ps)
	if err == nil {
		return simplerDiff
	}

	return diff
}

func makeMapDiff(
	ctx context.Context,
	key detailedDiffKey,
	tfs shim.Schema,
	ps *SchemaInfo,
	old, new resource.PropertyValue,
	oldOk, newOk bool,
) map[detailedDiffKey]*pulumirpc.PropertyDiff {
	diff := make(map[detailedDiffKey]*pulumirpc.PropertyDiff )
	oldMap := resource.PropertyMap{}
	newMap := resource.PropertyMap{}
	if isPresent(old, oldOk) && old.IsObject() {
		oldMap = old.ObjectValue()
	}
	if isPresent(new, newOk) && new.IsObject() {
		newMap = new.ObjectValue()
	}

	collectionForceNew := isForceNew(tfs, ps)
	for _, k := range sortedMergedKeys(oldMap, newMap) {
		subkey := key.SubKey(string(k))
		oldVal, oldOk := oldMap[k]
		newVal, newOk := newMap[k]

		pelem := &info.Schema{}
		if ps != nil {
			pelem = ps.Elem
		}
		elemDiff := makeElemDiff(ctx, subkey, tfs.Elem(), pelem, collectionForceNew, oldVal, newVal, oldOk, newOk)

		for subKey, subDiff := range elemDiff {
			diff[subKey] = subDiff
		}
	}

	simplerDiff, err := simplifyDiff(diff, key, old, new, oldOk, newOk, tfs, ps)
	if err == nil {
		return simplerDiff
	}

	return diff
}

func makePulumiDetailedDiffV2(
	ctx context.Context,
	tfs shim.SchemaMap,
	ps map[string]*SchemaInfo,
	oldState, plannedState resource.PropertyMap,
) map[string]*pulumirpc.PropertyDiff {
	diff := make(map[detailedDiffKey]*pulumirpc.PropertyDiff )
	for _, k := range sortedMergedKeys(oldState, plannedState) {
		old, oldOk := oldState[k]
		new, newOk := plannedState[k]
		_, etfs, eps := getInfoFromPulumiName(k, tfs, ps)

		key := detailedDiffKey(k)
		propDiff := makePropDiff(ctx, key, etfs, eps, old, new, oldOk, newOk)

		for subKey, subDiff := range propDiff {
			diff[subKey] = subDiff
		}
	}

	result := make(map[string]*pulumirpc.PropertyDiff)
	for k, v := range diff {
		result[k.String()] = v
	}

	return result
}
