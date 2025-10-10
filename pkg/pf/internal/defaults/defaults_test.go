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

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
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
		name                  string
		env                   map[string]string
		computeDefaultOptions info.ComputeDefaultOptions
		props                 resource.PropertyMap
		expected              resource.PropertyMap
		fieldInfos            map[string]*info.Schema
		providerConfig        resource.PropertyMap
	}

	testFrom := func(res *info.PulumiResource) (interface{}, error) {
		n := res.URN.Name() + "-"
		a := []rune("12345")
		unique, err := resource.NewUniqueName(res.Seed, n, 3, 12, a)
		return resource.NewStringProperty(unique), err
	}

	testFromAutoname := func(res *info.PulumiResource) (interface{}, error) {
		return resource.NewStringProperty(res.Autonaming.ProposedName), nil
	}

	testComputeDefaults := func(
		t *testing.T,
		expectPriorValue resource.PropertyValue,
	) func(context.Context, info.ComputeDefaultOptions) (interface{}, error) {
		return func(_ context.Context, opts info.ComputeDefaultOptions) (interface{}, error) {
			require.Equal(t, expectPriorValue, opts.PriorValue)
			n := opts.URN.Name() + "-"
			a := []rune("12345")
			unique, err := resource.NewUniqueName(opts.Seed, n, 3, 12, a)
			return resource.NewStringProperty(unique), err
		}
	}

	testCases := []testCase{
		{
			name: "simple top-level string",
			fieldInfos: map[string]*info.Schema{
				"string_prop": {
					Default: &info.Default{
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
			fieldInfos: map[string]*info.Schema{
				"object_prop": {
					Fields: map[string]*info.Schema{
						"y_prop": {
							Default: &info.Default{
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
			fieldInfos: map[string]*info.Schema{
				"object_prop": {
					Fields: map[string]*info.Schema{
						"y_prop": {
							Default: &info.Default{
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
			fieldInfos: map[string]*info.Schema{
				"string_prop": {
					Default: &info.Default{
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
			fieldInfos: map[string]*info.Schema{
				"int_prop": {
					Default: &info.Default{
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
			fieldInfos: map[string]*info.Schema{
				"float_prop": {
					Default: &info.Default{
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
			fieldInfos: map[string]*info.Schema{
				"bool_prop": {
					Default: &info.Default{
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
			name: "ComputeDefaults function can compute defaults",
			fieldInfos: map[string]*info.Schema{
				"string_prop": {
					Default: &info.Default{
						ComputeDefault: testComputeDefaults(t,
							resource.NewStringProperty("oldString")),
					},
				},
			},
			computeDefaultOptions: info.ComputeDefaultOptions{
				URN:        "urn:pulumi:test::test::pkgA:index:t1::n1",
				Properties: resource.PropertyMap{},
				Seed:       []byte(`123`),
				PriorState: resource.PropertyMap{
					"stringProp": resource.NewStringProperty("oldString"),
				},
			},
			expected: resource.PropertyMap{
				"stringProp": resource.NewStringProperty("n1-453"),
			},
		},
		{
			name: "From function can compute defaults",
			fieldInfos: map[string]*info.Schema{
				"string_prop": {
					Default: &info.Default{
						From: testFrom,
					},
				},
			},
			computeDefaultOptions: info.ComputeDefaultOptions{
				URN:        "urn:pulumi:test::test::pkgA:index:t1::n1",
				Properties: resource.PropertyMap{},
				Seed:       []byte(`123`),
			},
			expected: resource.PropertyMap{
				"stringProp": resource.NewStringProperty("n1-453"),
			},
		},
		{
			name: "From function can compute defaults with autoname",
			fieldInfos: map[string]*info.Schema{
				"string_prop": {
					Default: &info.Default{
						From: testFromAutoname,
					},
				},
			},
			computeDefaultOptions: info.ComputeDefaultOptions{
				URN:        "urn:pulumi:test::test::pkgA:index:t1::n1",
				Properties: resource.PropertyMap{},
				Seed:       []byte(`123`),
				Autonaming: &info.ComputeDefaultAutonamingOptions{
					ProposedName: "n1-777",
					Mode:         info.ComputeDefaultAutonamingModePropose,
				},
			},
			expected: resource.PropertyMap{
				"stringProp": resource.NewStringProperty("n1-777"),
			},
		},
		{
			name: "ComputeDefaults function can compute nested defaults",
			fieldInfos: map[string]*info.Schema{
				"object_prop": {
					Fields: map[string]*info.Schema{
						"y_prop": {
							Default: &info.Default{
								ComputeDefault: testComputeDefaults(t,
									resource.NewStringProperty("oldY")),
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
			computeDefaultOptions: info.ComputeDefaultOptions{
				URN:        "urn:pulumi:test::test::pkgA:index:t1::n1",
				Properties: resource.PropertyMap{},
				Seed:       []byte(`123`),
				PriorState: resource.PropertyMap{
					"objectProp": resource.NewObjectProperty(resource.PropertyMap{
						"xProp": resource.NewStringProperty("oldX"),
						"yProp": resource.NewStringProperty("oldY"),
					}),
				},
			},
			expected: resource.PropertyMap{
				"objectProp": resource.NewObjectProperty(resource.PropertyMap{
					"xProp": resource.NewStringProperty("X"),
					"yProp": resource.NewStringProperty("n1-453"),
				}),
			},
		},
		{
			name: "From function can compute nested defaults",
			fieldInfos: map[string]*info.Schema{
				"object_prop": {
					Fields: map[string]*info.Schema{
						"y_prop": {
							Default: &info.Default{
								From: testFrom,
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
			computeDefaultOptions: info.ComputeDefaultOptions{
				URN:        "urn:pulumi:test::test::pkgA:index:t1::n1",
				Properties: resource.PropertyMap{},
				Seed:       []byte(`123`),
			},
			expected: resource.PropertyMap{
				"objectProp": resource.NewObjectProperty(resource.PropertyMap{
					"xProp": resource.NewStringProperty("X"),
					"yProp": resource.NewStringProperty("n1-453"),
				}),
			},
		},
		{
			name: "Defaults can be copied from provider configuration",
			fieldInfos: map[string]*info.Schema{
				"string_prop": {
					Default: &info.Default{
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
			fieldInfos: map[string]*info.Schema{
				"int_prop": {
					Default: &info.Default{
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
				SchemaMap:             schemaMap,
				SchemaInfos:           tc.fieldInfos,
				ComputeDefaultOptions: tc.computeDefaultOptions,
				PropertyMap:           tc.props,
				ProviderConfig:        tc.providerConfig,
			})
			assert.Equal(t, tc.expected, actual)
		})
	}
}
