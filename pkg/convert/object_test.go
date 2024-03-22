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
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

// When PropertyMap is missing an entry, do not assume that the result is null but instead call the
// matching encoder. Some encoders like flattened would prefer to return empty lists.
func TestObjectEncoderRecursesWhenMissing(t *testing.T) {
	pn := &trivialLocalPropertyNames{}
	innerType := tftypes.Object{
		AttributeTypes: map[string]tftypes.Type{
			"x": tftypes.String,
		},
	}
	innerEncoders := map[string]Encoder{"x": &stringEncoder{}}
	innerEnc, err := newObjectEncoder(innerType, innerEncoders, pn)
	require.NoError(t, err)

	collectionType := tftypes.List{ElementType: innerType}

	ty := tftypes.Object{AttributeTypes: map[string]tftypes.Type{"prop": collectionType}}

	encoders := map[string]Encoder{"prop": &flattenedEncoder{
		collectionType: collectionType,
		elementEncoder: innerEnc,
	}}

	encoder, err := newObjectEncoder(ty, encoders, &trivialLocalPropertyNames{})
	require.NoError(t, err)

	v, err := EncodePropertyMap(encoder, resource.PropertyMap{})
	require.NoError(t, err)

	expected := tftypes.NewValue(ty, map[string]tftypes.Value{
		"prop": tftypes.NewValue(collectionType, []tftypes.Value{}),
	})

	require.Truef(t, expected.Equal(v), "exp: %s\n\ngot: %s\n\n", expected.String(), v.String())
}
