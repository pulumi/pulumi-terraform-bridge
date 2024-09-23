package tfbridge

import (
	"context"
	"fmt"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

func isFlattened(s shim.Schema) bool {
	if s.Type() != shim.TypeList && s.Type() != shim.TypeSet {
		return false
	}

	return s.MaxItems() == 1
}

func isDunder(k resource.PropertyKey) bool {
	return len(k) > 1 && k[0] == '_' && k[1] == '_'
}

func makeTopPropDiff(
	old, new resource.PropertyValue,
	oldOk, newOk bool,
) *pulumirpc.PropertyDiff {
	if !oldOk {
		if !newOk {
			return nil
		}
		if new == resource.NewNullProperty() {
			// TODO: Should we handle this here or make sure the inputs are meaningfully present?
			return nil
		}

		return &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_ADD}
	}
	if !newOk {
		if old == resource.NewNullProperty() {
			// TODO: Should we handle this here or make sure the inputs are meaningfully present?
			return nil
		}

		return &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_DELETE}
	}
	if !old.DeepEquals(new) {
		return &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_UPDATE}
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
	topDiff := makeTopPropDiff(old, new, oldOk, newOk)
	if topDiff == nil {
		return nil
	}

	res := make(map[string]*pulumirpc.PropertyDiff)
	res[string(key)] = topDiff

	if etf == nil {
		// If the schema is nil, we just return the top-level diff
		return res
	}

	if isFlattened(etf) {
		pelem := &info.Schema{}
		if eps != nil {
			pelem = eps.Elem
		}
		diff := makeElemDiff(ctx, key, etf.Elem(), pelem, old, new, oldOk, newOk)
		for subKey, subDiff := range diff {
			res[subKey] = subDiff
		}
	} else if etf.Type() == shim.TypeList {
		diff := makeListDiff(ctx, key, etf, eps, old, new, oldOk, newOk)
		for subKey, subDiff := range diff {
			res[subKey] = subDiff
		}
	} else if etf.Type() == shim.TypeMap {
		diff := makeMapDiff(ctx, key, etf, eps, old, new)
		for subKey, subDiff := range diff {
			res[subKey] = subDiff
		}
	}
	// TODO: Other types
	return res
}

func makeObjectDiff(
	ctx context.Context,
	key resource.PropertyKey,
	etf shim.SchemaMap,
	eps map[string]*SchemaInfo,
	old, new resource.PropertyValue,
) map[string]*pulumirpc.PropertyDiff {
	diff := make(map[string]*pulumirpc.PropertyDiff)
	if !old.IsObject() || !new.IsObject() {
		diff[string(key)] = &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_UPDATE}
		return diff
	}

	oldObj := old.ObjectValue()
	newObj := new.ObjectValue()
	keys := make(map[resource.PropertyKey]struct{})
	for k := range oldObj {
		keys[k] = struct{}{}
	}
	for k := range newObj {
		keys[k] = struct{}{}
	}

	for k := range keys {
		// TODO: is escaping needed here?
		key := string(key) + "." + string(k)
		oldVal, oldOk := oldObj[k]
		newVal, newOk := newObj[k]
		_, etf, eps := getInfoFromPulumiName(k, etf, eps)

		propDiff := makePropDiff(ctx, resource.PropertyKey(key), etf, eps, oldVal, newVal, oldOk, newOk)

		for subKey, subDiff := range propDiff {
			diff[subKey] = subDiff
		}
	}

	return diff
}

func makeElemDiff(
	ctx context.Context,
	key resource.PropertyKey,
	etf interface{},
	eps *SchemaInfo,
	old, new resource.PropertyValue,
	oldOk, newOk bool,
) map[string]*pulumirpc.PropertyDiff {
	diff := make(map[string]*pulumirpc.PropertyDiff)
	if !oldOk {
		diff[string(key)] = &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_ADD}
		return diff
	}
	if !newOk {
		diff[string(key)] = &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_DELETE}
		return diff
	}

	if _, ok := etf.(shim.Resource); ok {
		fields := map[string]*SchemaInfo{}
		if eps != nil {
			fields = eps.Fields
		}
		d := makeObjectDiff(
			ctx,
			key,
			etf.(shim.Resource).Schema(),
			fields,
			old,
			new,
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
			true,
			true,
		)
		for subKey, subDiff := range d {
			diff[subKey] = subDiff
		}
	} else {
		d := makePropDiff(ctx, key, nil, eps.Elem, old, new, true, true)
		for subKey, subDiff := range d {
			diff[subKey] = subDiff
		}
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
	if !oldOk {
		diff[string(key)] = &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_ADD}
		return diff
	}
	if !newOk {
		diff[string(key)] = &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_DELETE}
		return diff
	}
	oldList := old.ArrayValue()
	newList := new.ArrayValue()

	// naive diffing of lists
	shorterLen := min(len(oldList), len(newList))
	for i := 0; i < shorterLen; i++ {
		elemKey := string(key) + "[" + fmt.Sprintf("%d", i) + "]"
		d := makeElemDiff(ctx, resource.PropertyKey(elemKey), etf.Elem(), eps, oldList[i], newList[i], true, true)
		for subKey, subDiff := range d {
			diff[subKey] = subDiff
		}
	}

	// if the lists are different lengths, add the remaining elements as adds or deletes
	if len(oldList) > len(newList) {
		for i := len(newList); i < len(oldList); i++ {
			elemKey := string(key) + "[" + fmt.Sprintf("%d", i) + "]"
			diff[elemKey] = &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_DELETE}
		}
	} else if len(newList) > len(oldList) {
		for i := len(oldList); i < len(newList); i++ {
			elemKey := string(key) + "[" + fmt.Sprintf("%d", i) + "]"
			diff[elemKey] = &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_ADD}
		}
	}

	return diff
}

func makeMapDiff(
	ctx context.Context,
	key resource.PropertyKey,
	etf shim.Schema,
	eps *SchemaInfo,
	old, new resource.PropertyValue,
) map[string]*pulumirpc.PropertyDiff {
	diff := make(map[string]*pulumirpc.PropertyDiff)
	if !old.IsObject() || !new.IsObject() {
		diff[string(key)] = &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_UPDATE}
		return diff
	}

	oldMap := old.ObjectValue()
	newMap := new.ObjectValue()
	keys := make(map[resource.PropertyKey]struct{})
	for k := range oldMap {
		keys[k] = struct{}{}
	}
	for k := range newMap {
		keys[k] = struct{}{}
	}

	for k := range keys {
		key := string(key) + "." + string(k)
		oldVal, oldOk := oldMap[k]
		newVal, newOk := newMap[k]
		
		pelem := &info.Schema{}
		if eps != nil {
			pelem = eps.Elem
		}
		elemDiff := makeElemDiff(ctx, resource.PropertyKey(key), etf.Elem(), pelem, oldVal, newVal, oldOk, newOk)

		for subKey, subDiff := range elemDiff {
			diff[subKey] = subDiff
		}
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
