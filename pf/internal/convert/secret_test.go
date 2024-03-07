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
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

// Due to https://github.com/pulumi/pulumi/issues/11971 provider may receive secret values that are not explicitly
// wrapped, and should tolerate it.
func TestRelaxedSecretHandling(t *testing.T) {
	ty := tftypes.Object{
		AttributeTypes: map[string]tftypes.Type{
			"x": tftypes.String,
		},
	}

	encoder, err := newObjectEncoder(ty, map[TerraformPropertyName]Encoder{
		"x": newStringEncoder(),
	}, &trivialLocalPropertyNames{})
	require.NoError(t, err)

	v, err := EncodePropertyMap(encoder, resource.PropertyMap{"x": resource.NewStringProperty("OK")})
	require.NoError(t, err)

	expect := tftypes.NewValue(ty, map[string]tftypes.Value{
		"x": tftypes.NewValue(tftypes.String, "OK"),
	})

	require.Equal(t, expect, v)
}

type trivialLocalPropertyNames struct{}

func (*trivialLocalPropertyNames) PropertyKey(property TerraformPropertyName, t tftypes.Type) resource.PropertyKey {
	return resource.PropertyKey(property)
}

// Like PropertyNames but specialized to either a type by token or config property.
var _ localPropertyNames = &trivialLocalPropertyNames{}
