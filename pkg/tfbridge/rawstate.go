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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"sort"

	"github.com/hashicorp/go-cty/cty"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/reservedkeys"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/log"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/rawstate"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/walk"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/valueshim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/unstable/propertyvalue"
)

// RawStateDelta encodes how to translate PropertyMap to RawState.
//
// Pulumi stores Terraform resource states as PropertyMap in the state. But providers expect to interact with the
// RawState format. This is especially pertinent when upgrading provider versions. The new Terraform provider code on
// V-next expects to receive the verbatim raw state JSON written by V-prev and possibly massage it with state upgrade
// code toward the newly changed schema.
//
// It is not possible to reconstruct RawState from PropertyMap directly, because the transformation to PropertyMap is
// schema aware and may rename fields or even change their type, and the V-prev schema is not available at read time.
//
// It is possible to store RawState alongside PropertyMap in Pulumi state, but this approach would double the required
// storage. More pertinently, it would complicate editing state files in scenarios requiring state repair.
//
// Instead of storing RawState directly, the chosen approach uses RawStateDelta representing the difference between a
// PropertyMap and RawState, so that RawState need not be stored itself. In typical scenarios RawStateDelta is fairly
// small. It falls back to storing the entirety of RawState if an efficient encoding cannot be computed.
//
// Invariant: this struct represents a serializable union type. At most one of the fields must be set. The zero value
// represents the case where RawState can be naturally reconstructed from PropertyMap without any further info.
type RawStateDelta struct {
	Pluralize  *pluralizeDelta  `json:"plu,omitempty"`
	Map        *mapDelta        `json:"map,omitempty"`
	Obj        *objDelta        `json:"obj,omitempty"`
	ArrayOrSet *arrayOrSetDelta `json:"arr,omitempty"`
	Asset      *assetDelta      `json:"asset,omitempty"`
	Num        *numDelta        `json:"num,omitempty"`
	Replace    *replaceDelta    `json:"replace,omitempty"`
}

// Patches a Pulumi value with the delta to recover the original RawState.
func (d RawStateDelta) Recover(pv resource.PropertyValue) (shim.RawState, error) {
	b, err := d.recoverRepr(pv)
	if err != nil {
		return nil, err
	}
	return shim.RawState(b.Build()), nil
}

func (d RawStateDelta) recoverRepr(pv resource.PropertyValue) (rawstate.Builder, error) {
	if pv.IsSecret() {
		return d.recoverRepr(pv.SecretValue().Element)
	}
	if pv.IsOutput() && pv.OutputValue().Known {
		return d.recoverRepr(pv.OutputValue().Element)
	}
	isUnknown := pv.IsComputed() || pv.IsOutput() && !pv.OutputValue().Known
	contract.Assertf(!isUnknown, "rawStateRecover cannot process unknown values")

	switch {
	case d.isEmpty():
		return rawStateRecoverNatural(pv)
	case d.Replace != nil:
		return rawstate.RawMessage(json.RawMessage(d.Replace.Raw)), nil
	case d.Pluralize != nil:
		switch {
		case pv.IsNull():
			return rawstate.Array(), nil
		default:
			v, err := d.Pluralize.Inner.recoverRepr(pv)
			if err != nil {
				return rawstate.Null(), err
			}
			return rawstate.Array(v), nil
		}
	case d.Map != nil:
		if !pv.IsObject() {
			return rawstate.Null(), errors.New("expected PropertyValue to be an Object encoding a map")
		}
		pm := pv.ObjectValue()
		recovered := map[string]rawstate.Builder{}
		for k, v := range pm {
			element, err := d.Map.ElementDeltas[k].recoverRepr(v)
			if err != nil {
				return rawstate.Null(), err
			}
			recovered[string(k)] = element
		}
		return rawstate.Object(recovered), nil
	case d.Obj != nil:
		if !pv.IsObject() {
			return rawstate.Null(), errors.New(
				"expected PropertyValue to be an Object encoding a Terraform object",
			)
		}
		pm := pv.ObjectValue()
		recovered := map[string]rawstate.Builder{}
		ignoredSet := map[resource.PropertyKey]struct{}{}
		if d.Obj.Ignored != nil {
			for _, ign := range d.Obj.Ignored {
				ignoredSet[ign] = struct{}{}
			}
		}
		for k, v := range pm {
			if reservedkeys.IsBridgeReservedKey(string(k)) {
				continue
			}

			if _, ign := ignoredSet[k]; ign {
				continue
			}
			name, gotName := d.Obj.Renamed[k]
			if !gotName {
				name = PulumiToTerraformName(string(k), nil, nil)
			}
			prop, err := d.Obj.PropertyDeltas[k].recoverRepr(v)
			if err != nil {
				return rawstate.Null(), err
			}
			recovered[name] = prop
		}

		// PropertyDeltas may be written for keys omitted from the map. These are typically the replace delta
		// implying an insertion of an element not present in Pulumi. Apply them to NewNullProperty.
		for k, pd := range d.Obj.PropertyDeltas {
			if _, seen := pm[k]; seen {
				continue
			}
			name, gotName := d.Obj.Renamed[k]
			if !gotName {
				name = PulumiToTerraformName(string(k), nil, nil)
			}
			prop, err := pd.recoverRepr(resource.NewNullProperty())
			if err != nil {
				return rawstate.Null(), err
			}
			recovered[name] = prop
		}

		return rawstate.Object(recovered), nil
	case d.ArrayOrSet != nil:
		if !pv.IsArray() {
			return rawstate.Null(), errors.New("expected PropertyValue to be an Array")
		}
		arr := pv.ArrayValue()
		n := len(arr)
		for k := range d.ArrayOrSet.ElementDeltas {
			if k < 0 || k >= n {
				return rawstate.Null(), fmt.Errorf("Invalid array delta index %d", k)
			}
		}
		if n == 0 {
			return rawstate.Array(), nil
		}
		var elements []rawstate.Builder
		for k, v := range arr {
			r, err := d.ArrayOrSet.ElementDeltas[k].recoverRepr(v)
			if err != nil {
				return rawstate.Null(), err
			}
			elements = append(elements, r)
		}
		return rawstate.Array(elements...), nil
	case d.Asset != nil:
		at := AssetTranslation{
			Kind:      d.Asset.Kind,
			Format:    d.Asset.Format,
			HashField: d.Asset.HashField,
		}
		var assetOrArchiveValue any
		switch {
		case pv.IsAsset():
			assetValue, err := at.TranslateAsset(pv.AssetValue())
			if err != nil {
				return rawstate.Null(), fmt.Errorf("TranslateAsset failed: %w", err)
			}
			assetOrArchiveValue = assetValue
		case pv.IsArchive():
			archiveValue, err := at.TranslateArchive(pv.ArchiveValue())
			if err != nil {
				return rawstate.Null(), fmt.Errorf("TranslateArchive failed: %w", err)
			}
			assetOrArchiveValue = archiveValue
		default:
			return rawstate.Null(), errors.New("Expected PropertyValue to be an Asset or an Archive")
		}
		return rawStateEncodeAssetOrArhiveValue(assetOrArchiveValue)
	case d.Num != nil:
		if !pv.IsString() {
			return rawstate.Null(), errors.New("Expected PropertyValue to be a String")
		}
		v := json.Number(pv.StringValue())
		return rawstate.Number(v), nil
	default:
		contract.Failf("RawStateDelta.Recover does not recognize this rawStateDelta case")
		return rawstate.Null(), errors.New("impossible")
	}
}

func rawStateRecoverNatural(pv resource.PropertyValue) (rawstate.Builder, error) {
	switch {
	case pv.IsString():
		return rawstate.String(pv.StringValue()), nil
	case pv.IsBool():
		return rawstate.Bool(pv.BoolValue()), nil
	case pv.IsNumber():
		return rawstate.Number(json.Number(big.NewFloat(pv.NumberValue()).Text('f', -1))), nil
	case pv.IsSecret():
		return rawStateRecoverNatural(pv.SecretValue().Element)
	case pv.IsOutput():
		ov := pv.OutputValue()
		contract.Assertf(ov.Known, "rawStateRecoverNatural cannot process unknowns")
		return rawStateRecoverNatural(ov.Element)
	case pv.IsComputed():
		contract.Failf("rawStateRecoverNatural cannot process Computed values")
		return rawstate.Null(), errors.New("rawStateRecoverNatural cannot process Computed values")
	case pv.IsArray():
		var elements []rawstate.Builder
		for _, v := range pv.ArrayValue() {
			vv, err := rawStateRecoverNatural(v)
			if err != nil {
				return rawstate.Null(), err
			}
			elements = append(elements, vv)
		}
		return rawstate.Array(elements...), nil
	case pv.IsObject():
		return rawstate.Null(), errors.New(
			"rawStateRecoverNatural cannot process Object values due to map vs object confusion",
		)
	case pv.IsNull():
		return rawstate.Null(), nil
	case pv.IsArchive():
		return rawstate.Null(), errors.New("rawStateRecoverNatural cannot process Archive values")
	case pv.IsAsset():
		return rawstate.Null(), errors.New("rawStateRecoverNatural cannot process Asset values")
	case pv.IsResourceReference():
		return rawstate.Null(), errors.New("rawStateRecoverNatural cannot process ResourceReference values")
	default:
		contract.Failf("rawStateRecoverNatural does not recognize this PropertyValue case")
		return rawstate.Null(), errors.New("impossible")
	}
}

func (d RawStateDelta) isEmpty() bool {
	if d.Pluralize != nil {
		return false
	}
	if d.Map != nil {
		return false
	}
	if d.Obj != nil {
		return false
	}
	if d.ArrayOrSet != nil {
		return false
	}
	if d.Asset != nil {
		return false
	}
	if d.Num != nil {
		return false
	}
	if d.Replace != nil {
		return false
	}
	return true
}

func (d RawStateDelta) Marshal() resource.PropertyValue {
	if d.isEmpty() {
		return resource.NewNullProperty()
	}

	bytes, err := json.Marshal(d)
	contract.AssertNoErrorf(err, "json.Marshal should not fail on rawStateDelta")

	var result any
	err = json.Unmarshal(bytes, &result)
	contract.AssertNoErrorf(err, "json.Unmarshal should not fail on marshalled rawStateDelta")

	replv := func(i interface{}) (resource.PropertyValue, bool) {
		switch i := i.(type) {
		case map[string]any:
			if _, ok := i["replace"]; ok {
				// the replaceDelta case needs to be secreted.
				return resource.MakeSecret(resource.NewPropertyValue(i)), true
			}
		}
		return resource.PropertyValue{}, false
	}

	return resource.NewPropertyValueRepl(result, nil /*replk*/, replv)
}

func UnmarshalRawStateDelta(pv resource.PropertyValue) (RawStateDelta, error) {
	pvNoSecret := propertyvalue.RemoveSecrets(pv)
	bytes, err := json.Marshal(pvNoSecret.Mappable())
	contract.AssertNoErrorf(err, "Failed to json.Marshal(pv.Mappable())")
	var rsd RawStateDelta
	err = json.Unmarshal(bytes, &rsd)
	if err != nil {
		return RawStateDelta{}, err
	}
	return rsd, nil
}

// Reverses Pulumi MaxItems=1 flattening.
//
// null becomes []
// x becomes [x]
type pluralizeDelta struct {
	// Delta to apply to `x` before pluralizing.
	Inner RawStateDelta `json:"i,omitempty"`
}

// Distinguishes maps from objects when recovering. Stores deltas for map elements.
type mapDelta struct {
	ElementDeltas map[resource.PropertyKey]RawStateDelta `json:"el,omitempty"`
}

func (mi *mapDelta) set(key resource.PropertyKey, value RawStateDelta) {
	if value.isEmpty() {
		return
	}
	if mi.ElementDeltas == nil {
		mi.ElementDeltas = map[resource.PropertyKey]RawStateDelta{}
	}
	mi.ElementDeltas[key] = value
}

// Distinguish objects from maps when recovering. Stores deltas for and renaming for object properties.
type objDelta struct {
	// Store properties found in PropertyMap that have no equivalent in Terraform and need ignoring.
	Ignored []resource.PropertyKey `json:"ignored,omitempty"`

	// Store a TF property name for non-typical properties.
	//
	// For typical properties, [PulumiToTerraformName] without any schema will compute the matching TF name. These
	// are omitted to minimize the payload. All other property names are stored under [Renamed].
	Renamed map[resource.PropertyKey]string `json:"renamed,omitempty"`

	// Store deltas for property values.
	PropertyDeltas map[resource.PropertyKey]RawStateDelta `json:"ps,omitempty"`
}

func (oi *objDelta) set(key string, propertyKey resource.PropertyKey, infl RawStateDelta) {
	if PulumiToTerraformName(string(propertyKey), nil, nil) != key {
		if oi.Renamed == nil {
			oi.Renamed = make(map[resource.PropertyKey]string)
		}
		oi.Renamed[propertyKey] = key
	}
	if infl.isEmpty() {
		return
	}
	if oi.PropertyDeltas == nil {
		oi.PropertyDeltas = map[resource.PropertyKey]RawStateDelta{}
	}
	oi.PropertyDeltas[propertyKey] = infl
}

func (oi *objDelta) ignore(key resource.PropertyKey) {
	for _, i := range oi.Ignored {
		if i == key {
			return
		}
	}
	oi.Ignored = append(oi.Ignored, key)
	index := sort.Search(len(oi.Ignored), func(i int) bool { return oi.Ignored[i] >= key })
	oi.Ignored = append(oi.Ignored[:index], append([]resource.PropertyKey{key}, oi.Ignored[index:]...)...)
}

// Encodes inner deltas on array or set elements.
type arrayOrSetDelta struct {
	ElementDeltas map[int]RawStateDelta `json:"el,omitempty"`
}

func (ai *arrayOrSetDelta) set(key int, value RawStateDelta) {
	if value.isEmpty() {
		return
	}
	if ai.ElementDeltas == nil {
		ai.ElementDeltas = map[int]RawStateDelta{}
	}
	ai.ElementDeltas[key] = value
}

// Used when a TF number is expected, but Pulumi representation is a string. This is the case, for example, for large
// integers and floats that do not fit the float64 constraints of Pulumi PropertyValue numbers.
type numDelta struct{}

// Encodes an AssetTranslation to help with decoding assets and archives.
type assetDelta struct {
	Kind      AssetTranslationKind   `json:"kind"`
	Format    resource.ArchiveFormat `json:"archiveFormat,omitempty"`
	HashField string                 `json:"hashField,omitempty"`
}

// Used as fallback when efficient delta computation fails. Ignores any PropertyMap information at this point and
// carries the RawState as it was encountered. NOTE that this can leak sensitive information to the state and must be
// secreted.
type replaceDelta struct {
	Raw shim.RawState `json:"raw"`
}

func rawStateEncodeAssetOrArhiveValue(value any) (rawstate.Builder, error) {
	switch value := value.(type) {
	case string:
		return rawstate.String(value), nil
	case []byte:
		return rawstate.String(string(value)), nil
	default:
		return rawstate.Null(), fmt.Errorf("Expected TranslateAsset or TranslateArchive to return string|[]byte")
	}
}

func RawStateComputeDelta(
	ctx context.Context,
	schemaMap shim.SchemaMap, // top-level schema for a resource
	schemaInfos map[string]*SchemaInfo, // top-level schema overrides for a resource
	outMap resource.PropertyMap,
	v valueshim.Value,
) (RawStateDelta, error) {
	ih := &rawStateDeltaHelper{
		schemaMap:   schemaMap,
		schemaInfos: schemaInfos,
		logger:      log.TryGetLogger(ctx),
	}
	pv := resource.NewObjectProperty(outMap)

	vWithoutTimeouts := v.Remove("timeouts")
	delta := ih.delta(pv, vWithoutTimeouts)
	err := delta.turnaroundCheck(newRawStateFromValue(vWithoutTimeouts), pv)
	if err != nil {
		return RawStateDelta{}, err
	}
	return delta, nil
}

func (d RawStateDelta) turnaroundCheck(rawState shim.RawState, pv resource.PropertyValue) error {
	// Double-check that recovering works as expected, before it is written to the state.
	rawStateRecovered, err := d.Recover(pv)
	if err != nil {
		return fmt.Errorf("failed recovering value for turnaround check: %w", err)
	}

	rawStateWithoutTimeoutsBytes, err := json.Marshal(rawState)
	if err != nil {
		return err
	}

	rawStateRecoveredBytes, err := json.Marshal(rawStateRecovered)
	if err != nil {
		return err
	}

	if !bytes.Equal(rawStateRecoveredBytes, rawStateWithoutTimeoutsBytes) {
		return fmt.Errorf("recovered raw state does not byte-for-byte match the original raw state")
		// return fmt.Errorf("recovered raw state does not byte-for-byte match the original raw state\n"+
		// 	"rawStateWithoutTimeoutsBytes=%s\n"+
		// 	"rawStateRecoveredBytes=%s\n"+
		// 	"pv=%s\n"+
		// 	"delta=%s", string(rawStateWithoutTimeoutsBytes),
		// 	string(rawStateRecoveredBytes),
		// 	pv.String(),
		// 	d.Marshal().String(),
		// )
	}

	return nil
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
		if v.IsKnown() && !v.IsNull() && v.Type().Equals(cty.Number) {
			bigFloat := big.NewFloat(0.).
				Copy(v.AsBigFloat()).
				SetMode(big.AwayFromZero).
				SetPrec(8)
			return cty.NumberVal(bigFloat), nil
		}
		return v, nil
	})
	if err != nil {
		return v
	}
	return v2
}

type rawStateDeltaHelper struct {
	schemaMap   shim.SchemaMap         // top-level schema for a resource
	schemaInfos map[string]*SchemaInfo // top-level schema overrides for a resource
	logger      log.Logger
}

func (ih *rawStateDeltaHelper) delta(pv resource.PropertyValue, v valueshim.Value) RawStateDelta {
	return ih.deltaAt(walk.NewSchemaPath(), pv, v)
}

func (ih *rawStateDeltaHelper) deltaAt(
	path walk.SchemaPath,
	pv resource.PropertyValue,
	v valueshim.Value,
) RawStateDelta {
	delta, err := ih.computeDeltaAt(path, pv, v)
	if err != nil {
		return ih.replaceDeltaAt(path, pv, v, err)
	}
	return delta
}

func (ih *rawStateDeltaHelper) replaceDeltaAt(
	path walk.SchemaPath,
	pv resource.PropertyValue,
	v valueshim.Value,
	err error,
) RawStateDelta {
	if ih.logger != nil {
		ih.logger.Debug(fmt.Sprintf("[rawstate] Failed computing delta at %q for pv=%s and v=%s: %v",
			path.MustEncodeSchemaPath(),
			pv.String(),
			v.GoString(),
			err,
		))
	}
	return RawStateDelta{Replace: &replaceDelta{Raw: newRawStateFromValue(v)}}
}

// Errors returned from this inner function are simply missed opportunities for optimization, as [deltaAt] will always
// compensate for these errors.
func (ih *rawStateDeltaHelper) computeDeltaAt(
	path walk.SchemaPath,
	pv resource.PropertyValue,
	v valueshim.Value,
) (RawStateDelta, error) {
	if pv.IsSecret() {
		return ih.deltaAt(path, pv.SecretValue().Element, v), nil
	}
	if pv.IsOutput() && pv.OutputValue().Known {
		return ih.deltaAt(path, pv.OutputValue().Element, v), nil
	}
	isUnknown := pv.IsComputed() || pv.IsOutput() && !pv.OutputValue().Known
	contract.Assertf(!isUnknown, "rawStateDeltaHelper cannot process unknown PropertyValue values")

	// Timeouts are a special property that accidentally gets pushed here for historical reasons; it is not
	// relevant for the permanent RawState storage. Ignore it for now.
	if len(path) == 1 {
		if step, ok := path[0].(walk.GetAttrStep); ok {
			if step.Name == "timeouts" {
				return RawStateDelta{}, nil
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
		return RawStateDelta{Asset: &assetDelta{
			Kind:      at.Kind,
			Format:    at.Format,
			HashField: at.HashField,
		}}, nil
	}

	switch {
	case v.IsNull() && pv.IsNull():
		return RawStateDelta{}, nil

	case v.Type().IsStringType() && pv.IsString():
		return RawStateDelta{}, nil

	case v.Type().IsBooleanType() && pv.IsBool():
		return RawStateDelta{}, nil

	case v.Type().IsNumberType() && pv.IsNumber():
		return RawStateDelta{}, nil

	case v.Type().IsNumberType() && pv.IsString():
		return RawStateDelta{Num: &numDelta{}}, nil

	case v.Type().IsListType() && !v.IsNull():
		elements := v.AsValueSlice()

		// Checking if [] got encoded as Null due to MaxItems=1.
		if len(elements) == 0 && pv.IsNull() {
			return RawStateDelta{Pluralize: &pluralizeDelta{}}, nil
		}

		// Checking if [x] got encoded as x due to MaxItems=1.
		if len(elements) == 1 && !pv.IsArray() {
			subPath := path.Element()
			inner := ih.deltaAt(subPath, pv, elements[0])
			return RawStateDelta{Pluralize: &pluralizeDelta{Inner: inner}}, nil
		}

		// Otherwise PropertyValue should be an array just like the cty.Value is a list.
		if !pv.IsArray() {
			return RawStateDelta{}, errors.New("Expected an Array PropertyValue to match a List cty.Value")
		}

		pvElements := pv.ArrayValue()

		if len(pvElements) != len(elements) {
			return RawStateDelta{}, errors.New(
				"Expected array length parity for PropertyValue and matching cty.Value",
			)
		}

		if len(pvElements) == 0 {
			return RawStateDelta{ArrayOrSet: &arrayOrSetDelta{}}, nil
		}

		arrayInfl := arrayOrSetDelta{}

		subPath := path.Element()
		for k, e := range elements {
			infl := ih.deltaAt(subPath, pvElements[k], e)
			arrayInfl.set(k, infl)
		}

		return RawStateDelta{ArrayOrSet: &arrayInfl}, nil

	case v.Type().IsMapType() && !v.IsNull():
		elements := v.AsValueMap()
		if !pv.IsObject() {
			return RawStateDelta{}, errors.New("Expected an Object PropertyValue to match a Map cty.Value")
		}

		pvElements := pv.ObjectValue()

		if len(pvElements) != len(elements) {
			return RawStateDelta{}, errors.New(
				"Expected map length parity for PropertyValue and matching cty.Value",
			)
		}

		if len(pvElements) == 0 {
			return RawStateDelta{Map: &mapDelta{}}, nil
		}

		mapInfl := mapDelta{}

		subPath := path.Element()
		for k, e := range elements {
			key := resource.PropertyKey(k)
			pv, ok := pvElements[key]
			if !ok {
				return RawStateDelta{}, fmt.Errorf("Expected matching map keys, missing %q", key)
			}
			infl := ih.deltaAt(subPath, pv, e)
			mapInfl.set(key, infl)
		}

		return RawStateDelta{Map: &mapInfl}, nil

	case v.Type().IsSetType() && !v.IsNull():
		// Key assumption here is that when Pulumi translates Set values in states and projects them as Array
		// PropertyValue, the Array preserves the original ordering.

		elements := v.AsValueSlice()

		// MaxItems=1 handling is exactly similar to lists.
		//
		// Checking if [] got encoded as Null due to MaxItems=1.
		if len(elements) == 0 && pv.IsNull() {
			return RawStateDelta{Pluralize: &pluralizeDelta{}}, nil
		}

		// Checking if [x] got encoded as x due to MaxItems=1.
		if len(elements) == 1 && !pv.IsArray() {
			subPath := path.Element()
			inner := ih.deltaAt(subPath, pv, elements[0])
			return RawStateDelta{Pluralize: &pluralizeDelta{Inner: inner}}, nil
		}

		// Otherwise PropertyValue should be an array just like the cty.Value is a set.
		if !pv.IsArray() {
			return RawStateDelta{}, errors.New("Expected an Array PropertyValue to match a Set cty.Value")
		}

		pvElements := pv.ArrayValue()

		if len(pvElements) != len(elements) {
			return RawStateDelta{}, errors.New(
				"Expected array length parity for PropertyValue and matching Set cty.Value",
			)
		}

		if len(pvElements) == 0 {
			return RawStateDelta{ArrayOrSet: &arrayOrSetDelta{}}, nil
		}

		setInfl := arrayOrSetDelta{}

		subPath := path.Element()
		for k, e := range elements {
			infl := ih.deltaAt(subPath, pvElements[k], e)
			setInfl.set(k, infl)
		}

		return RawStateDelta{ArrayOrSet: &setInfl}, nil

	case v.Type().IsObjectType() && !v.IsNull():
		if !pv.IsObject() {
			return RawStateDelta{}, errors.New("Expected an Object PropertyValue for an Object cty.Value")
		}

		elements := v.AsValueMap()
		pvElements := pv.ObjectValue()
		infl := objDelta{}
		handledKeys := map[resource.PropertyKey]struct{}{}

		for k, v := range elements {
			subPath := path.GetAttr(k)
			keyRaw, err := TerraformToPulumiNameAtPath(subPath, ih.schemaMap, ih.schemaInfos)
			if err != nil {
				return RawStateDelta{}, err
			}
			contract.Assertf(!reservedkeys.IsBridgeReservedKey(keyRaw),
				"Terraform resource uses a key reserved for Pulumi bridged providers: %q", keyRaw)
			key := resource.PropertyKey(keyRaw)

			var delta RawStateDelta
			if _, isIntersectingKey := pvElements[key]; isIntersectingKey {
				delta = ih.deltaAt(subPath, pv, v)
			} else {
				// Missing matching PropertyValue for key, generate a replace delta.
				n := resource.NewNullProperty()
				delta = ih.replaceDeltaAt(subPath, n, v, fmt.Errorf("No PropertyValue at key"))
			}
			infl.set(k, key, delta)
			handledKeys[key] = struct{}{}
		}

		// Any non-reserved keys in pv that are not handled yet should be marked as ignored.
		for k := range pvElements {
			if reservedkeys.IsBridgeReservedKey(string(k)) {
				continue
			}
			if _, ok := handledKeys[k]; !ok {
				infl.ignore(k)
			}
		}

		return RawStateDelta{Obj: &infl}, nil

	default:
		return RawStateDelta{}, fmt.Errorf("no efficient delta for type %s", v.Type().GoString())
	}
}

func newRawStateFromValue(v valueshim.Value) shim.RawState {
	raw, err := v.Marshal()
	contract.AssertNoErrorf(err, "v.Marshal() failed")
	return shim.RawState(raw)
}
