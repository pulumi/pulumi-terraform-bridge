// Copyright 2016-2022, Pulumi Corporation.
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

// Converts between Pulumi and Terraform value representations. Conversions are type-driven and require to know
// Terraform types since Terraform values must be tagged with their corresponding types. Pulumi type metadata is also
// required to make finer grained distinctions such as int vs float, and to correctly handle object properties that have
// Pulumi names that differ from their Terraform names.
package convert

import (
	"fmt"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/pulumi/pulumi-terraform-bridge/pf/internal/propertyvalue"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

// An alias to assist marking Terraform-level property names (see for example AttributeTypes in tftypes.Object). Pulumi
// may rename properties and it is important to keep track of which name is being used during conversion.
type TerraformPropertyName = string

type Encoding interface {
	NewConfigEncoder(tftypes.Object) (Encoder, error)
	NewResourceDecoder(tokens.Type, tftypes.Object) (Decoder, error)
	NewResourceEncoder(tokens.Type, tftypes.Object) (Encoder, error)
	NewDataSourceDecoder(tokens.ModuleMember, tftypes.Object) (Decoder, error)
	NewDataSourceEncoder(tokens.ModuleMember, tftypes.Object) (Encoder, error)
}

type PropertyNames interface {
	// Translates a Terraform property name for a given type to a Pulumi PropertyKey.
	//
	// typeToken identifies the resource, data source, or named object type.
	PropertyKey(typeToken tokens.Token, property TerraformPropertyName, t tftypes.Type) resource.PropertyKey

	// Same as PropertyKey but for provider-level configuration properties.
	ConfigPropertyKey(property TerraformPropertyName, t tftypes.Type) resource.PropertyKey
}

// Like PropertyNames but specialized to either a type by token or config property.
type LocalPropertyNames interface {
	PropertyKey(property TerraformPropertyName, t tftypes.Type) resource.PropertyKey
}

type typeLocalPropertyNames struct {
	propertyNames PropertyNames
	typeToken     tokens.Token
}

func (l *typeLocalPropertyNames) PropertyKey(property TerraformPropertyName, t tftypes.Type) resource.PropertyKey {
	return l.propertyNames.PropertyKey(l.typeToken, property, t)
}

func NewTypeLocalPropertyNames(pn PropertyNames, tok tokens.Token) LocalPropertyNames {
	return &typeLocalPropertyNames{pn, tok}
}

type configLocalPropertyNames struct {
	propertyNames PropertyNames
}

func (l *configLocalPropertyNames) PropertyKey(property TerraformPropertyName, t tftypes.Type) resource.PropertyKey {
	return l.propertyNames.ConfigPropertyKey(property, t)
}

func NewConfigPropertyNames(pn PropertyNames) LocalPropertyNames {
	return &configLocalPropertyNames{pn}
}

type Encoder interface {
	fromPropertyValue(resource.PropertyValue) (tftypes.Value, error)
}

type Decoder interface {
	toPropertyValue(tftypes.Value) (resource.PropertyValue, error)
}

func EncodePropertyMap(enc Encoder, pmap resource.PropertyMap) (tftypes.Value, error) {
	return enc.fromPropertyValue(propertyvalue.RemoveSecrets(resource.NewObjectProperty(pmap)))
}

func DecodePropertyMap(dec Decoder, v tftypes.Value) (resource.PropertyMap, error) {
	pv, err := dec.toPropertyValue(v)
	if err != nil {
		return nil, err
	}
	if !pv.IsObject() {
		return nil, fmt.Errorf("Expected an Object, got: %v", pv.String())
	}
	return pv.ObjectValue(), nil
}

func EncodePropertyMapToDynamic(enc Encoder, objectType tftypes.Object,
	pmap resource.PropertyMap) (*tfprotov6.DynamicValue, error) {
	v, err := EncodePropertyMap(enc, pmap)
	if err != nil {
		return nil, err
	}
	dv, err := tfprotov6.NewDynamicValue(objectType, v)
	return &dv, err
}

func DecodePropertyMapFromDynamic(dec Decoder, objectType tftypes.Object,
	dv *tfprotov6.DynamicValue) (resource.PropertyMap, error) {
	v, err := dv.Unmarshal(objectType)
	if err != nil {
		return nil, fmt.Errorf("DynamicValue.Unmarshal failed: %w", err)
	}
	return DecodePropertyMap(dec, v)
}
