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

type setEncoder struct {
	elementType    tftypes.Type
	elementEncoder Encoder
}

type setDecoder struct {
	elementDecoder Decoder
}

func newSetEncoder(elementType tftypes.Type, elementEncoder Encoder) (Encoder, error) {
	return &setEncoder{
		elementType:    elementType,
		elementEncoder: elementEncoder,
	}, nil
}

func newSetDecoder(elementDecoder Decoder) (Decoder, error) {
	return &setDecoder{
		elementDecoder: elementDecoder,
	}, nil
}

func (enc *setEncoder) fromPropertyValue(p resource.PropertyValue) (tftypes.Value, error) {
	setTy := tftypes.Set{ElementType: enc.elementType}

	if propertyValueIsUnkonwn(p) {
		return tftypes.NewValue(setTy, tftypes.UnknownValue), nil
	}
	if p.IsNull() {
		return tftypes.NewValue(setTy, nil), nil
	}

	retErr := func(msg string, args ...any) error {
		return fmt.Errorf("set encoder failed: "+msg, args...)
	}

	if !p.IsArray() {
		return tftypes.NewValue(setTy, nil),
			retErr("expected an Array PropertyValue, got a %T", p)
	}
	var values []tftypes.Value
	for i, pv := range p.ArrayValue() {
		v, err := enc.elementEncoder.fromPropertyValue(pv)
		if err != nil {
			return tftypes.NewValue(setTy, nil),
				retErr("encoding element %d (%v): %w", i, pv, err)
		}
		values = append(values, v)
	}
	return tftypes.NewValue(setTy, values), nil
}

func (dec *setDecoder) toPropertyValue(v tftypes.Value) (resource.PropertyValue, error) {
	if !v.IsKnown() {
		return unknownProperty(), nil
	}
	if v.IsNull() {
		return resource.NewPropertyValue(nil), nil
	}

	retErr := func(msg string, args ...any) error {
		return fmt.Errorf("set decoder failed: "+msg, args...)
	}

	var elements []tftypes.Value
	if err := v.As(&elements); err != nil {
		return resource.PropertyValue{},
			retErr("could not convert %s to an []Value: %w", v.String(), err)
	}
	values := []resource.PropertyValue{}
	for i, e := range elements {
		ev, err := dec.elementDecoder.toPropertyValue(e)
		if err != nil {
			return resource.PropertyValue{},
				retErr("could not decode element %d (%v): %w", i, ev, err)
		}
		values = append(values, ev)
	}
	return resource.NewArrayProperty(values), nil
}
