// Copyright 2016-2025, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tfbridge

import (
	"errors"
	"fmt"

	"github.com/hashicorp/go-cty/cty"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/walk"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

type rawStateInflections interface {
	inflection()
}

// Reverses Pulumi MaxItems=1 flattening.
//
// null becomes []
// x becomes [x]
type pluralize struct {
	// Only needed for recovering from a null PropertyValue.
	elementType cty.Type

	// Inflections to apply to `x` before pluralizing.
	inner rawStateInflections
}

func (pluralize) inflection() {}

var _ rawStateInflections = pluralize{}

// To recover nulls to cty.Value they need a type.
type typedNull struct {
	t cty.Type
}

func (typedNull) inflection() {}

var _ rawStateInflections = typedNull{}

// Primarily to distinguish maps from objects when recovering. Stores the type for empty maps.
type mapInflections struct {
	t                  cty.Type
	elementInflections map[resource.PropertyKey]rawStateInflections
}

func (mi *mapInflections) set(key resource.PropertyKey, value rawStateInflections) {
	if value == nil {
		return
	}
	if mi.elementInflections == nil {
		mi.elementInflections = map[resource.PropertyKey]rawStateInflections{}
	}
	mi.elementInflections[key] = value
}

func (mapInflections) inflection() {}

var _ rawStateInflections = mapInflections{}

// Distinguish objects from maps when recovering, record renamed properties.
type objInflections struct {
	ignored            map[resource.PropertyKey]struct{}
	renamed            map[resource.PropertyKey]string
	elementInflections map[resource.PropertyKey]rawStateInflections
}

func (oi *objInflections) set(key string, propertyKey resource.PropertyKey, infl rawStateInflections) {
	if string(propertyKey) != key {
		if oi.renamed == nil {
			oi.renamed = make(map[resource.PropertyKey]string)
		}
		oi.renamed[propertyKey] = key
	}
	if infl == nil {
		return
	}
	if oi.elementInflections == nil {
		oi.elementInflections = map[resource.PropertyKey]rawStateInflections{}
	}
	oi.elementInflections[propertyKey] = infl
}

func (oi *objInflections) ignore(key resource.PropertyKey) {
	if oi.ignored == nil {
		oi.ignored = map[resource.PropertyKey]struct{}{}
	}
	oi.ignored[key] = struct{}{}
}

func (objInflections) inflection() {}

var _ rawStateInflections = objInflections{}

// Exists to encode inner inflections on array elements. Stores the type for empty arrays.
type arrayInflections struct {
	t                  cty.Type
	elementInflections map[int]rawStateInflections
}

func (ai *arrayInflections) set(key int, value rawStateInflections) {
	if value == nil {
		return
	}
	if ai.elementInflections == nil {
		ai.elementInflections = map[int]rawStateInflections{}
	}
	ai.elementInflections[key] = value
}

func (arrayInflections) inflection() {}

var _ rawStateInflections = arrayInflections{}

func rawStateRecover(pv resource.PropertyValue, infl rawStateInflections) (cty.Value, error) {
	switch infl := infl.(type) {
	case nil:
		return rawStateRecoverNatural(pv)
	case typedNull:
		if !pv.IsNull() {
			return cty.Value{}, errors.New("expected PropertyValue to be Null")
		}
		return cty.NullVal(infl.t), nil
	case pluralize:
		switch {
		case pv.IsNull():
			return cty.ListValEmpty(infl.elementType), nil
		default:
			v, err := rawStateRecover(pv, infl.inner)
			if err != nil {
				return cty.Value{}, err
			}
			return cty.ListVal([]cty.Value{v}), nil
		}
	case mapInflections:
		if !pv.IsObject() {
			return cty.Value{}, errors.New("expected PropertyValue to be an Object encoding a map")
		}
		pm := pv.ObjectValue()
		recovered := map[string]cty.Value{}
		for k, v := range pm {
			elementInfl := infl.elementInflections[k]
			element, err := rawStateRecover(v, elementInfl)
			if err != nil {
				return cty.Value{}, err
			}
			recovered[string(k)] = element
		}
		if len(recovered) == 0 {
			return cty.MapValEmpty(infl.t), nil
		}
		return cty.MapVal(recovered), nil
	case objInflections:
		if !pv.IsObject() {
			return cty.Value{}, errors.New(
				"expected PropertyValue to be an Object encoding a cty.Value object",
			)
		}
		pm := pv.ObjectValue()
		recovered := map[string]cty.Value{}
		for k, v := range pm {
			if infl.ignored != nil {
				if _, ign := infl.ignored[k]; ign {
					continue
				}
			}
			name, gotName := infl.renamed[k]
			if !gotName {
				name = string(k)
			}
			elementInfl := infl.elementInflections[k]
			element, err := rawStateRecover(v, elementInfl)
			if err != nil {
				return cty.Value{}, err
			}
			recovered[name] = element
		}
		if len(recovered) == 0 {
			return cty.EmptyObjectVal, nil
		}
		return cty.ObjectVal(recovered), nil
	case arrayInflections:
		if !pv.IsArray() {
			return cty.Value{}, errors.New("expected PropertyValue to be an Array")
		}
		arr := pv.ArrayValue()
		n := len(arr)
		for k := range infl.elementInflections {
			if k < 0 || k >= n {
				return cty.Value{}, fmt.Errorf("Invalid array inflection index %d", k)
			}
		}
		if n == 0 {
			return cty.ListValEmpty(infl.t), nil
		}
		var elements []cty.Value
		for k, v := range arr {
			r, err := rawStateRecover(v, infl.elementInflections[k])
			if err != nil {
				return cty.Value{}, err
			}
			elements = append(elements, r)
		}
		return cty.ListVal(elements), nil
	default:
		contract.Failf("rawStateRecover does not recognize this rawStateInflections case")
		return cty.Value{}, errors.New("impossible")
	}
}

func rawStateRecoverNatural(pv resource.PropertyValue) (cty.Value, error) {
	switch {
	case pv.IsString():
		return cty.StringVal(pv.StringValue()), nil
	case pv.IsBool():
		return cty.BoolVal(pv.BoolValue()), nil
	case pv.IsNumber():
		n := pv.NumberValue()
		return cty.NumberFloatVal(n), nil
	case pv.IsArray():
		var elements []cty.Value
		for _, v := range pv.ArrayValue() {
			vv, err := rawStateRecoverNatural(v)
			if err != nil {
				return cty.Value{}, err
			}
			elements = append(elements, vv)
		}
		return cty.ListVal(elements), nil
	case pv.IsObject():
		return cty.Value{}, errors.New(
			"rawStateRecoverNatural cannot process Object values due to map vs object confusion",
		)
	case pv.IsNull():
		return cty.Value{}, errors.New(
			"rawStateRecoverNatural cannot process Null values as they require a type in cty.Value",
		)
	case pv.IsArchive():
		return cty.Value{}, errors.New("rawStateRecoverNatural cannot process Archive values")
	case pv.IsAsset():
		return cty.Value{}, errors.New("rawStateRecoverNatural cannot process Asset values")
	case pv.IsComputed():
		return cty.Value{}, errors.New("rawStateRecoverNatural cannot process Computed values")
	case pv.IsResourceReference():
		return cty.Value{}, errors.New("rawStateRecoverNatural cannot process ResourceReference values")
	case pv.IsSecret():
		return cty.Value{}, errors.New("rawStateRecoverNatural cannot process Secret values")
	default:
		contract.Failf("rawStateRecoverNatural does not recognize this PropertyValue case")
		return cty.Value{}, errors.New("impossible")
	}
}

type inflectHelper struct {
	schemaMap   shim.SchemaMap         // top-level schema for a resource
	schemaInfos map[string]*SchemaInfo // top-level schema overrides for a resource
}

func (ih *inflectHelper) inflections(
	path walk.SchemaPath,
	pv resource.PropertyValue,
	v cty.Value,
) (rawStateInflections, error) {
	contract.Assertf(v.IsKnown(), "rawStateComputeInflections cannot handle unknowns")
	switch {
	case v.IsNull():
		return &typedNull{t: v.Type()}, nil
	case v.Type().IsPrimitiveType():
		return nil, nil
	case v.Type().IsListType():
		elements := v.AsValueSlice()

		// Checking if [] got encoded as Null due to MaxItems=1.
		if len(elements) == 0 && pv.IsNull() {
			return pluralize{elementType: v.Type().ElementType()}, nil
		}

		// Checking if [x] got encoded as x due to MaxItems=1.
		if len(elements) == 1 && !pv.IsArray() {
			subPath := path.Element()
			inner, err := ih.inflections(subPath, pv, elements[0])
			if err != nil {
				return nil, err
			}
			return pluralize{inner: inner}, nil
		}

		// Otherwise PropertyValue should be an array just like the cty.Value is a list.
		contract.Assertf(pv.IsArray(), "Expected an Array PropertyValue to match a List cty.Value")

		pvElements := pv.ArrayValue()

		contract.Assertf(len(pvElements) == len(elements),
			"Expected array length parity for PropertyValue and matching cty.Value")

		if len(pvElements) == 0 {
			return arrayInflections{t: v.Type()}, nil
		}

		arrayInfl := arrayInflections{}

		subPath := path.Element()
		for k, e := range elements {
			infl, err := ih.inflections(subPath, pvElements[k], e)
			if err != nil {
				return nil, err
			}
			arrayInfl.set(k, infl)
		}

		return arrayInfl, nil
	case v.Type().IsMapType():
		elements := v.AsValueMap()
		contract.Assertf(pv.IsObject(), "Expected an Object PropertyValue to match a Map cty.Value")

		pvElements := pv.ObjectValue()

		contract.Assertf(len(pvElements) == len(elements),
			"Expected map length parity for PropertyValue and matching cty.Value")

		if len(pvElements) == 0 {
			return mapInflections{t: v.Type()}, nil
		}

		mapInfl := mapInflections{}

		subPath := path.Element()
		for k, e := range elements {
			key := resource.PropertyKey(k)
			infl, err := ih.inflections(subPath, pvElements[key], e)
			if err != nil {
				return nil, err
			}
			mapInfl.set(key, infl)
		}

		return mapInfl, nil
	case v.Type().IsObjectType():
		elements := v.AsValueMap()

		pvElements := pv.ObjectValue()

		if len(pvElements) == 0 {
			infl := objInflections{}
			for k := range pvElements {
				infl.ignore(k)
			}
			return infl, nil
		}

		infl := objInflections{}

		keySetWithCtyValueMatches := map[resource.PropertyKey]struct{}{}

		for k, v := range elements {
			subPath := path.GetAttr(k)
			keyRaw, err := TerraformToPulumiNameAtPath(path, ih.schemaMap, ih.schemaInfos)
			if err != nil {
				return nil, err
			}
			key := resource.PropertyKey(keyRaw)
			kInfl, err := ih.inflections(subPath, pvElements[key], v)
			if err != nil {
				return nil, err
			}
			keySetWithCtyValueMatches[key] = struct{}{}
			infl.set(k, key, kInfl)
		}

		for k := range pvElements {
			if _, ok := keySetWithCtyValueMatches[k]; !ok {
				infl.ignore(k)
			}
		}

		return infl, nil
	case v.Type().IsSetType():
		panic("TODO")
	case v.Type().IsTupleType():
		panic("TODO")
	case v.Type().IsCapsuleType():
		return nil, errors.New("cty.Value CapsuleType is not supported")
	default:
		contract.Failf("rawStateComputeInflections does not recognize this cty.Value case")
		return nil, errors.New("impossible")
	}
}
