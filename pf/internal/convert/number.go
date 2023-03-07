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

	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

type numberEncoder struct{}
type numberDecoder struct{}

func newNumberEncoder() Encoder {
	return &numberEncoder{}
}

func newNumberDecoder() Decoder {
	return &numberDecoder{}
}

func (*numberEncoder) FromPropertyValue(p resource.PropertyValue) (tftypes.Value, error) {
	if propertyValueIsUnkonwn(p) {
		return tftypes.NewValue(tftypes.Number, tftypes.UnknownValue), nil
	}
	if p.IsNull() {
		return tftypes.NewValue(tftypes.Number, nil), nil
	}
	if !p.IsNumber() {
		return tftypes.NewValue(tftypes.Number, nil),
			fmt.Errorf("Expected a Number")
	}
	return tftypes.NewValue(tftypes.Number, p.NumberValue()), nil
}

func (*numberDecoder) ToPropertyValue(v tftypes.Value) (resource.PropertyValue, error) {
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
