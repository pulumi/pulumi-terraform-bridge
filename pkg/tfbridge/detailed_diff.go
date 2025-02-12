package tfbridge

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"sort"

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/walk"
	"github.com/pulumi/pulumi-terraform-bridge/v3/unstable/difft"
	"github.com/pulumi/pulumi-terraform-bridge/v3/unstable/propertyvalue"
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

func isTypeShapeMismatched(val resource.PropertyValue, propType shim.ValueType) bool {
	contract.Assertf(!val.IsSecret() || val.IsOutput(), "secrets and outputs are not handled")
	if val.IsComputed() {
		return false
	}
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

func containsReplace(m map[string]*pulumirpc.PropertyDiff) bool {
	for _, v := range m {
		switch v.GetKind() {
		case pulumirpc.PropertyDiff_UPDATE_REPLACE,
			pulumirpc.PropertyDiff_ADD_REPLACE,
			pulumirpc.PropertyDiff_DELETE_REPLACE:
			return true
		}
	}
	return false
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

func demoteToNoReplace(diff *pulumirpc.PropertyDiff) *pulumirpc.PropertyDiff {
	if diff == nil {
		return nil
	}
	kind := diff.GetKind()
	switch kind {
	case pulumirpc.PropertyDiff_ADD_REPLACE:
		return &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_ADD}
	case pulumirpc.PropertyDiff_DELETE_REPLACE:
		return &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_DELETE}
	case pulumirpc.PropertyDiff_UPDATE_REPLACE:
		return &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_UPDATE}
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

// validInputsFromPlan returns true if the given plan property value could originate from the given inputs.
// Under the hood, it walks the plan and the inputs and checks that all differences stem from computed properties.
// Any differences coming from properties which are not computed will be rejected.
// Note that we are relying on the fact that the inputs will have defaults already applied.
// Also note that nested sets will only get matched if they are exactly the same.
func validInputsFromPlan(
	path propertyPath,
	inputs resource.PropertyValue,
	plan resource.PropertyValue,
	tfs shim.SchemaMap,
	ps map[string]*info.Schema,
) bool {
	abortErr := errors.New("abort")
	visitor := func(
		subpath propertyPath, inputsSubVal, planSubVal resource.PropertyValue,
	) error {
		contract.Assertf(
			!inputsSubVal.IsSecret() && !planSubVal.IsSecret() && !inputsSubVal.IsOutput() && !planSubVal.IsOutput(),
			"validInputsFromPlan does not support secrets or outputs")
		// Do not compare and do not descend into internal properties.
		if subpath.IsReservedKey() {
			return SkipChildrenError{}
		}

		tfs, _, err := lookupSchemas(subpath, tfs, ps)
		if err != nil {
			return abortErr
		}

		if tfs.Computed() && inputsSubVal.IsNull() {
			// This is a computed property populated by the plan. We should not recurse into it.
			return SkipChildrenError{}
		}

		if tfs.Type() == shim.TypeList || tfs.Type() == shim.TypeSet {
			// Note that nested sets will likely get their elements reordered.
			// This means that nested sets will not be matched correctly but that should be a rare case.
			if inputsSubVal.IsNull() {
				// The plan is allowed to populate a nil list with an empty value.
				if (planSubVal.IsArray() && len(planSubVal.ArrayValue()) == 0) || planSubVal.IsNull() {
					return nil
				}
				return abortErr
			}
			if planSubVal.IsNull() {
				// The plan is not allowed to replace an empty list with a nil value.
				return abortErr
			}

			if !inputsSubVal.IsArray() || !planSubVal.IsArray() {
				return abortErr
			}

			// all non-empty lists will get their element values checked.
			return nil
		}

		if tfs.Type() == shim.TypeMap {
			if inputsSubVal.IsNull() {
				// The plan is allowed to populate a nil map with an empty value.
				if (planSubVal.IsObject() && len(planSubVal.ObjectValue()) == 0) || planSubVal.IsNull() {
					return nil
				}
				return abortErr
			}
			if planSubVal.IsNull() {
				// The plan is not allowed to replace an empty map with a nil value.
				return abortErr
			}

			if !inputsSubVal.IsObject() || !planSubVal.IsObject() {
				return abortErr
			}

			// all non-empty maps will get their element values checked.
			return nil
		}

		if inputsSubVal.DeepEquals(planSubVal) {
			return nil
		}

		return abortErr
	}
	err := walkTwoPropertyValues(
		path,
		inputs,
		plan,
		visitor,
	)
	if err == abortErr || errors.Is(err, TypeMismatchError{}) {
		return false
	}
	contract.AssertNoErrorf(err, "TransformPropertyValue should only return an abort error")
	return true
}

type detailedDiffKey string

type detailedDiffer struct {
	ctx context.Context
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

func (differ detailedDiffer) isForceNew(path propertyPath) bool {
	tfs, ps, err := lookupSchemas(path, differ.tfs, differ.ps)
	if err != nil {
		return false
	}
	return isForceNew(tfs, ps)
}

// makeSingleTerraformInput converts a single Pulumi property value into a plain go value suitable for use by Terraform.
// makeSingleTerraformInput does not apply any defaults or other transformations.
// Note that makeSingleTerraformInput uses UseTFSetTypes=true, so it will return a TF Set for any sets it encounters.
func makeSingleTerraformInput(
	ctx context.Context, name string, val resource.PropertyValue, tfs shim.Schema, ps *SchemaInfo,
) (interface{}, error) {
	cctx := &conversionContext{
		Ctx:                   ctx,
		ComputeDefaultOptions: ComputeDefaultOptions{},
		ProviderConfig:        nil,
		ApplyDefaults:         false,
		ApplyTFDefaults:       false,
		Assets:                AssetTable{},
		UseTFSetTypes:         true,
	}

	return cctx.makeTerraformInput(name, resource.NewNullProperty(), val, tfs, ps)
}

type (
	setHash    int
	arrayIndex int
)

type hashIndexMap map[setHash]arrayIndex

func (differ detailedDiffer) calculateSetHashIndexMap(
	path propertyPath, setElements []resource.PropertyValue,
) hashIndexMap {
	identities := make(hashIndexMap)

	tfs, ps, err := lookupSchemas(path, differ.tfs, differ.ps)
	if err != nil {
		return nil
	}

	convertedElements := []interface{}{}

	for _, elem := range setElements {
		convertedElem, err := makeSingleTerraformInput(
			differ.ctx, path.String(), elem, tfs, ps)
		if err != nil {
			return nil
		}
		convertedElements = append(convertedElements, convertedElem)
	}

	// Calculate the identity of each element. Note that the SetHash function can panic
	// in the case of custom SetHash functions which get unexpected inputs.
	for i, newElem := range convertedElements {
		elementHash := func() int {
			defer func() {
				if r := recover(); r != nil {
					GetLogger(differ.ctx).Warn(fmt.Sprintf(
						"Failed to calculate preview for element in %s: %v",
						path.String(), r))
				}
			}()
			return tfs.SetHash(newElem)
		}()
		identities[setHash(elementHash)] = arrayIndex(i)
	}
	return identities
}

// makePlainPropDiff is used for plain properties and ones with an unknown schema.
// It does not access the TF schema, so it does not know about the type of the property.
func (differ detailedDiffer) makePlainPropDiff(
	path propertyPath, old, new resource.PropertyValue,
) map[detailedDiffKey]*pulumirpc.PropertyDiff {
	baseDiff := makeBaseDiff(old, new)
	isReplacement := propertyPathTriggersReplacement(path, differ.tfs, differ.ps)
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
	if new.IsComputed() && propertyPathTriggersReplacement(path, differ.tfs, differ.ps) {
		propDiff = promoteToReplace(propDiff)
	} else if !new.IsNull() && !new.IsComputed() && propertyValueTriggersReplacement(path, new, differ.tfs, differ.ps) {
		propDiff = promoteToReplace(propDiff)
	} else if !old.IsNull() && propertyValueTriggersReplacement(path, old, differ.tfs, differ.ps) {
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
	if isTypeShapeMismatched(old, propType) || isTypeShapeMismatched(new, propType) {
		return differ.makePlainPropDiff(path, old, new)
	}

	if !isPresent(old) {
		old = resource.NewNullProperty()
	}
	if !new.IsComputed() && !isPresent(new) {
		new = resource.NewNullProperty()
	}
	if old.IsNull() || new.IsNull() || new.IsComputed() {
		return differ.makeShortCircuitDiff(path, old, new)
	}

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

// makeListAttributeDiff should only be called for lists of scalar values.
// Note that the algorithm used is ~N^2, so it should not be used for large lists.
func makeListAttributeDiff(
	path propertyPath, old, new []resource.PropertyValue,
) map[detailedDiffKey]*pulumirpc.PropertyDiff {
	contract.Assertf(len(old) < 1000 && len(new) < 1000, "makeListAttributeDiff should not be used for large lists")
	diff := make(map[detailedDiffKey]*pulumirpc.PropertyDiff)
	type valIndex struct {
		Value resource.PropertyValue
		Index int
	}

	oldVals := []valIndex{}
	for i, v := range old {
		oldVals = append(oldVals, valIndex{Value: v, Index: i})
	}
	newVals := []valIndex{}
	for i, v := range new {
		newVals = append(newVals, valIndex{Value: v, Index: i})
	}

	edits := difft.DiffT(oldVals, newVals, difft.DiffOptions[valIndex]{
		Equals: func(a, b valIndex) bool {
			return a.Value.DeepEquals(b.Value)
		},
	})

	for _, edit := range edits {
		if edit.Change == difft.Insert {
			key := path.Index(edit.Element.Index)
			if diff[key.Key()] == nil {
				diff[key.Key()] = &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_ADD}
			} else {
				diff[key.Key()] = &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_UPDATE}
			}
		}
		if edit.Change == difft.Remove {
			key := path.Index(edit.Element.Index)
			if diff[key.Key()] == nil {
				diff[key.Key()] = &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_DELETE}
			} else {
				diff[key.Key()] = &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_UPDATE}
			}
		}
	}

	return diff
}

func (differ detailedDiffer) makeListDiff(
	path propertyPath, old, new resource.PropertyValue,
) map[detailedDiffKey]*pulumirpc.PropertyDiff {
	diff := make(map[detailedDiffKey]*pulumirpc.PropertyDiff)
	oldList := old.ArrayValue()
	newList := new.ArrayValue()

	tfs, _, err := lookupSchemas(path, differ.tfs, differ.ps)
	if err != nil {
		return nil
	}

	// We attempt to optimize the diff displayed for list attributes with a reasonable number of elements.
	_, scalarElemType := tfs.Elem().(shim.Schema)
	if scalarElemType && len(oldList) < 1000 && len(newList) < 1000 {
		listDiff := makeListAttributeDiff(path, oldList, newList)
		if tfs.ForceNew() {
			for k, v := range listDiff {
				diff[k] = promoteToReplace(v)
			}
		} else {
			for k, v := range listDiff {
				diff[k] = v
			}
		}
		return diff
	}

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

func computeSetHashChanges(
	oldIdentities, newIdentities hashIndexMap,
) (removed, added []arrayIndex) {
	removed = []arrayIndex{}
	added = []arrayIndex{}

	for elementHash := range oldIdentities {
		if _, ok := newIdentities[elementHash]; !ok {
			removed = append(removed, oldIdentities[elementHash])
		}
	}

	for elementHash := range newIdentities {
		if _, ok := oldIdentities[elementHash]; !ok {
			added = append(added, newIdentities[elementHash])
		}
	}

	sort.Slice(removed, func(i, j int) bool {
		return removed[i] < removed[j]
	})
	sort.Slice(added, func(i, j int) bool {
		return added[i] < added[j]
	})

	return
}

// matchPlanElementsToInputs is used to match the plan elements to the inputs.
// It returns a map of inputs indices to the planned state indices.
func (differ detailedDiffer) matchPlanElementsToInputs(
	path propertyPath, changedPlanIndices []arrayIndex, plannedState []resource.PropertyValue,
	rootNewInputs resource.PropertyMap,
) map[arrayIndex]arrayIndex {
	newInputsList := []resource.PropertyValue{}

	newInputs, newInputsOk := path.GetFromMap(rootNewInputs)
	if newInputsOk && isPresent(newInputs) && newInputs.IsArray() {
		newInputsList = newInputs.ArrayValue()
	}

	if len(newInputsList) < len(plannedState) {
		// If the number of inputs is less than the number of planned state elements,
		// we can't match the elements to the inputs.
		return nil
	}

	matched := make(map[arrayIndex]arrayIndex)
	used := make(map[int]bool, len(newInputsList))
	for k := range used {
		used[k] = false
	}

	for _, index := range changedPlanIndices {
		for i, input := range newInputsList {
			if used[i] {
				// This input has already been used to match an element in the planned state.
				continue
			}

			match := validInputsFromPlan(path.Index(int(index)), input, plannedState[index], differ.tfs, differ.ps)
			if match {
				matched[arrayIndex(i)] = index
				used[i] = true
				break
			}
		}
	}

	return matched
}

type setChange struct {
	oldChanged   bool
	newChanged   bool
	plannedIndex arrayIndex
}

func makeSetChangeMap(
	removed []arrayIndex,
	matchedInputIndices map[arrayIndex]arrayIndex,
) map[arrayIndex]setChange {
	changes := map[arrayIndex]setChange{}
	for _, index := range removed {
		changes[index] = setChange{oldChanged: true, newChanged: false}
	}
	for inputIndex, planIndex := range matchedInputIndices {
		if _, ok := changes[inputIndex]; !ok {
			changes[inputIndex] = setChange{oldChanged: false, newChanged: true, plannedIndex: planIndex}
		} else {
			changes[inputIndex] = setChange{oldChanged: true, newChanged: true, plannedIndex: planIndex}
		}
	}
	return changes
}

func (differ detailedDiffer) makeSetDiffElementResult(
	path propertyPath,
	changes map[arrayIndex]setChange,
	oldList, newList []resource.PropertyValue,
) map[detailedDiffKey]*pulumirpc.PropertyDiff {
	diff := make(map[detailedDiffKey]*pulumirpc.PropertyDiff)
	for index, change := range changes {
		oldVal := resource.NewNullProperty()
		if change.oldChanged {
			oldVal = oldList[index]
		}
		newVal := resource.NewNullProperty()
		if change.newChanged {
			newVal = newList[change.plannedIndex]
		}

		elemDiff := differ.makePropDiff(path.Index(int(index)), oldVal, newVal)
		for subKey, subDiff := range elemDiff {
			diff[subKey] = subDiff
		}
	}

	return diff
}

func (differ detailedDiffer) makeSetDiff(
	path propertyPath, old, new resource.PropertyValue,
) map[detailedDiffKey]*pulumirpc.PropertyDiff {
	diff := make(map[detailedDiffKey]*pulumirpc.PropertyDiff)
	oldList := old.ArrayValue()
	newList := new.ArrayValue()

	oldIdentities := differ.calculateSetHashIndexMap(path, oldList)
	newIdentities := differ.calculateSetHashIndexMap(path, newList)

	removed, added := computeSetHashChanges(oldIdentities, newIdentities)

	if len(removed) == 0 && len(added) == 0 {
		return nil
	}

	// We need to match the new indices to the inputs to ensure that the identity of the
	// elements is preserved - this is necessary since the planning process can reorder
	// the elements.
	matchedInputIndices := differ.matchPlanElementsToInputs(path, added, newList, differ.newInputs)
	if matchedInputIndices == nil || len(matchedInputIndices) != len(added) {
		// If we can't match the elements to the inputs, we will return a diff against the whole set.
		// This ensures we still display a diff to the user, even if the algorithm can't determine
		// the correct diff for each element.
		if differ.isForceNew(path) {
			diff[path.Key()] = &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_UPDATE_REPLACE}
		} else {
			diff[path.Key()] = &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_UPDATE}
		}
		return diff
	}

	// We've managed to match all elements to the inputs, so we can safely build an element-wise diff.
	changes := makeSetChangeMap(removed, matchedInputIndices)
	diff = differ.makeSetDiffElementResult(path, changes, oldList, newList)

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

// MakeDetailedDiffV2 is the main entry point for calculating the detailed diff.
// This is an internal function that should not be used outside of the pulumi-terraform-bridge.
//
// The `replaceOverride` parameter is used to override the replace behavior of the detailed diff.
// If true, the diff will be overridden to return a replace.
// If false, the diff will be overridden to not return a replace.
// If nil, the detailed diff will be returned as is.
func MakeDetailedDiffV2(
	ctx context.Context,
	tfs shim.SchemaMap,
	ps map[string]*SchemaInfo,
	priorProps, props, newInputs resource.PropertyMap,
	replaceOverride *bool,
) map[string]*pulumirpc.PropertyDiff {
	// Strip secrets and outputs from the properties before calculating the diff.
	// This allows the rest of the algorithm to focus on the actual changes and not
	// have to deal with the extra noise.
	// This is safe to do here because the detailed diff we return to the engine
	// is only represented by paths to the values and not the values themselves.
	// The engine will then takes care of masking secrets.
	stripSecretsAndOutputs := func(props resource.PropertyMap) resource.PropertyMap {
		propsVal := propertyvalue.RemoveSecretsAndOutputs(resource.NewProperty(props))
		return propsVal.ObjectValue()
	}
	priorProps = stripSecretsAndOutputs(priorProps)
	props = stripSecretsAndOutputs(props)
	newInputs = stripSecretsAndOutputs(newInputs)
	differ := detailedDiffer{ctx: ctx, tfs: tfs, ps: ps, newInputs: newInputs}
	res := differ.makeDetailedDiffPropertyMap(priorProps, props)

	if replaceOverride != nil {
		if *replaceOverride {
			// We need to make sure there is a replace.
			if !containsReplace(res) {
				// We use the internal __meta property to trigger a replace when we have failed to
				// determine the correct detailed diff for it.
				res[metaKey] = &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_UPDATE_REPLACE}
			}
		} else {
			// There is an override for no replaces, so ensure we don't have any.
			for k, v := range res {
				res[k] = demoteToNoReplace(v)
			}
		}
	}

	return res
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
	replaceOverride *bool,
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

	return MakeDetailedDiffV2(ctx, tfs, ps, priorProps, props, newInputs, replaceOverride), nil
}
