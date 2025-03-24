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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"os"

	"github.com/hashicorp/go-cty/cty"
	ctyjson "github.com/hashicorp/go-cty/cty/json"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/log"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/walk"
	"github.com/pulumi/pulumi-terraform-bridge/v3/unstable/propertyvalue"
)

// rawStateDeltaKey is where [rawStateDelta] is stored under [metaKey].
const rawStateDeltaKey = "__pulumi_raw_state_delta"

// Represents an efficient delta between PropertyMap and cty.Value representations of resource state.
//
// Pulumi stores Terraform resource states as PropertyMap in the state file. But providers expect to interact with
// cty.Value versions of the state. This is especially pertinent when upgrading provider versions. The new Terraform
// provider code on V-next expects to receive the verbatim raw state JSON written by V-prev and possibly massage it
// with state upgrade code toward the newly changed schema.
//
// cty.Value has slightly more information that strictly necessary but is taken as an approximation for the Terraform
// raw state in this code.
//
// It is not possible to reconstruct cty.Value from PropertyMap because the transformation is schema aware, and the
// V-prev schema is not available at read time.
//
// It should be possible to store cty.Value alongside PropertyMap in Pulumi state, but this approach would require
// additional storage. More pertinently, it would complicate editing state files in scenarios requiring state repair.
//
// The code takes a hybrid approach instead. It computes a RawStateDelta representing the difference between a
// PropertyMap and a cty.Value, so that cty.Value need not be stored itself. In typical scenarios RawStateDelta is
// fairly small. It falls back to storing the entirety of cty.Value if an efficient encoding cannot be computed.
//
// At read time, PropertyMap and RawStateDelta are read, and the original cty.Value is reconstructed from the pair.
type RawStateDelta struct {
	TypedNull *typedNullDelta `json:"null,omitempty"`
	Pluralize *pluralizeDelta `json:"plu,omitempty"`
	Map       *mapDelta       `json:"map,omitempty"`
	Obj       *objDelta       `json:"obj,omitempty"`
	Array     *arrayDelta     `json:"arr,omitempty"`
	Set       *setDelta       `json:"set,omitempty"`
	Asset     *assetDelta     `json:"asset,omitempty"`
	Num       *numDelta       `json:"num,omitempty"`
	Replace   *replaceDelta   `json:"replace,omitempty"`
}

func (d RawStateDelta) Recover(pv resource.PropertyValue) (cty.Value, error) {
	return rawStateRecover(pv, d)
}

func (d RawStateDelta) isEmpty() bool {
	if d.Pluralize != nil {
		return false
	}
	if d.TypedNull != nil {
		return false
	}
	if d.Map != nil {
		return false
	}
	if d.Obj != nil {
		return false
	}
	if d.Array != nil {
		return false
	}
	if d.Set != nil {
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

func (d RawStateDelta) ToPropertyValue() resource.PropertyValue {
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
			if _, ok := i["replacement"]; ok {
				// the replaceDelta case needs to be secreted.
				return resource.MakeSecret(resource.NewPropertyValue(i)), true
			}
		}
		return resource.PropertyValue{}, false
	}

	return resource.NewPropertyValueRepl(result, nil /*replk*/, replv)
}

func NewRawStateDeltaFromPropertyValue(pv resource.PropertyValue) (RawStateDelta, error) {
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
	// Only needed for recovering from a null PropertyValue.
	ElementType *cty.Type `json:"t,omitempty"`

	// Delta to apply to `x` before pluralizing.
	Inner RawStateDelta `json:"i,omitempty"`

	// This is a set and not a list.
	IsSet bool `json:"set,omitempty"`
}

// To recover nulls to cty.Value they need a type.
type typedNullDelta struct {
	T cty.Type `json:"t"`
}

// Primarily to distinguish maps from objects when recovering. Stores the type for empty maps.
type mapDelta struct {
	T             *cty.Type                              `json:"t,omitempty"`
	ElementDeltas map[resource.PropertyKey]RawStateDelta `json:"m,omitempty"`
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

// Distinguish objects from maps when recovering, record renamed properties.
type objDelta struct {
	Ignored map[resource.PropertyKey]struct{} `json:"ignored,omitempty"`

	// Store a TF property name for non-typical properties.
	//
	// For typical properties, [PulumiToTerraformName] without any schema will compute the matching TF name. These
	// are omitted to minimize the payload. All other property names are stored under [Renamed].
	Renamed map[resource.PropertyKey]string `json:"renamed,omitempty"`

	ElementDeltas map[resource.PropertyKey]RawStateDelta `json:"o,omitempty"`
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
	if oi.ElementDeltas == nil {
		oi.ElementDeltas = map[resource.PropertyKey]RawStateDelta{}
	}
	oi.ElementDeltas[propertyKey] = infl
}

func (oi *objDelta) ignore(key resource.PropertyKey) {
	if oi.Ignored == nil {
		oi.Ignored = map[resource.PropertyKey]struct{}{}
	}
	oi.Ignored[key] = struct{}{}
}

// Exists to encode inner deltas on array elements. Stores the type for empty arrays.
type arrayDelta struct {
	T             *cty.Type             `json:"t,omitempty"`
	ElementDeltas map[int]RawStateDelta `json:"arr"`
}

func (ai *arrayDelta) set(key int, value RawStateDelta) {
	if value.isEmpty() {
		return
	}
	if ai.ElementDeltas == nil {
		ai.ElementDeltas = map[int]RawStateDelta{}
	}
	ai.ElementDeltas[key] = value
}

// Exists to encode inner deltas on set elements. Stores the type for empty sets.
type setDelta struct {
	T *cty.Type `json:"t,omitempty"`

	// Key assumption here is that no re-ordering is possible here.
	//
	// Alternatively we could index on hashes of PropertyValue, and check for hash-collisions at write time.
	ElementDeltas map[int]RawStateDelta `json:"set"`
}

func (ai *setDelta) set(key int, value RawStateDelta) {
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
// carries the raw cty.Value as it was encountered. NOTE that this can leak sensitive information to the state and must
// be secreted.
type replaceDelta struct {
	T cty.Type        `json:"t"`
	V json.RawMessage `json:"replacement"`
}

func (d replaceDelta) Value() cty.Value {
	value, err := ctyjson.Unmarshal(d.V, d.T)
	contract.AssertNoErrorf(err, "replaceDelta failed to unmarshal")
	return value
}

func newReplaceDelta(value cty.Value) *replaceDelta {
	bytes, err := ctyjson.Marshal(value, value.Type())
	contract.AssertNoErrorf(err, "replaceDelta failed to marshal")
	return &replaceDelta{V: bytes, T: value.Type()}
}

func rawStateRecover(pv resource.PropertyValue, infl RawStateDelta) (cty.Value, error) {
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
	case infl.Replace != nil:
		return infl.Replace.Value(), nil
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
			elementInfl := infl.Map.ElementDeltas[k]
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
			if k == metaKey || k == rawStateDeltaKey {
				continue
			}

			if infl.Obj.Ignored != nil {
				if _, ign := infl.Obj.Ignored[k]; ign {
					continue
				}
			}
			name, gotName := infl.Obj.Renamed[k]
			if !gotName {
				name = PulumiToTerraformName(string(k), nil, nil)
			}
			elementInfl := infl.Obj.ElementDeltas[k]
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
		for k := range infl.Array.ElementDeltas {
			if k < 0 || k >= n {
				return cty.Value{}, fmt.Errorf("Invalid array delta index %d", k)
			}
		}
		if n == 0 {
			return cty.ListValEmpty(*infl.Array.T), nil
		}
		var elements []cty.Value
		for k, v := range arr {
			r, err := rawStateRecover(v, infl.Array.ElementDeltas[k])
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
		for k := range infl.Set.ElementDeltas {
			if k < 0 || k >= n {
				return cty.Value{}, fmt.Errorf("Invalid set delta index %d", k)
			}
		}
		if n == 0 {
			return cty.SetValEmpty(*infl.Set.T), nil
		}
		var elements []cty.Value
		for k, v := range arr {
			r, err := rawStateRecover(v, infl.Set.ElementDeltas[k])
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

	case infl.Num != nil:
		if !pv.IsString() {
			return cty.Value{}, errors.New("Expected PropertyValue to be a String")
		}
		v, err := cty.ParseNumberVal(pv.StringValue())
		if err != nil {
			return cty.Value{}, fmt.Errorf("Foo: %w", err)
		}
		return v, nil

	default:
		contract.Failf("rawStateRecover does not recognize this rawStateDelta case")
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

func RawStateComputeDelta(
	ctx context.Context,
	schemaMap shim.SchemaMap, // top-level schema for a resource
	schemaInfos map[string]*SchemaInfo, // top-level schema overrides for a resource
	outMap resource.PropertyMap,
	rawState cty.Value,
) RawStateDelta {
	ih := &rawStateDeltaHelper{
		schemaMap:   schemaMap,
		schemaInfos: schemaInfos,
		logger:      log.TryGetLogger(ctx),
	}
	pv := resource.NewObjectProperty(outMap)
	delta := ih.delta(pv, rawState)

	err := rawStateTurnaroundCheck(rawState, pv, delta)
	contract.AssertNoErrorf(err, "rawstate: failed computing delta")

	return delta
}

func rawStateTurnaroundCheck(rawState cty.Value, pv resource.PropertyValue, infl RawStateDelta) error {
	mm := rawState.AsValueMap()
	delete(mm, "timeouts")
	rawStateWithoutTimeouts := cty.ObjectVal(mm)

	// Double-check that recovering the cty.Value works as expected, before it is written to the state.
	ctyValueRecovered, err := rawStateRecover(pv, infl)
	if err != nil {
		return fmt.Errorf("[rawstate]: failed recovering value for turnaround check: %w", err)
	}

	if !rawStateReducePrecision(ctyValueRecovered).RawEquals(
		rawStateReducePrecision(rawStateWithoutTimeouts),
	) {
		if cmdutil.IsTruthy(os.Getenv("PULUMI_DEBUG")) {
			return fmt.Errorf("[rawstate]: turnaround check failed\nrecovered=%s\n"+
				"rawState =%s\ndelta=%#v",
				ctyValueRecovered.GoString(),
				rawStateWithoutTimeouts.GoString(),
				infl.ToPropertyValue().String(),
			)
		}
		return errors.New("[rawstate]: turnaround check failed")
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

func (ih *rawStateDeltaHelper) delta(pv resource.PropertyValue, v cty.Value) RawStateDelta {
	return ih.deltaAt(walk.NewSchemaPath(), pv, v)
}

func (ih *rawStateDeltaHelper) deltaAt(
	path walk.SchemaPath,
	pv resource.PropertyValue,
	v cty.Value,
) RawStateDelta {
	delta, err := ih.computeDeltaAt(path, pv, v)
	if err == nil {
		return delta
	}
	if ih.logger != nil {
		ih.logger.Debug(fmt.Sprintf("[rawstate] Failed computing delta at %q for pv=%s and v=%s: %v",
			path.MustEncodeSchemaPath(),
			pv.String(),
			v.GoString(),
			err,
		))
	}
	return RawStateDelta{Replace: newReplaceDelta(v)}
}

func (ih *rawStateDeltaHelper) computeDeltaAt(
	path walk.SchemaPath,
	pv resource.PropertyValue,
	v cty.Value,
) (RawStateDelta, error) {
	if pv.IsSecret() {
		return ih.deltaAt(path, pv.SecretValue().Element, v), nil
	}
	if pv.IsOutput() && pv.OutputValue().Known {
		return ih.deltaAt(path, pv.OutputValue().Element, v), nil
	}
	isUnknown := pv.IsComputed() || pv.IsOutput() && !pv.OutputValue().Known
	contract.Assertf(!isUnknown, "rawStateDeltaHelper cannot process unknown PropertyValue values")
	contract.Assertf(v.IsKnown(), "rawStateDeltaHelper cannot process unknown cty.Value values")

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
	case v.IsNull():
		return RawStateDelta{TypedNull: &typedNullDelta{T: v.Type()}}, nil
	case v.Type().Equals(cty.Number) && pv.IsString():
		return RawStateDelta{Num: &numDelta{}}, nil
	case v.Type().IsPrimitiveType():
		return RawStateDelta{}, nil
	case v.Type().IsListType():
		elements := v.AsValueSlice()

		// Checking if [] got encoded as Null due to MaxItems=1.
		if len(elements) == 0 && pv.IsNull() {
			t := v.Type().ElementType()
			return RawStateDelta{Pluralize: &pluralizeDelta{ElementType: &t}}, nil
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
			return RawStateDelta{
				Array: &arrayDelta{T: v.Type().ListElementType()},
			}, nil
		}

		arrayInfl := arrayDelta{}

		subPath := path.Element()
		for k, e := range elements {
			infl := ih.deltaAt(subPath, pvElements[k], e)
			arrayInfl.set(k, infl)
		}

		return RawStateDelta{Array: &arrayInfl}, nil
	case v.Type().IsMapType():
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
			return RawStateDelta{Map: &mapDelta{T: v.Type().MapElementType()}}, nil
		}

		mapInfl := mapDelta{}

		subPath := path.Element()
		for k, e := range elements {
			key := resource.PropertyKey(k)
			infl := ih.deltaAt(subPath, pvElements[key], e)
			mapInfl.set(key, infl)
		}

		return RawStateDelta{Map: &mapInfl}, nil

	case v.Type().IsSetType():
		// Key assumption here is that when Pulumi translates Set values in states and projects them as Array
		// PropertyValue, the Array preserves the original ordering.

		elements := v.AsValueSlice()

		// MaxItems=1 handling is exactly similar to lists.
		//
		// Checking if [] got encoded as Null due to MaxItems=1.
		if len(elements) == 0 && pv.IsNull() {
			t := v.Type().ElementType()
			return RawStateDelta{
				Pluralize: &pluralizeDelta{
					ElementType: &t,
					IsSet:       true,
				},
			}, nil
		}

		// Checking if [x] got encoded as x due to MaxItems=1.
		if len(elements) == 1 && !pv.IsArray() {
			subPath := path.Element()
			inner := ih.deltaAt(subPath, pv, elements[0])
			return RawStateDelta{Pluralize: &pluralizeDelta{
				Inner: inner,
				IsSet: true,
			}}, nil
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
			return RawStateDelta{
				Set: &setDelta{T: v.Type().SetElementType()},
			}, nil
		}

		setInfl := setDelta{}

		subPath := path.Element()
		for k, e := range elements {
			infl := ih.deltaAt(subPath, pvElements[k], e)
			setInfl.set(k, infl)
		}

		return RawStateDelta{Set: &setInfl}, nil

	case v.Type().IsObjectType():
		elements := v.AsValueMap()
		pvElements := pv.ObjectValue()

		if len(pvElements) == 0 {
			infl := objDelta{}
			for k := range pvElements {
				if k == metaKey || k == rawStateDeltaKey || k == defaultsKey {
					continue
				}
				infl.ignore(k)
			}
			return RawStateDelta{Obj: &infl}, nil
		}

		infl := objDelta{}

		keySetWithCtyValueMatches := map[resource.PropertyKey]struct{}{}

		for k, v := range elements {
			subPath := path.GetAttr(k)
			keyRaw, err := TerraformToPulumiNameAtPath(subPath, ih.schemaMap, ih.schemaInfos)
			if err != nil {
				return RawStateDelta{}, err
			}
			key := resource.PropertyKey(keyRaw)
			delta := ih.deltaAt(subPath, pvElements[key], v)
			keySetWithCtyValueMatches[key] = struct{}{}
			infl.set(k, key, delta)
		}

		for k := range pvElements {
			if k == metaKey || k == rawStateDeltaKey || k == defaultsKey {
				continue
			}
			if _, ok := keySetWithCtyValueMatches[k]; !ok {
				infl.ignore(k)
			}
		}

		return RawStateDelta{Obj: &infl}, nil
	case v.Type().IsTupleType():
		return RawStateDelta{}, errors.New("cty.Value TupleType is not supported")
	case v.Type().IsCapsuleType():
		return RawStateDelta{}, errors.New("cty.Value CapsuleType is not supported")
	default:
		contract.Failf("rawStateDeltaHelper does not recognize this cty.Value case")
		return RawStateDelta{}, errors.New("impossible")
	}
}
