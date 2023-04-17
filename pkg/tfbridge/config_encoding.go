// Copyright 2016-2023, Pulumi Corporation.
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
	"sort"

	"github.com/golang/protobuf/ptypes/struct"

	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
)

type configEncoding struct {
	fieldTypes map[resource.PropertyKey]shim.ValueType
}

func newConfigEncoding(config shim.SchemaMap, configInfos map[string]*SchemaInfo) *configEncoding {
	var tfKeys []string
	config.Range(func(tfKey string, value shim.Schema) bool {
		tfKeys = append(tfKeys, tfKey)
		return true
	})
	fieldTypes := make(map[resource.PropertyKey]shim.ValueType)
	for _, tfKey := range tfKeys {
		pulumiKey := resource.PropertyKey(TerraformToPulumiNameV2(tfKey, config, configInfos))
		fieldTypes[pulumiKey] = config.Get(tfKey).Type()
	}
	return &configEncoding{fieldTypes: fieldTypes}
}

func (*configEncoding) convertStringToPropertyValue(s string, typ shim.ValueType) (resource.PropertyValue, error) {
	// If the schema expects a string, we can just return this as-is.
	if typ == shim.TypeString {
		return resource.NewStringProperty(s), nil
	}

	// Otherwise, we will attempt to deserialize the input string as JSON and convert the result into a Pulumi
	// property. If the input string is empty, we will return an appropriate zero value.
	if s == "" {
		switch typ {
		case shim.TypeBool:
			return resource.NewPropertyValue(false), nil
		case shim.TypeInt, shim.TypeFloat:
			return resource.NewPropertyValue(0), nil
		case shim.TypeList, shim.TypeSet:
			return resource.NewPropertyValue([]interface{}{}), nil
		default:
			return resource.NewPropertyValue(map[string]interface{}{}), nil
		}
	}

	var jsonValue interface{}
	if err := json.Unmarshal([]byte(s), &jsonValue); err != nil {
		return resource.PropertyValue{}, err
	}
	return resource.NewPropertyValue(jsonValue), nil
}

// Like plugin.UnmarshalPropertyValue but overrides string parsing with convertStringToPropertyValue.
func (enc *configEncoding) UnmarshalPropertyValue(key resource.PropertyKey, v *structpb.Value,
	opts plugin.MarshalOptions) (*resource.PropertyValue, error) {

	shimType, gotShimType := enc.fieldTypes[key]
	_, vIsString := v.GetKind().(*structpb.Value_StringValue)

	if vIsString && gotShimType {
		v, err := enc.convertStringToPropertyValue(v.GetStringValue(), shimType)
		if err != nil {
			return nil, fmt.Errorf("error unmarshalling property %q: %w", key, err)
		}
		return &v, nil
	}

	return plugin.UnmarshalPropertyValue(key, v, opts)
}

// Inline from plugin.UnmarshalProperties substituting plugin.UnmarshalPropertyValue.
func (enc *configEncoding) UnmarshalProperties(props *structpb.Struct,
	opts plugin.MarshalOptions) (resource.PropertyMap, error) {

	result := make(resource.PropertyMap)

	// First sort the keys so we enumerate them in order (in case errors happen, we want determinism).
	var keys []string
	if props != nil {
		for k := range props.Fields {
			keys = append(keys, k)
		}
		sort.Strings(keys)
	}

	// And now unmarshal every field it into the map.
	for _, key := range keys {
		pk := resource.PropertyKey(key)
		v, err := enc.UnmarshalPropertyValue(pk, props.Fields[key], opts)
		if err != nil {
			return nil, err
		} else if v != nil {
			logging.V(9).Infof("Unmarshaling property for RPC[%s]: %s=%v", opts.Label, key, v)
			if opts.SkipNulls && v.IsNull() {
				logging.V(9).Infof("Skipping unmarshaling for RPC[%s]: %s is null", opts.Label, key)
			} else if opts.SkipInternalKeys && resource.IsInternalPropertyKey(pk) {
				logging.V(9).Infof("Skipping unmarshaling for RPC[%s]: %s is internal", opts.Label, key)
			} else {
				result[pk] = *v
			}
		}
	}

	return result, nil
}

// Invesrse of UnmarshalProperties.
func (enc *configEncoding) MarshalProperties(props resource.PropertyMap,
	opts plugin.MarshalOptions) (*structpb.Struct, error) {
	copy := make(resource.PropertyMap)
	for k, v := range props {
		_, knownKey := enc.fieldTypes[k]
		switch {
		case knownKey && v.IsNull():
			copy[k] = resource.NewStringProperty("")
		case knownKey && !v.IsNull() && !v.IsString():
			encoded, err := json.Marshal(v.Mappable())
			if err != nil {
				return nil, fmt.Errorf("JSON encoding error while marshalling property %q: %w", k, err)
			}
			copy[k] = resource.NewStringProperty(string(encoded))
		default:
			copy[k] = v
		}
	}
	return plugin.MarshalProperties(copy, opts)
}
