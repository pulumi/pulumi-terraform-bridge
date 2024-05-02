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
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/hexops/autogold/v2"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/schema"
)

func TestSecretDecoderInjectsSchemaSecrets(t *testing.T) {
	type testCase struct {
		name        string
		schemaMap   schema.SchemaMap
		schemaInfos map[string]*tfbridge.SchemaInfo
		val         tftypes.Value
		expect      autogold.Value
	}
	testCases := []testCase{
		{
			name: "secret-schema-map",
			schemaMap: schema.SchemaMap{
				"secret_value": (&schema.Schema{
					Type:      shim.TypeString,
					Sensitive: true,
				}).Shim(),
			},
			val: tftypes.NewValue(tftypes.Object{
				AttributeTypes: map[string]tftypes.Type{
					"secret_value": tftypes.String,
				},
			}, map[string]tftypes.Value{
				"secret_value": tftypes.NewValue(tftypes.String, "secret"),
			}),
			expect: autogold.Expect(resource.PropertyMap{resource.PropertyKey("secretValue"): resource.PropertyValue{
				V: &resource.Secret{Element: resource.PropertyValue{
					V: "secret",
				}},
			}}),
		},
		{
			name: "secret-schema-info",
			schemaMap: schema.SchemaMap{
				"secret_value": (&schema.Schema{
					Type: shim.TypeString,
				}).Shim(),
			},
			schemaInfos: map[string]*tfbridge.SchemaInfo{
				"secret_value": {
					Type:   "string",
					Secret: tfbridge.True(),
				},
			},
			val: tftypes.NewValue(tftypes.Object{
				AttributeTypes: map[string]tftypes.Type{
					"secret_value": tftypes.String,
				},
			}, map[string]tftypes.Value{
				"secret_value": tftypes.NewValue(tftypes.String, "secret"),
			}),
			expect: autogold.Expect(resource.PropertyMap{resource.PropertyKey("secretValue"): resource.PropertyValue{
				V: &resource.Secret{Element: resource.PropertyValue{
					V: "secret",
				}},
			}}),
		},
		{
			name: "unsecret-schema-info",
			schemaMap: schema.SchemaMap{
				"secret_value": (&schema.Schema{
					Type:      shim.TypeString,
					Sensitive: true,
				}).Shim(),
			},
			schemaInfos: map[string]*tfbridge.SchemaInfo{
				"secret_value": {
					Type:   "string",
					Secret: tfbridge.False(),
				},
			},
			val: tftypes.NewValue(tftypes.Object{
				AttributeTypes: map[string]tftypes.Type{
					"secret_value": tftypes.String,
				},
			}, map[string]tftypes.Value{
				"secret_value": tftypes.NewValue(tftypes.String, "secret"),
			}),
			expect: autogold.Expect(resource.PropertyMap{resource.PropertyKey("secretValue"): resource.PropertyValue{
				V: "secret",
			}}),
		},
		{
			name: "secret-unknown",
			schemaMap: schema.SchemaMap{
				"secret_value": (&schema.Schema{
					Type:      shim.TypeString,
					Sensitive: true,
				}).Shim(),
			},
			val: tftypes.NewValue(tftypes.Object{
				AttributeTypes: map[string]tftypes.Type{
					"secret_value": tftypes.String,
				},
			}, map[string]tftypes.Value{
				"secret_value": tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
			}),
			expect: autogold.Expect(resource.PropertyMap{resource.PropertyKey("secretValue"): resource.PropertyValue{
				V: resource.Output{},
			}}),
		},
	}
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			d, err := NewObjectDecoder(ObjectSchema{
				SchemaMap:   tc.schemaMap,
				SchemaInfos: tc.schemaInfos,
			})
			require.NoError(t, err)
			pm, err := DecodePropertyMap(context.Background(), d, tc.val)
			require.NoError(t, err)
			tc.expect.Equal(t, pm)
		})
	}
}

// Due to https://github.com/pulumi/pulumi/issues/11971 provider may receive secret values that are not explicitly
// wrapped, and should tolerate it.
func TestRelaxedSecretHandling(t *testing.T) {
	ty := tftypes.Object{
		AttributeTypes: map[string]tftypes.Type{
			"x": tftypes.String,
		},
	}

	encoder, err := newObjectEncoder(ty, map[terraformPropertyName]Encoder{
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

func (*trivialLocalPropertyNames) PropertyKey(property terraformPropertyName, t tftypes.Type) resource.PropertyKey {
	return resource.PropertyKey(property)
}

// Like PropertyNames but specialized to either a type by token or config property.
var _ localPropertyNames = &trivialLocalPropertyNames{}
