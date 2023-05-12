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

package defaults

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/schema"
)

func TestApplyDefaultInfoValues(t *testing.T) {

	var schemaMap shim.SchemaMap = schema.SchemaMap{
		"string_prop": (&schema.Schema{Type: shim.TypeString, Optional: true}).Shim(),

		"int_prop": (&schema.Schema{Type: shim.TypeInt, Optional: true}).Shim(),

		"float_prop": (&schema.Schema{Type: shim.TypeFloat, Optional: true}).Shim(),

		"bool_prop": (&schema.Schema{Type: shim.TypeBool, Optional: true}).Shim(),

		"object_prop": (&schema.Schema{
			Type:     shim.TypeMap,
			Optional: true,
			Elem: (&schema.Resource{
				Schema: schema.SchemaMap{
					"x_prop": (&schema.Schema{Type: shim.TypeString, Optional: true}).Shim(),
					"y_prop": (&schema.Schema{Type: shim.TypeString, Optional: true}).Shim(),
				},
			}).Shim(),
		}).Shim(),
	}

	type testCase struct {
		name             string
		env              map[string]string
		resourceInstance *tfbridge.PulumiResource
		props            resource.PropertyMap
		expected         resource.PropertyMap
		fieldInfos       map[string]*tfbridge.SchemaInfo
		providerConfig   resource.PropertyMap
	}

	testCases := []testCase{
		{
			name: "simple top-level string",
			fieldInfos: map[string]*tfbridge.SchemaInfo{
				"string_prop": {
					Default: &tfbridge.DefaultInfo{
						Value: "defaultValue",
					},
				},
			},
			expected: resource.PropertyMap{
				"stringProp": resource.NewStringProperty("defaultValue"),
			},
		},
		{
			name: "nested string",
			fieldInfos: map[string]*tfbridge.SchemaInfo{
				"object_prop": {
					Fields: map[string]*tfbridge.SchemaInfo{
						"y_prop": {
							Default: &tfbridge.DefaultInfo{
								Value: "Y",
							},
						},
					},
				},
			},
			props: resource.PropertyMap{
				"objectProp": resource.NewObjectProperty(resource.PropertyMap{
					"xProp": resource.NewStringProperty("X"),
				}),
			},
			expected: resource.PropertyMap{
				"objectProp": resource.NewObjectProperty(resource.PropertyMap{
					"xProp": resource.NewStringProperty("X"),
					"yProp": resource.NewStringProperty("Y"),
				}),
			},
		},
		{
			name: "nested string does not create object",
			fieldInfos: map[string]*tfbridge.SchemaInfo{
				"object_prop": {
					Fields: map[string]*tfbridge.SchemaInfo{
						"y_prop": {
							Default: &tfbridge.DefaultInfo{
								Value: "Y",
							},
						},
					},
				},
			},
			props:    resource.PropertyMap{},
			expected: resource.PropertyMap{},
		},
		{
			name: "string prop can be set from environment",
			fieldInfos: map[string]*tfbridge.SchemaInfo{
				"string_prop": {
					Default: &tfbridge.DefaultInfo{
						EnvVars: []string{"FOO", "BAR"},
					},
				},
			},
			env: map[string]string{
				"FOO": "S",
			},
			expected: resource.PropertyMap{
				"stringProp": resource.NewStringProperty("S"),
			},
		},
		{
			name: "int prop can be set from environment",
			fieldInfos: map[string]*tfbridge.SchemaInfo{
				"int_prop": {
					Default: &tfbridge.DefaultInfo{
						EnvVars: []string{"FOO", "BAR"},
					},
				},
			},
			env: map[string]string{
				"FOO": "42",
			},
			expected: resource.PropertyMap{
				"intProp": resource.NewNumberProperty(42),
			},
		},
		{
			name: "float prop can be set from environment",
			fieldInfos: map[string]*tfbridge.SchemaInfo{
				"float_prop": {
					Default: &tfbridge.DefaultInfo{
						EnvVars: []string{"FOO", "BAR"},
					},
				},
			},
			env: map[string]string{
				"FOO": "42.25",
			},
			expected: resource.PropertyMap{
				"floatProp": resource.NewNumberProperty(42.25),
			},
		},
		{
			name: "bool prop can be set from environment",
			fieldInfos: map[string]*tfbridge.SchemaInfo{
				"bool_prop": {
					Default: &tfbridge.DefaultInfo{
						EnvVars: []string{"FOO", "BAR"},
					},
				},
			},
			env: map[string]string{
				"FOO": "true",
			},
			expected: resource.PropertyMap{
				"boolProp": resource.NewBoolProperty(true),
			},
		},
		{
			name: "From function can compute defaults",
			fieldInfos: map[string]*tfbridge.SchemaInfo{
				"string_prop": {
					Default: &tfbridge.DefaultInfo{
						From: func(
							res *tfbridge.PulumiResource,
							_ ...tfbridge.DefaultContextOption,
						) (interface{}, error) {
							return resource.NewStringProperty("OK"), nil
						},
					},
				},
			},
			resourceInstance: &tfbridge.PulumiResource{},
			expected: resource.PropertyMap{
				"stringProp": resource.NewStringProperty("OK"),
			},
		},
		{
			name: "Defaults can be copied from provider configuration",
			fieldInfos: map[string]*tfbridge.SchemaInfo{
				"string_prop": {
					Default: &tfbridge.DefaultInfo{
						Config: "providerStringProp",
					},
				},
			},
			providerConfig: resource.PropertyMap{
				"providerStringProp": resource.NewStringProperty("OK"),
			},
			expected: resource.PropertyMap{
				"stringProp": resource.NewStringProperty("OK"),
			},
		},
		{
			name: "Empty env var is not a numeric zero",
			fieldInfos: map[string]*tfbridge.SchemaInfo{
				"int_prop": {
					Default: &tfbridge.DefaultInfo{
						EnvVars: []string{"INT_PROP"},
					},
				},
			},
			env: map[string]string{
				"INT_PROP": "",
			},
			expected: resource.PropertyMap{},
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			ctx := context.Background()
			actual := ApplyDefaultInfoValues(ctx, ApplyDefaultInfoValuesArgs{
				SchemaMap:        schemaMap,
				SchemaInfos:      tc.fieldInfos,
				ResourceInstance: tc.resourceInstance,
				PropertyMap:      tc.props,
				ProviderConfig:   tc.providerConfig,
			})
			assert.Equal(t, tc.expected, actual)
		})
	}

}
