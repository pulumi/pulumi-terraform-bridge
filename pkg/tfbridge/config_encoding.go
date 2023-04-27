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
	fieldTypes      map[resource.PropertyKey]shim.ValueType
	sensitiveFields map[resource.PropertyKey]struct{}
}

func newConfigEncoding(config shim.SchemaMap, configInfos map[string]*SchemaInfo) *configEncoding {
	var tfKeys []string
	config.Range(func(tfKey string, value shim.Schema) bool {
		tfKeys = append(tfKeys, tfKey)
		return true
	})
	fieldTypes := make(map[resource.PropertyKey]shim.ValueType)
	sensitiveFields := make(map[resource.PropertyKey]struct{})

	for _, tfKey := range tfKeys {
		pulumiKey := resource.PropertyKey(TerraformToPulumiNameV2(tfKey, config, configInfos))
		schema := config.Get(tfKey)
		fieldTypes[pulumiKey] = schema.Type()
		if containsSensitiveElements(schema) {
			sensitiveFields[pulumiKey] = struct{}{}
		}

	}
	return &configEncoding{
		fieldTypes:      fieldTypes,
		sensitiveFields: sensitiveFields,
	}
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

// Compute an approximation of which properties should be secret. A property should be secret if it is statically marked
// as secret in the schema, or has nested sub-properties marked in this way. A property should also be secret if the
// dynamic value sent from the program is marked as secret.
func (enc *configEncoding) ComputeSecrets(props *structpb.Struct,
	opts plugin.MarshalOptions) map[resource.PropertyKey]struct{} {
	opts.KeepSecrets = true

	secrets := make(map[resource.PropertyKey]struct{})

	for k := range enc.sensitiveFields {
		secrets[k] = struct{}{}
	}

	pm, err := enc.UnmarshalProperties(props, opts)
	if err == nil {
		for k, v := range pm {
			if v.ContainsSecrets() {
				secrets[k] = struct{}{}
			}
		}
	}
	return secrets
}

// Ensure that the sensitive top-level properties are marked as secret.
func (enc *configEncoding) MarkSecrets(secrets map[resource.PropertyKey]struct{},
	pm resource.PropertyMap) resource.PropertyMap {
	copy := make(resource.PropertyMap)
	for k, v := range pm {
		_, secret := secrets[k]
		if secret {
			copy[k] = resource.MakeSecret(v)
		} else {
			copy[k] = v
		}
	}
	return copy
}

// Like plugin.UnmarshalPropertyValue but overrides string parsing with convertStringToPropertyValue.
func (enc *configEncoding) UnmarshalPropertyValue(key resource.PropertyKey, v *structpb.Value,
	opts plugin.MarshalOptions) (*resource.PropertyValue, error) {

	pv, err := plugin.UnmarshalPropertyValue(key, v, opts)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling property %q: %w", key, err)
	}
	shimType, gotShimType := enc.fieldTypes[key]

	// Only apply JSON-encoded recognition for known fields.
	if !gotShimType {
		return pv, nil
	}

	var jsonString string
	var jsonStringDetected, jsonStringSecret bool

	if pv.IsString() {
		jsonString = pv.StringValue()
		jsonStringDetected = true
	}

	if opts.KeepSecrets && pv.IsSecret() && pv.SecretValue().Element.IsString() {
		jsonString = pv.SecretValue().Element.StringValue()
		jsonStringDetected = true
		jsonStringSecret = true
	}

	if jsonStringDetected {
		v, err := enc.convertStringToPropertyValue(jsonString, shimType)
		if err != nil {
			return nil, fmt.Errorf("error unmarshalling property %q: %w", key, err)
		}
		if jsonStringSecret {
			s := resource.MakeSecret(v)
			return &s, nil
		}
		return &v, nil
	}

	return pv, nil
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

// Inverse of UnmarshalProperties, with additional support for top-level (but not nested) secrets.
func (enc *configEncoding) MarshalProperties(props resource.PropertyMap,
	opts plugin.MarshalOptions) (*structpb.Struct, error) {
	opts.KeepSecrets = true
	copy := make(resource.PropertyMap)
	for k, v := range props {
		var err error
		copy[k], err = enc.jsonEncodePropertyValue(k, v)
		if err != nil {
			return nil, err
		}
	}
	return plugin.MarshalProperties(copy, opts)
}

func (enc *configEncoding) jsonEncodePropertyValue(k resource.PropertyKey,
	v resource.PropertyValue) (resource.PropertyValue, error) {
	if v.ContainsUnknowns() {
		return resource.NewStringProperty(plugin.UnknownStringValue), nil
	}
	if v.IsSecret() {
		encoded, err := enc.jsonEncodePropertyValue(k, v.SecretValue().Element)
		if err != nil {
			return v, err
		}
		return resource.NewSecretProperty(&resource.Secret{Element: encoded}), err
	}
	_, knownKey := enc.fieldTypes[k]
	switch {
	case knownKey && v.IsNull():
		return resource.NewStringProperty(""), nil
	case knownKey && !v.IsNull() && !v.IsString():
		encoded, err := json.Marshal(v.Mappable())
		if err != nil {
			return v, fmt.Errorf("JSON encoding error while marshalling property %q: %w", k, err)
		}
		return resource.NewStringProperty(string(encoded)), nil
	default:
		return v, nil
	}
}

func containsSensitiveElements(x shim.Schema) bool {
	if x.Sensitive() {
		return true
	}
	switch elem := x.Elem().(type) {
	case shim.Schema:
		return containsSensitiveElements(elem)
	case shim.Resource:
		sensitive := false
		s := elem.Schema()
		s.Range(func(key string, value shim.Schema) bool {
			if containsSensitiveElements(value) {
				sensitive = true
			}
			return true
		})
		return sensitive
	default:
		return false
	}
}
