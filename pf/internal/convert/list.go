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

type listEncoder struct {
	elementType    tftypes.Type
	elementEncoder Encoder
}

type listDecoder struct {
	elementDecoder Decoder
}

func newListEncoder(elementType tftypes.Type, elementEncoder Encoder) (Encoder, error) {
	return &listEncoder{
		elementType:    elementType,
		elementEncoder: elementEncoder,
	}, nil
}

func newListDecoder(elementDecoder Decoder) (Decoder, error) {
	return &listDecoder{
		elementDecoder: elementDecoder,
	}, nil
}

func (enc *listEncoder) FromPropertyValue(p resource.PropertyValue) (tftypes.Value, error) {
	listTy := tftypes.List{ElementType: enc.elementType}

	if propertyValueIsUnkonwn(p) {
		return tftypes.NewValue(listTy, tftypes.UnknownValue), nil
	}
	if p.IsNull() {
		return tftypes.NewValue(listTy, nil), nil
	}
	if !p.IsArray() {
		return tftypes.NewValue(listTy, nil),
			fmt.Errorf("Expected an Array PropertyValue")
	}
	var values []tftypes.Value
	for i, pv := range p.ArrayValue() {
		v, err := enc.elementEncoder.FromPropertyValue(pv)
		if err != nil {
			return tftypes.NewValue(listTy, nil),
				fmt.Errorf("encList failed while encoding element %d (%v): %w",
					i, pv, err)
		}
		values = append(values, v)
	}
	return tftypes.NewValue(listTy, values), nil
}

func (dec *listDecoder) ToPropertyValue(v tftypes.Value) (resource.PropertyValue, error) {
	if !v.IsKnown() {
		zero := resource.NewArrayProperty([]resource.PropertyValue{})
		return resource.NewComputedProperty(resource.Computed{Element: zero}), nil
	}
	if v.IsNull() {
		return resource.NewPropertyValue(nil), nil
	}
	var elements []tftypes.Value
	if err := v.As(&elements); err != nil {
		return resource.PropertyValue{},
			fmt.Errorf("decList fails with %s: %w", v.String(), err)
	}
	values := []resource.PropertyValue{}
	for _, e := range elements {
		ev, err := dec.elementDecoder.ToPropertyValue(e)
		if err != nil {
			return resource.PropertyValue{},
				fmt.Errorf("decList fails with %s: %w", e.String(), err)
		}
		values = append(values, ev)
	}
	return resource.NewArrayProperty(values), nil
}
