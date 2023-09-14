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

type boolEncoder struct{}
type boolDecoder struct{}

func newBoolEncoder() Encoder {
	return &boolEncoder{}
}

func newBoolDecoder() Decoder {
	return &boolDecoder{}
}

func (*boolEncoder) fromPropertyValue(_ convertCtx, p resource.PropertyValue) (tftypes.Value, error) {
	if propertyValueIsUnkonwn(p) {
		return tftypes.NewValue(tftypes.Bool, tftypes.UnknownValue), nil
	}
	if p.IsNull() {
		return tftypes.NewValue(tftypes.Bool, nil), nil
	}
	if !p.IsBool() {
		return tftypes.NewValue(tftypes.Bool, nil),
			fmt.Errorf("Expected a Boolean")
	}
	return tftypes.NewValue(tftypes.Bool, p.BoolValue()), nil
}

func (*boolDecoder) toPropertyValue(v tftypes.Value) (resource.PropertyValue, error) {
	if !v.IsKnown() {
		return unknownProperty(), nil
	}
	if v.IsNull() {
		return resource.NewPropertyValue(nil), nil
	}
	var b bool
	if err := v.As(&b); err != nil {
		return resource.PropertyValue{},
			fmt.Errorf("decBool fails with %s: %w", v.String(), err)
	}
	return resource.NewBoolProperty(b), nil
}
