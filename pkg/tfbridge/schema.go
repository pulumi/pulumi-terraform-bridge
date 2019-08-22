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
	"reflect"
	"sort"
	"strconv"

	"github.com/golang/glog"

	pbstruct "github.com/golang/protobuf/ptypes/struct"
	"github.com/hashicorp/terraform/config/hcl2shim"
	"github.com/hashicorp/terraform/flatmap"
	"github.com/hashicorp/terraform/helper/schema"
	"github.com/hashicorp/terraform/terraform"
	"github.com/mitchellh/copystructure"
	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/resource/plugin"
	"github.com/pulumi/pulumi/pkg/util/contract"
)

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
	tfs map[string]*schema.Schema, ps map[string]*SchemaInfo) bool {

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

// MakeTerraformInputs takes a property map plus custom schema info and does whatever is necessary
// to prepare it for use by Terraform.  Note that this function may have side effects, for instance
// if it is necessary to spill an asset to disk in order to create a name out of it.  Please take
// care not to call it superfluously!
func MakeTerraformInputs(res *PulumiResource, olds, news resource.PropertyMap,
	tfs map[string]*schema.Schema, ps map[string]*SchemaInfo, assets AssetTable, config resource.PropertyMap,
	defaults, useRawNames bool) (map[string]interface{}, error) {

	result := make(map[string]interface{})

	// Enumerate the inputs provided and add them to the map using their Terraform names.
	for key, value := range news {
		// If this is a reserved property, ignore it.
		switch key {
		case defaultsKey, metaKey:
			continue
		}

		// First translate the Pulumi property name to a Terraform name.
		name, tfi, psi := getInfoFromPulumiName(key, tfs, ps, useRawNames)
		contract.Assert(name != "")

		var old resource.PropertyValue
		if defaults && olds != nil {
			old = olds[key]
		}

		// And then translate the property value.
		v, err := MakeTerraformInput(
			res, name, old, value, tfi, psi, assets, config, defaults, useRawNames)
		if err != nil {
			return nil, err
		}
		result[name] = v
		glog.V(9).Infof("Created Terraform input: %v = %v", name, v)
	}

	// Now enumerate and propagate defaults if the corresponding values are still missing.
	if defaults {
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
		for name, sch := range tfs {
			if _, has := result[name]; has {
				for _, conflictingName := range sch.ConflictsWith {
					conflictsWith[conflictingName] = struct{}{}
				}
			}
		}

		// First, attempt to use the overlays.
		for name, info := range ps {
			if _, conflicts := conflictsWith[name]; conflicts {
				continue
			}
			sch := tfs[name]
			if sch != nil && (sch.Removed != "" || sch.Deprecated != "" && !sch.Required) {
				continue
			}

			if _, has := result[name]; !has && info.HasDefault() {
				var defaultValue interface{}
				var source string

				// If we already have a default value from a previous version of this resource, use that instead.
				key, tfi, psi := getInfoFromTerraformName(name, tfs, ps, useRawNames)
				if old, hasold := olds[key]; hasold && useOldDefault(key) {
					v, err := MakeTerraformInput(
						res, name, resource.PropertyValue{}, old, tfi, psi, assets, config, false, useRawNames)
					if err != nil {
						return nil, err
					}
					defaultValue, source = v, "old default"
				} else if envVars := info.Default.EnvVars; len(envVars) != 0 {
					v, err := schema.MultiEnvDefaultFunc(envVars, info.Default.Value)()
					if err != nil {
						return nil, err
					}
					if str, ok := v.(string); ok && sch != nil {
						switch sch.Type {
						case schema.TypeBool:
							v = false
							if str != "" {
								if v, err = strconv.ParseBool(str); err != nil {
									return nil, err
								}
							}
						case schema.TypeInt:
							v = int(0)
							if str != "" {
								iv, iverr := strconv.ParseInt(str, 0, 0)
								if iverr != nil {
									return nil, iverr
								}
								v = int(iv)
							}
						case schema.TypeFloat:
							v = float64(0.0)
							if str != "" {
								if v, err = strconv.ParseFloat(str, 64); err != nil {
									return nil, err
								}
							}
						case schema.TypeString:
							// nothing to do
						default:
							return nil, errors.Errorf("unknown type for default value: %s", sch.Type)
						}
					}
					defaultValue, source = v, "env vars"
				} else if configKey := info.Default.Config; configKey != "" {
					if v := config[resource.PropertyKey(configKey)]; !v.IsNull() {
						tv, err := MakeTerraformInput(
							res, name, resource.PropertyValue{}, v, tfi, psi, assets, config, false, useRawNames)
						if err != nil {
							return nil, err
						}
						defaultValue, source = tv, "config"
					}
				} else if info.Default.Value != nil {
					defaultValue, source = info.Default.Value, "Pulumi schema"
				} else if from := info.Default.From; from != nil {
					v, err := from(res)
					if err != nil {
						return nil, err
					}
					defaultValue, source = v, "func"
				}

				if defaultValue != nil {
					glog.V(9).Infof("Created Terraform input: %v = %v (from %s)", name, defaultValue, source)
					result[name] = defaultValue
					newDefaults = append(newDefaults, key)
				}
			}
		}

		// Next, populate defaults from the Terraform schema.
		for name, sch := range tfs {
			if sch.Removed != "" {
				continue
			}
			if sch.Deprecated != "" && !sch.Required {
				continue
			}
			if _, conflicts := conflictsWith[name]; conflicts {
				continue
			}

			if _, has := result[name]; !has {
				var source string

				// Check for a default value from Terraform. If there is not default from terraform, skip this name.
				dv, err := sch.DefaultValue()
				if err != nil {
					return nil, err
				} else if dv == nil {
					continue
				}

				// Next, if we already have a default value from a previous version of this resource, use that instead.
				key, tfi, psi := getInfoFromTerraformName(name, tfs, ps, useRawNames)
				if old, hasold := olds[key]; hasold && useOldDefault(key) {
					v, err := MakeTerraformInput(
						res, name, resource.PropertyValue{}, old, tfi, psi, assets, config, false, useRawNames)
					if err != nil {
						return nil, err
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
		}

		sort.Slice(newDefaults, func(i, j int) bool {
			return newDefaults[i].(resource.PropertyKey) < newDefaults[j].(resource.PropertyKey)
		})
		result[defaultsKey] = newDefaults
	}

	if glog.V(5) {
		for k, v := range result {
			glog.V(5).Infof("Terraform input %v = %#v", k, v)
		}
	}

	return result, nil
}

// makeTerraformUnknownElement creates an unknown value to be used as an element of a list or set using the given
// element schema to guide the shape of the value.
func makeTerraformUnknownElement(elem interface{}) interface{} {
	// If we have no element schema, just return a simple unknown.
	if elem == nil {
		return hcl2shim.UnknownVariableValue
	}

	switch e := elem.(type) {
	case *schema.Schema:
		// If the element uses a normal schema, defer to makeTerraformUnknown.
		return makeTerraformUnknown(e)
	case *schema.Resource:
		// If the element uses a resource schema, fill in unknown values for any required properties.
		res := make(map[string]interface{})
		for k, v := range e.Schema {
			if v.Required {
				res[k] = makeTerraformUnknown(v)
			}
		}
		return res
	default:
		return hcl2shim.UnknownVariableValue
	}
}

// makeTerraformUnknown creates an unknown value with the shape indicated by the given schema.
//
// It is important that we use the TF schema (if available) to decide what shape the unknown value should have:
// e.g. TF does not play nicely with unknown lists, instead expecting a list of unknowns.
func makeTerraformUnknown(tfs *schema.Schema) interface{} {
	if tfs == nil {
		return hcl2shim.UnknownVariableValue
	}

	switch tfs.Type {
	case schema.TypeList, schema.TypeSet:
		// TF does not accept unknown lists or sets. Instead, it accepts lists or sets of unknowns.
		count := 1
		if tfs.MinItems > 0 {
			count = tfs.MinItems
		}
		arr := make([]interface{}, count)
		for i := range arr {
			arr[i] = makeTerraformUnknownElement(tfs.Elem)
		}
		return arr
	default:
		return hcl2shim.UnknownVariableValue
	}
}

// MakeTerraformInput takes a single property plus custom schema info and does whatever is necessary to prepare it for
// use by Terraform.  Note that this function may have side effects, for instance if it is necessary to spill an asset
// to disk in order to create a name out of it.  Please take care not to call it superfluously!
func MakeTerraformInput(res *PulumiResource, name string,
	old, v resource.PropertyValue, tfs *schema.Schema, ps *SchemaInfo, assets AssetTable, config resource.PropertyMap,
	defaults, rawNames bool) (interface{}, error) {

	// For TypeList or TypeSet with MaxItems==1, we will have projected as a scalar nested value, and need to wrap it
	// into a single-element array before passing to Terraform.
	if IsMaxItemsOne(tfs, ps) {
		old = resource.NewArrayProperty([]resource.PropertyValue{old})
		v = resource.NewArrayProperty([]resource.PropertyValue{v})
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
		if name != "" {
			return nil, errors.Errorf("unexpected null property %v", name)
		}
		return nil, errors.New("unexpected null property")
	case v.IsBool():
		return v.BoolValue(), nil
	case v.IsNumber():
		if tfs != nil && tfs.Type == schema.TypeFloat {
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

		var etfs *schema.Schema
		if tfs != nil {
			if sch, issch := tfs.Elem.(*schema.Schema); issch {
				etfs = sch
			} else if res, isres := tfs.Elem.(*schema.Resource); isres {
				// The IsObject case below expects a schema whose `Elem` is
				// a Resource, so create a placeholder schema wrapping this resource.
				etfs = &schema.Schema{Elem: res}
			}
		}
		var eps *SchemaInfo
		if ps != nil {
			eps = ps.Elem
		}

		var arr []interface{}
		for i, elem := range v.ArrayValue() {
			var oldElem resource.PropertyValue
			if i < len(oldArr) {
				oldElem = oldArr[i]
			}
			elemName := fmt.Sprintf("%v[%v]", name, i)
			e, err := MakeTerraformInput(
				res, elemName, oldElem, elem, etfs, eps, assets, config, defaults, rawNames)
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
		if assets != nil {
			_, has := assets[ps]
			contract.Assertf(!has, "duplicate schema info for asset")
			assets[ps] = v
		}
		return ps.Asset.TranslateAsset(v.AssetValue())
	case v.IsArchive():
		// We require that there be archive information, otherwise an error occurs.
		if ps == nil || ps.Asset == nil {
			return nil, errors.Errorf("unexpected archive %s", name)
		}
		if assets != nil {
			_, has := assets[ps]
			contract.Assertf(!has, "duplicate schema info for asset")
			assets[ps] = v
		}
		return ps.Asset.TranslateArchive(v.ArchiveValue())
	case v.IsObject():
		var tfflds map[string]*schema.Schema
		if tfs != nil {
			if res, isres := tfs.Elem.(*schema.Resource); isres {
				tfflds = res.Schema
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

		input, err := MakeTerraformInputs(res, oldObject, v.ObjectValue(),
			tfflds, psflds, assets, config, defaults, rawNames || useRawNames(tfs))
		if err != nil {
			return nil, err
		}

		// If we have schema information that indicates that this value is being presented to a map-typed field whose
		// Elem is a *schema.Resource, wrap the value in an array in order to work around a bug in Terraform.
		if tfs != nil && tfs.Type == schema.TypeMap {
			if _, hasResourceElem := tfs.Elem.(*schema.Resource); hasResourceElem {
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

// metaKey is the key in a TF bridge result that is used to store a resource's meta-attributes.
const metaKey = "__meta"

// MakeTerraformResult expands a Terraform state into an expanded Pulumi resource property map.  This respects
// the property maps so that results end up with their correct Pulumi names when shipping back to the engine.
func MakeTerraformResult(state *terraform.InstanceState,
	tfs map[string]*schema.Schema, ps map[string]*SchemaInfo, assets AssetTable,
	supportsSecrets bool) (resource.PropertyMap, error) {

	var outs map[string]interface{}
	if state != nil {
		outs = make(map[string]interface{})
		attrs := state.Attributes

		reader := &schema.MapFieldReader{
			Schema: tfs,
			Map:    schema.BasicMapReader(attrs),
		}
		for _, key := range flatmap.Map(attrs).Keys() {
			res, err := reader.ReadField([]string{key})
			if err != nil {
				return nil, err
			}
			if res.Value != nil {
				outs[key] = res.Value
			}
		}

		// Populate the "id" property if it is not set. Most schemas do not include this property, and leaving it out
		// can cause unnecessary diffs when refreshing/updating resources after a provider upgrade.
		if _, ok := outs["id"]; !ok {
			outs["id"] = attrs["id"]
		}
	}
	outMap := MakeTerraformOutputs(outs, tfs, ps, assets, false, supportsSecrets)

	// If there is any Terraform metadata associated with this state, record it.
	if state != nil && len(state.Meta) != 0 {
		metaJSON, err := json.Marshal(state.Meta)
		contract.Assert(err == nil)
		outMap[metaKey] = resource.NewStringProperty(string(metaJSON))
	}

	return outMap, nil
}

// MakeTerraformOutputs takes an expanded Terraform property map and returns a Pulumi equivalent.  This respects
// the property maps so that results end up with their correct Pulumi names when shipping back to the engine.
func MakeTerraformOutputs(outs map[string]interface{},
	tfs map[string]*schema.Schema, ps map[string]*SchemaInfo, assets AssetTable, rawNames,
	supportsSecrets bool) resource.PropertyMap {
	result := make(resource.PropertyMap)

	for key, value := range outs {
		// First do a lookup of the name/info.
		name, tfi, psi := getInfoFromTerraformName(key, tfs, ps, rawNames)
		contract.Assert(name != "")

		// Next perform a translation of the value accordingly.
		result[name] = MakeTerraformOutput(value, tfi, psi, assets, rawNames, supportsSecrets)
	}

	if glog.V(5) {
		for k, v := range result {
			glog.V(5).Infof("Terraform output %v = %v", k, v)
		}
	}

	return result
}

// MakeTerraformOutput takes a single Terraform property and returns the Pulumi equivalent.
func MakeTerraformOutput(v interface{},
	tfs *schema.Schema, ps *SchemaInfo, assets AssetTable, rawNames, supportsSecrets bool) resource.PropertyValue {

	buildOutput := func(v interface{},
		tfs *schema.Schema, ps *SchemaInfo, assets AssetTable, rawNames, supportsSecrets bool) resource.PropertyValue {
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
		if set, isset := v.(*schema.Set); isset {
			v = set.List()
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
			if t == hcl2shim.UnknownVariableValue {
				return resource.NewComputedProperty(
					resource.Computed{Element: resource.NewStringProperty("")})
			}

			// Is there a schema available to us? If not, it's definitely just a string.
			if tfs == nil {
				return resource.NewStringProperty(t)
			}

			// Otherwise, it might be a string that needs to be coerced to match the Terraform schema type. Coerce the
			// string to the Go value of the correct type and, if the coercion produced something different than the string
			// value we already have, re-make the output.
			coerced, err := CoerceTerraformString(tfs.Type, t)
			if err != nil || coerced == t {
				return resource.NewStringProperty(t)
			}
			return MakeTerraformOutput(coerced, tfs, ps, assets, rawNames, supportsSecrets)
		case reflect.Slice:
			elems := []interface{}{}
			for i := 0; i < val.Len(); i++ {
				elems = append(elems, val.Index(i).Interface())
			}
			var tfes *schema.Schema
			if tfs != nil {
				if sch, issch := tfs.Elem.(*schema.Schema); issch {
					tfes = sch
				} else if _, isres := tfs.Elem.(*schema.Resource); isres {
					// The map[string]interface{} case below expects a schema whose
					// `Elem` is a Resource, so just pass the full List schema
					tfes = tfs
				}
			}
			var pes *SchemaInfo
			if ps != nil {
				pes = ps.Elem
			}
			var arr []resource.PropertyValue
			for _, elem := range elems {
				arr = append(arr, MakeTerraformOutput(elem, tfes, pes, assets, rawNames, supportsSecrets))
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
			var tfflds map[string]*schema.Schema
			if tfs != nil {
				if res, isres := tfs.Elem.(*schema.Resource); isres {
					tfflds = res.Schema
				}
			}
			var psflds map[string]*SchemaInfo
			if ps != nil {
				psflds = ps.Fields
			}
			obj := MakeTerraformOutputs(outs, tfflds, psflds, assets, rawNames || useRawNames(tfs), supportsSecrets)
			return resource.NewObjectProperty(obj)
		default:
			contract.Failf("Unexpected TF output property value: %#v", v)
			return resource.NewNullProperty()
		}
	}

	output := buildOutput(v, tfs, ps, assets, rawNames, supportsSecrets)

	if tfs != nil && tfs.Sensitive && supportsSecrets {
		return resource.MakeSecret(output)
	}

	return output
}

// MakeTerraformConfig creates a Terraform config map, used in state and diff calculations, from a Pulumi property map.
func MakeTerraformConfig(res *PulumiResource, m resource.PropertyMap,
	tfs map[string]*schema.Schema, ps map[string]*SchemaInfo, assets AssetTable,
	config resource.PropertyMap, defaults bool) (*terraform.ResourceConfig, error) {
	// Convert the resource bag into an untyped map, and then create the resource config object.
	inputs, err := MakeTerraformInputs(res, nil, m, tfs, ps, assets, config, defaults, false)
	if err != nil {
		return nil, err
	}
	return MakeTerraformConfigFromInputs(inputs)
}

// MakeTerraformConfigFromRPC creates a Terraform config map from a Pulumi RPC property map.
func MakeTerraformConfigFromRPC(res *PulumiResource, m *pbstruct.Struct,
	tfs map[string]*schema.Schema, ps map[string]*SchemaInfo, assets AssetTable,
	config resource.PropertyMap, allowUnknowns, defaults bool, label string) (*terraform.ResourceConfig, error) {
	props, err := plugin.UnmarshalProperties(m,
		plugin.MarshalOptions{Label: label, KeepUnknowns: allowUnknowns, SkipNulls: true})
	if err != nil {
		return nil, err
	}
	cfg, err := MakeTerraformConfig(res, props, tfs, ps, assets, config, defaults)
	if err != nil {
		return nil, err
	}
	return cfg, nil
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
func MakeTerraformConfigFromInputs(inputs map[string]interface{}) (*terraform.ResourceConfig, error) {
	raw := makeConfig(inputs).(map[string]interface{})
	return &terraform.ResourceConfig{
		Raw:    raw,
		Config: raw,
	}, nil
}

// MakeTerraformAttributes converts a Pulumi property bag into its Terraform equivalent.  This requires
// flattening everything and serializing individual properties as strings.  This is a little awkward, but it's how
// Terraform represents resource properties (schemas are simply sugar on top).
func MakeTerraformAttributes(res *schema.Resource, m resource.PropertyMap,
	tfs map[string]*schema.Schema, ps map[string]*SchemaInfo, config resource.PropertyMap,
	defaults bool) (map[string]string, map[string]interface{}, error) {

	// Parse out any metadata from the state.
	var meta map[string]interface{}
	if metaProperty, hasMeta := m[metaKey]; hasMeta && metaProperty.IsString() {
		if err := json.Unmarshal([]byte(metaProperty.StringValue()), &meta); err != nil {
			return nil, nil, err
		}
	} else if res.SchemaVersion > 0 {
		// If there was no metadata in the inputs and this resource has a non-zero schema version, return a meta bag
		// with the current schema version. This helps avoid migration issues.
		meta = map[string]interface{}{"schema_version": strconv.Itoa(res.SchemaVersion)}
	}

	// Turn the resource properties into a map.  For the most part, this is a straight Mappable, but we use MapReplace
	// because we use float64s and Terraform uses ints, to represent numbers.
	inputs, err := MakeTerraformInputs(nil, nil, m, tfs, ps, nil, config, defaults, false)
	if err != nil {
		return nil, nil, err
	}

	attrs, err := MakeTerraformAttributesFromInputs(inputs, tfs)
	if err != nil {
		return nil, nil, err
	}
	return attrs, meta, nil
}

// MakeTerraformAttributesFromRPC unmarshals an RPC property map and calls through to MakeTerraformAttributes.
func MakeTerraformAttributesFromRPC(res *schema.Resource, m *pbstruct.Struct,
	tfs map[string]*schema.Schema, ps map[string]*SchemaInfo, config resource.PropertyMap,
	allowUnknowns, defaults bool, label string) (map[string]string, map[string]interface{}, error) {
	props, err := plugin.UnmarshalProperties(m,
		plugin.MarshalOptions{Label: label, KeepUnknowns: allowUnknowns, SkipNulls: true})
	if err != nil {
		return nil, nil, err
	}
	return MakeTerraformAttributes(res, props, tfs, ps, config, defaults)
}

// flattenValue takes a single value and recursively flattens its properties into the given string -> string map under
// the provided prefix. It expects that the value has been "schema-fied" by being read out of a schema.FieldReader (in
// particular, all sets *must* be represented as schema.Set values). The flattened value may then be used as the value
// of a terraform.InstanceState.Attributes field.
//
// Note that this duplicates much of the logic in TF's schema.MapFieldWriter. Ideally, we would just use that type,
// but there are various API/implementation challenges that preclude that option. The most worrying (and potentially
// fragile) piece of duplication is the code that calculates a set member's hash code; see the code under
// `case *schema.Set`.
func flattenValue(result map[string]string, prefix string, value interface{}) {
	if value == nil {
		return
	}

	switch t := value.(type) {
	case bool:
		if t {
			result[prefix] = "true"
		} else {
			result[prefix] = "false"
		}
	case int:
		result[prefix] = strconv.FormatInt(int64(t), 10)
	case float64:
		result[prefix] = strconv.FormatFloat(t, 'G', -1, 64)
	case string:
		result[prefix] = t
	case []interface{}:
		// Flatten each element.
		for i, elem := range t {
			flattenValue(result, prefix+"."+strconv.FormatInt(int64(i), 10), elem)
		}

		// Set the count.
		result[prefix+".#"] = strconv.FormatInt(int64(len(t)), 10)
	case *schema.Set:
		// Flatten each element.
		setList := t.List()
		for _, elem := range setList {
			// Note that the logic below is duplicated from `scheme.Set.hash`. If that logic ever changes, this will
			// need to change in kind.
			code := t.F(elem)
			if code < 0 {
				code = -code
			}

			flattenValue(result, prefix+"."+strconv.Itoa(code), elem)
		}

		// Set the count.
		result[prefix+".#"] = strconv.FormatInt(int64(len(setList)), 10)
	case map[string]interface{}:
		for k, v := range t {
			flattenValue(result, prefix+"."+k, v)
		}

		// Set the count.
		result[prefix+".%"] = strconv.Itoa(len(t))
	default:
		contract.Failf("Unexpected TF input value: %v", t)
	}
}

// MakeTerraformAttributesFromInputs creates a flat Terraform map from a structured set of Terraform inputs.
func MakeTerraformAttributesFromInputs(inputs map[string]interface{},
	tfs map[string]*schema.Schema) (map[string]string, error) {

	// In order to flatten the TF inputs into a TF attribute map, we must first schema-ify them by reading them out of
	// a FieldReader. The most straightforward way to do this is to turn the inputs into a TF config.Config value and
	// use the same to create a schema.ConfigFieldReader.
	cfg, err := MakeTerraformConfigFromInputs(inputs)
	if err != nil {
		return nil, err
	}

	// Read each top-level value out of the config we created above using a ConfigFieldReader and recursively flatten
	// them into their TF attribute form. The result is our set of TF attributes.
	result := make(map[string]string)
	reader := &schema.ConfigFieldReader{Config: cfg, Schema: tfs}
	for k := range tfs {
		// Elide nil values.
		if v, ok := inputs[k]; ok && v == nil {
			continue
		}

		f, err := reader.ReadField([]string{k})
		if err != nil {
			return nil, errors.Wrapf(err, "could not read field %v", k)
		}

		flattenValue(result, k, f.Value)
	}

	return result, nil
}

// IsMaxItemsOne returns true if the schema/info pair represents a TypeList or TypeSet which should project
// as a scalar, else returns false.
func IsMaxItemsOne(tfs *schema.Schema, info *SchemaInfo) bool {
	if tfs == nil {
		return false
	}
	if tfs.Type != schema.TypeList && tfs.Type != schema.TypeSet {
		return false
	}
	if info != nil && info.MaxItemsOne != nil {
		return *info.MaxItemsOne
	}
	return tfs.MaxItems == 1
}

// useRawNames returns true if raw, unmangled names should be preserved.  This is only true for Terraform maps with
// an Elem that is not a *schema.Resource.
func useRawNames(tfs *schema.Schema) bool {
	if tfs == nil || tfs.Type != schema.TypeMap {
		return false
	}
	_, hasResourceElem := tfs.Elem.(*schema.Resource)
	return !hasResourceElem
}

// getInfoFromTerraformName does a map lookup to find the Pulumi name and schema info, if any.
func getInfoFromTerraformName(key string,
	tfs map[string]*schema.Schema, ps map[string]*SchemaInfo, rawName bool) (resource.PropertyKey,
	*schema.Schema, *SchemaInfo) {
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
			name = TerraformToPulumiName(key, tfs[key], false)
		}
	}

	return resource.PropertyKey(name), tfs[key], info
}

// getInfoFromPulumiName does a reverse map lookup to find the Terraform name and schema info for a Pulumi name, if any.
func getInfoFromPulumiName(key resource.PropertyKey,
	tfs map[string]*schema.Schema, ps map[string]*SchemaInfo, rawName bool) (string,
	*schema.Schema, *SchemaInfo) {
	// To do this, we will first look to see if there's a known custom schema that uses this name.  If yes, we
	// prefer to use that.  To do this, we must use a reverse lookup.  (In the future we may want to make a
	// lookaside map to avoid the traversal of this map.)  Otherwise, use the standard name mangling scheme.
	ks := string(key)
	for tfname, schinfo := range ps {
		if schinfo != nil && schinfo.Name == ks {
			return tfname, tfs[tfname], schinfo
		}
	}
	var name string
	if rawName {
		// If raw names are requested, they will not have been mangled, so preserve the name as-is.
		name = ks
	} else {
		// Otherwise, transform the Pulumi name to the Terraform name using the standard mangling scheme.
		name = PulumiToTerraformName(ks, tfs)
	}
	return name, tfs[name], ps[name]
}

// CleanTerraformSchema recursively removes "Removed" properties from a map[string]*schema.Schema.
func CleanTerraformSchema(tfs map[string]*schema.Schema) map[string]*schema.Schema {
	cleaned := make(map[string]*schema.Schema)
	for key := range tfs {
		sch := tfs[key]

		if sch.Removed == "" {
			if resource, ok := sch.Elem.(*schema.Resource); ok {
				resource.Schema = CleanTerraformSchema(resource.Schema)
			}
			cleaned[key] = sch
		}
	}
	return cleaned
}

// CoerceTerraformString coerces a string value to a Go value whose type is the type requested by the Terraform schema
// type. Returns an error if the string can't be successfully coerced to the requested type.
func CoerceTerraformString(schType schema.ValueType, stringValue string) (interface{}, error) {
	switch schType {
	case schema.TypeInt:
		intVal, err := strconv.ParseInt(stringValue, 0, 0)
		if err != nil {
			return nil, err
		}
		return float64(intVal), nil
	case schema.TypeBool:
		boolVal, err := strconv.ParseBool(stringValue)
		if err != nil {
			return nil, err
		}
		return boolVal, nil
	case schema.TypeFloat:
		floatVal, err := strconv.ParseFloat(stringValue, 64)
		if err != nil {
			return nil, err
		}
		return floatVal, nil
	}

	// Else it's just a string.
	return stringValue, nil
}

// propagateDefaultAnnotations recursively propagates the `__defaults` annotation on the given value.
//
// An entry in the `__defaults` annotation is only propagated if the named property has a semantically identical value
// in the old and new inputs. Otherwise, the entry is removed.
//
// This function returns `true` if the old and new inputs have semantically identical values.
func propagateDefaultAnnotations(oldInput, newInput resource.PropertyValue, tfs *schema.Schema, ps *SchemaInfo,
	createIfMissing bool) bool {

	switch {
	case oldInput.IsArray() && newInput.IsArray():
		var etfs *schema.Schema
		if tfs != nil {
			if sch, issch := tfs.Elem.(*schema.Schema); issch {
				etfs = sch
			} else if res, isres := tfs.Elem.(*schema.Resource); isres {
				// The IsObject case below expects a schema whose `Elem` is
				// a Resource, so create a placeholder schema wrapping this resource.
				etfs = &schema.Schema{Elem: res}
			}
		}
		var eps *SchemaInfo
		if ps != nil {
			eps = ps.Elem
		}

		possibleDefault := true
		oldArray, newArray := oldInput.ArrayValue(), newInput.ArrayValue()
		for i := range oldArray {
			if i >= len(newArray) {
				possibleDefault = false
				break
			}
			if !propagateDefaultAnnotations(oldArray[i], newArray[i], etfs, eps, createIfMissing) {
				possibleDefault = false
			}
		}
		return possibleDefault
	case oldInput.IsObject() && newInput.IsObject():
		var tfflds map[string]*schema.Schema
		if tfs != nil {
			if res, isres := tfs.Elem.(*schema.Resource); isres {
				tfflds = res.Schema
			}
		}
		var psflds map[string]*SchemaInfo
		if ps != nil {
			psflds = ps.Fields
		}

		possibleDefaultsNames := map[resource.PropertyKey]bool{}

		possibleDefault := true
		oldMap, newMap := oldInput.ObjectValue(), newInput.ObjectValue()
		for name, newValue := range newMap {
			if oldValue, ok := oldMap[name]; ok {
				_, etfs, eps := getInfoFromPulumiName(name, tfflds, psflds, false)

				if propagateDefaultAnnotations(oldValue, newValue, etfs, eps, createIfMissing) {
					newMap[name] = oldMap[name]
					possibleDefaultsNames[name] = true
				} else {
					possibleDefault = false
				}
			} else {
				possibleDefault = false
			}
		}
		for name := range oldMap {
			if _, ok := newMap[name]; !ok {
				possibleDefault = false
			}
		}

		// If we have a list of inputs that were populated by defaults, filter out any properties that changed and add
		// the result to the new inputs.
		if oldDefaultNames, ok := oldMap[defaultsKey]; ok {
			newDefaultNames := []resource.PropertyValue{}
			for _, nameValue := range oldDefaultNames.ArrayValue() {
				if possibleDefaultsNames[resource.PropertyKey(nameValue.StringValue())] {
					newDefaultNames = append(newDefaultNames, nameValue)
				}
			}
			newMap[defaultsKey] = resource.NewArrayProperty(newDefaultNames)
		} else if createIfMissing {
			newMap[defaultsKey] = resource.NewArrayProperty([]resource.PropertyValue{})
		}
		return possibleDefault
	case oldInput.IsString() && newInput.IsString():
		oldStr, newStr := oldInput.StringValue(), newInput.StringValue()
		if tfs != nil && tfs.StateFunc != nil {
			oldStr = tfs.StateFunc(oldStr)
		}
		return oldStr == newStr
	default:
		return oldInput.DeepEquals(newInput)
	}
}

func extractInputsFromOutputs(oldInputs, outs resource.PropertyMap,
	tfs map[string]*schema.Schema, ps map[string]*SchemaInfo, isRefresh bool) (resource.PropertyMap, error) {

	inputs := make(resource.PropertyMap)
	for name, value := range outs {
		// If this property is not an input, ignore it.
		_, sch, _ := getInfoFromPulumiName(name, tfs, ps, false)
		if sch == nil || (!sch.Optional && !sch.Required) {
			continue
		}

		// Otherwise, copy it to the result.
		copy, err := copystructure.Copy(value)
		if err != nil {
			return nil, err
		}
		inputs[name] = copy.(resource.PropertyValue)
	}

	// Propagate default annotations from the old inputs.
	sch := &schema.Schema{Elem: &schema.Resource{Schema: tfs}}
	pss := &SchemaInfo{Fields: ps}
	propagateDefaultAnnotations(
		resource.NewObjectProperty(oldInputs), resource.NewObjectProperty(inputs), sch, pss, !isRefresh)

	return inputs, nil
}
