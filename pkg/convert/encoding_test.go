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
	"fmt"
	"testing"

	"github.com/hexops/autogold/v2"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/schema"
)

func TestResourceDecoder(t *testing.T) {
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
			got, err := DecodePropertyMap(decoder, tc.val)
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
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.testName, func(t *testing.T) {
			enc := NewEncoding(makeProvider(tc.schema).Shim(), tc.info)
			decoder, err := enc.NewDataSourceDecoder(myDataSource, tc.typ)
			require.NoError(t, err)
			got, err := DecodePropertyMap(decoder, tc.val)
			require.NoError(t, err)
			tc.expect.Equal(t, got)
		})
	}
}

func TestDataSourceEncoder(t *testing.T) {
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
