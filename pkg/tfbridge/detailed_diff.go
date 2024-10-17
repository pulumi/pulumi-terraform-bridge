package tfbridge

import (
	"cmp"
	"context"
	"slices"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"

	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/walk"
)

func isPresent(val resource.PropertyValue) bool {
	return !val.IsNull() &&
		!(val.IsArray() && val.ArrayValue() == nil) &&
		!(val.IsObject() && val.ObjectValue() == nil)
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

type detailedDiffKey string

type detailedDiffer struct {
	tfs shim.SchemaMap
	ps  map[string]*SchemaInfo
	// These are used to convert set indices back to something the engine can reference.
	newInputs resource.PropertyMap
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

type hashIndexMap map[int]int

func (differ detailedDiffer) calculateSetHashIndexMap(path propertyPath, listVal resource.PropertyValue) hashIndexMap {
	identities := make(hashIndexMap)

	tfs, ps, err := lookupSchemas(path, differ.tfs, differ.ps)
	if err != nil {
		return nil
	}

	convertedVal, err := makeSingleTerraformInput(context.Background(), path.String(), listVal, tfs, ps)
	if err != nil {
		return nil
	}

	if convertedVal == nil {
		return nil
	}

	convertedListVal, ok := convertedVal.([]interface{})
	contract.Assertf(ok, "converted value should be a list")

	// Calculate the identity of each element
	for i, newElem := range convertedListVal {
		hash := tfs.SetHash(newElem)
		identities[hash] = i
	}
	return identities
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

	switch propType {
	case shim.TypeList:
		return differ.makeListDiff(path, old, new)
	case shim.TypeSet:
		return differ.makeSetDiff(path, old, new)
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
	if !isPresent(old) || !old.IsArray() {
		old = resource.NewNullProperty()
	}
	if (!isPresent(new) || !new.IsArray()) && !new.IsComputed() {
		new = resource.NewNullProperty()
	}
	if old.IsNull() || new.IsNull() || new.IsComputed() {
		return differ.makeShortCircuitDiff(path, old, new)
	}

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

type setChangeIndex struct {
	engineIndex   int
	newStateIndex int
	oldChanged    bool
	newChanged    bool
}

func (differ detailedDiffer) makeSetDiff(
	path propertyPath, old, new resource.PropertyValue,
) map[detailedDiffKey]*pulumirpc.PropertyDiff {
	if !isPresent(old) || !old.IsArray() {
		old = resource.NewNullProperty()
	}
	if (!isPresent(new) || !new.IsArray()) && !new.IsComputed() {
		new = resource.NewNullProperty()
	}
	if old.IsNull() || new.IsNull() || new.IsComputed() {
		return differ.makeShortCircuitDiff(path, old, new)
	}

	diff := make(map[detailedDiffKey]*pulumirpc.PropertyDiff)
	oldList := old.ArrayValue()
	newList := new.ArrayValue()
	newInputsList := []resource.PropertyValue{}

	newInputs, newInputsOk := path.GetFromMap(differ.newInputs)
	if newInputsOk && isPresent(newInputs) && newInputs.IsArray() {
		newInputsList = newInputs.ArrayValue()
	}

	oldIdentities := differ.calculateSetHashIndexMap(path, resource.NewArrayProperty(oldList))
	newIdentities := differ.calculateSetHashIndexMap(path, resource.NewArrayProperty(newList))

	// TODO: We can not hash the inputs as they might not have the correct shape!
	inputIdentities := differ.calculateSetHashIndexMap(path, resource.NewArrayProperty(newInputsList))

	// The old indices and new inputs are the indices the engine can reference
	// The new state indices need to be translated to new input indices when presenting the diff
	setIndices := make(map[int]setChangeIndex)
	for hash, oldIndex := range oldIdentities {
		if _, newOk := newIdentities[hash]; !newOk {
			setIndices[oldIndex] = setChangeIndex{engineIndex: oldIndex, oldChanged: true, newStateIndex: -1, newChanged: false}
		}
	}
	for hash, newIndex := range newIdentities {
		if _, oldOk := oldIdentities[hash]; !oldOk {
			inputIndex := inputIdentities[hash]
			// TODO: make this a warning instead
			contract.Assertf(inputIndex != -1, "could not find index of element in new inputs")
			_, oldChanged := setIndices[inputIndex]
			setIndices[inputIndex] = setChangeIndex{
				engineIndex: inputIndex, oldChanged: oldChanged, newStateIndex: newIndex, newChanged: true,
			}
		}
	}

	for index, setChange := range setIndices {
		oldEl := resource.NewNullProperty()
		if setChange.oldChanged {
			oldEl = oldList[setChange.engineIndex]
		}
		newEl := resource.NewNullProperty()
		if setChange.newChanged {
			contract.Assertf(setChange.newStateIndex != -1, "new state index should be set")
			newEl = newList[setChange.newStateIndex]
		}
		d := differ.makePropDiff(path.Index(index), oldEl, newEl)
		for subKey, subDiff := range d {
			diff[subKey] = subDiff
		}
	}

	return diff
}

func (differ detailedDiffer) makeMapDiff(
	path propertyPath, old, new resource.PropertyValue,
) map[detailedDiffKey]*pulumirpc.PropertyDiff {
	if !isPresent(old) || !old.IsObject() {
		old = resource.NewNullProperty()
	}
	if !isPresent(new) || !new.IsObject() && !new.IsComputed() {
		new = resource.NewNullProperty()
	}
	if old.IsNull() || new.IsNull() || new.IsComputed() {
		return differ.makeShortCircuitDiff(path, old, new)
	}

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
	newInputs resource.PropertyMap,
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

	differ := detailedDiffer{tfs: tfs, ps: ps, newInputs: newInputs}
	return differ.makeDetailedDiffPropertyMap(priorProps, props), nil
}
