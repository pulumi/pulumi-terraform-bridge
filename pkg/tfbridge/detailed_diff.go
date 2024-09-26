package tfbridge

import (
	"context"
	"errors"
	"fmt"
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

func isDunder(k resource.PropertyKey) bool {
	return len(k) > 1 && k[0] == '_' && k[1] == '_'
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

func getSubPath(key, subkey resource.PropertyKey) resource.PropertyKey {
	if key == "" {
		return subkey
	}
	if strings.ContainsAny(string(subkey), `."[]`) {
		return resource.PropertyKey(fmt.Sprintf(`%s["%s"]`, key, strings.ReplaceAll(string(subkey), `"`, `\"`)))
	}
	return resource.PropertyKey(string(key) + "." + string(subkey))
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

func baseDiffToPropertyDiff(diff baseDiff, etf shim.Schema, eps *SchemaInfo) *pulumirpc.PropertyDiff {
	contract.Assertf(diff != Undecided, "diff should not be undecided")
	switch diff {
	case Add:
		res := &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_ADD}
		return propertyDiffResult(etf, eps, res)
	case Delete:
		res := &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_DELETE}
		return propertyDiffResult(etf, eps, res)
	case Update:
		res := &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_UPDATE}
		return propertyDiffResult(etf, eps, res)
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

func isForceNew(etf shim.Schema, eps *SchemaInfo) bool {
	if etf != nil && etf.ForceNew() {
		return true
	}
	if eps != nil && eps.ForceNew != nil && *eps.ForceNew {
		return true
	}
	return false
}

func mapHasReplacements(m map[string]*pulumirpc.PropertyDiff) bool {
	for _, diff := range m {
		if diff.GetKind() == pulumirpc.PropertyDiff_ADD_REPLACE ||
			diff.GetKind() == pulumirpc.PropertyDiff_DELETE_REPLACE ||
			diff.GetKind() == pulumirpc.PropertyDiff_UPDATE_REPLACE {
			return true
		}
	}
	return false
}

func simplifyDiff(
	diff map[string]*pulumirpc.PropertyDiff, key resource.PropertyKey, old, new resource.PropertyValue,
	oldOk, newOk bool, etf interface{}, eps *SchemaInfo,
) (map[string]*pulumirpc.PropertyDiff, error) {
	baseDiff := makeBaseDiff(old, new, oldOk, newOk)
	if baseDiff != Undecided {
		etf, ok := etf.(shim.Schema)
		if !ok {
			etf = nil
		}
		propDiff := baseDiffToPropertyDiff(baseDiff, etf, eps)
		if propDiff == nil {
			return nil, nil
		}
		if mapHasReplacements(diff) {
			propDiff = promoteToReplace(propDiff)
		}
		return map[string]*pulumirpc.PropertyDiff{string(key): propDiff}, nil
	}
	return nil, errors.New("diff is not simplified")
}

func propertyDiffResult(etf shim.Schema, eps *SchemaInfo, diff *pulumirpc.PropertyDiff) *pulumirpc.PropertyDiff {
	// See pkg/cross-tests/diff_cross_test.go
	// TestAttributeCollectionForceNew, TestBlockCollectionForceNew, TestBlockCollectionElementForceNew
	// for a full case study of replacements in TF
	if isForceNew(etf, eps) {
		return promoteToReplace(diff)
	}
	return diff
}

func makeTopPropDiff(
	old, new resource.PropertyValue,
	oldOk, newOk bool,
	etf shim.Schema,
	eps *SchemaInfo,
) *pulumirpc.PropertyDiff {
	baseDiff := makeBaseDiff(old, new, oldOk, newOk)
	if baseDiff != Undecided {
		return baseDiffToPropertyDiff(baseDiff, etf, eps)
	}

	if !old.DeepEquals(new) {
		return propertyDiffResult(etf, eps, &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_UPDATE})
	}
	return nil
}

func makePropDiff(
	ctx context.Context,
	key resource.PropertyKey,
	etf shim.Schema,
	eps *SchemaInfo,
	old, new resource.PropertyValue,
	oldOk, newOk bool,
) map[string]*pulumirpc.PropertyDiff {
	// ignore dunder properties - these are internal to Pulumi and should not be surfaced in the diff
	if isDunder(key) {
		return nil
	}
	res := make(map[string]*pulumirpc.PropertyDiff)

	if etf == nil {
		// If the schema is nil, we just return the top-level diff
		topDiff := makeTopPropDiff(old, new, oldOk, newOk, etf, eps)
		if topDiff == nil {
			return nil
		}
		res[string(key)] = topDiff
		return res
	}

	if isFlattened(etf, eps) {
		pelem := &info.Schema{}
		if eps != nil {
			pelem = eps.Elem
		}
		collectionForceNew := isForceNew(etf, eps)
		diff := makeElemDiff(ctx, key, etf.Elem(), pelem, collectionForceNew, old, new, oldOk, newOk)
		for subKey, subDiff := range diff {
			res[subKey] = subDiff
		}
	} else if etf.Type() == shim.TypeList {
		diff := makeListDiff(ctx, key, etf, eps, old, new, oldOk, newOk)
		for subKey, subDiff := range diff {
			res[subKey] = subDiff
		}
	} else if etf.Type() == shim.TypeMap {
		diff := makeMapDiff(ctx, key, etf, eps, old, new, oldOk, newOk)
		for subKey, subDiff := range diff {
			res[subKey] = subDiff
		}
	} else if etf.Type() == shim.TypeSet {
		// TODO: Implement set diffing
		diff := makeListDiff(ctx, key, etf, eps, old, new, oldOk, newOk)
		for subKey, subDiff := range diff {
			res[subKey] = subDiff
		}
	} else {
		topDiff := makeTopPropDiff(old, new, oldOk, newOk, etf, eps)
		if topDiff == nil {
			return nil
		}
		res[string(key)] = topDiff
	}

	return res
}

func makeObjectDiff(
	ctx context.Context,
	key resource.PropertyKey,
	etf shim.SchemaMap,
	eps map[string]*SchemaInfo,
	old, new resource.PropertyValue,
	oldOk, newOk bool,
) map[string]*pulumirpc.PropertyDiff {
	diff := make(map[string]*pulumirpc.PropertyDiff)
	oldObj := resource.PropertyMap{}
	newObj := resource.PropertyMap{}
	if isPresent(old, oldOk) && old.IsObject() {
		oldObj = old.ObjectValue()
	}
	if isPresent(new, newOk) && new.IsObject() {
		newObj = new.ObjectValue()
	}

	keys := make(map[resource.PropertyKey]struct{})
	for k := range oldObj {
		keys[k] = struct{}{}
	}
	for k := range newObj {
		keys[k] = struct{}{}
	}

	for k := range keys {
		key := getSubPath(key, k)
		oldVal, oldOk := oldObj[k]
		newVal, newOk := newObj[k]
		_, etf, eps := getInfoFromPulumiName(k, etf, eps)

		propDiff := makePropDiff(ctx, key, etf, eps, oldVal, newVal, oldOk, newOk)

		for subKey, subDiff := range propDiff {
			diff[subKey] = subDiff
		}
	}

	simplerDiff, err := simplifyDiff(diff, key, old, new, oldOk, newOk, etf, nil)
	if err == nil {
		return simplerDiff
	}

	return diff
}

func makeElemDiff(
	ctx context.Context,
	key resource.PropertyKey,
	etf interface{},
	eps *SchemaInfo,
	parentForceNew bool,
	old, new resource.PropertyValue,
	oldOk, newOk bool,
) map[string]*pulumirpc.PropertyDiff {
	diff := make(map[string]*pulumirpc.PropertyDiff)
	if _, ok := etf.(shim.Resource); ok {
		fields := map[string]*SchemaInfo{}
		if eps != nil {
			fields = eps.Fields
		}
		etfSch := etf.(shim.Resource).Schema()
		d := makeObjectDiff(
			ctx,
			key,
			etfSch,
			fields,
			old,
			new,
			oldOk,
			newOk,
		)
		for subKey, subDiff := range d {
			diff[subKey] = subDiff
		}
	} else if _, ok := etf.(shim.Schema); ok {
		d := makePropDiff(
			ctx,
			key,
			etf.(shim.Schema),
			eps,
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
		d := makePropDiff(ctx, key, nil, eps.Elem, old, new, oldOk, newOk)
		for subKey, subDiff := range d {
			if parentForceNew {
				subDiff = promoteToReplace(subDiff)
			}
			diff[subKey] = subDiff
		}
	}

	simplerDiff, err := simplifyDiff(diff, key, old, new, oldOk, newOk, etf, eps)
	if err == nil {
		return simplerDiff
	}

	return diff
}

func makeListDiff(
	ctx context.Context,
	key resource.PropertyKey,
	etf shim.Schema,
	eps *info.Schema,
	old, new resource.PropertyValue,
	oldOk, newOk bool,
) map[string]*pulumirpc.PropertyDiff {
	diff := make(map[string]*pulumirpc.PropertyDiff)
	oldList := []resource.PropertyValue{}
	newList := []resource.PropertyValue{}
	if isPresent(old, oldOk) && old.IsArray() {
		oldList = old.ArrayValue()
	}
	if isPresent(new, newOk) && new.IsArray() {
		newList = new.ArrayValue()
	}

	// naive diffing of lists
	// TODO: implement a more sophisticated diffing algorithm
	// TODO: investigate how this interacts with force new - is identity preserved or just order
	collectionForceNew := isForceNew(etf, eps)
	longerLen := max(len(oldList), len(newList))
	for i := 0; i < longerLen; i++ {
		elemKey := string(key) + "[" + fmt.Sprintf("%d", i) + "]"
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
			ctx, resource.PropertyKey(elemKey), etf.Elem(), eps, collectionForceNew, oldVal, newVal, oldOk, newOk)
		for subKey, subDiff := range d {
			diff[subKey] = subDiff
		}
	}

	simplerDiff, err := simplifyDiff(diff, key, old, new, oldOk, newOk, etf, eps)
	if err == nil {
		return simplerDiff
	}

	return diff
}

func makeMapDiff(
	ctx context.Context,
	key resource.PropertyKey,
	etf shim.Schema,
	eps *SchemaInfo,
	old, new resource.PropertyValue,
	oldOk, newOk bool,
) map[string]*pulumirpc.PropertyDiff {
	diff := make(map[string]*pulumirpc.PropertyDiff)
	oldMap := resource.PropertyMap{}
	newMap := resource.PropertyMap{}
	if isPresent(old, oldOk) && old.IsObject() {
		oldMap = old.ObjectValue()
	}
	if isPresent(new, newOk) && new.IsObject() {
		newMap = new.ObjectValue()
	}

	keys := make(map[resource.PropertyKey]struct{})
	for k := range oldMap {
		keys[k] = struct{}{}
	}
	for k := range newMap {
		keys[k] = struct{}{}
	}

	collectionForceNew := isForceNew(etf, eps)
	for k := range keys {
		key := getSubPath(key, k)
		oldVal, oldOk := oldMap[k]
		newVal, newOk := newMap[k]

		pelem := &info.Schema{}
		if eps != nil {
			pelem = eps.Elem
		}
		elemDiff := makeElemDiff(ctx, key, etf.Elem(), pelem, collectionForceNew, oldVal, newVal, oldOk, newOk)

		for subKey, subDiff := range elemDiff {
			diff[subKey] = subDiff
		}
	}

	simplerDiff, err := simplifyDiff(diff, key, old, new, oldOk, newOk, etf, eps)
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
	keys := make(map[resource.PropertyKey]struct{})
	for k := range oldState {
		keys[k] = struct{}{}
	}
	for k := range plannedState {
		keys[k] = struct{}{}
	}

	diff := make(map[string]*pulumirpc.PropertyDiff)
	for k := range keys {
		old, oldOk := oldState[k]
		new, newOk := plannedState[k]
		_, etf, eps := getInfoFromPulumiName(k, tfs, ps)

		propDiff := makePropDiff(ctx, k, etf, eps, old, new, oldOk, newOk)

		for subKey, subDiff := range propDiff {
			diff[subKey] = subDiff
		}
	}

	return diff
}
