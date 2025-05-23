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
	"math/big"
	"strconv"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

type (
	numberEncoder struct{}
	numberDecoder struct{}
)

func newNumberEncoder() Encoder {
	return &numberEncoder{}
}

func newNumberDecoder() Decoder {
	return &numberDecoder{}
}

func (ne *numberEncoder) fromPropertyValue(p resource.PropertyValue) (tftypes.Value, error) {
	if propertyValueIsUnknown(p) {
		return tftypes.NewValue(tftypes.Number, tftypes.UnknownValue), nil
	}
	if p.IsNull() {
		return tftypes.NewValue(tftypes.Number, nil), nil
	}
	// TODO[pulumi/pulumi-terraform-bridge#1667] workaround a problem in
	// https://github.com/pulumi/pulumi-aws/issues/5222 where SDKv2 TypeNullableInt gets parsed
	// into a schema that now expects a number. This case interprets an empty string value as a
	// nil number when parsing numbers.
	if p.IsString() {
		if p.StringValue() == "" {
			return tftypes.NewValue(tftypes.Number, nil), nil
		}
		v, ok := ne.tryParseNumber(p.StringValue())
		if ok {
			return tftypes.NewValue(tftypes.Number, v), nil
		}
	}
	if !p.IsNumber() {
		return tftypes.NewValue(tftypes.Number, nil),
			fmt.Errorf("Expected a Number, got %#v %v", p, p.IsString())
	}
	return tftypes.NewValue(tftypes.Number, p.NumberValue()), nil
}

func (*numberEncoder) tryParseNumber(s string) (any, bool) {
	if v, err := strconv.ParseInt(s, 10 /* base */, 64 /* bitSize */); err == nil {
		return v, true
	}
	if v, err := strconv.ParseFloat(s, 64); err == nil {
		return v, true
	}
	return nil, false
}

func (*numberDecoder) toPropertyValue(v tftypes.Value) (resource.PropertyValue, error) {
	if !v.IsKnown() {
		return unknownProperty(), nil
	}
	if v.IsNull() {
		return resource.NewPropertyValue(nil), nil
	}
	var n big.Float
	if err := v.As(&n); err != nil {
		return resource.PropertyValue{},
			fmt.Errorf("decNumber fails with %s: %w", v.String(), err)
	}
	f64, _ := n.Float64()
	return resource.NewNumberProperty(f64), nil
}
