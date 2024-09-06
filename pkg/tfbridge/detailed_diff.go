package tfbridge

import (
	"context"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

func isBlock(s shim.Schema) bool {
	if s.Elem() == nil {
		return false
	}
	_, ok := s.Elem().(shim.Resource)
	return ok
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


}


// Diffs two plain properties (i.e. not collections or objects)
func makePlainPropDiff(
	_ context.Context,
	key resource.PropertyKey,
	_ shim.Schema,
	_ *info.Schema,
	old, new resource.PropertyValue,
	oldOk, newOk bool,
) *pulumirpc.PropertyDiff {
	if !oldOk {
		return &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_ADD}
	}
	if !newOk {
		return &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_DELETE}
	}
	if old != new {
		return &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_UPDATE}
	}
	return nil
}

func makeBlockDiff(
	ctx context.Context,
	key resource.PropertyKey,
	etf shim.Schema,
	eps *info.Schema,
	old, new resource.PropertyValue,
	oldOk, newOk bool,
) map[string]*pulumirpc.PropertyDiff {
	diff := make(map[string]*pulumirpc.PropertyDiff)
	resElem := etf.Elem().(shim.Resource)
	if !oldOk {
		diff[string(key)] = &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_ADD}
		return diff
	}
	if !newOk {
		diff[string(key)] = &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_DELETE}
		return diff
	}
	blockDiff := makeObjectDiff(ctx, resElem.Schema(), eps.Fields, old.ObjectValue(), new.ObjectValue())
	if len(blockDiff) > 0 {
		diff[string(key)] = &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_UPDATE}
		for subKey, subDiff := range blockDiff {
			// TODO: do we need to prefix the subKey with the block name?
			diff[string(key)+"."+subKey] = subDiff
		}
	}
	return diff
}

func makeObjectDiff(ctx context.Context, tfs shim.SchemaMap, ps map[string]*SchemaInfo,
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

		if isBlock(etf) {
			blockDiff := makeBlockDiff(ctx, k, etf, eps, old, new, oldOk, newOk)
			for subKey, subDiff := range blockDiff {
				diff[subKey] = subDiff
			}
			continue
		} else {
			d := makePropDiff(ctx, k, etf, eps, old, new, oldOk, newOk)
			if d != nil {
				diff[string(k)] = d
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
	return makeObjectDiff(ctx, tfs, ps, oldState, plannedState)
}
