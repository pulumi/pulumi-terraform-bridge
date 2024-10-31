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

	"github.com/golang/protobuf/ptypes/struct"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"

	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/unstable/propertyvalue"
)

type ConfigEncoding struct {
	fieldTypes map[resource.PropertyKey]shim.ValueType
}

func NewConfigEncoding(config shim.SchemaMap, configInfos map[string]*SchemaInfo) *ConfigEncoding {
	var tfKeys []string
	config.Range(func(tfKey string, value shim.Schema) bool {
		tfKeys = append(tfKeys, tfKey)
		return true
	})
	fieldTypes := make(map[resource.PropertyKey]shim.ValueType)

	for _, tfKey := range tfKeys {
		pulumiKey := resource.PropertyKey(TerraformToPulumiNameV2(tfKey, config, configInfos))
		schema := config.Get(tfKey)
		fieldTypes[pulumiKey] = schema.Type()
	}
	return &ConfigEncoding{fieldTypes}
}

func (*ConfigEncoding) tryUnwrapSecret(encoded any) (any, bool) {
	m, ok := encoded.(map[string]any)
	if !ok {
		return nil, false
	}
	sig, ok := m["4dabf18193072939515e22adb298388d"]
	if !ok {
		return nil, false
	}
	ss, ok := sig.(string)
	if !ok {
		return nil, false
	}
	if ss != "1b47061264138c4ac30d75fd1eb44270" {
		return nil, false
	}
	value, ok := m["value"]
	return value, ok
}

func (enc *ConfigEncoding) convertStringToPropertyValue(s string, typ shim.ValueType) (resource.PropertyValue, error) {
	// If the schema expects a string, we can just return this as-is.
	if typ == shim.TypeString {
		return resource.NewStringProperty(s), nil
	}

	// Otherwise, we will attempt to deserialize the input string as JSON and convert the result into a Pulumi
	// property. If the input string is empty, we will return an appropriate zero value.
	if s == "" {
		return enc.zeroValue(typ), nil
	}

	var jsonValue interface{}
	if err := json.Unmarshal([]byte(s), &jsonValue); err != nil {
		return resource.PropertyValue{}, err
	}

	// Instead of using resource.NewPropertyValue, specialize it to detect nested json-encoded secrets.
	var replv func(encoded any) (resource.PropertyValue, bool)
	replv = func(encoded any) (resource.PropertyValue, bool) {
		encodedSecret, isSecret := enc.tryUnwrapSecret(encoded)
		if !isSecret {
			return resource.NewNullProperty(), false
		}

		v := resource.NewPropertyValueRepl(encodedSecret, nil, replv)

		return v, true
	}

	return resource.NewPropertyValueRepl(jsonValue, nil, replv), nil
}

func (*ConfigEncoding) zeroValue(typ shim.ValueType) resource.PropertyValue {
	switch typ {
	case shim.TypeBool:
		return resource.NewPropertyValue(false)
	case shim.TypeInt, shim.TypeFloat:
		return resource.NewPropertyValue(0)
	case shim.TypeList, shim.TypeSet:
		return resource.NewPropertyValue([]interface{}{})
	default:
		return resource.NewPropertyValue(map[string]interface{}{})
	}
}

func (enc *ConfigEncoding) UnmarshalProperties(props *structpb.Struct) (resource.PropertyMap, error) {
	m, err := plugin.UnmarshalProperties(props, plugin.MarshalOptions{
		Label:        "config",
		KeepUnknowns: true,
		SkipNulls:    true,
		RejectAssets: true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal to provider values: %w", err)
	}

	return enc.UnfoldProperties(m)
}

func (enc *ConfigEncoding) UnfoldProperties(m resource.PropertyMap) (resource.PropertyMap, error) {
	result := make(resource.PropertyMap, len(m))
	for _, pk := range m.StableKeys() {
		v := m[pk]

		// If we don't have type information on the field, we don't attempt to do
		// anything with it.
		typ, hasType := enc.fieldTypes[pk]
		if !hasType {
			result[pk] = v
			continue
		}

		contract.Assertf(!v.IsSecret() && !(v.IsOutput() && v.OutputValue().Secret),
			"The bridge does not accept secrets, so we should not encounter them here")

		if v.IsString() {
			prop, err := enc.convertStringToPropertyValue(v.StringValue(), typ)
			if err != nil {
				return nil, err
			}
			v = prop
		}

		// Computed sentinels are coming in as always having an empty string, but
		// the encoding coerces them to a zero value of the appropriate type.
		if v.IsComputed() && v.V.(resource.Computed).Element.V == "" {
			v = resource.MakeComputed(enc.zeroValue(enc.fieldTypes[pk]))
		}

		result[pk] = v
	}

	return result, nil
}

// Inverse of [ConfigEncoding.UnfoldProperties], with additional support for
// secrets. Since the encoding cannot represent nested secrets, any nested secrets will be
// approximated by making the entire top-level property secret.
func (enc *ConfigEncoding) FoldProperties(props resource.PropertyMap) (resource.PropertyMap, error) {
	dst := make(resource.PropertyMap, len(props))
	for k, v := range props {
		var err error
		dst[k], err = enc.jsonEncodePropertyValue(k, v)
		if err != nil {
			return nil, err
		}
	}
	return dst, nil
}

// Inverse of UnmarshalProperties, with additional support for secrets. Since the encoding cannot represent nested
// secrets, any nested secrets will be approximated by making the entire top-level property secret.
func (enc *ConfigEncoding) MarshalProperties(props resource.PropertyMap) (*structpb.Struct, error) {
	copy, err := enc.FoldProperties(props)
	if err != nil {
		return nil, err
	}
	return plugin.MarshalProperties(copy, plugin.MarshalOptions{
		Label:        "config",
		KeepUnknowns: true,
		SkipNulls:    true,
		RejectAssets: true,
		KeepSecrets:  true,
	})
}

func (enc *ConfigEncoding) jsonEncodePropertyValue(k resource.PropertyKey,
	v resource.PropertyValue,
) (resource.PropertyValue, error) {
	if v.ContainsUnknowns() {
		return resource.NewStringProperty(plugin.UnknownStringValue), nil
	}
	if v.ContainsSecrets() {
		encoded, err := enc.jsonEncodePropertyValue(k, propertyvalue.RemoveSecrets(v))
		if err != nil {
			return v, err
		}
		return resource.MakeSecret(encoded), err
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
