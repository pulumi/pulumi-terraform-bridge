// Copyright 2016-2023, Pulumi Corporation.
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

// Secrets only have a Decoder, not an Encoder because they are discarded when reading a resource.PropertyValue before
// proceeding with encoding. First-class secrets have no representation in tftypes.Value.
type secretDecoder struct {
	elementDecoder Decoder
}

func newSecretDecoder(elementDecoder Decoder) (Decoder, error) {
	return &secretDecoder{
		elementDecoder: elementDecoder,
	}, nil
}

func (dec *secretDecoder) toPropertyValue(v tftypes.Value) (resource.PropertyValue, error) {
	encoded, err := dec.elementDecoder.toPropertyValue(v)
	if err != nil {
		return resource.PropertyValue{}, err
	}
	// Not entirely certain here if Pulumi needs nil and unknown secrets to wrapped in resource.MakeSecret or not,
	// assuming they do not need to be wrapped.
	if !v.IsKnown() || v.IsNull() {
		return encoded, nil
	}
	return resource.MakeSecret(encoded), nil
}
