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

type secretEncoder struct {
	elementEncoder Encoder
	tfType         tftypes.Type
}

type secretDecoder struct {
	elementDecoder Decoder
}

func newSecretEncoder(elementEncoder Encoder, tfType tftypes.Type) (Encoder, error) {
	return &secretEncoder{
		elementEncoder: elementEncoder,
		tfType:         tfType,
	}, nil
}

func newSecretDecoder(elementDecoder Decoder) (Decoder, error) {
	return &secretDecoder{
		elementDecoder: elementDecoder,
	}, nil
}

func (enc *secretEncoder) FromPropertyValue(p resource.PropertyValue) (tftypes.Value, error) {
	if propertyValueIsUnkonwn(p) {
		return tftypes.NewValue(enc.tfType, tftypes.UnknownValue), nil
	}
	if p.IsNull() {
		return tftypes.NewValue(enc.tfType, nil), nil
	}
	if !p.IsSecret() {
		return tftypes.Value{}, fmt.Errorf("PropertyValue should be secret but is not")
	}
	v := p.SecretValue().Element
	return enc.elementEncoder.FromPropertyValue(v)
}

func (dec *secretDecoder) ToPropertyValue(v tftypes.Value) (resource.PropertyValue, error) {
	if !v.IsKnown() || v.IsNull() {
		return dec.elementDecoder.ToPropertyValue(v)
	}
	encoded, err := dec.elementDecoder.ToPropertyValue(v)
	if err != nil {
		return resource.PropertyValue{}, err
	}
	return resource.MakeSecret(encoded), nil
}
