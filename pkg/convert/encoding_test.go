// Copyright 2016-2024, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package convert

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/hexops/autogold/v2"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/schema"
)

func TestResourceDecoder(t *testing.T) {
	t.Parallel()
	myResource := "my_resource"

	type testCase struct {
		testName  string
		schema    *schema.SchemaMap
		info      *tfbridge.ProviderInfo
		typ       tftypes.Object
		val       tftypes.Value
		expect    autogold.Value
		expectMap resource.PropertyMap
	}

	makeProvider := func(schemaMap *schema.SchemaMap) *schema.Provider {
		return &schema.Provider{
			ResourcesMap: schema.ResourceMap{
				myResource: (&schema.Resource{
					Schema: schemaMap,
				}).Shim(),
			},
		}
	}

	testCases := []testCase{
		{
			testName: "basic",
			schema: &schema.SchemaMap{
				"id": (&schema.Schema{
					Type: shim.TypeString,
				}).Shim(),
				"foo": (&schema.Schema{
					Type:     shim.TypeString,
					Optional: true,
				}).Shim(),
			},
			info: nil,
			typ: tftypes.Object{
				AttributeTypes: map[string]tftypes.Type{
					"id":  tftypes.String,
					"foo": tftypes.String,
				},
			},
			val: tftypes.NewValue(tftypes.Object{
				AttributeTypes: map[string]tftypes.Type{
					"id":  tftypes.String,
					"foo": tftypes.String,
				},
			}, map[string]tftypes.Value{
				"id":  tftypes.NewValue(tftypes.String, "myid"),
				"foo": tftypes.NewValue(tftypes.String, "bar"),
			}),
			expect: autogold.Expect(resource.PropertyMap{
				resource.PropertyKey("foo"): resource.PropertyValue{
					V: "bar",
				},
				resource.PropertyKey("id"): resource.PropertyValue{V: "myid"},
			}),
		},
	}

	for _, schemaHasID := range []bool{false, true} {
		for _, dataHasID := range []bool{false, true} {
			s := &schema.SchemaMap{
				"id": (&schema.Schema{
					Type: shim.TypeString,
				}).Shim(),
				"foo": (&schema.Schema{
					Type:     shim.TypeString,
					Optional: true,
				}).Shim(),
			}
			ty := tftypes.Object{
				AttributeTypes: map[string]tftypes.Type{
					"id":  tftypes.String,
					"foo": tftypes.String,
				},
			}
			if !schemaHasID {
				s = &schema.SchemaMap{
					"id": (&schema.Schema{
						Type: shim.TypeString,
					}).Shim(),
					"foo": (&schema.Schema{
						Type:     shim.TypeString,
						Optional: true,
					}).Shim(),
				}
				ty = tftypes.Object{
					AttributeTypes: map[string]tftypes.Type{
						"foo": tftypes.String,
					},
				}
			}

			value := tftypes.NewValue(tftypes.Object{
				AttributeTypes: map[string]tftypes.Type{
					"id":  tftypes.String,
					"foo": tftypes.String,
				},
			}, map[string]tftypes.Value{
				"id":  tftypes.NewValue(tftypes.String, "myid"),
				"foo": tftypes.NewValue(tftypes.String, "bar"),
			})
			expect := resource.PropertyMap{
				resource.PropertyKey("foo"): resource.PropertyValue{V: "bar"},
				resource.PropertyKey("id"):  resource.PropertyValue{V: "myid"},
			}
			if !dataHasID {
				value = tftypes.NewValue(tftypes.Object{
					AttributeTypes: map[string]tftypes.Type{
						"foo": tftypes.String,
					},
				}, map[string]tftypes.Value{
					"foo": tftypes.NewValue(tftypes.String, "bar"),
				})
			}
			if !dataHasID || !schemaHasID {
				expect = resource.PropertyMap{
					resource.PropertyKey("foo"): resource.PropertyValue{V: "bar"},
				}
			}

			tc := testCase{
				testName:  fmt.Sprintf("schemaid-%v-dataid-%v", schemaHasID, dataHasID),
				schema:    s,
				typ:       ty,
				val:       value,
				expectMap: expect,
			}
			testCases = append(testCases, tc)
		}
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.testName, func(t *testing.T) {
			enc := NewEncoding(makeProvider(tc.schema).Shim(), tc.info)
			decoder, err := enc.NewResourceDecoder(myResource, tc.typ)
			require.NoError(t, err)
			got, err := DecodePropertyMap(context.Background(), decoder, tc.val)
			require.NoError(t, err)
			if tc.expectMap != nil {
				require.Equal(t, tc.expectMap, got)
			} else {
				tc.expect.Equal(t, got)
			}
		})
	}
}

func TestResourceEncoder(t *testing.T) {
	t.Parallel()
	myResource := "my_resource"

	type testCase struct {
		testName string
		schema   *schema.SchemaMap
		info     *tfbridge.ProviderInfo
		typ      tftypes.Object
		val      resource.PropertyMap
		expect   autogold.Value
	}

	makeProvider := func(schemaMap *schema.SchemaMap) *schema.Provider {
		return &schema.Provider{
			ResourcesMap: schema.ResourceMap{
				myResource: (&schema.Resource{
					Schema: schemaMap,
				}).Shim(),
			},
		}
	}

	testCases := []testCase{
		{
			testName: "basic",
			schema: &schema.SchemaMap{
				"id": (&schema.Schema{
					Type: shim.TypeString,
				}).Shim(),
				"foo": (&schema.Schema{
					Type:     shim.TypeString,
					Optional: true,
				}).Shim(),
			},
			info: nil,
			typ: tftypes.Object{
				AttributeTypes: map[string]tftypes.Type{
					"id":  tftypes.String,
					"foo": tftypes.String,
				},
			},
			val: resource.PropertyMap{
				resource.PropertyKey("foo"): resource.PropertyValue{
					V: "bar",
				},
				resource.PropertyKey("id"): resource.PropertyValue{V: "myid"},
			},
			//nolint:lll
			expect: autogold.Expect(`tftypes.Object["foo":tftypes.String, "id":tftypes.String]<"foo":tftypes.String<"bar">, "id":tftypes.String<"myid">>`),
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.testName, func(t *testing.T) {
			enc := NewEncoding(makeProvider(tc.schema).Shim(), tc.info)
			encoder, err := enc.NewResourceEncoder(myResource, tc.typ)
			require.NoError(t, err)
			got, err := EncodePropertyMap(encoder, tc.val)
			require.NoError(t, err)
			tc.expect.Equal(t, got.String())
		})
	}
}

func TestDataSourceDecoder(t *testing.T) {
	t.Parallel()
	myDataSource := "my_datasource"

	type testCase struct {
		testName string
		schema   *schema.SchemaMap
		info     *tfbridge.ProviderInfo
		typ      tftypes.Object
		val      tftypes.Value
		expect   autogold.Value
	}

	makeProvider := func(schemaMap *schema.SchemaMap) *schema.Provider {
		return &schema.Provider{
			DataSourcesMap: schema.ResourceMap{
				myDataSource: (&schema.Resource{
					Schema: schemaMap,
				}).Shim(),
			},
		}
	}

	testCases := []testCase{
		{
			testName: "basic",
			schema: &schema.SchemaMap{
				"foo": (&schema.Schema{
					Type:     shim.TypeString,
					Optional: true,
				}).Shim(),
			},
			info: nil,
			typ: tftypes.Object{
				AttributeTypes: map[string]tftypes.Type{
					"foo": tftypes.String,
				},
			},
			val: tftypes.NewValue(tftypes.Object{
				AttributeTypes: map[string]tftypes.Type{
					"foo": tftypes.String,
				},
			}, map[string]tftypes.Value{
				"foo": tftypes.NewValue(tftypes.String, "bar"),
			}),
			expect: autogold.Expect(resource.PropertyMap{
				resource.PropertyKey("foo"): resource.PropertyValue{
					V: "bar",
				},
			}),
		},
		{
			testName: "respect name overrides",
			schema: &schema.SchemaMap{
				"foo": (&schema.Schema{
					Type:     shim.TypeString,
					Optional: true,
				}).Shim(),
			},
			info: &tfbridge.ProviderInfo{
				DataSources: map[string]*tfbridge.DataSourceInfo{
					myDataSource: {
						Fields: map[string]*tfbridge.SchemaInfo{
							"foo": {
								Name: "renamedFoo",
							},
						},
					},
				},
			},
			typ: tftypes.Object{
				AttributeTypes: map[string]tftypes.Type{
					"foo": tftypes.String,
				},
			},
			val: tftypes.NewValue(tftypes.Object{
				AttributeTypes: map[string]tftypes.Type{
					"foo": tftypes.String,
				},
			}, map[string]tftypes.Value{
				"foo": tftypes.NewValue(tftypes.String, "bar"),
			}),
			expect: autogold.Expect(resource.PropertyMap{
				resource.PropertyKey("renamedFoo"): resource.PropertyValue{
					V: "bar",
				},
			}),
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.testName, func(t *testing.T) {
			enc := NewEncoding(makeProvider(tc.schema).Shim(), tc.info)
			decoder, err := enc.NewDataSourceDecoder(myDataSource, tc.typ)
			require.NoError(t, err)
			got, err := DecodePropertyMap(context.Background(), decoder, tc.val)
			require.NoError(t, err)
			tc.expect.Equal(t, got)
		})
	}
}

func TestDataSourceEncoder(t *testing.T) {
	t.Parallel()
	myDataSource := "my_datasource"

	type testCase struct {
		testName string
		schema   *schema.SchemaMap
		info     *tfbridge.ProviderInfo
		typ      tftypes.Object
		val      resource.PropertyMap
		expect   autogold.Value
	}

	makeProvider := func(schemaMap *schema.SchemaMap) *schema.Provider {
		return &schema.Provider{
			DataSourcesMap: schema.ResourceMap{
				myDataSource: (&schema.Resource{
					Schema: schemaMap,
				}).Shim(),
			},
		}
	}

	testCases := []testCase{
		{
			testName: "basic",
			schema: &schema.SchemaMap{
				"foo": (&schema.Schema{
					Type:     shim.TypeString,
					Optional: true,
				}).Shim(),
			},
			info: nil,
			typ: tftypes.Object{
				AttributeTypes: map[string]tftypes.Type{
					"foo": tftypes.String,
				},
			},
			val: resource.PropertyMap{
				resource.PropertyKey("foo"): resource.PropertyValue{
					V: "bar",
				},
			},
			expect: autogold.Expect(`tftypes.Object["foo":tftypes.String]<"foo":tftypes.String<"bar">>`),
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.testName, func(t *testing.T) {
			enc := NewEncoding(makeProvider(tc.schema).Shim(), tc.info)
			encoder, err := enc.NewDataSourceEncoder(myDataSource, tc.typ)
			require.NoError(t, err)
			got, err := EncodePropertyMap(encoder, tc.val)
			require.NoError(t, err)
			tc.expect.Equal(t, got.String())
		})
	}
}

func TestConfigEncoder(t *testing.T) {
	t.Parallel()
	type testCase struct {
		testName string
		schema   *schema.SchemaMap
		info     *tfbridge.ProviderInfo
		typ      tftypes.Object
		val      resource.PropertyMap
		expect   autogold.Value
	}

	makeProvider := func(schemaMap *schema.SchemaMap) *schema.Provider {
		return &schema.Provider{
			Schema: schemaMap,
		}
	}

	testCases := []testCase{
		{
			testName: "basic",
			schema: &schema.SchemaMap{
				"foo": (&schema.Schema{
					Type:     shim.TypeString,
					Optional: true,
				}).Shim(),
			},
			info: nil,
			typ: tftypes.Object{
				AttributeTypes: map[string]tftypes.Type{
					"foo": tftypes.String,
				},
			},
			val: resource.PropertyMap{
				resource.PropertyKey("foo"): resource.PropertyValue{
					V: "bar",
				},
			},
			expect: autogold.Expect(`tftypes.Object["foo":tftypes.String]<"foo":tftypes.String<"bar">>`),
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.testName, func(t *testing.T) {
			enc := NewEncoding(makeProvider(tc.schema).Shim(), tc.info)
			encoder, err := enc.NewConfigEncoder(tc.typ)
			require.NoError(t, err)
			got, err := EncodePropertyMap(encoder, tc.val)
			require.NoError(t, err)
			tc.expect.Equal(t, got.String())
		})
	}
}

// Boost coverage of deriveEncoder, deriveDecoder over collections especially.
func TestTypeDerivations(t *testing.T) {
	t.Parallel()
	type testCase struct {
		name      string
		schemaMap schema.SchemaMap
		sample    resource.PropertyMap
		expected  autogold.Value
	}

	intSchema := (&schema.Schema{
		Type: shim.TypeInt,
	}).Shim()

	testCases := []testCase{
		{
			"int list",
			schema.SchemaMap{
				"x": (&schema.Schema{
					Type: shim.TypeList,
					Elem: intSchema,
				}).Shim(),
			},
			resource.PropertyMap{"xes": resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewNumberProperty(1),
			})},
			//nolint:lll
			autogold.Expect(`tftypes.Object["x":tftypes.List[tftypes.Number]]<"x":tftypes.List[tftypes.Number]<tftypes.Number<"1">>>`),
		},
		{
			"int map",
			schema.SchemaMap{
				"x": (&schema.Schema{
					Type: shim.TypeMap,
					Elem: intSchema,
				}).Shim(),
			},
			resource.PropertyMap{"x": resource.NewObjectProperty(resource.PropertyMap{
				"one": resource.NewNumberProperty(1),
				"two": resource.NewNumberProperty(2),
			})},
			//nolint:lll
			autogold.Expect(`tftypes.Object["x":tftypes.Map[tftypes.Number]]<"x":tftypes.Map[tftypes.Number]<"one":tftypes.Number<"1">, "two":tftypes.Number<"2">>>`),
		},
		{
			"int set",
			schema.SchemaMap{
				"x": (&schema.Schema{
					Type: shim.TypeSet,
					Elem: intSchema,
				}).Shim(),
			},
			resource.PropertyMap{"xes": resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewNumberProperty(1),
				resource.NewNumberProperty(2),
			})},
			//nolint:lll
			autogold.Expect(`tftypes.Object["x":tftypes.Set[tftypes.Number]]<"x":tftypes.Set[tftypes.Number]<tftypes.Number<"1">, tftypes.Number<"2">>>`),
		},
		{
			"object",
			schema.SchemaMap{
				"x": (&schema.Schema{
					Type: shim.TypeMap,
					Elem: (&schema.Resource{Schema: schema.SchemaMap{
						"prop": intSchema,
					}}).Shim(),
				}).Shim(),
			},
			resource.PropertyMap{"x": resource.NewObjectProperty(resource.PropertyMap{"prop": resource.NewNumberProperty(1)})},
			//nolint:lll
			autogold.Expect(`tftypes.Object["x":tftypes.Object["prop":tftypes.Number]]<"x":tftypes.Object["prop":tftypes.Number]<"prop":tftypes.Number<"1">>>`),
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			os := ObjectSchema{SchemaMap: tc.schemaMap}

			enc, err := NewObjectEncoder(os)
			require.NoError(t, err)

			dec, err := NewObjectDecoder(os)
			require.NoError(t, err)

			tfv, err := EncodePropertyMap(enc, tc.sample)
			require.NoError(t, err)

			tc.expected.Equal(t, tfv.String())

			back, err := DecodePropertyMap(context.Background(), dec, tfv)
			require.NoError(t, err)
			require.Equal(t, tc.sample, back)
		})
	}
}

// Tuple types need coverage as well.
func TestTupleDerivations(t *testing.T) {
	t.Parallel()
	type testCase struct {
		name      string
		schemaMap schema.SchemaMap
		sample    resource.PropertyMap
		typ       tftypes.Type
		expected  autogold.Value
	}

	intType := (&schema.Schema{
		Type: shim.TypeInt,
	}).Shim()

	stringType := (&schema.Schema{
		Type: shim.TypeString,
	}).Shim()

	tupleType := (&schema.Schema{
		Type: shim.TypeMap,
		Elem: (&schema.Resource{
			Schema: schema.SchemaMap{
				"t0": intType,
				"t1": stringType,
			},
		}).Shim(),
	}).Shim()

	testCases := []testCase{
		{
			"simple-tuple",
			schema.SchemaMap{
				"x": tupleType,
			},
			resource.PropertyMap{"x": resource.NewObjectProperty(
				resource.PropertyMap{
					"t0": resource.NewNumberProperty(1),
					"t1": resource.NewStringProperty("OK"),
				},
			)},
			tftypes.Tuple{ElementTypes: []tftypes.Type{
				tftypes.Number,
				tftypes.String,
			}},
			//nolint:lll
			autogold.Expect(`tftypes.Object["x":tftypes.Tuple[tftypes.Number, tftypes.String]]<"x":tftypes.Tuple[tftypes.Number, tftypes.String]<tftypes.Number<"1">, tftypes.String<"OK">>>`),
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			os := ObjectSchema{
				SchemaMap: tc.schemaMap,
				Object: &tftypes.Object{AttributeTypes: map[string]tftypes.Type{
					"x": tc.typ,
				}},
			}

			enc, err := NewObjectEncoder(os)
			require.NoError(t, err)

			dec, err := NewObjectDecoder(os)
			require.NoError(t, err)

			tfv, err := EncodePropertyMap(enc, tc.sample)
			require.NoError(t, err)

			tc.expected.Equal(t, tfv.String())

			back, err := DecodePropertyMap(context.Background(), dec, tfv)
			require.NoError(t, err)
			require.Equal(t, tc.sample, back)
		})
	}
}

func TestAdapter(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    resource.PropertyValue
		expected tftypes.Value
		error    bool
	}{
		{
			name:     "valid",
			input:    resource.NewProperty("123"),
			expected: tftypesNewValue(tftypes.Number, 123),
		},
		{
			name:  "invalid",
			input: resource.NewProperty("abc"),
			error: true,
		},
		{
			name:     "computed-output",
			input:    resource.NewOutputProperty(resource.Output{}),
			expected: tftypes.NewValue(tftypes.Number, tftypes.UnknownValue),
		},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			t.Run("encoder", func(t *testing.T) {
				v, err := newIntOverrideStringEncoder().fromPropertyValue(tt.input)
				if !tt.error {
					assert.NoError(t, err)
					assert.True(t, v.Equal(tt.expected))
				} else {
					assert.Error(t, err)
				}
			})
			t.Run("decoder", func(t *testing.T) {
				if tt.error {
					t.Logf("skipping since the encoder should error")
					return
				}
				v, err := decode(newStringOverIntDecoder(), tt.expected, DecodeOptions{})
				assert.NoError(t, err)
				if !assert.True(t, v.DeepEquals(tt.input)) {
					assert.Equal(t, v, tt.input)
				}
			})
		})
	}
}
