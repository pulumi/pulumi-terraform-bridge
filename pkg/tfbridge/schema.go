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
	"fmt"
	"reflect"
	"strconv"

	"github.com/golang/glog"

	pbstruct "github.com/golang/protobuf/ptypes/struct"
	"github.com/hashicorp/terraform/config"
	"github.com/hashicorp/terraform/flatmap"
	"github.com/hashicorp/terraform/helper/schema"
	"github.com/hashicorp/terraform/terraform"
	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/resource/plugin"
	"github.com/pulumi/pulumi/pkg/util/contract"
)

// AssetTable is used to record which properties in a call to MakeTerraformInputs were assets so that they can be
// marshaled back to assets by MakeTerraformOutputs.
type AssetTable map[*SchemaInfo]resource.PropertyValue

// MakeTerraformInputs takes a property map plus custom schema info and does whatever is necessary
// to prepare it for use by Terraform.  Note that this function may have side effects, for instance
// if it is necessary to spill an asset to disk in order to create a name out of it.  Please take
// care not to call it superfluously!
func MakeTerraformInputs(res *PulumiResource, olds, news resource.PropertyMap,
	tfs map[string]*schema.Schema, ps map[string]*SchemaInfo, assets AssetTable,
	defaults, useRawNames bool) (map[string]interface{}, error) {

	result := make(map[string]interface{})

	// Enumerate the inputs provided and add them to the map using their Terraform names.
	for key, value := range news {
		// First translate the Pulumi property name to a Terraform name.
		name, tfi, psi := getInfoFromPulumiName(key, tfs, ps, useRawNames)
		contract.Assert(name != "")

		var old resource.PropertyValue
		if defaults && olds != nil {
			old, _ = olds[key]
		}

		// And then translate the property value.
		v, err := MakeTerraformInput(res, name, old, value, tfi, psi, assets, defaults, useRawNames)
		if err != nil {
			return nil, err
		}
		result[name] = v
		glog.V(9).Infof("Created Terraform input: %v = %v", name, v)
	}

	// Now enumerate and propagate defaults if the corresponding values are still missing.
	if defaults {
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

			if _, has := result[name]; !has && info.HasDefault() {
				// If we already have a default value from a previous version of this resource, use that instead.
				key, tfi, psi := getInfoFromTerraformName(name, tfs, ps, useRawNames)
				if old, hasold := olds[key]; hasold {
					v, err := MakeTerraformInput(res, name, resource.PropertyValue{}, old, tfi, psi, assets,
						false, useRawNames)
					if err != nil {
						return nil, err
					}
					result[name] = v
					glog.V(9).Infof("Created Terraform input: %v = %v (old default)", key, old)
				} else if info.Default.Value != nil {
					result[name] = info.Default.Value
					glog.V(9).Infof("Created Terraform input: %v = %v (default)", name, result[name])
				} else if from := info.Default.From; from != nil {
					v, err := from(res)
					if err != nil {
						return nil, err
					}

					result[name] = v
					glog.V(9).Infof("Created Terraform input: %v = %v (default from fnc)", name, result[name])
				}
			}
		}

		// Next, populate defaults from the Terraform schema.
		for name, sch := range tfs {
			if _, conflicts := conflictsWith[name]; conflicts {
				continue
			}

			if _, has := result[name]; !has {
				// Check for a default value from Terraform. If there is not default from terraform, skip this name.
				dv, err := sch.DefaultValue()
				if err != nil {
					return nil, err
				} else if dv == nil {
					continue
				}

				// Next, if we already have a default value from a previous version of this resource, use that instead.
				key, tfi, psi := getInfoFromTerraformName(name, tfs, ps, useRawNames)
				if old, hasold := olds[key]; hasold {
					v, err := MakeTerraformInput(res, name, resource.PropertyValue{}, old, tfi, psi, assets,
						false, useRawNames)
					if err != nil {
						return nil, err
					}
					result[name] = v
					glog.V(9).Infof("Create Terraform input: %v = %v (old default)", name, old)
				} else {
					result[name] = dv
					glog.V(9).Infof("Created Terraform input: %v = %v (default from TF)", name, result[name])
				}
			}
		}
	}

	if glog.V(5) {
		for k, v := range result {
			glog.V(5).Infof("Terraform input %v = %v", k, v)
		}
	}

	return result, nil
}

// MakeTerraformInput takes a single property plus custom schema info and does whatever is necessary to prepare it for
// use by Terraform.  Note that this function may have side effects, for instance if it is necessary to spill an asset
// to disk in order to create a name out of it.  Please take care not to call it superfluously!
func MakeTerraformInput(res *PulumiResource, name string,
	old, v resource.PropertyValue, tfs *schema.Schema, ps *SchemaInfo, assets AssetTable,
	defaults, rawNames bool) (interface{}, error) {

	// For TypeList or TypeSet with MaxItems==1, we will have projected as a scalar nested value, and need to wrap it
	// into a single-element array before passing to Terraform.
	if IsMaxItemsOne(tfs, ps) {
		old = resource.NewArrayProperty([]resource.PropertyValue{old})
		v = resource.NewArrayProperty([]resource.PropertyValue{v})
	}

	switch {
	case v.IsNull():
		return nil, nil
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
			e, err := MakeTerraformInput(res, elemName, oldElem, elem, etfs, eps, assets, defaults, rawNames)
			if err != nil {
				return nil, err
			}
			arr = append(arr, e)
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
		} else if !ps.Asset.IsArchive() {
			return nil, errors.Errorf("expected an archive, but %s is not an archive", name)
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
			tfflds, psflds, assets, defaults, rawNames || useRawNames(tfs))
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
		//
		// It is important that we use the TF schema (if available) to decide what shape the unknown value should have:
		// e.g. TF does not play nicely with unknown lists, instead expecting a list of unknowns.
		if tfs == nil {
			return config.UnknownVariableValue, nil
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
				arr[i] = config.UnknownVariableValue
			}
			return arr, nil
		default:
			return config.UnknownVariableValue, nil
		}
	default:
		contract.Failf("Unexpected value marshaled: %v", v)
		return nil, nil
	}
}

// MakeTerraformResult expands a Terraform state into an expanded Pulumi resource property map.  This respects
// the property maps so that results end up with their correct Pulumi names when shipping back to the engine.
func MakeTerraformResult(state *terraform.InstanceState,
	tfs map[string]*schema.Schema, ps map[string]*SchemaInfo) resource.PropertyMap {
	var outs map[string]interface{}
	if state != nil {
		outs = make(map[string]interface{})
		attrs := state.Attributes
		for _, key := range flatmap.Map(attrs).Keys() {
			outs[key] = flatmap.Expand(attrs, key)
		}
	}
	return MakeTerraformOutputs(outs, tfs, ps, nil, false)
}

// MakeTerraformOutputs takes an expanded Terraform property map and returns a Pulumi equivalent.  This respects
// the property maps so that results end up with their correct Pulumi names when shipping back to the engine.
func MakeTerraformOutputs(outs map[string]interface{},
	tfs map[string]*schema.Schema, ps map[string]*SchemaInfo, assets AssetTable, rawNames bool) resource.PropertyMap {
	result := make(resource.PropertyMap)
	for key, value := range outs {
		// First do a lookup of the name/info.
		name, tfi, psi := getInfoFromTerraformName(key, tfs, ps, rawNames)
		contract.Assert(name != "")

		// Next perform a translation of the value accordingly.
		result[name] = MakeTerraformOutput(value, tfi, psi, assets, rawNames)
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
	tfs *schema.Schema, ps *SchemaInfo, assets AssetTable, rawNames bool) resource.PropertyValue {

	if assets != nil && ps != nil && ps.Asset != nil {
		if asset, has := assets[ps]; has {
			// if we have the value, it better actually be an asset or an archive.
			contract.Assert(asset.IsAsset() || asset.IsArchive())
			return asset
		}

		// we might not have the asset value if this was something computed. in that
		// case just return an appropriate sentinel indicating that was the case.

		t, ok := v.(string)
		contract.Assert(ok)
		contract.Assert(t == config.UnknownVariableValue)

		return resource.NewComputedProperty(
			resource.Computed{Element: resource.NewStringProperty("")})
	}

	if v == nil {
		return resource.NewNullProperty()
	}

	// We use reflection instead of a type switch so that we can support mapping values whose underlying type is
	// supported into a Pulumi value, even if they stored as a wrapper type (such as a strongly-typed enum).
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
		if t == config.UnknownVariableValue {
			return resource.NewComputedProperty(
				resource.Computed{Element: resource.NewStringProperty("")})
		}
		// Else it's just a string.
		return resource.NewStringProperty(t)
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
			arr = append(arr, MakeTerraformOutput(elem, tfes, pes, assets, rawNames))
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
		obj := MakeTerraformOutputs(outs, tfflds, psflds, assets, rawNames || useRawNames(tfs))
		return resource.NewObjectProperty(obj)
	default:
		contract.Failf("Unexpected TF output property value: %v", v)
		return resource.NewNullProperty()
	}
}

// MakeTerraformConfig creates a Terraform config map, used in state and diff calculations, from a Pulumi property map.
func MakeTerraformConfig(res *PulumiResource, m resource.PropertyMap,
	tfs map[string]*schema.Schema, ps map[string]*SchemaInfo, defaults bool) (*terraform.ResourceConfig, error) {
	// Convert the resource bag into an untyped map, and then create the resource config object.
	inputs, err := MakeTerraformInputs(res, nil, m, tfs, ps, nil, defaults, false)
	if err != nil {
		return nil, err
	}
	return MakeTerraformConfigFromInputs(inputs)
}

// MakeTerraformConfigFromRPC creates a Terraform config map from a Pulumi RPC property map.
func MakeTerraformConfigFromRPC(res *PulumiResource, m *pbstruct.Struct,
	tfs map[string]*schema.Schema, ps map[string]*SchemaInfo,
	allowUnknowns, defaults bool, label string) (*terraform.ResourceConfig, error) {
	props, err := plugin.UnmarshalProperties(m,
		plugin.MarshalOptions{Label: label, KeepUnknowns: allowUnknowns, SkipNulls: true})
	if err != nil {
		return nil, err
	}
	return MakeTerraformConfig(res, props, tfs, ps, defaults)
}

// MakeTerraformConfigFromInputs creates a new Terraform configuration object from a set of Terraform inputs.
func MakeTerraformConfigFromInputs(inputs map[string]interface{}) (*terraform.ResourceConfig, error) {
	return &terraform.ResourceConfig{
		Raw:    inputs,
		Config: inputs,
	}, nil
}

// MakeTerraformAttributes converts a Pulumi property bag into its Terraform equivalent.  This requires
// flattening everything and serializing individual properties as strings.  This is a little awkward, but it's how
// Terraform represents resource properties (schemas are simply sugar on top).
func MakeTerraformAttributes(res *PulumiResource, m resource.PropertyMap,
	tfs map[string]*schema.Schema, ps map[string]*SchemaInfo, defaults bool) (map[string]string, error) {
	// Turn the resource properties into a map.  For the most part, this is a straight Mappable, but we use MapReplace
	// because we use float64s and Terraform uses ints, to represent numbers.
	inputs, err := MakeTerraformInputs(res, nil, m, tfs, ps, nil, defaults, false)
	if err != nil {
		return nil, err
	}
	return MakeTerraformAttributesFromInputs(inputs, tfs)
}

// MakeTerraformAttributesFromRPC unmarshals an RPC property map and calls through to MakeTerraformAttributes.
func MakeTerraformAttributesFromRPC(res *PulumiResource, m *pbstruct.Struct,
	tfs map[string]*schema.Schema, ps map[string]*SchemaInfo,
	allowUnknowns, defaults bool, label string) (map[string]string, error) {
	props, err := plugin.UnmarshalProperties(m,
		plugin.MarshalOptions{Label: label, KeepUnknowns: allowUnknowns, SkipNulls: true})
	if err != nil {
		return nil, err
	}
	return MakeTerraformAttributes(res, props, tfs, ps, defaults)
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
	return name, tfs[name], ps[ks]
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
