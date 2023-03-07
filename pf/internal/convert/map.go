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

type mapEncoder struct {
	elementType    tftypes.Type
	elementEncoder Encoder
}

type mapDecoder struct {
	elementDecoder Decoder
}

func newMapEncoder(elementType tftypes.Type, elementEncoder Encoder) (Encoder, error) {
	return &mapEncoder{
		elementType:    elementType,
		elementEncoder: elementEncoder,
	}, nil
}

func newMapDecoder(elementDecoder Decoder) (Decoder, error) {
	return &mapDecoder{
		elementDecoder: elementDecoder,
	}, nil
}

func (enc *mapEncoder) FromPropertyValue(p resource.PropertyValue) (tftypes.Value, error) {
	mapTy := tftypes.Map{ElementType: enc.elementType}
	if propertyValueIsUnkonwn(p) {
		return tftypes.NewValue(mapTy, tftypes.UnknownValue), nil
	}
	if p.IsNull() {
		return tftypes.NewValue(mapTy, nil), nil
	}
	if !p.IsObject() {
		return tftypes.NewValue(mapTy, nil),
			fmt.Errorf("Expected an Object PropertyValue")
	}
	values := map[string]tftypes.Value{}
	for key, pv := range p.ObjectValue() {
		v, err := enc.elementEncoder.FromPropertyValue(pv)
		if err != nil {
			return tftypes.NewValue(mapTy, nil),
				fmt.Errorf("encMap failed on %v", pv)
		}
		values[string(key)] = v
	}
	return tftypes.NewValue(mapTy, values), nil
}

func (dec *mapDecoder) ToPropertyValue(v tftypes.Value) (resource.PropertyValue, error) {
	if !v.IsKnown() {
		return unknownProperty(), nil
	}
	if v.IsNull() {
		return resource.NewPropertyValue(nil), nil
	}
	elements := map[string]tftypes.Value{}
	if err := v.As(&elements); err != nil {
		return resource.PropertyValue{},
			fmt.Errorf("decMap fails with %s: %w", v.String(), err)
	}

	values := make(resource.PropertyMap)
	for k, e := range elements {
		ev, err := dec.elementDecoder.ToPropertyValue(e)
		if err != nil {
			return resource.PropertyValue{},
				fmt.Errorf("decMap fails with %s: %w", e.String(), err)
		}
		values[resource.PropertyKey(k)] = ev
	}
	return resource.NewObjectProperty(values), nil
}
