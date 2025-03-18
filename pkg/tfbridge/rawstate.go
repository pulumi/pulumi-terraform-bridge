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
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"os"

	"github.com/hashicorp/go-cty/cty"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"

	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/walk"
)

// What about type changes like strings to ints and so forth, do we need to account for that?
//
// Could we bail from translating and encode cty.Value as is in the delta?
//
// Should we store the type, is that sufficient to code PropertyValue to cty.Value, to compensate for num/string etc?
type rawStateInflections struct {
	TypedNull *typedNull        `json:"null,omitempty"`
	Pluralize *pluralize        `json:"plu,omitempty"`
	Map       *mapInflections   `json:"map,omitempty"`
	Obj       *objInflections   `json:"obj,omitempty"`
	Array     *arrayInflections `json:"arr,omitempty"`
	Set       *setInflections   `json:"set,omitempty"`
	Asset     *assetInflections `json:"asset,omitempty"`
}

func (i rawStateInflections) isEmpty() bool {
	if i.Pluralize != nil {
		return false
	}
	if i.TypedNull != nil {
		return false
	}
	if i.Map != nil {
		return false
	}
	if i.Obj != nil {
		return false
	}
	if i.Array != nil {
		return false
	}
	if i.Set != nil {
		return false
	}
	if i.Asset != nil {
		return false
	}
	return true
}

// Reverses Pulumi MaxItems=1 flattening.
//
// null becomes []
// x becomes [x]
type pluralize struct {
	// Only needed for recovering from a null PropertyValue.
	ElementType *cty.Type `json:"t,omitempty"`

	// Inflections to apply to `x` before pluralizing.
	Inner rawStateInflections `json:"i,omitempty"`

	// This is a set and not a list.
	IsSet bool `json:"set,omitempty"`
}

// To recover nulls to cty.Value they need a type.
type typedNull struct {
	T cty.Type `json:"t"`
}

// Primarily to distinguish maps from objects when recovering. Stores the type for empty maps.
type mapInflections struct {
	T                  *cty.Type                                    `json:"t,omitempty"`
	ElementInflections map[resource.PropertyKey]rawStateInflections `json:"m,omitempty"`
}

func (mi *mapInflections) set(key resource.PropertyKey, value rawStateInflections) {
	if value.isEmpty() {
		return
	}
	if mi.ElementInflections == nil {
		mi.ElementInflections = map[resource.PropertyKey]rawStateInflections{}
	}
	mi.ElementInflections[key] = value
}

// Distinguish objects from maps when recovering, record renamed properties.
type objInflections struct {
	Ignored            map[resource.PropertyKey]struct{}            `json:"ignored,omitempty"`
	Renamed            map[resource.PropertyKey]string              `json:"renamed,omitempty"`
	ElementInflections map[resource.PropertyKey]rawStateInflections `json:"o,omitempty"`
}

func (oi *objInflections) set(key string, propertyKey resource.PropertyKey, infl rawStateInflections) {
	if string(propertyKey) != key {
		if oi.Renamed == nil {
			oi.Renamed = make(map[resource.PropertyKey]string)
		}
		oi.Renamed[propertyKey] = key
	}
	if infl.isEmpty() {
		return
	}
	if oi.ElementInflections == nil {
		oi.ElementInflections = map[resource.PropertyKey]rawStateInflections{}
	}
	oi.ElementInflections[propertyKey] = infl
}

func (oi *objInflections) ignore(key resource.PropertyKey) {
	if oi.Ignored == nil {
		oi.Ignored = map[resource.PropertyKey]struct{}{}
	}
	oi.Ignored[key] = struct{}{}
}

// Exists to encode inner inflections on array elements. Stores the type for empty arrays.
type arrayInflections struct {
	T                  *cty.Type                   `json:"t,omitempty"`
	ElementInflections map[int]rawStateInflections `json:"arr"`
}

func (ai *arrayInflections) set(key int, value rawStateInflections) {
	if value.isEmpty() {
		return
	}
	if ai.ElementInflections == nil {
		ai.ElementInflections = map[int]rawStateInflections{}
	}
	ai.ElementInflections[key] = value
}

// Exists to encode inner inflections on set elements. Stores the type for empty sets.
type setInflections struct {
	T *cty.Type `json:"t,omitempty"`

	// Key assumption here is that no re-ordering is possible here.
	//
	// Alternatively we could index on hashes of PropertyValue, and check for hash-collisions at write time.
	ElementInflections map[int]rawStateInflections `json:"set"`
}

func (ai *setInflections) set(key int, value rawStateInflections) {
	if value.isEmpty() {
		return
	}
	if ai.ElementInflections == nil {
		ai.ElementInflections = map[int]rawStateInflections{}
	}
	ai.ElementInflections[key] = value
}

// Encodes an AssetTranslation to help with decoding assets and archives.
type assetInflections struct {
	Kind      AssetTranslationKind   `json:"kind"`
	Format    resource.ArchiveFormat `json:"archiveFormat,omitempty"`
	HashField string                 `json:"hashField,omitempty"`
}

func rawStateRecover(pv resource.PropertyValue, infl rawStateInflections) (cty.Value, error) {
	if pv.IsSecret() {
		return rawStateRecover(pv.SecretValue().Element, infl)
	}
	if pv.IsOutput() && pv.OutputValue().Known {
		return rawStateRecover(pv.OutputValue().Element, infl)
	}
	isUnknown := pv.IsComputed() || pv.IsOutput() && !pv.OutputValue().Known
	contract.Assertf(!isUnknown, "rawStateRecover cannot process unknown values")

	switch {
	case infl.isEmpty():
		return rawStateRecoverNatural(pv)
	case infl.TypedNull != nil:
		if !pv.IsNull() {
			return cty.Value{}, errors.New("expected PropertyValue to be Null")
		}
		return cty.NullVal(infl.TypedNull.T), nil
	case infl.Pluralize != nil:
		if infl.Pluralize.IsSet {
			switch {
			case pv.IsNull():
				return cty.SetValEmpty(*infl.Pluralize.ElementType), nil
			default:
				v, err := rawStateRecover(pv, infl.Pluralize.Inner)
				if err != nil {
					return cty.Value{}, err
				}
				return cty.SetVal([]cty.Value{v}), nil
			}
		}
		switch {
		case pv.IsNull():
			return cty.ListValEmpty(*infl.Pluralize.ElementType), nil
		default:
			v, err := rawStateRecover(pv, infl.Pluralize.Inner)
			if err != nil {
				return cty.Value{}, err
			}
			return cty.ListVal([]cty.Value{v}), nil
		}
	case infl.Map != nil:
		if !pv.IsObject() {
			return cty.Value{}, errors.New("expected PropertyValue to be an Object encoding a map")
		}
		pm := pv.ObjectValue()
		recovered := map[string]cty.Value{}
		for k, v := range pm {
			elementInfl := infl.Map.ElementInflections[k]
			element, err := rawStateRecover(v, elementInfl)
			if err != nil {
				return cty.Value{}, err
			}
			recovered[string(k)] = element
		}
		if len(recovered) == 0 {
			return cty.MapValEmpty(*infl.Map.T), nil
		}
		return cty.MapVal(recovered), nil
	case infl.Obj != nil:
		if !pv.IsObject() {
			return cty.Value{}, errors.New(
				"expected PropertyValue to be an Object encoding a cty.Value object",
			)
		}
		pm := pv.ObjectValue()
		recovered := map[string]cty.Value{}
		for k, v := range pm {
			if k == metaKey {
				continue
			}

			if infl.Obj.Ignored != nil {
				if _, ign := infl.Obj.Ignored[k]; ign {
					continue
				}
			}
			name, gotName := infl.Obj.Renamed[k]
			if !gotName {
				name = string(k)
			}
			elementInfl := infl.Obj.ElementInflections[k]
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
	case infl.Array != nil:
		if !pv.IsArray() {
			return cty.Value{}, errors.New("expected PropertyValue to be an Array")
		}
		arr := pv.ArrayValue()
		n := len(arr)
		for k := range infl.Array.ElementInflections {
			if k < 0 || k >= n {
				return cty.Value{}, fmt.Errorf("Invalid array inflection index %d", k)
			}
		}
		if n == 0 {
			return cty.ListValEmpty(*infl.Array.T), nil
		}
		var elements []cty.Value
		for k, v := range arr {
			r, err := rawStateRecover(v, infl.Array.ElementInflections[k])
			if err != nil {
				return cty.Value{}, err
			}
			elements = append(elements, r)
		}
		return cty.ListVal(elements), nil
	case infl.Set != nil:
		if !pv.IsArray() {
			return cty.Value{}, errors.New("expected PropertyValue to be an Array")
		}
		arr := pv.ArrayValue()
		n := len(arr)
		for k := range infl.Set.ElementInflections {
			if k < 0 || k >= n {
				return cty.Value{}, fmt.Errorf("Invalid set inflection index %d", k)
			}
		}
		if n == 0 {
			return cty.SetValEmpty(*infl.Set.T), nil
		}
		var elements []cty.Value
		for k, v := range arr {
			r, err := rawStateRecover(v, infl.Set.ElementInflections[k])
			if err != nil {
				return cty.Value{}, err
			}
			elements = append(elements, r)
		}
		return cty.SetVal(elements), nil

	case infl.Asset != nil:
		at := AssetTranslation{
			Kind:      infl.Asset.Kind,
			Format:    infl.Asset.Format,
			HashField: infl.Asset.HashField,
		}
		var assetOrArchiveValue any
		switch {
		case pv.IsAsset():
			assetValue, err := at.TranslateAsset(pv.AssetValue())
			if err != nil {
				return cty.Value{}, fmt.Errorf("TranslateAsset failed: %w", err)
			}
			assetOrArchiveValue = assetValue
		case pv.IsArchive():
			archiveValue, err := at.TranslateArchive(pv.ArchiveValue())
			if err != nil {
				return cty.Value{}, fmt.Errorf("TranslateArchive failed: %w", err)
			}
			assetOrArchiveValue = archiveValue
		default:
			return cty.Value{}, errors.New("Expected PropertyValue to be an Asset or an Archive")
		}
		return rawStateEncodeAssetOrArhiveValue(assetOrArchiveValue)
	default:
		contract.Failf("rawStateRecover does not recognize this rawStateInflections case")
		return cty.Value{}, errors.New("impossible")
	}
}

func rawStateEncodeAssetOrArhiveValue(value any) (cty.Value, error) {
	switch value := value.(type) {
	case string:
		return cty.StringVal(value), nil
	case []byte:
		return cty.StringVal(string(value)), nil
	default:
		return cty.Value{}, fmt.Errorf("Expected TranslateAsset or TranslateArchive to return string|[]byte")
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
	case pv.IsSecret():
		return rawStateRecoverNatural(pv.SecretValue().Element)
	case pv.IsOutput():
		ov := pv.OutputValue()
		contract.Assertf(ov.Known, "rawStateRecoverNatural cannot process unknowns")
		return rawStateRecoverNatural(ov.Element)
	case pv.IsComputed():
		contract.Failf("rawStateRecoverNatural cannot process Computed values")
		return cty.Value{}, errors.New("rawStateRecoverNatural cannot process Computed values")
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
	case pv.IsResourceReference():
		return cty.Value{}, errors.New("rawStateRecoverNatural cannot process ResourceReference values")
	default:
		contract.Failf("rawStateRecoverNatural does not recognize this PropertyValue case")
		return cty.Value{}, errors.New("impossible")
	}
}

func rawStateComputeInflections(
	schemaMap shim.SchemaMap, // top-level schema for a resource
	schemaInfos map[string]*SchemaInfo, // top-level schema overrides for a resource
	outMap resource.PropertyMap,
	rawState cty.Value,
) (any, error) {
	ih := &inflectHelper{
		schemaMap:   schemaMap,
		schemaInfos: schemaInfos,
	}
	pv := resource.NewObjectProperty(outMap)
	infl, err := ih.inflections(pv, rawState)
	if err != nil {
		return nil, fmt.Errorf("[rawstate]: failed computing inflections: %w", err)
	}

	// Double-check that recovering the cty.Value works as expected, before it is written to the state.
	ctyValueRecovered, err := rawStateRecover(pv, infl)
	if err != nil {
		return nil, fmt.Errorf("[rawstate]: failed recovering value for turnaround check: %w", err)
	}

	mm := rawState.AsValueMap()
	delete(mm, "timeouts")
	rawStateWithoutTimeouts := cty.ObjectVal(mm)

	inflE, err := rawStateEncodeInflections(infl)
	contract.AssertNoErrorf(err, "rawStateEncodeInflections failed")

	if !rawStateReducePrecision(ctyValueRecovered).RawEquals(
		rawStateReducePrecision(rawStateWithoutTimeouts),
	) {
		if cmdutil.IsTruthy(os.Getenv("PULUMI_DEBUG")) {
			return nil, fmt.Errorf("[rawstate]: turnaround check failed\nrecovered=%s\n"+
				"rawState =%s\ninfle=%#v",
				ctyValueRecovered.GoString(),
				rawStateWithoutTimeouts.GoString(),
				inflE,
			)
		}
		return nil, errors.New("[rawstate]: turnaround check failed")
	}

	inflEnc, err := rawStateEncodeInflections(infl)
	if err != nil {
		return nil, fmt.Errorf("[rawstate]: encoding failed")
	}
	return inflEnc, nil
}

// Reduce float precision.
//
// When comparing values for the turnaround check, precision-induced false positives need to be avoided, e.g:
//
//	a := cty.NumberFloatVal(1.1)
//	b := cty.MustParseNumberVal("1.1")
//	a.RawEquals(b) == false
//
// In contrast:
//
//	rawStateReducePrecision(a).RawEquals(rawStateReducePrecision(b)) == true
func rawStateReducePrecision(v cty.Value) cty.Value {
	v2, err := cty.Transform(v, func(p cty.Path, v cty.Value) (cty.Value, error) {
		if v.IsKnown() && v.Type().Equals(cty.Number) {
			bigFloat := v.AsBigFloat()
			bigFloat = bigFloat.SetMode(big.AwayFromZero)
			bigFloat = bigFloat.SetPrec(8)
			return cty.NumberVal(bigFloat), nil
		}
		return v, nil
	})
	if err != nil {
		return v
	}
	return v2
}

type inflectHelper struct {
	schemaMap   shim.SchemaMap         // top-level schema for a resource
	schemaInfos map[string]*SchemaInfo // top-level schema overrides for a resource
}

func (ih *inflectHelper) inflections(pv resource.PropertyValue, v cty.Value) (rawStateInflections, error) {
	return ih.inflectionsAt(walk.NewSchemaPath(), pv, v)
}

func (ih *inflectHelper) inflectionsAt(
	path walk.SchemaPath,
	pv resource.PropertyValue,
	v cty.Value,
) (rawStateInflections, error) {
	if pv.IsSecret() {
		return ih.inflectionsAt(path, pv.SecretValue().Element, v)
	}
	if pv.IsOutput() && pv.OutputValue().Known {
		return ih.inflectionsAt(path, pv.OutputValue().Element, v)
	}
	isUnknown := pv.IsComputed() || pv.IsOutput() && !pv.OutputValue().Known
	contract.Assertf(!isUnknown, "inflectHelper cannot process unknown values")

	// Timeouts are a special property that accidentally gets pushed here for historical reasons; it is not
	// relevant for the permanent RawState storage. Ignore it for now.
	if len(path) == 1 {
		if step, ok := path[0].(walk.GetAttrStep); ok {
			if step.Name == "timeouts" {
				return rawStateInflections{}, nil
			}
		}
	}

	// For assets and archives, save their AssetTranslation, so that at read time this AssetTranslation can be
	// invoked to TranslateAsset or TranslateArchive.
	if pv.IsAsset() || pv.IsArchive() {
		schemaInfo := LookupSchemaInfoMapPath(path, ih.schemaInfos)
		contract.Assertf(schemaInfo != nil && schemaInfo.Asset != nil,
			"Assets must be matched with SchemaInfo with AssetTranslation [%q]",
			path.MustEncodeSchemaPath())
		at := schemaInfo.Asset
		return rawStateInflections{Asset: &assetInflections{
			Kind:      at.Kind,
			Format:    at.Format,
			HashField: at.HashField,
		}}, nil
	}

	contract.Assertf(v.IsKnown(), "rawStateComputeInflections cannot handle unknowns")
	switch {
	case v.IsNull():
		return rawStateInflections{TypedNull: &typedNull{T: v.Type()}}, nil
	case v.Type().IsPrimitiveType():
		return rawStateInflections{}, nil
	case v.Type().IsListType():
		elements := v.AsValueSlice()

		// Checking if [] got encoded as Null due to MaxItems=1.
		if len(elements) == 0 && pv.IsNull() {
			t := v.Type().ElementType()
			return rawStateInflections{Pluralize: &pluralize{ElementType: &t}}, nil
		}

		// Checking if [x] got encoded as x due to MaxItems=1.
		if len(elements) == 1 && !pv.IsArray() {
			subPath := path.Element()
			inner, err := ih.inflectionsAt(subPath, pv, elements[0])
			if err != nil {
				return rawStateInflections{}, err
			}
			return rawStateInflections{Pluralize: &pluralize{Inner: inner}}, nil
		}

		// Otherwise PropertyValue should be an array just like the cty.Value is a list.
		contract.Assertf(pv.IsArray(), "Expected an Array PropertyValue to match a List cty.Value")

		pvElements := pv.ArrayValue()

		contract.Assertf(len(pvElements) == len(elements),
			"Expected array length parity for PropertyValue and matching cty.Value")

		if len(pvElements) == 0 {
			return rawStateInflections{
				Array: &arrayInflections{T: v.Type().ListElementType()},
			}, nil
		}

		arrayInfl := arrayInflections{}

		subPath := path.Element()
		for k, e := range elements {
			infl, err := ih.inflectionsAt(subPath, pvElements[k], e)
			if err != nil {
				return rawStateInflections{}, err
			}
			arrayInfl.set(k, infl)
		}

		return rawStateInflections{Array: &arrayInfl}, nil
	case v.Type().IsMapType():
		elements := v.AsValueMap()
		contract.Assertf(pv.IsObject(), "Expected an Object PropertyValue to match a Map cty.Value")

		pvElements := pv.ObjectValue()

		contract.Assertf(len(pvElements) == len(elements),
			"Expected map length parity for PropertyValue and matching cty.Value")

		if len(pvElements) == 0 {
			t := v.Type()
			return rawStateInflections{Map: &mapInflections{T: &t}}, nil
		}

		mapInfl := mapInflections{}

		subPath := path.Element()
		for k, e := range elements {
			key := resource.PropertyKey(k)
			infl, err := ih.inflectionsAt(subPath, pvElements[key], e)
			if err != nil {
				return rawStateInflections{}, err
			}
			mapInfl.set(key, infl)
		}

		return rawStateInflections{Map: &mapInfl}, nil

	case v.Type().IsSetType():
		// Key assumption here is that when Pulumi translates Set values in states and projects them as Array
		// PropertyValue, the Array preserves the original ordering.

		elements := v.AsValueSlice()

		// MaxItems=1 handling is exactly similar to lists.
		//
		// Checking if [] got encoded as Null due to MaxItems=1.
		if len(elements) == 0 && pv.IsNull() {
			t := v.Type().ElementType()
			return rawStateInflections{
				Pluralize: &pluralize{
					ElementType: &t,
					IsSet:       true,
				},
			}, nil
		}

		// Checking if [x] got encoded as x due to MaxItems=1.
		if len(elements) == 1 && !pv.IsArray() {
			subPath := path.Element()
			inner, err := ih.inflectionsAt(subPath, pv, elements[0])
			if err != nil {
				return rawStateInflections{}, err
			}
			return rawStateInflections{Pluralize: &pluralize{
				Inner: inner,
				IsSet: true,
			}}, nil
		}

		// Otherwise PropertyValue should be an array just like the cty.Value is a set.
		contract.Assertf(pv.IsArray(), "Expected an Array PropertyValue to match a Set cty.Value")

		pvElements := pv.ArrayValue()

		contract.Assertf(len(pvElements) == len(elements),
			"Expected array length parity for PropertyValue and matching Set cty.Value")

		if len(pvElements) == 0 {
			return rawStateInflections{
				Set: &setInflections{T: v.Type().SetElementType()},
			}, nil
		}

		setInfl := setInflections{}

		subPath := path.Element()
		for k, e := range elements {
			infl, err := ih.inflectionsAt(subPath, pvElements[k], e)
			if err != nil {
				return rawStateInflections{}, err
			}
			setInfl.set(k, infl)
		}

		return rawStateInflections{Set: &setInfl}, nil

	case v.Type().IsObjectType():
		elements := v.AsValueMap()
		pvElements := pv.ObjectValue()

		if len(pvElements) == 0 {
			infl := objInflections{}
			for k := range pvElements {
				infl.ignore(k)
			}
			return rawStateInflections{Obj: &infl}, nil
		}

		infl := objInflections{}

		keySetWithCtyValueMatches := map[resource.PropertyKey]struct{}{}

		for k, v := range elements {
			subPath := path.GetAttr(k)
			keyRaw, err := TerraformToPulumiNameAtPath(subPath, ih.schemaMap, ih.schemaInfos)
			if err != nil {
				return rawStateInflections{}, err
			}
			key := resource.PropertyKey(keyRaw)
			kInfl, err := ih.inflectionsAt(subPath, pvElements[key], v)
			if err != nil {
				return rawStateInflections{}, err
			}
			keySetWithCtyValueMatches[key] = struct{}{}
			infl.set(k, key, kInfl)
		}

		for k := range pvElements {
			if _, ok := keySetWithCtyValueMatches[k]; !ok {
				infl.ignore(k)
			}
		}

		return rawStateInflections{Obj: &infl}, nil
	case v.Type().IsTupleType():
		panic("TODO TypeType")
	case v.Type().IsCapsuleType():
		return rawStateInflections{}, errors.New("cty.Value CapsuleType is not supported")
	default:
		contract.Failf("rawStateComputeInflections does not recognize this cty.Value case")
		return rawStateInflections{}, errors.New("impossible")
	}
}

func rawStateParseInflections(rawData any) (rawStateInflections, error) {
	bytes, err := json.Marshal(rawData)
	if err != nil {
		return rawStateInflections{}, err
	}

	var result rawStateInflections
	if err := json.Unmarshal(bytes, &result); err != nil {
		return rawStateInflections{}, nil
	}

	return result, nil
}

func rawStateEncodeInflections(infl rawStateInflections) (any, error) {
	bytes, err := json.Marshal(infl)
	if err != nil {
		return nil, err
	}

	var result any
	if err := json.Unmarshal(bytes, &result); err != nil {
		return nil, err
	}

	return result, nil
}
