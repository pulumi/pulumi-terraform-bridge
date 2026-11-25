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

type flattenedEncoder struct {
	collectionType tftypes.Type
	elementEncoder Encoder
}

func (enc *flattenedEncoder) fromPropertyValue(v resource.PropertyValue) (tftypes.Value, error) {
	// FIX: Handle cases where IsMaxItemsOne incorrectly identified a multi-item list
	// If the input is an array (which it shouldn't be for true MaxItemsOne), encode all elements
	if v.IsArray() {
		list := []tftypes.Value{}
		for _, elem := range v.ArrayValue() {
			encoded, err := enc.elementEncoder.fromPropertyValue(elem)
			if err != nil {
				return tftypes.Value{}, fmt.Errorf("encoding list element: %w", err)
			}
			if !encoded.IsNull() {
				list = append(list, encoded)
			}
		}
		return tftypes.NewValue(enc.collectionType, list), nil
	}

	// Original single-element encoding logic
	encoded, err := enc.elementEncoder.fromPropertyValue(v)
	if err != nil {
		return tftypes.Value{}, err
	}

	list := []tftypes.Value{}
	if !encoded.IsNull() {
		list = append(list, encoded)
	}

	return tftypes.NewValue(enc.collectionType, list), nil
}

type flattenedDecoder struct {
	elementDecoder Decoder
}

func (dec *flattenedDecoder) toPropertyValue(v tftypes.Value) (resource.PropertyValue, error) {
	var list []tftypes.Value
	if err := v.As(&list); err != nil {
		return resource.PropertyValue{}, err
	}
	switch len(list) {
	case 0:
		return resource.NewNullProperty(), nil
	case 1:
		return decode(dec.elementDecoder, list[0])
	default:
		// FIX: Handle cases where IsMaxItemsOne incorrectly identified a multi-item list
		// Instead of erroring, decode all elements and return as an array
		// This fixes a bug where Plugin Framework ListNestedBlock types were being
		// truncated to 1 element in the Go SDK.
		values := []resource.PropertyValue{}
		for _, e := range list {
			ev, err := decode(dec.elementDecoder, e)
			if err != nil {
				return resource.PropertyValue{}, fmt.Errorf("decoding list element: %w", err)
			}
			values = append(values, ev)
		}
		return resource.NewArrayProperty(values), nil
	}
}
