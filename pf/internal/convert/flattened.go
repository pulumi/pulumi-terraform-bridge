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

func (enc *flattenedEncoder) fromPropertyValue(ctx convertCtx, v resource.PropertyValue) (tftypes.Value, error) {
	encoded, err := enc.elementEncoder.fromPropertyValue(ctx, v)
	if err != nil {
		return tftypes.Value{}, nil
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
		return resource.PropertyValue{}, nil
	}
	switch len(list) {
	case 0:
		return resource.NewNullProperty(), nil
	case 1:
		return dec.elementDecoder.toPropertyValue(list[0])
	default:
		msg := "IsMaxItemsOne list or set has too many (%d) values"
		err := fmt.Errorf(msg, len(list))
		return resource.PropertyValue{}, err
	}
}
