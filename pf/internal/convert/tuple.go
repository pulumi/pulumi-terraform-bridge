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

var _ Encoder = ((*tupleEncoder)(nil))

type tupleEncoder struct {
	types    []tftypes.Type
	encoders []Encoder
}

var _ Decoder = ((*tupleDecoder)(nil))

type tupleDecoder struct {
	decoders []Decoder
}

func (enc *tupleEncoder) FromPropertyValue(p resource.PropertyValue) (tftypes.Value, error) {
	typ := tftypes.Tuple{ElementTypes: enc.types}
	if propertyValueIsUnkonwn(p) {
		return tftypes.NewValue(typ, tftypes.UnknownValue), nil
	}
	if p.IsNull() {
		return tftypes.NewValue(typ, nil), nil
	}
	if !p.IsArray() || len(p.ArrayValue()) != len(enc.types) {
		return tftypes.NewValue(typ, nil),
			fmt.Errorf("Expected an Array PropertyValue of length %d", len(enc.types))
	}

	var values []tftypes.Value
	for i, pv := range p.ArrayValue() {
		v, err := enc.encoders[i].FromPropertyValue(pv)
		if err != nil {
			return tftypes.NewValue(typ, nil),
				fmt.Errorf("failed to encode '%v' into tuple[%d] (%v): %w",
					pv, i, enc.types[i], err)
		}
		values = append(values, v)
	}
	return tftypes.NewValue(typ, values), nil
}

func (dec *tupleDecoder) ToPropertyValue(v tftypes.Value) (resource.PropertyValue, error) {
	if !v.IsKnown() {
		zero := resource.NewArrayProperty([]resource.PropertyValue{})
		return resource.MakeComputed(zero), nil
	}
	if v.IsNull() {
		return resource.NewNullProperty(), nil
	}

	var elements []tftypes.Value
	if err := v.As(&elements); err != nil {
		return resource.PropertyValue{},
			fmt.Errorf("failed to decode tuple (%s): %w", v, err)
	}
	values := make([]resource.PropertyValue, len(elements))
	for i, e := range elements {
		var err error
		values[i], err = dec.decoders[i].ToPropertyValue(e)
		if err != nil {
			return resource.PropertyValue{},
				fmt.Errorf("failed to decode tuple[%d] (%s): %w", i, v, err)
		}
	}
	return resource.NewArrayProperty(values), nil
}
