package tfbridge

import (
	"context"
	"fmt"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

func isBlock(s shim.Schema) bool {
	// TODO: handle maps with resource elems?
	if s.Elem() == nil {
		return false
	}
	_, ok := s.Elem().(shim.Resource)
	return ok
}

func makeTopPropDiff(
	old, new resource.PropertyValue,
	oldOk, newOk bool,
) *pulumirpc.PropertyDiff {
	if !oldOk {
		if !newOk {
			return nil
		}

		return &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_ADD}
	}
	if !newOk {
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
	topDiff := makeTopPropDiff(old, new, oldOk, newOk)
	if topDiff == nil {
		return nil
	}

	res := make(map[string]*pulumirpc.PropertyDiff)
	res[string(key)] = topDiff

	if etf.Type() == shim.TypeList {
		diff := makeListDiff(ctx, key, etf, eps, old, new, oldOk, newOk)
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
		if _, ok := etf.Elem().(shim.Resource); ok {
			d := makeObjectDiff(
				ctx,
				resource.PropertyKey(elemKey),
				etf.Elem().(shim.Resource).Schema(),
				eps.Fields,
				oldList[i],
				newList[i],
			)
			for subKey, subDiff := range d {
				diff[subKey] = subDiff
			}
		} else if _, ok := etf.Elem().(shim.Schema); ok {
			d := makePropDiff(
				ctx,
				resource.PropertyKey(elemKey),
				etf.Elem().(shim.Schema),
				eps.Elem,
				oldList[i],
				newList[i],
				true,
				true,
			)
			for subKey, subDiff := range d {
				diff[subKey] = subDiff
			}
		} else {
			d := makePropDiff(ctx, resource.PropertyKey(elemKey), nil, eps.Elem, oldList[i], newList[i], true, true)
			for subKey, subDiff := range d {
				diff[subKey] = subDiff
			}
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
