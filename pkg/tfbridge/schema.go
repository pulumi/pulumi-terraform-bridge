// Copyright 2016-2018, Pulumi Corporation.
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
	"fmt"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/golang/glog"
	pbstruct "github.com/golang/protobuf/ptypes/struct"
	"github.com/mitchellh/copystructure"
	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"

	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/schema"
)

// TerraformUnknownVariableValue is the sentinal defined in github.com/hashicorp/terraform/configs/hcl2shim,
// representing a variable whose value is not known at some particular time. The value is duplicated here in
// order to prevent an additional dependency - it is unlikely to ever change upstream since that would break
// rather a lot of things.
const TerraformUnknownVariableValue = "74D93920-ED26-11E3-AC10-0800200C9A66"

// defaultsKey is the name of the input property that is used to track which property keys were populated using
// default values from the resource's schema. This information is used to inform which input properties should be
// populated using old defaults in subsequent updates. When populating the default value for an input property, the
// property's old value will only be used as the default if the property's key is present in the defaults list for
// the old property bag.
const defaultsKey = "__defaults"

// AssetTable is used to record which properties in a call to MakeTerraformInputs were assets so that they can be
// marshaled back to assets by MakeTerraformOutputs.
type AssetTable map[*SchemaInfo]resource.PropertyValue

// nameRequiresDeleteBeforeReplace returns true if the given set of resource inputs includes an autonameable
// property with a value that was not populated by the autonamer.
func nameRequiresDeleteBeforeReplace(inputs resource.PropertyMap,
	tfs shim.SchemaMap, ps map[string]*SchemaInfo) bool {

	defaults, hasDefaults := inputs[defaultsKey]
	if !hasDefaults || !defaults.IsArray() {
		// If there is no list of properties that were populated using defaults, consider the resource autonamed.
		// This avoids setting delete-before-replace for resources that were created before the defaults list existed.
		return false
	}

	hasDefault := map[resource.PropertyKey]bool{}
	for _, key := range defaults.ArrayValue() {
		if !key.IsString() {
			continue
		}
		hasDefault[resource.PropertyKey(key.StringValue())] = true
	}

	for key := range inputs {
		_, _, psi := getInfoFromPulumiName(key, tfs, ps, false)
		if psi != nil && psi.HasDefault() && psi.Default.AutoNamed && !hasDefault[key] {
			return true
		}
	}

	return false
}

func multiEnvDefault(names []string, dv interface{}) interface{} {
	for _, n := range names {
		if v := os.Getenv(n); v != "" {
			return v
		}
	}
	return dv
}

func getSchema(m shim.SchemaMap, key string) shim.Schema {
	if m == nil {
		return nil
	}
	return m.Get(key)
}

func elemSchemas(sch shim.Schema, ps *SchemaInfo) (shim.Schema, *SchemaInfo) {
	var esch shim.Schema
	if sch != nil {
		switch e := sch.Elem().(type) {
		case shim.Schema:
			esch = e
		case shim.Resource:
			esch = (&schema.Schema{Elem: e}).Shim()
		default:
			esch = nil
		}
	}

	var eps *SchemaInfo
	if ps != nil {
		eps = ps.Elem
	}

	return esch, eps
}

type conversionContext struct {
	Instance       *PulumiResource
	ProviderConfig resource.PropertyMap
	ApplyDefaults  bool
	Assets         AssetTable
}

func MakeTerraformInputs(instance *PulumiResource, config resource.PropertyMap, olds, news resource.PropertyMap,
	tfs shim.SchemaMap, ps map[string]*SchemaInfo) (map[string]interface{}, AssetTable, error) {

	ctx := &conversionContext{
		Instance:       instance,
		ProviderConfig: config,
		ApplyDefaults:  true,
		Assets:         AssetTable{},
	}
	inputs, err := ctx.MakeTerraformInputs(olds, news, tfs, ps, false)
	if err != nil {
		return nil, nil, err
	}
	return inputs, ctx.Assets, err
}

// MakeTerraformInput takes a single property plus custom schema info and does whatever is necessary to prepare it for
// use by Terraform.  Note that this function may have side effects, for instance if it is necessary to spill an asset
// to disk in order to create a name out of it.  Please take care not to call it superfluously!
func (ctx *conversionContext) MakeTerraformInput(name string, old, v resource.PropertyValue,
	tfs shim.Schema, ps *SchemaInfo, rawNames bool) (interface{}, error) {

	// For TypeList or TypeSet with MaxItems==1, we will have projected as a scalar nested value, and need to wrap it
	// into a single-element array before passing to Terraform.
	if IsMaxItemsOne(tfs, ps) {
		if old.IsNull() {
			old = resource.NewArrayProperty([]resource.PropertyValue{})
		} else {
			old = resource.NewArrayProperty([]resource.PropertyValue{old})
		}
		if v.IsNull() {
			v = resource.NewArrayProperty([]resource.PropertyValue{})
		} else {
			v = resource.NewArrayProperty([]resource.PropertyValue{v})
		}
	}

	// If there is a custom transform for this value, run it before processing the value.
	if ps != nil && ps.Transform != nil {
		nv, err := ps.Transform(v)
		if err != nil {
			return nil, err
		}
		v = nv
	}

	switch {
	case v.IsNull():
		return nil, nil
	case v.IsBool():
		return v.BoolValue(), nil
	case v.IsNumber():
		if tfs != nil && tfs.Type() == shim.TypeFloat {
			return v.NumberValue(), nil
		}
		return int(v.NumberValue()), nil // convert floats to ints.
	case v.IsString():
		return v.StringValue(), nil
	case v.IsArray():
		var oldArr []resource.PropertyValue
		if old.IsArray() {
			oldArr = old.ArrayValue()
		}

		etfs, eps := elemSchemas(tfs, ps)

		var arr []interface{}
		for i, elem := range v.ArrayValue() {
			var oldElem resource.PropertyValue
			if i < len(oldArr) {
				oldElem = oldArr[i]
			}
			elemName := fmt.Sprintf("%v[%v]", name, i)
			e, err := ctx.MakeTerraformInput(elemName, oldElem, elem, etfs, eps, rawNames)
			if err != nil {
				return nil, err
			}

			if ps != nil && ps.SuppressEmptyMapElements != nil && *ps.SuppressEmptyMapElements {
				if eMap, ok := e.(map[string]interface{}); ok && len(eMap) > 0 {
					arr = append(arr, e)
				}
			} else {
				arr = append(arr, e)
			}
		}
		return arr, nil
	case v.IsAsset():
		// We require that there be asset information, otherwise an error occurs.
		if ps == nil || ps.Asset == nil {
			return nil, errors.Errorf("unexpected asset %s", name)
		} else if !ps.Asset.IsAsset() {
			return nil, errors.Errorf("expected an asset, but %s is not an asset", name)
		}
		if ctx.Assets != nil {
			_, has := ctx.Assets[ps]
			contract.Assertf(!has, "duplicate schema info for asset")
			ctx.Assets[ps] = v
		}
		return ps.Asset.TranslateAsset(v.AssetValue())
	case v.IsArchive():
		// We require that there be archive information, otherwise an error occurs.
		if ps == nil || ps.Asset == nil {
			return nil, errors.Errorf("unexpected archive %s", name)
		}
		if ctx.Assets != nil {
			_, has := ctx.Assets[ps]
			contract.Assertf(!has, "duplicate schema info for asset")
			ctx.Assets[ps] = v
		}
		return ps.Asset.TranslateArchive(v.ArchiveValue())
	case v.IsObject():
		var tfflds shim.SchemaMap
		if tfs != nil {
			if res, isres := tfs.Elem().(shim.Resource); isres {
				tfflds = res.Schema()
			}
		}
		var psflds map[string]*SchemaInfo
		if ps != nil {
			psflds = ps.Fields
		}
		var oldObject resource.PropertyMap
		if old.IsObject() {
			oldObject = old.ObjectValue()
		}

		input, err := ctx.MakeTerraformInputs(oldObject, v.ObjectValue(),
			tfflds, psflds, rawNames || useRawNames(tfs))
		if err != nil {
			return nil, err
		}

		// If we have schema information that indicates that this value is being presented to a map-typed field whose
		// Elem is a shim.Resource, wrap the value in an array in order to work around a bug in Terraform.
		if tfs != nil && tfs.Type() == shim.TypeMap {
			if _, hasResourceElem := tfs.Elem().(shim.Resource); hasResourceElem {
				return []interface{}{input}, nil
			}
		}
		return input, nil
	case v.IsComputed() || v.IsOutput():
		// If any variables are unknown, we need to mark them in the inputs so the config map treats it right.  This
		// requires the use of the special UnknownVariableValue sentinel in Terraform, which is how it internally stores
		// interpolated variables whose inputs are currently unknown.
		return makeTerraformUnknown(tfs), nil
	default:
		contract.Failf("Unexpected value marshaled: %v", v)
		return nil, nil
	}

}

// MakeTerraformInputs takes a property map plus custom schema info and does whatever is necessary
// to prepare it for use by Terraform.  Note that this function may have side effects, for instance
// if it is necessary to spill an asset to disk in order to create a name out of it.  Please take
// care not to call it superfluously!
func (ctx *conversionContext) MakeTerraformInputs(olds, news resource.PropertyMap,
	tfs shim.SchemaMap, ps map[string]*SchemaInfo, rawNames bool) (map[string]interface{}, error) {

	result := make(map[string]interface{})

	// Enumerate the inputs provided and add them to the map using their Terraform names.
	for key, value := range news {
		// If this is a reserved property, ignore it.
		switch key {
		case defaultsKey, metaKey:
			continue
		}

		// First translate the Pulumi property name to a Terraform name.
		name, tfi, psi := getInfoFromPulumiName(key, tfs, ps, rawNames)
		contract.Assert(name != "")

		var old resource.PropertyValue
		if ctx.ApplyDefaults && olds != nil {
			old = olds[key]
		}

		// And then translate the property value.
		v, err := ctx.MakeTerraformInput(name, old, value, tfi, psi, rawNames)
		if err != nil {
			return nil, err
		}
		result[name] = v
		glog.V(9).Infof("Created Terraform input: %v = %v", name, v)
	}

	// Now enumerate and propagate defaults if the corresponding values are still missing.
	if err := ctx.applyDefaults(result, olds, news, tfs, ps, rawNames); err != nil {
		return nil, err
	}

	if glog.V(5) {
		for k, v := range result {
			glog.V(5).Infof("Terraform input %v = %#v", k, v)
		}
	}

	return result, nil

}

func (ctx *conversionContext) applyDefaults(result map[string]interface{}, olds, news resource.PropertyMap,
	tfs shim.SchemaMap, ps map[string]*SchemaInfo, rawNames bool) error {

	if !ctx.ApplyDefaults {
		return nil
	}

	// Create an array to track which properties are defaults.
	newDefaults := []interface{}{}

	// Pull the list of old defaults if any. If there is no list, then we will treat all old values as being usable
	// for new defaults. If there is a list, we will only propagate defaults that were themselves defaults.
	useOldDefault := func(key resource.PropertyKey) bool { return true }
	if oldDefaults, ok := olds[defaultsKey]; ok {
		oldDefaultSet := make(map[resource.PropertyKey]bool)
		for _, k := range oldDefaults.ArrayValue() {
			oldDefaultSet[resource.PropertyKey(k.StringValue())] = true
		}
		useOldDefault = func(key resource.PropertyKey) bool { return oldDefaultSet[key] }
	}

	// Compute any names for which setting a default would cause a conflict.
	conflictsWith := make(map[string]struct{})
	if tfs != nil {
		tfs.Range(func(name string, sch shim.Schema) bool {
			if _, has := result[name]; has {
				for _, conflictingName := range sch.ConflictsWith() {
					conflictsWith[conflictingName] = struct{}{}
				}
			}
			return true
		})
	}

	// First, attempt to use the overlays.
	for name, info := range ps {
		if info.Removed {
			continue
		}
		if _, conflicts := conflictsWith[name]; conflicts {
			continue
		}
		sch := getSchema(tfs, name)
		if sch != nil && (sch.Removed() != "" || sch.Deprecated() != "" && !sch.Required()) {
			continue
		}

		if _, has := result[name]; !has && info.HasDefault() {
			var defaultValue interface{}
			var source string

			// If we already have a default value from a previous version of this resource, use that instead.
			key, tfi, psi := getInfoFromTerraformName(name, tfs, ps, rawNames)
			if old, hasold := olds[key]; hasold && useOldDefault(key) {
				v, err := ctx.MakeTerraformInput(name, resource.PropertyValue{}, old, tfi, psi, rawNames)
				if err != nil {
					return err
				}
				defaultValue, source = v, "old default"
			} else if envVars := info.Default.EnvVars; len(envVars) != 0 {
				var err error
				v := multiEnvDefault(envVars, info.Default.Value)
				if str, ok := v.(string); ok && sch != nil {
					switch sch.Type() {
					case shim.TypeBool:
						v = false
						if str != "" {
							if v, err = strconv.ParseBool(str); err != nil {
								return err
							}
						}
					case shim.TypeInt:
						v = int(0)
						if str != "" {
							iv, iverr := strconv.ParseInt(str, 0, 0)
							if iverr != nil {
								return iverr
							}
							v = int(iv)
						}
					case shim.TypeFloat:
						v = float64(0.0)
						if str != "" {
							if v, err = strconv.ParseFloat(str, 64); err != nil {
								return err
							}
						}
					case shim.TypeString:
						// nothing to do
					default:
						return errors.Errorf("unknown type for default value: %v", sch.Type())
					}
				}
				defaultValue, source = v, "env vars"
			} else if configKey := info.Default.Config; configKey != "" {
				if v := ctx.ProviderConfig[resource.PropertyKey(configKey)]; !v.IsNull() {
					tv, err := ctx.MakeTerraformInput(name, resource.PropertyValue{}, v, tfi, psi, rawNames)
					if err != nil {
						return err
					}
					defaultValue, source = tv, "config"
				}
			} else if info.Default.Value != nil {
				defaultValue, source = info.Default.Value, "Pulumi schema"
			} else if from := info.Default.From; from != nil {
				v, err := from(ctx.Instance)
				if err != nil {
					return err
				}
				defaultValue, source = v, "func"
			}

			if defaultValue != nil {
				glog.V(9).Infof("Created Terraform input: %v = %v (from %s)", name, defaultValue, source)
				result[name] = defaultValue
				newDefaults = append(newDefaults, key)

				// Expand the conflicts map
				if sch != nil {
					for _, conflictingName := range sch.ConflictsWith() {
						conflictsWith[conflictingName] = struct{}{}
					}
				}
			}
		}
	}

	// Next, populate defaults from the Terraform schema.
	if tfs != nil {
		var valueErr error
		tfs.Range(func(name string, sch shim.Schema) bool {
			if sch.Removed() != "" {
				return true
			}
			if sch.Deprecated() != "" && !sch.Required() {
				return true
			}
			if _, conflicts := conflictsWith[name]; conflicts {
				return true
			}

			// If a conflicting field has a default value, don't set the default for the current field
			for _, conflictingName := range sch.ConflictsWith() {
				if conflictingSchema, exists := tfs.GetOk(conflictingName); exists {
					dv, _ := conflictingSchema.DefaultValue()
					if dv != nil {
						return true
					}
				}
			}

			if _, has := result[name]; !has {
				var source string

				// Check for a default value from Terraform. If there is not default from terraform, skip this name.
				dv, err := sch.DefaultValue()
				if err != nil {
					valueErr = err
					return false
				} else if dv == nil {
					return true
				}

				// Next, if we already have a default value from a previous version of this resource, use that instead.
				key, tfi, psi := getInfoFromTerraformName(name, tfs, ps, rawNames)
				if old, hasold := olds[key]; hasold && useOldDefault(key) {
					v, err := ctx.MakeTerraformInput(name, resource.PropertyValue{}, old, tfi, psi, rawNames)
					if err != nil {
						valueErr = err
						return false
					}
					dv, source = v, "old default"
				} else {
					source = "Terraform schema"
				}

				if dv != nil {
					glog.V(9).Infof("Created Terraform input: %v = %v (from %s)", name, dv, source)
					result[name] = dv
					newDefaults = append(newDefaults, key)
				}
			}

			return true
		})
		if valueErr != nil {
			return valueErr
		}
	}

	sort.Slice(newDefaults, func(i, j int) bool {
		return newDefaults[i].(resource.PropertyKey) < newDefaults[j].(resource.PropertyKey)
	})
	result[defaultsKey] = newDefaults

	return nil
}

// makeTerraformUnknownElement creates an unknown value to be used as an element of a list or set using the given
// element schema to guide the shape of the value.
func makeTerraformUnknownElement(elem interface{}) interface{} {
	// If we have no element schema, just return a simple unknown.
	if elem == nil {
		return TerraformUnknownVariableValue
	}

	switch e := elem.(type) {
	case shim.Schema:
		// If the element uses a normal schema, defer to makeTerraformUnknown.
		return makeTerraformUnknown(e)
	case shim.Resource:
		// If the element uses a resource schema, fill in unknown values for any required properties.
		res := make(map[string]interface{})
		e.Schema().Range(func(k string, v shim.Schema) bool {
			if v.Required() {
				res[k] = makeTerraformUnknown(v)
			}
			return true
		})
		return res
	default:
		return TerraformUnknownVariableValue
	}
}

// makeTerraformUnknown creates an unknown value with the shape indicated by the given schema.
//
// It is important that we use the TF schema (if available) to decide what shape the unknown value should have:
// e.g. TF does not play nicely with unknown lists, instead expecting a list of unknowns.
func makeTerraformUnknown(tfs shim.Schema) interface{} {
	if tfs == nil {
		return TerraformUnknownVariableValue
	}

	switch tfs.Type() {
	case shim.TypeList, shim.TypeSet:
		// TF does not accept unknown lists or sets. Instead, it accepts lists or sets of unknowns.
		count := 1
		if tfs.MinItems() > 0 {
			count = tfs.MinItems()
		}
		arr := make([]interface{}, count)
		for i := range arr {
			arr[i] = makeTerraformUnknownElement(tfs.Elem())
		}
		return arr
	default:
		return TerraformUnknownVariableValue
	}
}

// metaKey is the key in a TF bridge result that is used to store a resource's meta-attributes.
const metaKey = "__meta"

// MakeTerraformResult expands a Terraform state into an expanded Pulumi resource property map.  This respects
// the property maps so that results end up with their correct Pulumi names when shipping back to the engine.
func MakeTerraformResult(p shim.Provider, state shim.InstanceState,
	tfs shim.SchemaMap, ps map[string]*SchemaInfo, assets AssetTable,
	supportsSecrets bool) (resource.PropertyMap, error) {

	var outs map[string]interface{}
	if state != nil {
		obj, err := state.Object(tfs)
		if err != nil {
			return nil, err
		}
		outs = obj
	}

	outMap := MakeTerraformOutputs(p, outs, tfs, ps, assets, false, supportsSecrets)

	// If there is any Terraform metadata associated with this state, record it.
	if state != nil && len(state.Meta()) != 0 {
		metaJSON, err := json.Marshal(state.Meta())
		contract.Assert(err == nil)
		outMap[metaKey] = resource.NewStringProperty(string(metaJSON))
	}

	return outMap, nil
}

// MakeTerraformOutputs takes an expanded Terraform property map and returns a Pulumi equivalent.  This respects
// the property maps so that results end up with their correct Pulumi names when shipping back to the engine.
func MakeTerraformOutputs(p shim.Provider, outs map[string]interface{},
	tfs shim.SchemaMap, ps map[string]*SchemaInfo, assets AssetTable, rawNames,
	supportsSecrets bool) resource.PropertyMap {
	result := make(resource.PropertyMap)

	for key, value := range outs {
		// First do a lookup of the name/info.
		name, tfi, psi := getInfoFromTerraformName(key, tfs, ps, rawNames)
		contract.Assert(name != "")

		// Next perform a translation of the value accordingly.
		out := MakeTerraformOutput(p, value, tfi, psi, assets, rawNames, supportsSecrets)
		//if !out.IsNull() {
		result[name] = out
		//}
	}

	if glog.V(5) {
		for k, v := range result {
			glog.V(5).Infof("Terraform output %v = %v", k, v)
		}
	}

	return result
}

// MakeTerraformOutput takes a single Terraform property and returns the Pulumi equivalent.
func MakeTerraformOutput(p shim.Provider, v interface{},
	tfs shim.Schema, ps *SchemaInfo, assets AssetTable, rawNames, supportsSecrets bool) resource.PropertyValue {

	buildOutput := func(p shim.Provider, v interface{},
		tfs shim.Schema, ps *SchemaInfo, assets AssetTable, rawNames, supportsSecrets bool) resource.PropertyValue {
		if assets != nil && ps != nil && ps.Asset != nil {
			if asset, has := assets[ps]; has {
				// if we have the value, it better actually be an asset or an archive.
				contract.Assert(asset.IsAsset() || asset.IsArchive())
				return asset
			}

			// If we don't have the value, it is possible that the user supplied a value that was not an asset. Let the
			// normal marshalling logic handle it in that case.
		}

		if v == nil {
			return resource.NewNullProperty()
		}

		// Marshal sets as their list value.
		if list, ok := p.IsSet(v); ok {
			v = list
		}

		// We use reflection instead of a type switch so that we can support mapping values whose underlying type is
		// supported into a Pulumi value, even if they stored as a wrapper type (such as a strongly-typed enum).
		//
		// That said, Terraform often returns values of type String for fields whose schema does not indicate that the
		// value is actually a string. If we are given a string, and we'd otherwise return a string property, we'll also
		// inspect the schema if one exists to determine the actual value that we should return.
		val := reflect.ValueOf(v)
		switch val.Kind() {
		case reflect.Bool:
			return resource.NewBoolProperty(val.Bool())
		case reflect.Int:
			return resource.NewNumberProperty(float64(val.Int()))
		case reflect.Float64:
			return resource.NewNumberProperty(val.Float())
		case reflect.String:
			// If the string is the special unknown property sentinel, reflect back an unknown computed property.  Note that
			// Terraform doesn't carry the types along with it, so the best we can do is give back a computed string.
			t := val.String()
			if t == TerraformUnknownVariableValue {
				return resource.NewComputedProperty(
					resource.Computed{Element: resource.NewStringProperty("")})
			}

			// Is there a schema available to us? If not, it's definitely just a string.
			if tfs == nil {
				return resource.NewStringProperty(t)
			}

			// Otherwise, it might be a string that needs to be coerced to match the Terraform schema type. Coerce the
			// string to the Go value of the correct type and, if the coercion produced something different than the string
			// value we already have, re-make the output. We need to ensure that we take into account any Pulumi schema
			// overrides as part of this coercion
			coerced, err := CoerceTerraformString(tfs.Type(), ps, t)
			if err != nil || coerced == t {
				return resource.NewStringProperty(t)
			}
			return MakeTerraformOutput(p, coerced, tfs, ps, assets, rawNames, supportsSecrets)
		case reflect.Slice:
			elems := []interface{}{}
			for i := 0; i < val.Len(); i++ {
				elems = append(elems, val.Index(i).Interface())
			}

			tfes, pes := elemSchemas(tfs, ps)

			var arr []resource.PropertyValue
			for _, elem := range elems {
				arr = append(arr, MakeTerraformOutput(p, elem, tfes, pes, assets, rawNames, supportsSecrets))
			}
			// For TypeList or TypeSet with MaxItems==1, we will have projected as a scalar nested value, so need to extract
			// out the single element (or null).
			if IsMaxItemsOne(tfs, ps) {
				switch len(arr) {
				case 0:
					return resource.NewNullProperty()
				case 1:
					return arr[0]
				default:
					contract.Failf("Unexpected multiple elements in array with MaxItems=1")
				}
			}
			return resource.NewArrayProperty(arr)
		case reflect.Map:
			outs := map[string]interface{}{}
			for _, key := range val.MapKeys() {
				contract.Assert(key.Kind() == reflect.String)
				outs[key.String()] = val.MapIndex(key).Interface()
			}
			var tfflds shim.SchemaMap
			if tfs != nil {
				if res, isres := tfs.Elem().(shim.Resource); isres {
					tfflds = res.Schema()
				}
			}
			var psflds map[string]*SchemaInfo
			if ps != nil {
				psflds = ps.Fields
			}
			obj := MakeTerraformOutputs(p, outs, tfflds, psflds, assets, rawNames || useRawNames(tfs), supportsSecrets)
			return resource.NewObjectProperty(obj)
		default:
			contract.Failf("Unexpected TF output property value: %#v", v)
			return resource.NewNullProperty()
		}
	}

	output := buildOutput(p, v, tfs, ps, assets, rawNames, supportsSecrets)

	if tfs != nil && tfs.Sensitive() && supportsSecrets {
		return resource.MakeSecret(output)
	}

	return output
}

// MakeTerraformConfig creates a Terraform config map, used in state and diff calculations, from a Pulumi property map.
func MakeTerraformConfig(p *Provider, m resource.PropertyMap,
	tfs shim.SchemaMap, ps map[string]*SchemaInfo) (shim.ResourceConfig, AssetTable, error) {

	// Convert the resource bag into an untyped map, and then create the resource config object.
	ctx := conversionContext{
		ProviderConfig: p.configValues,
		Assets:         AssetTable{},
	}
	inputs, err := ctx.MakeTerraformInputs(nil, m, tfs, ps, false)
	if err != nil {
		return nil, nil, err
	}
	return MakeTerraformConfigFromInputs(p.tf, inputs), ctx.Assets, nil
}

// UnmarshalTerraformConfig creates a Terraform config map from a Pulumi RPC property map.
func UnmarshalTerraformConfig(p *Provider, m *pbstruct.Struct,
	tfs shim.SchemaMap, ps map[string]*SchemaInfo,
	label string) (shim.ResourceConfig, AssetTable, error) {

	props, err := plugin.UnmarshalProperties(m,
		plugin.MarshalOptions{Label: label, KeepUnknowns: true, SkipNulls: true})
	if err != nil {
		return nil, nil, err
	}
	return MakeTerraformConfig(p, props, tfs, ps)
}

// makeConfig is a helper for MakeTerraformConfigFromInputs that performs a deep-ish copy of its input, recursively
// removing Pulumi-internal properties as it goes.
func makeConfig(v interface{}) interface{} {
	switch v := v.(type) {
	case []interface{}:
		r := make([]interface{}, len(v))
		for i, e := range v {
			r[i] = makeConfig(e)
		}
		return r
	case map[string]interface{}:
		r := make(map[string]interface{})
		for k, e := range v {
			// If this is a reserved property, ignore it.
			switch k {
			case defaultsKey, metaKey:
				continue
			}
			r[k] = makeConfig(e)
		}
		return r
	default:
		return v
	}
}

// MakeTerraformConfigFromInputs creates a new Terraform configuration object from a set of Terraform inputs.
func MakeTerraformConfigFromInputs(p shim.Provider, inputs map[string]interface{}) shim.ResourceConfig {
	raw := makeConfig(inputs).(map[string]interface{})
	return p.NewResourceConfig(raw)
}

// MakeTerraformState converts a Pulumi property bag into its Terraform equivalent.  This requires
// flattening everything and serializing individual properties as strings.  This is a little awkward, but it's how
// Terraform represents resource properties (schemas are simply sugar on top).
func MakeTerraformState(res Resource, id string, m resource.PropertyMap) (shim.InstanceState, error) {
	// Parse out any metadata from the state.
	var meta map[string]interface{}
	if metaProperty, hasMeta := m[metaKey]; hasMeta && metaProperty.IsString() {
		if err := json.Unmarshal([]byte(metaProperty.StringValue()), &meta); err != nil {
			return nil, err
		}
	} else if res.TF.SchemaVersion() > 0 {
		// If there was no metadata in the inputs and this resource has a non-zero schema version, return a meta bag
		// with the current schema version. This helps avoid migration issues.
		meta = map[string]interface{}{"schema_version": strconv.Itoa(res.TF.SchemaVersion())}
	}

	// Turn the resource properties into a map. For the most part, this is a straight Mappable, but we use MapReplace
	// because we use float64s and Terraform uses ints, to represent numbers.
	ctx := &conversionContext{}
	inputs, err := ctx.MakeTerraformInputs(nil, m, res.TF.Schema(), res.Schema.Fields, false)
	if err != nil {
		return nil, err
	}

	return res.TF.InstanceState(id, inputs, meta)
}

// UnmarshalTerraformState unmarshals a Terraform instance state from an RPC property map.
func UnmarshalTerraformState(r Resource, id string, m *pbstruct.Struct, l string) (shim.InstanceState, error) {
	props, err := plugin.UnmarshalProperties(m, plugin.MarshalOptions{
		Label:     fmt.Sprintf("%s.state", l),
		SkipNulls: true,
	})
	if err != nil {
		return nil, err
	}
	return MakeTerraformState(r, id, props)
}

// IsMaxItemsOne returns true if the schema/info pair represents a TypeList or TypeSet which should project
// as a scalar, else returns false.
func IsMaxItemsOne(tfs shim.Schema, info *SchemaInfo) bool {
	if tfs == nil {
		return false
	}
	if tfs.Type() != shim.TypeList && tfs.Type() != shim.TypeSet {
		return false
	}
	if info != nil && info.MaxItemsOne != nil {
		return *info.MaxItemsOne
	}
	return tfs.MaxItems() == 1
}

// useRawNames returns true if raw, unmangled names should be preserved.  This is only true for Terraform maps with
// an Elem that is not a shim.Resource.
func useRawNames(tfs shim.Schema) bool {
	if tfs == nil || tfs.Type() != shim.TypeMap {
		return false
	}
	_, hasResourceElem := tfs.Elem().(shim.Resource)
	return !hasResourceElem
}

// getInfoFromTerraformName does a map lookup to find the Pulumi name and schema info, if any.
func getInfoFromTerraformName(key string,
	tfs shim.SchemaMap, ps map[string]*SchemaInfo, rawName bool) (resource.PropertyKey,
	shim.Schema, *SchemaInfo) {
	info := ps[key]

	var name string
	if info != nil {
		name = info.Name
	}
	if name == "" {
		if rawName {
			// If raw names are requested, simply preserve the Terraform name.
			name = key
		} else {
			// If no name override exists, use the default name mangling scheme.
			name = TerraformToPulumiName(key, getSchema(tfs, key), ps[key], false)
		}
	}

	return resource.PropertyKey(name), getSchema(tfs, key), info
}

// getInfoFromPulumiName does a reverse map lookup to find the Terraform name and schema info for a Pulumi name, if any.
func getInfoFromPulumiName(key resource.PropertyKey,
	tfs shim.SchemaMap, ps map[string]*SchemaInfo, rawName bool) (string,
	shim.Schema, *SchemaInfo) {
	// To do this, we will first look to see if there's a known custom schema that uses this name.  If yes, we
	// prefer to use that.  To do this, we must use a reverse lookup.  (In the future we may want to make a
	// lookaside map to avoid the traversal of this map.)  Otherwise, use the standard name mangling scheme.
	ks := string(key)
	for tfname, schinfo := range ps {
		if schinfo != nil && schinfo.Name == ks {
			return tfname, getSchema(tfs, tfname), schinfo
		}
	}
	var name string
	if rawName {
		// If raw names are requested, they will not have been mangled, so preserve the name as-is.
		name = ks
	} else {
		// Otherwise, transform the Pulumi name to the Terraform name using the standard mangling scheme.
		name = PulumiToTerraformName(ks, tfs, ps)
	}
	return name, getSchema(tfs, name), ps[name]
}

// CoerceTerraformString coerces a string value to a Go value whose type is the type requested by the Terraform schema
// type or the Pulumi SchemaInfo. We prefer the SchemaInfo overrides as it's an explicit call to action over the
// Terraform Schema. Returns an error if the string can't be successfully coerced to the requested type.
func CoerceTerraformString(schType shim.ValueType, ps *SchemaInfo, stringValue string) (interface{}, error) {
	// check for the override and use that over terraform if available
	// we do this to ensure that we are following the explicit call to action of the override
	// For now, we will only return nil when an override of the type is a boolean and there is no
	// default value supplied - this will allow us to replicate the nullable-esquq bools that Terraform are
	// creating by using strings in place of bools
	// if we return nil for *all* override types when there is an empty string, then we can hit an edge case of
	// breaking overrides where we have a string and a TransformJSONDocument (see pulumi/pulumi#4592)
	if ps != nil && ps.Type != "" {
		switch strings.ToLower(ps.Type.String()) {
		case "boolean":
			if stringValue == "" {
				return nil, nil
			}
			return convertTfStringToBool(stringValue)
		case "int":
			return convertTfStringToInt(stringValue)
		case "float":
			return convertTfStringToFloat(stringValue)
		}

		return stringValue, nil
	}

	switch schType {
	case shim.TypeInt:
		return convertTfStringToInt(stringValue)
	case shim.TypeBool:
		return convertTfStringToBool(stringValue)
	case shim.TypeFloat:
		return convertTfStringToFloat(stringValue)
	}

	// Else it's just a string.
	return stringValue, nil
}

func convertTfStringToBool(stringValue string) (interface{}, error) {
	boolVal, err := strconv.ParseBool(stringValue)
	if err != nil {
		return nil, err
	}
	return boolVal, nil
}

func convertTfStringToInt(stringValue string) (interface{}, error) {
	intVal, err := strconv.ParseInt(stringValue, 0, 0)
	if err != nil {
		return nil, err
	}
	return float64(intVal), nil
}

func convertTfStringToFloat(stringValue string) (interface{}, error) {
	floatVal, err := strconv.ParseFloat(stringValue, 64)
	if err != nil {
		return nil, err
	}
	return floatVal, nil
}

func min(a int, b int) int {
	if a < b {
		return a
	}
	return b
}

func extractInputs(oldInput, newState resource.PropertyValue, tfs shim.Schema, ps *SchemaInfo,
	rawNames bool) (resource.PropertyValue, bool) {

	possibleDefault := true
	switch {
	case oldInput.IsArray() && newState.IsArray():
		etfs, eps := elemSchemas(tfs, ps)

		oldArray, newArray := oldInput.ArrayValue(), newState.ArrayValue()
		for i := range oldArray {
			if i >= len(newArray) {
				possibleDefault = false
				break
			}

			defaultElem := false
			oldArray[i], defaultElem = extractInputs(oldArray[i], newArray[i], etfs, eps, rawNames)
			if !defaultElem {
				possibleDefault = false
			}
		}

		return resource.NewArrayProperty(oldArray[:min(len(oldArray), len(newArray))]), possibleDefault
	case oldInput.IsObject() && newState.IsObject():
		var tfflds shim.SchemaMap
		if tfs != nil {
			if res, isres := tfs.Elem().(shim.Resource); isres {
				tfflds = res.Schema()
			}
		}
		var psflds map[string]*SchemaInfo
		if ps != nil {
			psflds = ps.Fields
		}

		oldMap, newMap := oldInput.ObjectValue(), newState.ObjectValue()

		// If we have a list of inputs that were populated by defaults, filter out any properties that changed and add
		// the result to the new inputs.
		defaultNames := map[string]bool{}
		if oldDefaultNames, ok := oldMap[defaultsKey]; ok && oldDefaultNames.IsArray() {
			for _, k := range oldDefaultNames.ArrayValue() {
				if k.IsString() {
					defaultNames[k.StringValue()] = true
				}
			}
		}

		for name, oldValue := range oldMap {
			defaultElem := false
			if newValue, ok := newMap[name]; ok {
				_, etfs, eps := getInfoFromPulumiName(name, tfflds, psflds, rawNames || useRawNames(tfs))
				oldMap[name], defaultElem = extractInputs(oldValue, newValue, etfs, eps, rawNames || useRawNames(tfs))
			} else {
				delete(oldMap, name)
			}
			if !defaultElem {
				possibleDefault = false
				delete(defaultNames, string(name))
			}
		}

		if len(defaultNames) == 0 {
			delete(oldMap, defaultsKey)
		} else {
			defaults := make([]resource.PropertyValue, 0, len(defaultNames))
			for name := range defaultNames {
				defaults = append(defaults, resource.NewStringProperty(name))
			}
			sort.Slice(defaults, func(i, j int) bool { return defaults[i].StringValue() < defaults[j].StringValue() })

			oldMap[defaultsKey] = resource.NewArrayProperty(defaults)
		}

		return resource.NewObjectProperty(oldMap), possibleDefault
	case oldInput.IsString() && newState.IsString():
		// If this value has a StateFunc, its state value may not be compatible with its
		// input value. Ignore the difference.
		if tfs != nil && tfs.StateFunc() != nil {
			return oldInput, tfs.StateFunc()(oldInput.StringValue()) == newState.StringValue()
		}
		return newState, oldInput.StringValue() == newState.StringValue()
	default:
		return newState, oldInput.DeepEquals(newState)
	}
}

func addDefaultAnnotations(newInput resource.PropertyValue) {
	switch {
	case newInput.IsArray():
		newArray := newInput.ArrayValue()
		for i := range newArray {
			addDefaultAnnotations(newArray[i])
		}
	case newInput.IsObject():
		newMap := newInput.ObjectValue()
		for _, newValue := range newMap {
			addDefaultAnnotations(newValue)
		}
		newMap[defaultsKey] = resource.NewArrayProperty([]resource.PropertyValue{})
	}
}

func extractSchemaInputs(state resource.PropertyValue, tfs shim.SchemaMap,
	ps map[string]*SchemaInfo) (resource.PropertyValue, error) {
	inputs := make(resource.PropertyMap)
	for name, value := range state.ObjectValue() {
		// If this property is not an input, ignore it.
		_, sch, _ := getInfoFromPulumiName(name, tfs, ps, false)
		if sch == nil || (!sch.Optional() && !sch.Required()) {
			continue
		}

		// Otherwise, copy it to the result.
		copy, err := copystructure.Copy(value)
		if err != nil {
			return resource.PropertyValue{}, err
		}
		inputs[name] = copy.(resource.PropertyValue)
	}

	inputsValue := resource.NewObjectProperty(inputs)
	addDefaultAnnotations(inputsValue)
	return inputsValue, nil
}

func extractInputsFromOutputs(oldInputs, outs resource.PropertyMap,
	tfs shim.SchemaMap, ps map[string]*SchemaInfo, isRefresh bool) (resource.PropertyMap, error) {

	sch := (&schema.Schema{
		Elem: (&schema.Resource{
			Schema: tfs,
		}).Shim(),
	}).Shim()
	pss := &SchemaInfo{Fields: ps}

	var inputs resource.PropertyValue
	if isRefresh {
		// If this is a refresh, only extract new values for inputs that are already present.
		inputs, _ = extractInputs(resource.NewObjectProperty(oldInputs),
			resource.NewObjectProperty(outs), sch, pss, false)
	} else {
		// Otherwise, take a schema-directed approach that fills out all input-only properties.
		v, err := extractSchemaInputs(resource.NewObjectProperty(outs), tfs, ps)
		if err != nil {
			return nil, err
		}
		inputs = v
	}
	return inputs.ObjectValue(), nil
}
