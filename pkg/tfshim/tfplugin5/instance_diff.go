package tfplugin5

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/go-cty/cty/convert"

	shim "github.com/pulumi/pulumi-terraform-bridge/v2/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v2/pkg/tfshim/tfplugin5/proto"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/contract"
)

// This corresponds to the TF plugin SDK's timeouts key.
const timeoutsKey = "e2bfb730-ecaa-11e6-8f88-34363bc7c4c0"

var _ = shim.InstanceDiff((*instanceDiff)(nil))

type instanceDiff struct {
	planned     cty.Value
	meta        map[string]interface{}
	destroy     bool
	requiresNew bool
	attributes  map[string]shim.ResourceAttrDiff
}

func newInstanceDiff(prior, planned cty.Value, meta map[string]interface{},
	requiresReplace []*proto.AttributePath) *instanceDiff {

	attributes, requiresNew := computeDiff(prior, planned, requiresReplace)
	return &instanceDiff{
		planned:     planned,
		meta:        meta,
		destroy:     planned.IsNull(),
		requiresNew: requiresNew,
		attributes:  attributes,
	}
}

func (d *instanceDiff) Attribute(key string) *shim.ResourceAttrDiff {
	if diff, ok := d.attributes[key]; ok {
		return &diff
	}
	return nil
}

func (d *instanceDiff) Attributes() map[string]shim.ResourceAttrDiff {
	return d.attributes
}

func (d *instanceDiff) ProposedState(res shim.Resource, priorState shim.InstanceState) (shim.InstanceState, error) {
	plannedObject, err := ctyToGo(d.planned)
	if err != nil {
		return nil, err
	}

	var id string
	if priorState != nil {
		id = priorState.ID()
	}

	return &instanceState{
		resourceType: res.(*resource).resourceType,
		id:           id,
		object:       plannedObject.(map[string]interface{}),
		meta:         d.meta,
	}, nil
}

func (d *instanceDiff) Destroy() bool {
	return d.destroy
}

func (d *instanceDiff) RequiresNew() bool {
	return d.requiresNew
}

func (d *instanceDiff) IgnoreChanges(ignored map[string]bool) {
	for k := range d.attributes {
		if ignored[k] {
			delete(d.attributes, k)
		} else {
			for attr := range ignored {
				if strings.HasPrefix(k, attr+".") {
					delete(d.attributes, k)
					break
				}
			}
		}
	}
}

func (d *instanceDiff) EncodeTimeouts(timeouts *shim.ResourceTimeout) error {
	if timeouts == nil {
		return nil
	}

	timeoutsMap := map[string]interface{}{}
	if timeouts.Create != nil {
		timeoutsMap["create"] = timeouts.Create.Nanoseconds()
	}
	if timeouts.Update != nil {
		timeoutsMap["update"] = timeouts.Update.Nanoseconds()
	}
	if timeouts.Read != nil {
		timeoutsMap["read"] = timeouts.Read.Nanoseconds()
	}
	if timeouts.Delete != nil {
		timeoutsMap["delete"] = timeouts.Delete.Nanoseconds()
	}
	if timeouts.Default != nil {
		timeoutsMap["default"] = timeouts.Default.Nanoseconds()
	}

	if d.meta == nil {
		d.meta = map[string]interface{}{}
	}
	d.meta[timeoutsKey] = timeoutsMap
	return nil
}

func (d *instanceDiff) SetTimeout(timeout float64, timeoutKey string) {
	timeoutValue := time.Duration(timeout * 1000000000) //this turns seconds to nanoseconds - TF wants it in this format

	if d.meta == nil {
		d.meta = map[string]interface{}{}
	}
	timeoutsMap, ok := d.meta[timeoutsKey].(map[string]interface{})
	if !ok {
		timeoutsMap = map[string]interface{}{}
		d.meta[timeoutsKey] = timeoutsMap
	}

	switch timeoutKey {
	case shim.TimeoutCreate:
		timeoutsMap["create"] = timeoutValue.Nanoseconds()
	case shim.TimeoutRead:
		timeoutsMap["read"] = timeoutValue.Nanoseconds()
	case shim.TimeoutUpdate:
		timeoutsMap["update"] = timeoutValue.Nanoseconds()
	case shim.TimeoutDelete:
		timeoutsMap["delete"] = timeoutValue.Nanoseconds()
	case shim.TimeoutDefault:
		timeoutsMap["default"] = timeoutValue.Nanoseconds()
	}
}

type stringSet map[string]struct{}

func (ss stringSet) add(s string) {
	ss[s] = struct{}{}
}

func (ss stringSet) has(s string) bool {
	_, has := ss[s]
	return has
}

type differ struct {
	result        map[string]shim.ResourceAttrDiff
	requiresNew   stringSet
	isRequiresNew bool
}

func pathString(path *proto.AttributePath) string {
	var builder strings.Builder
	for _, s := range path.Steps {
		switch s := s.Selector.(type) {
		case *proto.AttributePath_Step_AttributeName:
			if builder.Len() != 0 {
				builder.WriteString(".")
			}
			builder.WriteString(s.AttributeName)
		case *proto.AttributePath_Step_ElementKeyString:
			if builder.Len() != 0 {
				builder.WriteString(".")
			}
			builder.WriteString(s.ElementKeyString)
		case *proto.AttributePath_Step_ElementKeyInt:
			if builder.Len() != 0 {
				builder.WriteString(".")
			}
			builder.WriteString(strconv.FormatInt(s.ElementKeyInt, 10))
		}
	}
	return builder.String()
}

func primitiveString(value cty.Value) string {
	contract.Assert(value.Type().IsPrimitiveType())

	switch {
	case value.IsNull():
		return ""
	case !value.IsKnown():
		return UnknownVariableValue
	default:
		str, err := convert.Convert(value, cty.String)
		contract.Assertf(err == nil, "could not convert %v to a string: %v", value, err)

		return str.AsString()
	}
}

func rangeValue(val cty.Value, each func(k, v cty.Value)) {
	iter := val.ElementIterator()
	for iter.Next() {
		k, v := iter.Element()
		each(k, v)
	}
}

func computeDiff(prior, planned cty.Value,
	requiresReplace []*proto.AttributePath) (map[string]shim.ResourceAttrDiff, bool) {

	requiresNew := stringSet{}
	for _, path := range requiresReplace {
		requiresNew.add(pathString(path))
	}

	d := &differ{
		result:      map[string]shim.ResourceAttrDiff{},
		requiresNew: requiresNew,
	}
	d.updateValue("", prior, planned, false)
	return d.result, d.isRequiresNew
}

func setIndex(val cty.Value) string {
	hash := val.Hash()
	if hash < 0 {
		hash = -hash
	}
	index := strconv.FormatInt(int64(hash), 10)
	if !val.IsWhollyKnown() {
		index = "~" + index
	}
	return index
}

func (d *differ) extendPath(path string, index interface{}) string {
	if path == "" {
		return fmt.Sprintf("%v", index)
	}
	return fmt.Sprintf("%v.%v", path, index)
}

func (d *differ) setDiff(path string, diff shim.ResourceAttrDiff) {
	if diff.RequiresNew {
		d.isRequiresNew = true
	}
	if existing, ok := d.result[path]; ok {
		if existing.Old == "" {
			existing.Old = diff.Old
		}
		if existing.New == "" {
			existing.New = diff.New
		}
		if existing.New != "" && existing.NewRemoved {
			existing.NewRemoved = false
		}
		d.result[path] = existing
	} else {
		d.result[path] = diff
	}
}

func (d *differ) addValue(path string, value cty.Value, requiresNew bool) {
	if value.IsNull() {
		return
	}

	requiresNew = requiresNew || d.requiresNew.has(path)

	switch {
	case value.Type().IsPrimitiveType():
		d.setDiff(path, shim.ResourceAttrDiff{
			New:         primitiveString(value),
			RequiresNew: requiresNew,
		})
	case value.Type().IsListType(), value.Type().IsTupleType():
		if !value.IsKnown() {
			d.addValue(d.extendPath(path, "#"), cty.UnknownVal(cty.Number), requiresNew)
			return
		}

		d.addValue(d.extendPath(path, "#"), value.Length(), requiresNew)
		rangeValue(value, func(i, element cty.Value) {
			index, _ := i.AsBigFloat().Int64()
			d.addValue(d.extendPath(path, int(index)), element, requiresNew)
		})
	case value.Type().IsSetType():
		if !value.IsKnown() {
			d.addValue(d.extendPath(path, "#"), cty.UnknownVal(cty.Number), requiresNew)
			return
		}

		d.addValue(d.extendPath(path, "#"), value.Length(), requiresNew)
		rangeValue(value, func(_, element cty.Value) {
			d.addValue(d.extendPath(path, setIndex(element)), element, requiresNew)
		})
	case value.Type().IsMapType():
		if !value.IsKnown() {
			d.addValue(d.extendPath(path, "%"), cty.UnknownVal(cty.Number), requiresNew)
			return
		}

		d.addValue(d.extendPath(path, "%"), value.Length(), requiresNew)
		rangeValue(value, func(key, value cty.Value) {
			contract.Assert(key.Type() == cty.String)
			contract.Assert(key.IsKnown())
			d.addValue(d.extendPath(path, key.AsString()), value, requiresNew)
		})
	case value.Type().IsObjectType():
		if !value.IsKnown() {
			for key, ty := range value.Type().AttributeTypes() {
				d.addValue(d.extendPath(path, key), cty.UnknownVal(ty), requiresNew)
			}
			return
		}

		rangeValue(value, func(key, value cty.Value) {
			d.addValue(d.extendPath(path, key.AsString()), value, requiresNew)
		})
	default:
		contract.Failf("internal error: unexpected value %v", value)
	}
}

func (d *differ) removeValue(path string, value cty.Value, requiresNew bool) {
	requiresNew = requiresNew || d.requiresNew.has(path)

	if value.IsNull() {
		d.setDiff(path, shim.ResourceAttrDiff{
			NewRemoved:  true,
			RequiresNew: requiresNew,
		})
	}

	switch {
	case value.Type().IsPrimitiveType():
		d.setDiff(path, shim.ResourceAttrDiff{
			Old:         primitiveString(value),
			NewRemoved:  true,
			RequiresNew: requiresNew,
		})
	case value.Type().IsListType(), value.Type().IsTupleType():
		d.removeValue(d.extendPath(path, "#"), value.Length(), requiresNew)
		rangeValue(value, func(i, element cty.Value) {
			index, _ := i.AsBigFloat().Int64()
			d.removeValue(d.extendPath(path, int(index)), element, requiresNew)
		})
	case value.Type().IsSetType():
		d.removeValue(d.extendPath(path, "#"), value.Length(), requiresNew)
		rangeValue(value, func(_, element cty.Value) {
			d.removeValue(d.extendPath(path, setIndex(element)), element, requiresNew)
		})
	case value.Type().IsMapType():
		d.removeValue(d.extendPath(path, "%"), value.Length(), requiresNew)
		rangeValue(value, func(key, value cty.Value) {
			contract.Assert(key.Type() == cty.String)
			contract.Assert(key.IsKnown())
			d.removeValue(d.extendPath(path, key.AsString()), value, requiresNew)
		})
	case value.Type().IsObjectType():
		rangeValue(value, func(key, value cty.Value) {
			d.removeValue(d.extendPath(path, key.AsString()), value, requiresNew)
		})
	default:
		contract.Failf("internal error: unexpected value %v", value)
	}
}

func (d *differ) updateValue(path string, prior, planned cty.Value, requiresNew bool) {
	if planned.IsNull() {
		if !prior.IsNull() {
			d.removeValue(path, prior, requiresNew)
		}
		return
	}
	if prior.IsNull() {
		d.addValue(path, planned, requiresNew)
		return
	}

	requiresNew = requiresNew || d.requiresNew.has(path)

	switch {
	case planned.Type().IsPrimitiveType():
		if prior.Type().IsPrimitiveType() {
			old, new := primitiveString(prior), primitiveString(planned)
			if new != old {
				d.setDiff(path, shim.ResourceAttrDiff{
					Old:         old,
					New:         new,
					NewRemoved:  planned.IsNull(),
					RequiresNew: requiresNew,
				})
			}
		} else {
			d.addValue(path, planned, requiresNew)
			d.removeValue(path, prior, requiresNew)
		}
	case planned.Type().IsListType(), planned.Type().IsTupleType():
		if prior.Type().IsListType() || prior.Type().IsTupleType() {
			if !planned.IsKnown() {
				d.updateValue(d.extendPath(path, "#"), prior.Length(), cty.UnknownVal(cty.Number), requiresNew)
				return
			}

			d.updateValue(d.extendPath(path, "#"), prior.Length(), planned.Length(), requiresNew)

			priorValues, plannedValues := prior.AsValueSlice(), planned.AsValueSlice()
			for i := 0; i < len(priorValues) && i < len(plannedValues); i++ {
				d.updateValue(d.extendPath(path, i), priorValues[i], plannedValues[i], requiresNew)
			}
			for i := len(priorValues); i < len(plannedValues); i++ {
				d.addValue(d.extendPath(path, i), plannedValues[i], requiresNew)
			}
			for i := len(plannedValues); i < len(priorValues); i++ {
				d.removeValue(d.extendPath(path, i), priorValues[i], requiresNew)
			}
		} else {
			d.addValue(path, planned, requiresNew)
			d.removeValue(path, prior, requiresNew)
		}
	case planned.Type().IsSetType():
		if prior.Type().IsSetType() {
			if !planned.IsKnown() {
				d.updateValue(d.extendPath(path, "#"), prior.Length(), cty.UnknownVal(cty.Number), requiresNew)
				return
			}

			d.updateValue(d.extendPath(path, "#"), prior.Length(), planned.Length(), requiresNew)

			priorSet, plannedSet := prior.AsValueSet(), planned.AsValueSet()
			for _, element := range plannedSet.Values() {
				if !priorSet.Has(element) {
					d.addValue(d.extendPath(path, setIndex(element)), element, requiresNew)
				}
			}
			for _, element := range priorSet.Values() {
				if !plannedSet.Has(element) {
					d.removeValue(d.extendPath(path, setIndex(element)), element, requiresNew)
				}
			}
		} else {
			d.addValue(path, planned, requiresNew)
			d.removeValue(path, prior, requiresNew)
		}
	case planned.Type().IsMapType():
		if prior.Type().IsMapType() {
			if !planned.IsKnown() {
				d.updateValue(d.extendPath(path, "%"), prior.Length(), cty.UnknownVal(cty.Number), requiresNew)
				return
			}

			d.updateValue(d.extendPath(path, "%"), prior.Length(), planned.Length(), requiresNew)

			priorMap, plannedMap := prior.AsValueMap(), planned.AsValueMap()
			for key, planned := range plannedMap {
				if prior, ok := priorMap[key]; ok {
					d.updateValue(d.extendPath(path, key), prior, planned, requiresNew)
				} else {
					d.addValue(d.extendPath(path, key), planned, requiresNew)
				}
			}
			for key, prior := range priorMap {
				if _, ok := plannedMap[key]; !ok {
					d.removeValue(d.extendPath(path, key), prior, requiresNew)
				}
			}
		} else {
			d.addValue(path, planned, requiresNew)
			d.removeValue(path, prior, requiresNew)
		}
	case planned.Type().IsObjectType():
		if prior.Type().IsObjectType() {
			if !planned.IsKnown() {
				for key, prior := range prior.AsValueMap() {
					d.updateValue(d.extendPath(path, key), prior, cty.UnknownVal(prior.Type()), requiresNew)
				}
				return
			}

			priorMap, plannedMap := prior.AsValueMap(), planned.AsValueMap()
			for key, planned := range plannedMap {
				if prior, ok := priorMap[key]; ok {
					d.updateValue(d.extendPath(path, key), prior, planned, requiresNew)
				} else {
					d.addValue(d.extendPath(path, key), planned, requiresNew)
				}
			}
			for key, prior := range priorMap {
				if _, ok := plannedMap[key]; !ok {
					d.removeValue(d.extendPath(path, key), prior, requiresNew)
				}
			}
		} else {
			d.addValue(path, planned, requiresNew)
			d.removeValue(path, prior, requiresNew)
		}
	default:
		contract.Failf("internal error: unexpected value %v", planned)
	}
}
