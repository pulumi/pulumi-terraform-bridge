// Copyright 2016-2026, Pulumi Corporation.
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

// promoteEncoder encodes a known, non-null, non-array value as a single-element collection.
type promoteEncoder struct {
	collectionEncoder Encoder
}

func promoteToCollection(pctx *schemaPropContext, enc Encoder) Encoder {
	if !pctx.promoteToCollection {
		return enc
	}
	return &promoteEncoder{collectionEncoder: enc}
}

func (enc *promoteEncoder) fromPropertyValue(p resource.PropertyValue) (tftypes.Value, error) {
	if !p.IsArray() && !p.IsNull() && !propertyValueIsUnknown(p) {
		p = resource.NewArrayProperty([]resource.PropertyValue{p})
	}
	return enc.collectionEncoder.fromPropertyValue(p)
}
