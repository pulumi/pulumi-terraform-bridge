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

package convert

import (
	"fmt"

	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

type objectEncoder struct {
	objectType       tftypes.Object
	propertyEncoders map[TerraformPropertyName]Encoder
	propertyNames    localPropertyNames
}

type objectDecoder struct {
	objectType       tftypes.Object
	propertyDecoders map[TerraformPropertyName]Decoder
	propertyNames    localPropertyNames
}

func newObjectEncoder(objectType tftypes.Object,
	propertyEncoders map[TerraformPropertyName]Encoder, propertyNames localPropertyNames) (Encoder, error) {
	for prop := range objectType.AttributeTypes {
		if _, ok := propertyEncoders[prop]; !ok {
			return nil, fmt.Errorf("Missing property encoder for %q", prop)
		}
	}
	return &objectEncoder{
		objectType:       objectType,
		propertyEncoders: propertyEncoders,
		propertyNames:    propertyNames,
	}, nil
}

func newObjectDecoder(objectType tftypes.Object,
	propertyDecoders map[TerraformPropertyName]Decoder, propertyNames localPropertyNames) (Decoder, error) {
	return &objectDecoder{
		objectType:       objectType,
		propertyDecoders: propertyDecoders,
		propertyNames:    propertyNames,
	}, nil
}

func (enc *objectEncoder) fromPropertyValue(p resource.PropertyValue) (tftypes.Value, error) {
	if propertyValueIsUnkonwn(p) {
		return tftypes.NewValue(enc.objectType, tftypes.UnknownValue), nil
	}
	if p.IsNull() {
		return tftypes.NewValue(enc.objectType, nil), nil
	}
	if !p.IsObject() {
		return tftypes.NewValue(enc.objectType, nil),
			fmt.Errorf("Expected an Object PropertyValue")
	}
	pulumiMap := p.ObjectValue()
	values := map[string]tftypes.Value{}
	for attr, attrEncoder := range enc.propertyEncoders {
		t := enc.objectType.AttributeTypes[attr]
		key := enc.propertyNames.PropertyKey(attr, t)
		pv, gotPV := pulumiMap[key]
		if !gotPV {
			pv = resource.NewNullProperty()
		}
		v, err := attrEncoder.fromPropertyValue(pv)
		if err != nil {
			return tftypes.NewValue(enc.objectType, nil),
				fmt.Errorf("objectEncoder failed on property %q: %w", attr, err)
		}
		values[attr] = v
	}
	return tftypes.NewValue(enc.objectType, values), nil
}

func (dec *objectDecoder) toPropertyValue(v tftypes.Value) (resource.PropertyValue, error) {
	if !v.IsKnown() {
		return unknownProperty(), nil
	}
	if v.IsNull() {
		return resource.NewPropertyValue(nil), nil
	}
	elements := map[string]tftypes.Value{}
	if err := v.As(&elements); err != nil {
		return resource.PropertyValue{},
			fmt.Errorf("objectDecoder fails with %s: %w", v.String(), err)
	}

	values := make(resource.PropertyMap)
	for attr, decoder := range dec.propertyDecoders {
		attrValue, gotAttrValue := elements[attr]
		if gotAttrValue {
			t := dec.objectType.AttributeTypes[attr]
			pv, err := decoder.toPropertyValue(attrValue)
			if err != nil {
				return resource.PropertyValue{},
					fmt.Errorf("objectDecoder fails on property %q (value %s): %w",
						attr, attrValue, err)
			}
			key := dec.propertyNames.PropertyKey(attr, t)
			if !pv.IsNull() {
				values[key] = pv
			}
		}
	}
	return resource.NewObjectProperty(values), nil
}
