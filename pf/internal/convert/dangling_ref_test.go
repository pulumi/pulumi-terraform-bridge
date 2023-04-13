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
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/stretchr/testify/require"
	"testing"
)

// AWS has a dangling enum ref, tracked in pulumi/pulumi-aws#2463 - this code needs to be able to handle this.
func TestRegressEncodingConfigWithDanglingEnumRefs(t *testing.T) {
	spec := PrecomputedPackageSpec(&schema.PackageSpec{
		Config: schema.ConfigSpec{
			Variables: map[string]schema.PropertySpec{
				"region": {
					TypeSpec: schema.TypeSpec{
						Type: "string",
						Ref:  "#/types/aws:index/region:Region",
					},
					DefaultInfo: &schema.DefaultSpec{
						Environment: []string{
							"AWS_REGION",
							"AWS_DEFAULT_REGION",
						},
					},
				},
			},
		},
		Types: map[string]schema.ComplexTypeSpec{
			"#/types/aws:index/Region:Region": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type: "string",
				},
				Enum: []schema.EnumValueSpec{
					{Name: "AFSouth1", Value: "af-south-1"},
				},
			},
		},
	})

	enc := NewEncoding(spec, &trivialPropertyNames{})

	ty := tftypes.Object{
		AttributeTypes: map[string]tftypes.Type{
			"region": tftypes.String,
		},
	}

	encoder, err := enc.NewConfigEncoder(ty)
	require.NoError(t, err)

	actual, err := encoder.fromPropertyValue(resource.NewObjectProperty(resource.PropertyMap{
		"region": resource.NewStringProperty("us-west-1"),
	}))
	require.NoError(t, err)

	expect := tftypes.NewValue(ty, map[string]tftypes.Value{
		"region": tftypes.NewValue(tftypes.String, "us-west-1"),
	})

	require.True(t, expect.Equal(actual))
}

type trivialPropertyNames struct{}

var _ PropertyNames = (*trivialPropertyNames)(nil)

func (*trivialPropertyNames) PropertyKey(typeToken tokens.Token, property TerraformPropertyName,
	t tftypes.Type) resource.PropertyKey {
	return resource.PropertyKey(string(typeToken))
}

func (*trivialPropertyNames) ConfigPropertyKey(property TerraformPropertyName, t tftypes.Type) resource.PropertyKey {
	return resource.PropertyKey(property)
}
