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
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

// To support DynamicPseudoType, perform a best-effort conversion at the data level without any type information.
func newDynamicEncoder() Encoder {
	return &dynamicEncoder{}
}

type dynamicEncoder struct{}

func (*dynamicEncoder) fromPropertyValue(p resource.PropertyValue) (tftypes.Value, error) {
	if propertyValueIsUnkonwn(p) {
		return tftypes.NewValue(tftypes.Object{}, tftypes.UnknownValue), nil
	}
	if p.IsNull() {
		return tftypes.NewValue(tftypes.Object{}, nil), nil
	}
	if p.IsString() {
		return tftypes.NewValue(tftypes.String, p.StringValue()), nil
	}

	panic("TODO More types needed!")
}

type dynamicDecoder struct{}

func newDynamicDecoder() Decoder {
	return &dynamicDecoder{}
}

func (*dynamicDecoder) toPropertyValue(v tftypes.Value) (resource.PropertyValue, error) {
	panic("TOOD implement dynamic decoder")
}
