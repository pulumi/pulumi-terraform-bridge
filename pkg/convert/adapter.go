// Copyright 2016-2024, Pulumi Corporation.
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
	"math/big"
	"strconv"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

// adaptedEncoder wraps an encoder in an adapter during encoding.
//
// Given [resource.PropertyValue] types P_i and P_j and an encoder P_j -> T, the adapter
// function should translate P_i -> P_j.
type adaptedEncoder[T Encoder] struct {
	adapter func(resource.PropertyValue) (resource.PropertyValue, error)
	encoder T
}

func (e adaptedEncoder[T]) fromPropertyValue(v resource.PropertyValue) (tftypes.Value, error) {
	adapted, err := e.adapter(v)
	if err != nil {
		return tftypes.Value{}, fmt.Errorf("failed to adapt for %T: %w", e.encoder, err)
	}
	return e.encoder.fromPropertyValue(adapted)
}

type adaptedDecoder[T Decoder] struct {
	adapter func(tftypes.Value) (tftypes.Value, error)
	decoder T
}

func (d adaptedDecoder[T]) toPropertyValue(v tftypes.Value) (resource.PropertyValue, error) {
	adapted, err := d.adapter(v)
	if err != nil {
		return resource.PropertyValue{}, fmt.Errorf("failed to adapt for %T: %w", d.decoder, err)
	}
	return decode(d.decoder, adapted)
}

func newIntOverrideStringEncoder() Encoder {
	return adaptedEncoder[*numberEncoder]{
		adapter: func(v resource.PropertyValue) (resource.PropertyValue, error) {
			if v.IsString() {
				f, err := strconv.ParseFloat(v.StringValue(), 64)
				if err != nil {
					return resource.PropertyValue{}, err
				}
				return resource.NewProperty(f), nil
			}
			return v, nil
		},
		encoder: &numberEncoder{},
	}
}

func newStringOverIntDecoder() Decoder {
	return adaptedDecoder[*stringDecoder]{
		adapter: func(v tftypes.Value) (tftypes.Value, error) {
			if !v.Type().Is(tftypes.Number) {
				return v, nil
			}
			if !v.IsKnown() {
				return tftypes.NewValue(tftypes.String, tftypes.UnknownValue), nil
			}
			var f big.Float
			if err := v.As(&f); err != nil {
				return tftypes.Value{}, err
			}
			return tftypes.NewValue(tftypes.String, f.Text('f', -1)), nil
		},
		decoder: &stringDecoder{},
	}
}
