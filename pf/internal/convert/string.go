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

type stringEncoder struct{}
type stringDecoder struct{}

func newStringEncoder() Encoder {
	return &stringEncoder{}
}

func newStringDecoder() Decoder {
	return &stringDecoder{}
}

func (*stringEncoder) fromPropertyValue(ctx convertCtx, p resource.PropertyValue) (tftypes.Value, error) {
	if propertyValueIsUnkonwn(p) {
		return tftypes.NewValue(tftypes.String, tftypes.UnknownValue), nil
	}

	switch {
	case p.IsNull():
		return tftypes.NewValue(tftypes.String, nil), nil

	// Special-case values that can be converted into strings for backward
	// comparability with SDKv{1,2} based resources.
	//
	// Unfortunately, it is not possible to round trip values that are not string
	// typed with full fidelity. For example, consider this simple YAML program:
	//
	//	resources:
	//	  r:
	//	    type: some:simple:Resource
	//	    properties:
	//	      stringType: 0x1234
	//
	// In YAML, 0x1234 is parsed as the number 4660, so our recourse will receive
	// `4660` as its output. This problem appears even for simple numbers:
	//
	//
	//	resources:
	//	  r:
	//	    type: some:simple:Resource
	//	    properties:
	//	      s1: 1
	//	      s2: 1.0
	//
	// Go formats float64(1) as "1", so we get the expected result. float64(1) is
	// equal to float64(1.0), so we are unable to distinguish between s1 and s2.
	//
	// We have the same problems with bools: YAML parses "YES" as true, so we are
	// unable to distinguish between the two.

	case p.IsBool():
		ctx.warn("converting from bool to string")
		return tftypes.NewValue(tftypes.String, fmt.Sprintf("%v", p.BoolValue())), nil
	case p.IsNumber():
		ctx.warn("converting from number to string")
		return tftypes.NewValue(tftypes.String, fmt.Sprintf("%v", p.NumberValue())), nil

	case p.IsString():
		return tftypes.NewValue(tftypes.String, p.StringValue()), nil

	default:
		return tftypes.NewValue(tftypes.String, nil),
			fmt.Errorf("Expected a string, got: %v", p)
	}
}

func (*stringDecoder) toPropertyValue(v tftypes.Value) (resource.PropertyValue, error) {
	if !v.IsKnown() {
		return unknownProperty(), nil
	}
	if v.IsNull() {
		return resource.NewPropertyValue(nil), nil
	}
	var s string
	if err := v.As(&s); err != nil {
		return resource.PropertyValue{},
			fmt.Errorf("tftypes.Value.As(string) failed: %w", err)
	}
	return resource.NewStringProperty(s), nil
}
