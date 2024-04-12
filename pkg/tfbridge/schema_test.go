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

// See the License for the specific language governing permissions and
// limitations under the License.

package tfbridge

import (
	"context"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	structpb "github.com/golang/protobuf/ptypes/struct"
	schemav1 "github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	schemav2 "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hexops/autogold/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"

	"github.com/pulumi/pulumi-terraform-bridge/v3/internal/testprovider"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/schema"
	shimv1 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v1"

	shimv2 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v2"
)

func makeTerraformInputsNoDefaults(olds, news resource.PropertyMap,
	tfs shim.SchemaMap, ps map[string]*SchemaInfo,
) (map[string]interface{}, AssetTable, error) {
	return makeTerraformInputsWithOptions(context.Background(), nil, nil, olds, news, tfs, ps,
		makeTerraformInputsOptions{DisableDefaults: true, DisableTFDefaults: true})
}

func makeTerraformInputsForConfig(olds, news resource.PropertyMap,
	tfs shim.SchemaMap, ps map[string]*SchemaInfo,
) (map[string]interface{}, AssetTable, error) {
	return makeTerraformInputsWithOptions(context.Background(), nil, nil, olds, news, tfs, ps,
		makeTerraformInputsOptions{})
}

func makeTerraformInputsForCreate(olds, news resource.PropertyMap,
	tfs shim.SchemaMap, ps map[string]*SchemaInfo,
) (map[string]interface{}, AssetTable, error) {
	return makeTerraformInputsWithOptions(context.Background(), nil, nil, olds, news, tfs, ps,
		makeTerraformInputsOptions{DisableDefaults: true, DisableTFDefaults: true, EnableMaxItemsOneDefaults: true})
}

func makeTerraformInput(v resource.PropertyValue, tfs shim.Schema, ps *SchemaInfo) (interface{}, error) {
	ctx := &conversionContext{}
	return ctx.makeTerraformInput("v", resource.PropertyValue{}, v, tfs, ps, false)
}

func TestMakeTerraformInputMixedMaxItemsOne(t *testing.T) {
	t.Parallel()

	typeString := (&schema.Schema{
		Type: shim.TypeString,
	}).Shim()

	tests := map[string]struct {
		maxItemsOne bool
		oldState    resource.PropertyValue
		newState    resource.PropertyValue
		tfs         *schema.Schema
		tfValue     interface{}
	}{
		// Scalars: The pulumi type is String.
		// The TF type is [String] (either [n; T] or [1; T]).
		"scalar-adding-max-items-one": {
			// The TF type has changed from [n; T] to [1; T], changing the
			// pulumi type from [T] -> T.
			maxItemsOne: true,
			oldState: resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewStringProperty("sc"),
			}),
			newState: resource.NewStringProperty("sc"),
			tfs: &schema.Schema{
				Type:     shim.TypeList,
				Elem:     typeString,
				MaxItems: 1,
			},
			tfValue: []interface{}{"sc"},
		},
		"scalar-removing-max-items-one": {
			// The TF type has changed from [1; T] to [n; T], changing the
			// pulumi type from T -> [T].
			maxItemsOne: false,
			oldState:    resource.NewStringProperty("sc"),
			newState: resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewStringProperty("sc"),
			}),
			tfs: &schema.Schema{
				Type: shim.TypeList,
				Elem: typeString,
			},
			tfValue: []interface{}{"sc"},
		},

		// Scalars: The pulumi type is String.
		// The TF type is [String] (either [n; T] or [1; T]).
		//
		// Here we have empty values, which are handled differently.
		"scalar-adding-null-max-items-one": {
			// The TF type has changed from [n; T] to [1; T], changing the
			// pulumi type from [T] -> T.
			maxItemsOne: true,
			oldState:    resource.NewNullProperty(),
			newState:    resource.NewNullProperty(),
			tfs: &schema.Schema{
				Type:     shim.TypeList,
				Elem:     typeString,
				MaxItems: 1,
			},
			tfValue: []interface{}(nil),
		},
		"scalar-removing-null-max-items-one": {
			// The TF type has changed from [1; T] to [n; T], changing the
			// pulumi type from T -> [T].
			maxItemsOne: false,
			oldState:    resource.NewArrayProperty([]resource.PropertyValue{}),
			newState:    resource.NewArrayProperty([]resource.PropertyValue{}),
			tfs: &schema.Schema{
				Type:     shim.TypeList,
				Elem:     typeString,
				MaxItems: 1,
			},
			tfValue: []interface{}(nil),
		},
		// // Lists: The pulumi type is [String].
		// // The TF type is [[String]] (either [m; [n; T]] or [1; [n; T]]).
		// //
		// // This is different because we can't know the type of an empty list. It
		// // could be of type [T] or [[T]]. In this case, we don't make an attempt
		// // at promotion.
		"list-adding-max-items-one": {
			// The TF type has changed from [m; [n; T]] to [1; [n; T]], changing the
			// pulumi type from [[T]] -> [T].
			maxItemsOne: true,
			oldState: resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewArrayProperty([]resource.PropertyValue{
					resource.NewStringProperty("sc"),
				}),
			}),
			newState: resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewStringProperty("sc"),
			}),
			tfs: &schema.Schema{
				Type:     shim.TypeList,
				MaxItems: 1,
				Elem: (&schema.Schema{
					Type: shim.TypeList,
					Elem: typeString,
				}).Shim(),
			},
			tfValue: []interface{}{[]interface{}{"sc"}},
		},
	}
	for name, tt := range tests {
		tt := tt
		t.Run(name, func(t *testing.T) {
			olds := resource.PropertyMap{
				"element": tt.oldState,
				"__defaults": resource.NewArrayProperty(
					[]resource.PropertyValue{
						resource.NewStringProperty("other"),
					},
				),
			}
			news := resource.PropertyMap{
				"element": tt.newState,
				"__defaults": resource.NewArrayProperty(
					[]resource.PropertyValue{
						resource.NewStringProperty("other"),
					},
				),
			}
			tfs := schema.SchemaMap{"element": tt.tfs.Shim()}
			result, _, err := makeTerraformInputsNoDefaults(
				olds, news, tfs, nil /* ps */)
			require.NoError(t, err)
			assert.Equal(t, map[string]interface{}{
				"element": tt.tfValue,
			}, result)
		})
	}
}

// Test that makeTerraformInputs variants work well with MaxItems=1 properties.
// missing MaxItems=1 properties should present to TF as missing in most cases (MakeTerraformInputs)
// missing MaxItems=1 properties should present to TF as missing
// when running validators (makeTerraformInputsWithoutTFDefaults)
// missing MaxItems=1 properties should present to TF as empty collections when creating
// a resource (makeTerraformInputsNoDefaultsWithMaxItemsOneDefaults)
func TestMakeTerraformInputsWithMaxItemsOne(t *testing.T) {
	typeString := (&schema.Schema{
		Type: shim.TypeString,
	}).Shim()

	resSchema := &schema.Schema{
		Type:     shim.TypeList,
		MaxItems: 1,
		Elem: (&schema.Schema{
			Type:    shim.TypeList,
			Elem:    typeString,
			Default: []string{"default"},
		}).Shim(),
	}
	tfs := schema.SchemaMap{"element": resSchema.Shim()}

	tests := map[string]struct {
		olds               resource.PropertyMap
		news               resource.PropertyMap
		expectedNoDefaults map[string]interface{}
		expectedForConfig  map[string]interface{}
		expectedForCreate  map[string]interface{}
	}{
		"empty-olds": {
			olds: resource.PropertyMap{},
			news: resource.PropertyMap{
				"__defaults": resource.NewArrayProperty(
					[]resource.PropertyValue{
						resource.NewStringProperty("other"),
					},
				),
			},
			expectedNoDefaults: map[string]interface{}{},
			expectedForConfig: map[string]interface{}{
				"__defaults": []interface{}{},
			},
			expectedForCreate: map[string]interface{}{
				"element": []interface{}{},
			},
		},
		"non-empty-olds": {
			olds: resource.PropertyMap{
				"element": resource.NewStringProperty("el"),
				"__defaults": resource.NewArrayProperty(
					[]resource.PropertyValue{
						resource.NewStringProperty("other"),
					},
				),
			},
			news: resource.PropertyMap{
				"__defaults": resource.NewArrayProperty(
					[]resource.PropertyValue{
						resource.NewStringProperty("other"),
					},
				),
			},
			expectedNoDefaults: map[string]interface{}{},
			expectedForConfig: map[string]interface{}{
				"__defaults": []interface{}{},
			},
			expectedForCreate: map[string]interface{}{
				"element": []interface{}{},
			},
		},
		"non-missing-news": {
			olds: resource.PropertyMap{
				"__defaults": resource.NewArrayProperty(
					[]resource.PropertyValue{
						resource.NewStringProperty("other"),
					},
				),
			},
			news: resource.PropertyMap{
				"element": resource.NewStringProperty("el"),
				"__defaults": resource.NewArrayProperty(
					[]resource.PropertyValue{
						resource.NewStringProperty("other"),
					},
				),
			},
			expectedNoDefaults: map[string]interface{}{
				"element": []interface{}{"el"},
			},
			expectedForConfig: map[string]interface{}{
				"__defaults": []interface{}{},
				"element":    []interface{}{"el"},
			},
			expectedForCreate: map[string]interface{}{
				"element": []interface{}{"el"},
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			resultNoDefaults, _, err := makeTerraformInputsNoDefaults(
				tt.olds, tt.news, tfs, nil /* ps */)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedNoDefaults, resultNoDefaults)

			resultForConfig, _, err := makeTerraformInputsForConfig(
				tt.olds, tt.news, tfs, nil /* ps */)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedForConfig, resultForConfig)

			resultForCreate, _, err := makeTerraformInputsForCreate(
				tt.olds, tt.news, tfs, nil /* ps */)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedForCreate, resultForCreate)
		})
	}
}

type MyString string

// TestTerraformOutputsWithSecretsSupported verifies that we translate Terraform outputs into Pulumi outputs and
// treating sensitive outputs as secrets
func TestTerraformOutputsWithSecretsSupported(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		tfValue    any
		tfType     *schema.Schema
		schemaInfo *SchemaInfo
		expect     autogold.Value
	}{
		{
			name:    "nil_property_value",
			tfValue: nil,
			expect:  autogold.Expect(resource.PropertyMap{resource.PropertyKey("nilPropertyValue"): resource.PropertyValue{}}),
		},
		{
			name:    "bool_property_value",
			tfValue: false,
			expect: autogold.Expect(resource.PropertyMap{resource.PropertyKey("boolPropertyValue"): resource.PropertyValue{
				V: false,
			}}),
		},
		{
			name:    "number_property_value",
			tfValue: 42,
			expect: autogold.Expect(resource.PropertyMap{resource.PropertyKey("numberPropertyValue"): resource.PropertyValue{
				V: 42,
			}}),
		},
		{
			name:    "float_property_value",
			tfValue: 99.6767932,
			tfType:  &schema.Schema{Type: shim.TypeFloat, Required: true},
			expect: autogold.Expect(resource.PropertyMap{resource.PropertyKey("floatPropertyValue"): resource.PropertyValue{
				V: 99.6767932,
			}}),
		},
		{
			name:    "string_property_value",
			tfValue: "ognirts",
			schemaInfo: &SchemaInfo{
				// Reverse map string_property_value to the stringo property.
				Name: "stringo",
			},
			expect: autogold.Expect(resource.PropertyMap{resource.PropertyKey("stringo"): resource.PropertyValue{
				V: "ognirts",
			}}),
		},
		{
			name:    "my_string_property_value",
			tfValue: MyString("ognirts"),
			tfType:  &schema.Schema{Type: shim.TypeString, Optional: true},
			expect: autogold.Expect(resource.PropertyMap{resource.PropertyKey("myStringPropertyValue"): resource.PropertyValue{
				V: "ognirts",
			}}),
		},
		{
			name:    "array_property_value",
			tfValue: []interface{}{"an array"},
			expect: autogold.Expect(resource.PropertyMap{resource.PropertyKey("arrayPropertyValue"): resource.PropertyValue{
				V: []resource.PropertyValue{{
					V: "an array",
				}},
			}}),
		},
		{
			name: "object_property_value",
			tfValue: map[string]interface{}{
				"property_a": "a",
				"property_b": true,
			},
			expect: autogold.Expect(resource.PropertyMap{resource.PropertyKey("objectPropertyValue"): resource.PropertyValue{
				V: resource.PropertyMap{
					resource.PropertyKey("propertyA"): resource.PropertyValue{
						V: "a",
					},
					resource.PropertyKey("propertyB"): resource.PropertyValue{V: true},
				},
			}}),
		},
		{
			name: "map_property_value",
			tfValue: map[string]interface{}{
				"propertyA": "a",
				"propertyB": true,
				"propertyC": map[string]interface{}{
					"nestedPropertyA": true,
				},
			},
			tfType: &schema.Schema{
				// Type mapPropertyValue as a map so that keys aren't mangled in the usual way.
				Type:     shim.TypeMap,
				Optional: true,
			},
			//nolint:lll
			expect: autogold.Expect(resource.PropertyMap{resource.PropertyKey("mapPropertyValue"): resource.PropertyValue{
				V: resource.PropertyMap{
					resource.PropertyKey("propertyA"): resource.PropertyValue{
						V: "a",
					},
					resource.PropertyKey("propertyB"): resource.PropertyValue{V: true},
					resource.PropertyKey("propertyC"): resource.PropertyValue{V: resource.PropertyMap{resource.PropertyKey("nestedPropertyA"): resource.PropertyValue{
						V: true,
					}}},
				},
			}}),
		},
		{
			name: "nested_resource",
			tfValue: []interface{}{
				map[string]interface{}{
					"configuration": map[string]interface{}{
						"configurationValue": true,
					},
				},
			},
			tfType: &schema.Schema{
				Type:     shim.TypeList,
				MaxItems: 2,
				// Embed a `*schema.Resource` to validate that type directed
				// walk of the schema successfully walks inside Resources as well
				// as Schemas.
				Optional: true,
				Elem: (&schema.Resource{
					Schema: schemaMap(map[string]*schema.Schema{
						"configuration": {Type: shim.TypeMap, Optional: true},
					}),
				}).Shim(),
			},
			expect: autogold.Expect(resource.PropertyMap{resource.PropertyKey("nestedResources"): resource.PropertyValue{
				V: []resource.PropertyValue{{
					V: resource.PropertyMap{resource.PropertyKey("configuration"): resource.PropertyValue{
						V: resource.PropertyMap{resource.PropertyKey("configurationValue"): resource.PropertyValue{
							V: true,
						}},
					}},
				}},
			}}),
		},
		{
			name: "optional_config",
			tfValue: []interface{}{
				map[string]interface{}{
					"some_value":       true,
					"some_other_value": "a value",
				},
			},
			tfType: &schema.Schema{
				Type:     shim.TypeList,
				MaxItems: 1,
				Optional: true,
				Elem: (&schema.Resource{
					Schema: schemaMap(map[string]*schema.Schema{
						"some_value":       {Type: shim.TypeBool, Optional: true},
						"some_other_value": {Type: shim.TypeString, Optional: true},
					}),
				}).Shim(),
			},
			expect: autogold.Expect(resource.PropertyMap{resource.PropertyKey("optionalConfig"): resource.PropertyValue{
				V: resource.PropertyMap{
					resource.PropertyKey("someOtherValue"): resource.PropertyValue{
						V: "a value",
					},
					resource.PropertyKey("someValue"): resource.PropertyValue{V: true},
				},
			}}),
		},
		{
			name: "optional_config_other",
			tfValue: []interface{}{
				map[string]interface{}{
					"some_value":       true,
					"some_other_value": "a value",
				},
			},
			tfType: &schema.Schema{
				Type:     shim.TypeList,
				Required: true,
				Elem: (&schema.Resource{
					Schema: schemaMap(map[string]*schema.Schema{
						"some_value":       {Type: shim.TypeBool, Optional: true},
						"some_other_value": {Type: shim.TypeString, Optional: true},
					}),
				}).Shim(),
			},
			schemaInfo: &SchemaInfo{
				Name:        "optionalConfigOther",
				MaxItemsOne: boolPointer(true),
			},
			expect: autogold.Expect(resource.PropertyMap{resource.PropertyKey("optionalConfigOther"): resource.PropertyValue{
				V: resource.PropertyMap{
					resource.PropertyKey("someOtherValue"): resource.PropertyValue{
						V: "a value",
					},
					resource.PropertyKey("someValue"): resource.PropertyValue{V: true},
				},
			}}),
		},
		{
			name:    "secret_value",
			tfValue: "MyPassword",
			tfType: &schema.Schema{
				Type:      shim.TypeString,
				Optional:  true,
				Sensitive: true,
			},
			expect: autogold.Expect(resource.PropertyMap{resource.PropertyKey("secretValue"): resource.PropertyValue{
				V: &resource.Secret{Element: resource.PropertyValue{
					V: "MyPassword",
				}},
			}}),
		},
		{
			name: "nested_secret_value",
			tfValue: []interface{}{
				map[string]interface{}{
					"secret_value": "MyPassword",
				},
			},
			tfType: &schema.Schema{
				Type:     shim.TypeList,
				MaxItems: 1,
				Optional: true,
				Elem: (&schema.Resource{
					Schema: schemaMap(map[string]*schema.Schema{
						"secret_value": {
							Type:      shim.TypeString,
							Sensitive: true,
							Required:  true,
						},
					}),
				}).Shim(),
			},
			expect: autogold.Expect(resource.PropertyMap{resource.PropertyKey("nestedSecretValue"): resource.PropertyValue{
				V: resource.PropertyMap{resource.PropertyKey("secretValue"): resource.PropertyValue{
					V: &resource.Secret{Element: resource.PropertyValue{
						V: "MyPassword",
					}},
				}},
			}}),
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			for _, f := range factories {
				f := f
				t.Run(f.SDKVersion(), func(t *testing.T) {
					t.Parallel()
					ctx := context.Background()

					var tfType map[string]*schema.Schema
					if tt.tfType != nil {
						tfType = map[string]*schema.Schema{
							tt.name: tt.tfType,
						}
					}

					schemaMap := f.NewSchemaMap(tfType)
					require.NoError(t, schemaMap.Validate())

					var schemaInfo map[string]*SchemaInfo
					if tt.schemaInfo != nil {
						schemaInfo = map[string]*SchemaInfo{
							tt.name: tt.schemaInfo,
						}
					}

					result := MakeTerraformOutputs(
						ctx,
						f.NewTestProvider(),
						map[string]any{
							tt.name: tt.tfValue,
						},
						schemaMap,
						schemaInfo,
						nil,   /* assets */
						false, /* useRawNames */
						true,  /* supportsSecrets */
					)
					tt.expect.Equal(t, result)
				})
			}
		})
	}
}

// TestTerraformOutputsWithSecretsUnsupported verifies that we translate Terraform outputs into Pulumi outputs without
// treating sensitive outputs as secrets
func TestTerraformOutputsWithSecretsUnsupported(t *testing.T) {
	ctx := context.Background()
	for _, f := range factories {
		t.Run(f.SDKVersion(), func(t *testing.T) {
			result := MakeTerraformOutputs(
				ctx,
				f.NewTestProvider(),
				map[string]interface{}{
					"secret_value": "MyPassword",
				},
				f.NewSchemaMap(map[string]*schema.Schema{
					"secret_value": {
						Type:      shim.TypeString,
						Optional:  true,
						Sensitive: true,
					},
				}),
				map[string]*SchemaInfo{},
				nil,   /* assets */
				false, /*useRawNames*/
				false,
			)
			assert.Equal(t, resource.NewPropertyMapFromMap(map[string]interface{}{
				"secretValue": "MyPassword",
			}), result)
		})
	}
}

func clearMeta(state shim.InstanceState) bool {
	if tf, ok := shimv1.IsInstanceState(state); ok {
		tf.Meta = map[string]interface{}{}
		return true
	}
	if tf, ok := shimv2.IsInstanceState(state); ok {
		tf.Meta = map[string]interface{}{}
		return true
	}
	return false
}

func clearID(state shim.InstanceState) bool {
	if tf, ok := shimv1.IsInstanceState(state); ok {
		tf.ID = ""
		return true
	}
	if tf, ok := shimv2.IsInstanceState(state); ok {
		tf.ID = ""
		return true
	}
	return false
}

// Test that meta-properties are correctly produced.
func TestMetaProperties(t *testing.T) {
	for _, f := range factories {
		t.Run(f.SDKVersion(), func(t *testing.T) {
			prov := f.NewTestProvider()
			ctx := context.Background()

			const resName = "example_resource"
			res := prov.ResourcesMap().Get(resName)

			state := f.NewInstanceState("0")
			read, err := prov.Refresh(ctx, resName, state, nil)
			assert.NoError(t, err)
			assert.NotNil(t, read)

			props, err := MakeTerraformResult(ctx, prov, read, res.Schema(), nil, nil, true)
			assert.NoError(t, err)
			assert.NotNil(t, props)

			state, err = makeTerraformStateWithOpts(
				ctx, Resource{TF: res, Schema: &ResourceInfo{}}, state.ID(), props, makeTerraformStateOpts{})
			assert.NoError(t, err)
			assert.NotNil(t, state)

			assert.Equal(t, strconv.Itoa(res.SchemaVersion()), state.Meta()["schema_version"])

			read2, err := prov.Refresh(ctx, resName, state, nil)
			assert.NoError(t, err)
			assert.NotNil(t, read2)
			assert.Equal(t, read, read2)

			// Delete the resource's meta-property and ensure that we re-populate its schema version.
			delete(props, metaKey)

			state, err = makeTerraformStateWithOpts(
				ctx, Resource{TF: res, Schema: &ResourceInfo{}}, state.ID(), props, makeTerraformStateOpts{})
			assert.NoError(t, err)
			assert.NotNil(t, state)

			assert.Equal(t, strconv.Itoa(res.SchemaVersion()), state.Meta()["schema_version"])

			// Remove the resource's meta-attributes and ensure that we do not include them in the result.
			ok := clearMeta(read2)
			assert.True(t, ok)
			props, err = MakeTerraformResult(ctx, prov, read2, res.Schema(), nil, nil, true)
			assert.NoError(t, err)
			assert.NotNil(t, props)
			assert.NotContains(t, props, metaKey)

			// Ensure that timeouts are populated and preserved.
			ok = clearID(state)
			assert.True(t, ok)
			cfg := prov.NewResourceConfig(ctx, map[string]interface{}{})

			// To populate default timeouts, we take the timeouts from the resource schema and insert them into the diff
			timeouts, err := res.DecodeTimeouts(cfg)
			assert.NoError(t, err)

			diff, err := prov.Diff(ctx, resName, state, cfg, shim.DiffOptions{
				TimeoutOptions: shim.TimeoutOptions{
					ResourceTimeout: timeouts,
				},
			})
			assert.NoError(t, err)

			assert.NoError(t, err)
			create, err := prov.Apply(ctx, resName, state, diff)
			assert.NoError(t, err)

			props, err = MakeTerraformResult(ctx, prov, create, res.Schema(), nil, nil, true)
			assert.NoError(t, err)
			assert.NotNil(t, props)

			state, err = makeTerraformStateWithOpts(
				ctx, Resource{TF: res, Schema: &ResourceInfo{}}, state.ID(), props, makeTerraformStateOpts{})
			assert.NoError(t, err)
			assert.NotNil(t, state)

			assert.Contains(t, state.Meta(), schemav1.TimeoutKey)
		})
	}
}

func TestInjectingCustomTimeouts(t *testing.T) {
	for _, f := range factories {
		t.Run(f.SDKVersion(), func(t *testing.T) {
			prov := f.NewTestProvider()
			ctx := context.Background()

			const resName = "second_resource"
			res := prov.ResourcesMap().Get(resName)

			state := f.NewInstanceState("0")
			read, err := prov.Refresh(ctx, resName, state, nil)
			assert.NoError(t, err)
			assert.NotNil(t, read)

			props, err := MakeTerraformResult(ctx, prov, read, res.Schema(), nil, nil, true)
			assert.NoError(t, err)
			assert.NotNil(t, props)

			state, err = makeTerraformStateWithOpts(
				ctx, Resource{TF: res, Schema: &ResourceInfo{}}, state.ID(), props, makeTerraformStateOpts{})
			assert.NoError(t, err)
			assert.NotNil(t, state)

			assert.Equal(t, strconv.Itoa(res.SchemaVersion()), state.Meta()["schema_version"])

			read2, err := prov.Refresh(ctx, resName, state, nil)
			assert.NoError(t, err)
			assert.NotNil(t, read2)
			assert.Equal(t, read, read2)

			// Delete the resource's meta-property and ensure that we re-populate its schema version.
			delete(props, metaKey)

			state, err = makeTerraformStateWithOpts(
				ctx, Resource{TF: res, Schema: &ResourceInfo{}}, state.ID(), props, makeTerraformStateOpts{})
			assert.NoError(t, err)
			assert.NotNil(t, state)

			assert.Equal(t, strconv.Itoa(res.SchemaVersion()), state.Meta()["schema_version"])

			// Remove the resource's meta-attributes and ensure that we do not include them in the result.
			ok := clearMeta(read2)
			assert.True(t, ok)
			props, err = MakeTerraformResult(ctx, prov, read2, res.Schema(), nil, nil, true)
			assert.NoError(t, err)
			assert.NotNil(t, props)
			assert.NotContains(t, props, metaKey)

			// Ensure that timeouts are populated and preserved.
			ok = clearID(state)
			assert.True(t, ok)
			cfg := prov.NewResourceConfig(ctx, map[string]interface{}{})

			// To populate default timeouts, we take the timeouts from the resource schema and insert them into the diff
			resourceTimeouts, err := res.DecodeTimeouts(cfg)
			assert.NoError(t, err)

			diff, err := prov.Diff(ctx, resName, state, cfg, shim.DiffOptions{
				TimeoutOptions: shim.TimeoutOptions{
					ResourceTimeout: resourceTimeouts,
					TimeoutOverrides: map[shim.TimeoutKey]time.Duration{
						shim.TimeoutCreate: 300 * time.Second,
					},
				},
			})
			assert.NoError(t, err)

			assert.NoError(t, err)
			create, err := prov.Apply(ctx, resName, state, diff)
			assert.NoError(t, err)

			props, err = MakeTerraformResult(ctx, prov, create, res.Schema(), nil, nil, true)
			assert.NoError(t, err)
			assert.NotNil(t, props)

			state, err = makeTerraformStateWithOpts(
				ctx, Resource{TF: res, Schema: &ResourceInfo{}}, state.ID(), props, makeTerraformStateOpts{})
			assert.NoError(t, err)
			assert.NotNil(t, state)

			switch f.SDKVersion() {
			case "v1":
				timeouts := state.Meta()[schemav1.TimeoutKey].(map[string]interface{})
				assert.NotNil(t, timeouts)
				assert.Contains(t, timeouts, schemav1.TimeoutCreate)
				assert.Equal(t, timeouts[schemav1.TimeoutCreate], float64(300000000000))
				assert.NotContains(t, timeouts, schemav1.TimeoutDelete)
				assert.Contains(t, timeouts, schemav1.TimeoutUpdate)
			case "v2":
				timeouts := state.Meta()[schemav2.TimeoutKey].(map[string]interface{})
				assert.NotNil(t, timeouts)
				assert.Contains(t, timeouts, schemav2.TimeoutCreate)
				assert.Equal(t, timeouts[schemav2.TimeoutCreate], float64(300000000000))
				assert.NotContains(t, timeouts, schemav2.TimeoutDelete)
				assert.Contains(t, timeouts, schemav2.TimeoutUpdate)
			default:
				assert.FailNow(t, "unexpected SDK version %v", f.SDKVersion())
			}
		})
	}
}

func getStateAttributes(state shim.InstanceState) (map[string]string, bool) {
	if tf, ok := shimv1.IsInstanceState(state); ok {
		return tf.Attributes, true
	}
	if tf, ok := shimv2.IsInstanceState(state); ok {
		return tf.Attributes, true
	}
	return nil, false
}

// Test that MakeTerraformResult reads property values appropriately.
func TestResultAttributesRoundTrip(t *testing.T) {
	for _, f := range factories {
		t.Run(f.SDKVersion(), func(t *testing.T) {
			prov := f.NewTestProvider()
			ctx := context.Background()

			const resName = "example_resource"
			res := prov.ResourcesMap().Get("example_resource")

			state := f.NewInstanceState("0")
			read, err := prov.Refresh(ctx, resName, state, nil)
			assert.NoError(t, err)
			assert.NotNil(t, read)

			props, err := MakeTerraformResult(ctx, prov, read, res.Schema(), nil, nil, true)
			assert.NoError(t, err)
			assert.NotNil(t, props)

			state, err = makeTerraformStateWithOpts(
				ctx, Resource{TF: res, Schema: &ResourceInfo{}}, state.ID(), props, makeTerraformStateOpts{})
			assert.NoError(t, err)
			assert.NotNil(t, state)

			readAttributes, ok := getStateAttributes(read)
			assert.True(t, ok)
			stateAttributes, ok := getStateAttributes(state)
			assert.True(t, ok)

			// We may add extra "%" fields to represent map counts. These diffs are innocuous. If we only see them in the
			// attributes produced by MakeTerraformResult, ignore them.
			for k, v := range stateAttributes {
				expected, ok := readAttributes[k]
				if !ok {
					assert.True(t, strings.HasSuffix(k, ".%"))
				} else {
					assert.Equal(t, expected, v)
				}
			}
		})
	}
}

func sortDefaultsList(m resource.PropertyMap) {
	defs := m[defaultsKey].ArrayValue()
	sort.Slice(defs, func(i, j int) bool { return defs[i].StringValue() < defs[j].StringValue() })
	m[defaultsKey] = resource.NewArrayProperty(defs)
}

func fixedDefault(value interface{}) func() (interface{}, error) {
	return func() (interface{}, error) { return value, nil }
}

func TestDefaults(t *testing.T) {
	ctx := context.Background()
	for _, f := range factories {
		t.Run(f.SDKVersion(), func(t *testing.T) {
			// Produce maps with the following properties, and then validate them:
			//     - aaa string; no defaults, no inputs => empty
			//     - bbb string; no defaults, input "BBB" => "BBB"
			//     - ccc string; TF default "CCC", no inputs => "CCC"
			//     - cc2 string; TF default "CC2" (func), no inputs => "CC2"
			//     - ddd string; TF default "TFD", input "DDD" => "DDD"
			//     - dd2 string; TF default "TD2" (func), input "DDD" => "DDD"
			//     - eee string; PS default "EEE", no inputs => "EEE"
			//     - ee2 string; PS default "EE2" (func), no inputs => "EE2"
			//     - fff string; PS default "PSF", input "FFF" => "FFF"
			//     - ff2 string; PS default "PF2", input "FFF" => "FFF"
			//     - ggg string; TF default "TFG", PS default "PSG", no inputs => "PSG" (PS wins)
			//     - hhh string; TF default "TFH", PS default "PSH", input "HHH" => "HHH"
			//     - iii string; old default "OLI", TF default "TFI", PS default "PSI", no input => "OLD"
			//     - jjj string: old input "OLJ", no defaults, no input => no merged input
			//     - lll: old default "OLL", TF default "TFL", no input => "OLL"
			//     - ll2: old input "OL2", TF default "TL2", no input => "TL2"
			//     - mmm: old default "OLM", PS default "PSM", no input => "OLM"
			//     - mm2: old input "OLM", PS default "PM2", no input => "PM2"
			//     - uuu: PS default "PSU", envvars w/o valiues => "PSU"
			//     - vvv: PS default 42, envvars with values => 1337
			//     - www: old default "OLW", deprecated, required, no input -> "OLW"
			//     - xxx: old default "OLX", deprecated, no input => nothing
			//     - yyy: TF default "TLY", deprecated, no input => nothing
			err := os.Setenv("PTFV2", "1337")
			assert.Nil(t, err)
			asset, err := resource.NewTextAsset("hello")
			assert.Nil(t, err)

			tfs := f.NewSchemaMap(map[string]*schema.Schema{
				"ccc": {Type: shim.TypeString, Default: "CCC"},
				"cc2": {Type: shim.TypeString, DefaultFunc: func() (interface{}, error) { return "CC2", nil }},
				"ddd": {Type: shim.TypeString, Default: "TFD"},
				"dd2": {Type: shim.TypeString, DefaultFunc: func() (interface{}, error) { return "TD2", nil }},
				"ggg": {Type: shim.TypeString, Default: "TFG"},
				"hhh": {Type: shim.TypeString, Default: "TFH"},
				"iii": {Type: shim.TypeString, Default: "TFI"},
				"jjj": {Type: shim.TypeString},
				"lll": {Type: shim.TypeString, Default: "TFL"},
				"ll2": {Type: shim.TypeString, Default: "TL2"},
				"mmm": {Type: shim.TypeString},
				"mm2": {Type: shim.TypeString},
				"sss": {Type: shim.TypeString, Removed: "removed"},
				"ttt": {Type: shim.TypeString, Removed: "removed", Default: "TFD"},
				"uuu": {Type: shim.TypeString},
				"vvv": {Type: shim.TypeInt},
				"www": {Type: shim.TypeString, Deprecated: "deprecated", Required: true},
				"xxx": {Type: shim.TypeString, Deprecated: "deprecated", Optional: true},
				"yyy": {Type: shim.TypeString, Default: "TLY", Deprecated: "deprecated", Optional: true},
				"zzz": {Type: shim.TypeString},

				// Default value application across types
				"x2stringxbool": {Type: shim.TypeString},
				"x2stringxint":  {Type: shim.TypeString},
			})
			ps := map[string]*SchemaInfo{
				"eee": {Default: &DefaultInfo{Value: "EEE"}},
				"ee2": {Default: &DefaultInfo{From: func(res *PulumiResource) (interface{}, error) { return "EE2", nil }}},
				"fff": {Default: &DefaultInfo{Value: "PSF"}},
				"ff2": {Default: &DefaultInfo{From: func(res *PulumiResource) (interface{}, error) { return "PF2", nil }}},
				"ggg": {Default: &DefaultInfo{Value: "PSG"}},
				"hhh": {Default: &DefaultInfo{Value: "PSH"}},
				"iii": {Default: &DefaultInfo{Value: "PSI"}},
				"mmm": {Default: &DefaultInfo{Value: "PSM"}},
				"mm2": {Default: &DefaultInfo{Value: "PM2"}},
				"sss": {Default: &DefaultInfo{Value: "PSS"}, Removed: true},
				"uuu": {Default: &DefaultInfo{Value: "PSU", EnvVars: []string{"PTFU", "PTFU2"}}},
				"vvv": {Default: &DefaultInfo{Value: 42, EnvVars: []string{"PTFV", "PTFV2"}}},
				"www": {Default: &DefaultInfo{Value: "PSW"}},
				"zzz": {Asset: &AssetTranslation{Kind: FileAsset}},

				// Default applications where the Default.Value doesn't
				// match all possible types because Pulumi and TF have
				// different types.
				"x2stringxbool": {Type: "bool", Default: &DefaultInfo{Value: true}},
				"x2stringxint":  {Type: "int", Default: &DefaultInfo{Value: 1}},
			}
			olds := resource.PropertyMap{
				defaultsKey: resource.NewPropertyValue([]interface{}{
					"iii", "jjj", "lll", "mmm", "www", "xxx",
				}),
				"iii": resource.NewStringProperty("OLI"),
				"jjj": resource.NewStringProperty("OLJ"),
				"lll": resource.NewStringProperty("OLL"),
				"ll2": resource.NewStringProperty("OL2"),
				"mmm": resource.NewStringProperty("OLM"),
				"mm2": resource.NewStringProperty("OM2"),
				"www": resource.NewStringProperty("OLW"),
				"xxx": resource.NewStringProperty("OLX"),
			}
			props := resource.PropertyMap{
				"bbb": resource.NewStringProperty("BBB"),
				"ddd": resource.NewStringProperty("DDD"),
				"dd2": resource.NewStringProperty("DDD"),
				"fff": resource.NewStringProperty("FFF"),
				"ff2": resource.NewStringProperty("FFF"),
				"hhh": resource.NewStringProperty("HHH"),
				"zzz": resource.NewAssetProperty(asset),
			}
			inputs, assets, err := makeTerraformInputsForConfig(olds, props, tfs, ps)
			assert.NoError(t, err)
			outputs := MakeTerraformOutputs(ctx, f.NewTestProvider(), inputs, tfs, ps, assets, false, true)

			// sort the defaults list before the equality test below.
			sortDefaultsList(outputs)

			assert.Equal(t, resource.NewPropertyMapFromMap(map[string]interface{}{
				defaultsKey: []interface{}{
					"cc2", "ccc", "ee2", "eee", "ggg", "iii", "ll2", "lll",
					"mm2", "mmm", "uuu", "vvv", "www",
					"x2stringxbool", "x2stringxint",
				},
				"bbb": "BBB",
				"ccc": "CCC",
				"cc2": "CC2",
				"ddd": "DDD",
				"dd2": "DDD",
				"eee": "EEE",
				"ee2": "EE2",
				"fff": "FFF",
				"ff2": "FFF",
				"ggg": "PSG",
				"hhh": "HHH",
				"iii": "OLI",
				"lll": "OLL",
				"ll2": "TL2",
				"mmm": "OLM",
				"mm2": "PM2",
				"uuu": "PSU",
				"vvv": 1337,
				"www": "OLW",
				"zzz": asset,

				"x2stringxbool": true,
				"x2stringxint":  1,
			}), outputs)

			// Now delete the defaults list from the olds and re-run. This will affect the values for "ll2" and "mm2", which
			// will be pulled from the old inputs instead of regenerated.
			delete(olds, defaultsKey)
			inputs, assets, err = makeTerraformInputsForConfig(olds, props, tfs, ps)
			assert.NoError(t, err)

			// Assert that types match their TF equivalent when in a TF shape.
			assert.Equal(t, "true", inputs["x2stringxbool"])
			assert.Equal(t, "1", inputs["x2stringxint"])

			outputs = MakeTerraformOutputs(ctx, f.NewTestProvider(), inputs, tfs, ps, assets, false, true)

			// sort the defaults list before the equality test below.
			sortDefaultsList(outputs)
			assert.Equal(t, resource.NewPropertyMapFromMap(map[string]interface{}{
				defaultsKey: []interface{}{
					"cc2", "ccc", "ee2", "eee", "ggg", "iii", "ll2", "lll",
					"mm2", "mmm", "uuu", "vvv", "www",

					"x2stringxbool", "x2stringxint",
				},
				"bbb": "BBB",
				"ccc": "CCC",
				"cc2": "CC2",
				"ddd": "DDD",
				"dd2": "DDD",
				"eee": "EEE",
				"ee2": "EE2",
				"fff": "FFF",
				"ff2": "FFF",
				"ggg": "PSG",
				"hhh": "HHH",
				"iii": "OLI",
				"lll": "OLL",
				"ll2": "OL2",
				"mmm": "OLM",
				"mm2": "OM2",
				"uuu": "PSU",
				"vvv": 1337,
				"www": "OLW",
				"zzz": asset,

				"x2stringxbool": true,
				"x2stringxint":  1,
			}), outputs)
		})
	}
}

func TestDefaultsConflictsWith(t *testing.T) {
	ctx := context.Background()
	for _, f := range factories {
		t.Run(f.SDKVersion(), func(t *testing.T) {
			x1ofN := []string{"x1of1", "x1of2", "x1of3"}
			tfs := f.NewSchemaMap(map[string]*schema.Schema{
				"xyz": {Type: shim.TypeString, ExactlyOneOf: []string{"xyz", "abc"}},
				"abc": {Type: shim.TypeString, Default: "ABC", ExactlyOneOf: []string{"xyz", "abc"}},
				"nnn": {Type: shim.TypeString, ConflictsWith: []string{"nn2"}, Default: "NNN"},
				"nn2": {Type: shim.TypeString, ConflictsWith: []string{"nnn"}, Default: "NN2"},
				"ooo": {Type: shim.TypeString, ConflictsWith: []string{"oo2"}, Default: "OOO"},
				"oo2": {Type: shim.TypeString, ConflictsWith: []string{"ooo"}},
				"oo3": {Type: shim.TypeString, ConflictsWith: []string{"nonexisting"}},

				// Test exactly one of behavior with default funcs:
				"x1of1": {Type: shim.TypeString, ExactlyOneOf: x1ofN, DefaultFunc: fixedDefault("x1of1-value")},
				"x1of2": {Type: shim.TypeString, ExactlyOneOf: x1ofN, DefaultFunc: fixedDefault(nil)},
				"x1of3": {Type: shim.TypeString, ExactlyOneOf: x1ofN, DefaultFunc: fixedDefault(nil)},
			})

			ps := map[string]*SchemaInfo{
				"oo2": {Default: &DefaultInfo{Value: "PO2"}},
			}
			olds := resource.PropertyMap{
				defaultsKey: resource.NewPropertyValue([]interface{}{}),
			}
			props := resource.PropertyMap{}

			inputs, assets, err := makeTerraformInputsForConfig(olds, props, tfs, ps)
			assert.NoError(t, err)
			outputs := MakeTerraformOutputs(ctx, f.NewTestProvider(), inputs, tfs, ps, assets, false, true)
			sortDefaultsList(outputs)

			assert.Equal(t, resource.NewPropertyMapFromMap(map[string]interface{}{
				defaultsKey: []interface{}{
					"abc", "oo2", "x1of1",
				},
				"abc": "ABC",
				// nnn/nn2 are NOT set as they conflict with each other
				// ooo is NOT set as it conflicts with oo2
				"oo2": "PO2",
				// xyz is NOT set as it has ExactlyOneOf with abc
				// x1of1 is set as it UNIQUELY has a default value in its ExactlyOneOf set (x1of1, x1of2, x1of3)
				"x1of1": "x1of1-value",
			}), outputs)

			delete(olds, defaultsKey)
			inputs, assets, err = makeTerraformInputsForConfig(olds, props, tfs, ps)
			assert.NoError(t, err)

			outputs = MakeTerraformOutputs(ctx, f.NewTestProvider(), inputs, tfs, ps, assets, false, true)
			sortDefaultsList(outputs)

			assert.Equal(t, resource.NewPropertyMapFromMap(map[string]interface{}{
				defaultsKey: []interface{}{
					"abc", "oo2", "x1of1",
				},
				"abc": "ABC",
				// nnn/nn2 are NOT set as they conflict with each other
				// ooo is NOT set as it conflicts with oo2
				"oo2": "PO2",
				// xyz is NOT set as it has ExactlyOneOf with abc
				// x1of1 is set as it UNIQUELY has a default value in its
				// ExactlyOneOf set (x1of1, x1of2, x1of3)
				"x1of1": "x1of1-value",
			}), outputs)
		})
	}
}

func TestComputedAsset(t *testing.T) {
	ctx := context.Background()
	tfs := shimv1.NewSchemaMap(map[string]*schemav1.Schema{
		"zzz": {Type: schemav1.TypeString},
	})
	ps := map[string]*SchemaInfo{
		"zzz": {Asset: &AssetTranslation{Kind: FileAsset}},
	}
	olds := resource.PropertyMap{}
	props := resource.PropertyMap{
		"zzz": resource.NewStringProperty(TerraformUnknownVariableValue),
	}
	inputs, assets, err := makeTerraformInputsNoDefaults(olds, props, tfs, ps)
	assert.NoError(t, err)
	outputs := MakeTerraformOutputs(ctx, shimv1.NewProvider(testTFProvider), inputs, tfs, ps, assets, false, true)
	assert.Equal(t, resource.PropertyMap{
		"zzz": resource.PropertyValue{V: resource.Computed{Element: resource.PropertyValue{V: ""}}},
	}, outputs)
}

func TestInvalidAsset(t *testing.T) {
	ctx := context.Background()
	tfs := shimv1.NewSchemaMap(map[string]*schemav1.Schema{
		"zzz": {Type: schemav1.TypeString},
	})
	ps := map[string]*SchemaInfo{
		"zzz": {Asset: &AssetTranslation{Kind: FileAsset}},
	}
	olds := resource.PropertyMap{}
	props := resource.PropertyMap{
		"zzz": resource.NewStringProperty("invalid"),
	}
	inputs, assets, err := makeTerraformInputsNoDefaults(olds, props, tfs, ps)
	assert.NoError(t, err)
	outputs := MakeTerraformOutputs(ctx, shimv1.NewProvider(testTFProvider), inputs, tfs, ps, assets, false, true)
	assert.Equal(t, resource.PropertyMap{
		"zzz": resource.NewStringProperty("invalid"),
	}, outputs)
}

func TestOverridingTFSchema(t *testing.T) {
	ctx := context.Background()

	tfInputs := map[string]interface{}{
		"pulumi_override_tf_string_to_boolean":    MyString("true"),
		"pulumi_override_tf_string_to_bool":       MyString("true"),
		"pulumi_empty_tf_override":                MyString("true"),
		"pulumi_override_tf_string_to_int":        MyString("1"),
		"pulumi_override_tf_string_to_integer":    MyString("1"),
		"tf_empty_string_to_pulumi_bool_override": MyString(""),
	}

	tfSchema := shimv1.NewSchemaMap(map[string]*schemav1.Schema{
		"pulumi_override_tf_string_to_boolean":    {Type: schemav1.TypeString},
		"pulumi_override_tf_string_to_bool":       {Type: schemav1.TypeString},
		"pulumi_empty_tf_override":                {Type: schemav1.TypeString},
		"pulumi_override_tf_string_to_int":        {Type: schemav1.TypeString},
		"pulumi_override_tf_string_to_integer":    {Type: schemav1.TypeString},
		"tf_empty_string_to_pulumi_bool_override": {Type: schemav1.TypeString},
	})

	typeOverrides := map[string]*SchemaInfo{
		"pulumi_override_tf_string_to_boolean": {
			Type: "boolean",
		},
		"pulumi_override_tf_string_to_bool": {
			Type: "bool",
		},
		"pulumi_empty_tf_override": {
			Type: "",
		},
		"pulumi_override_tf_string_to_int": {
			Type: "int",
		},
		"pulumi_override_tf_string_to_integer": {
			Type: "integer",
		},
		"tf_empty_string_to_pulumi_bool_override": {
			Type:           "boolean",
			MarkAsOptional: boolPointer(true),
		},
	}

	tfOutputs := resource.NewPropertyMapFromMap(map[string]interface{}{
		"pulumiOverrideTfStringToBoolean":   true,
		"pulumiOverrideTfStringToBool":      true,
		"pulumiEmptyTfOverride":             "true",
		"pulumiOverrideTfStringToInt":       1,
		"pulumiOverrideTfStringToInteger":   1,
		"tfEmptyStringToPulumiBoolOverride": nil,
	})

	t.Run("MakeTerraformOutputs", func(t *testing.T) {
		result := MakeTerraformOutputs(
			ctx,
			shimv1.NewProvider(testTFProvider),
			tfInputs,
			tfSchema,
			typeOverrides,
			nil,   /* assets */
			false, /*useRawNames*/
			true,
		)
		assert.Equal(t, tfOutputs, result)
	})
	t.Run("MakeTerraformInputs", func(t *testing.T) {
		result, _, err := makeTerraformInputsForConfig(
			nil,
			tfOutputs,
			tfSchema,
			typeOverrides,
		)
		require.NoError(t, err)
		expected := map[string]interface{}{
			// SDKv2 Providers have __defaults included.
			"__defaults": []interface{}{},
		}
		for k, v := range tfInputs {
			// We don't transform nil values because terraform distinguished
			// between nil and "" values.
			if s := string(v.(MyString)); s == "" {
				expected[k] = nil
			} else {
				expected[k] = s
			}
		}
		assert.Equal(t, expected, result)
	})
}

func TestArchiveAsAsset(t *testing.T) {
	ctx := context.Background()
	tfs := shimv1.NewSchemaMap(map[string]*schemav1.Schema{
		"zzz": {Type: schemav1.TypeString},
	})
	ps := map[string]*SchemaInfo{
		"zzz": {Asset: &AssetTranslation{Kind: FileAsset}},
	}
	olds := resource.PropertyMap{}
	asset, err := resource.NewTextAsset("bar")
	assert.NoError(t, err)
	archValue, err := resource.NewAssetArchive(map[string]interface{}{
		"foo": asset,
	})
	assert.NoError(t, err)
	arch := resource.NewPropertyValue(archValue)
	props := resource.PropertyMap{
		"zzz": arch,
	}
	inputs, assets, err := makeTerraformInputsNoDefaults(olds, props, tfs, ps)
	assert.NoError(t, err)
	outputs := MakeTerraformOutputs(ctx, shimv1.NewProvider(testTFProvider), inputs, tfs, ps, assets, false, true)
	assert.True(t, arch.DeepEquals(outputs["zzz"]))
}

func boolPointer(b bool) *bool {
	return &b
}

func TestCustomTransforms(t *testing.T) {
	doc := map[string]interface{}{
		"a": 99,
		"b": false,
	}
	tfs := shimv1.NewSchema(&schemav1.Schema{Type: schemav1.TypeString})
	psi := &SchemaInfo{Transform: TransformJSONDocument}

	v1, err := makeTerraformInput(resource.NewObjectProperty(resource.NewPropertyMapFromMap(doc)), tfs, psi)
	assert.NoError(t, err)
	assert.Equal(t, `{"a":99,"b":false}`, v1)

	array := []resource.PropertyValue{resource.NewObjectProperty(resource.NewPropertyMapFromMap(doc))}
	v1Array, err := makeTerraformInput(resource.NewArrayProperty(array), tfs, psi)
	assert.NoError(t, err)
	assert.Equal(t, `[{"a":99,"b":false}]`, v1Array)

	v2, err := makeTerraformInput(resource.NewStringProperty(`{"a":99,"b":false}`), tfs, psi)
	assert.NoError(t, err)
	assert.Equal(t, `{"a":99,"b":false}`, v2)

	doc["c"] = resource.Computed{Element: resource.PropertyValue{V: ""}}
	v3, err := makeTerraformInput(resource.NewObjectProperty(resource.NewPropertyMapFromMap(doc)), tfs, psi)
	assert.NoError(t, err)
	assert.Equal(t, TerraformUnknownVariableValue, v3)

	v4, err := makeTerraformInput(resource.MakeComputed(resource.NewStringProperty("")), tfs, psi)
	assert.NoError(t, err)
	assert.Equal(t, TerraformUnknownVariableValue, v4)

	// This checks the fix to the regression caused via CoerceTerraformString to ensure we handle nil in Transforms
	v5, err := makeTerraformInput(resource.NewNullProperty(), tfs, psi)
	assert.NoError(t, err)
	assert.Equal(t, "", v5)

	emptyDoc := ""
	v6, err := makeTerraformInput(resource.NewStringProperty(emptyDoc), tfs, psi)
	assert.NoError(t, err)
	assert.Equal(t, "", v6)
}

func TestImporterOnRead(t *testing.T) {
	tfProvider := makeTestTFProviderV1(
		map[string]*schemav1.Schema{
			"required_for_import": {
				Type: schemav1.TypeString,
			},
		},
		func(d *schemav1.ResourceData, meta interface{}) ([]*schemav1.ResourceData, error) {
			importValue := d.Id() + "-imported"
			testprovider.MustSet(d, "required_for_import", importValue)

			return []*schemav1.ResourceData{d}, nil
		})

	provider := &Provider{
		tf: shimv1.NewProvider(tfProvider),
		resources: map[tokens.Type]Resource{
			"importableResource": {
				TF:     shimv1.NewResource(tfProvider.ResourcesMap["importable_resource"]),
				TFName: "importable_resource",
				Schema: &ResourceInfo{
					Tok: tokens.NewTypeToken("module", "importableResource"),
				},
			},
		},
	}

	{
		urn := resource.NewURN("s", "pr", "pa", "importableResource", "n")
		resp, err := provider.Read(context.TODO(), &pulumirpc.ReadRequest{
			Id:  "MyID",
			Urn: string(urn),
		})

		assert.NoError(t, err)
		assert.Equal(t, "MyID-imported", resp.Properties.Fields["requiredForImport"].GetStringValue())
	}

	{
		urn := resource.NewURN("s", "pr", "pa", "importableResource", "n2")
		resp, err := provider.Read(context.TODO(), &pulumirpc.ReadRequest{
			Id:  "MyID",
			Urn: string(urn),
			Properties: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"name": {
						Kind: &structpb.Value_StringValue{StringValue: "IAmAlreadyset"},
					},
				},
			},
		})
		assert.NoError(t, err)
		assert.Nil(t, resp.Properties.Fields["requiredForImport"])
	}
}

func TestImporterWithNewID(t *testing.T) {
	tfProvider := makeTestTFProviderV1(
		map[string]*schemav1.Schema{
			"required_for_import": {
				Type: schemav1.TypeString,
			},
		},
		func(d *schemav1.ResourceData, meta interface{}) ([]*schemav1.ResourceData, error) {
			d.SetId(d.Id() + "-imported")
			return []*schemav1.ResourceData{d}, nil
		})

	provider := &Provider{
		tf: shimv1.NewProvider(tfProvider),
		resources: map[tokens.Type]Resource{
			"importableResource": {
				TF:     shimv1.NewResource(tfProvider.ResourcesMap["importable_resource"]),
				TFName: "importable_resource",
				Schema: &ResourceInfo{
					Tok: tokens.NewTypeToken("module", "importableResource"),
				},
			},
		},
	}

	{
		urn := resource.NewURN("s", "pr", "pa", "importableResource", "n")
		resp, err := provider.Read(context.TODO(), &pulumirpc.ReadRequest{
			Id:  "MyID",
			Urn: string(urn),
		})

		assert.NoError(t, err)
		assert.Equal(t, "MyID-imported", resp.Id)
	}
}

func TestImporterWithMultipleResourceTypes(t *testing.T) {
	tfProvider := makeTestTFProviderV1(
		map[string]*schemav1.Schema{
			"required_for_import": {
				Type: schemav1.TypeString,
			},
		},
		func(d *schemav1.ResourceData, meta interface{}) ([]*schemav1.ResourceData, error) {
			d.SetId(d.Id() + "-imported")

			d2 := &schemav1.ResourceData{}
			d2.SetType("other_importable_resource")
			d2.SetId(d.Id())

			return []*schemav1.ResourceData{d, d2}, nil
		})

	provider := &Provider{
		tf: shimv1.NewProvider(tfProvider),
		resources: map[tokens.Type]Resource{
			"importableResource": {
				TF:     shimv1.NewResource(tfProvider.ResourcesMap["importable_resource"]),
				TFName: "importable_resource",
				Schema: &ResourceInfo{
					Tok: tokens.NewTypeToken("module", "importableResource"),
				},
			},
		},
	}

	{
		urn := resource.NewURN("s", "pr", "pa", "importableResource", "n")
		resp, err := provider.Read(context.TODO(), &pulumirpc.ReadRequest{
			Id:  "MyID",
			Urn: string(urn),
		})

		assert.NoError(t, err)
		assert.Equal(t, "MyID-imported", resp.Id)
	}
}

func TestImporterWithMultipleResources(t *testing.T) {
	tfProvider := makeTestTFProviderV1(
		map[string]*schemav1.Schema{
			"required_for_import": {
				Type: schemav1.TypeString,
			},
		},
		func(d *schemav1.ResourceData, meta interface{}) ([]*schemav1.ResourceData, error) {
			d.SetId(d.Id())

			d2 := &schemav1.ResourceData{}
			d2.SetType(d.State().Ephemeral.Type)
			d2.SetId(d.Id() + "-imported")

			return []*schemav1.ResourceData{d, d2}, nil
		})

	provider := &Provider{
		tf: shimv1.NewProvider(tfProvider),
		resources: map[tokens.Type]Resource{
			"importableResource": {
				TF:     shimv1.NewResource(tfProvider.ResourcesMap["importable_resource"]),
				TFName: "importable_resource",
				Schema: &ResourceInfo{
					Tok: tokens.NewTypeToken("module", "importableResource"),
				},
			},
		},
	}

	{
		urn := resource.NewURN("s", "pr", "pa", "importableResource", "n")
		resp, err := provider.Read(context.TODO(), &pulumirpc.ReadRequest{
			Id:  "MyID",
			Urn: string(urn),
		})

		assert.NoError(t, err)
		assert.Equal(t, "MyID", resp.Id)
	}
}

func TestImporterWithMultipleNewIDs(t *testing.T) {
	tfProvider := makeTestTFProviderV1(
		map[string]*schemav1.Schema{
			"required_for_import": {
				Type: schemav1.TypeString,
			},
		},
		func(d *schemav1.ResourceData, meta interface{}) ([]*schemav1.ResourceData, error) {
			d.SetId(d.Id() + "-imported")

			d2 := &schemav1.ResourceData{}
			d2.SetType(d.State().Ephemeral.Type)
			d2.SetId(d.Id() + "-2")

			return []*schemav1.ResourceData{d, d2}, nil
		})

	provider := &Provider{
		tf: shimv1.NewProvider(tfProvider),
		resources: map[tokens.Type]Resource{
			"importableResource": {
				TF:     shimv1.NewResource(tfProvider.ResourcesMap["importable_resource"]),
				TFName: "importable_resource",
				Schema: &ResourceInfo{
					Tok: tokens.NewTypeToken("module", "importableResource"),
				},
			},
		},
	}

	{
		urn := resource.NewURN("s", "pr", "pa", "importableResource", "n")
		_, err := provider.Read(context.TODO(), &pulumirpc.ReadRequest{
			Id:  "MyID",
			Urn: string(urn),
		})

		assert.Error(t, err)
	}
}

func TestImporterWithNoResource(t *testing.T) {
	tfProvider := makeTestTFProviderV1(map[string]*schemav1.Schema{},
		func(d *schemav1.ResourceData, meta interface{}) ([]*schemav1.ResourceData, error) {
			// Return nothing
			return []*schemav1.ResourceData{}, nil
		})

	provider := &Provider{
		tf: shimv1.NewProvider(tfProvider),
		resources: map[tokens.Type]Resource{
			"importableResource": {
				TF:     shimv1.NewResource(tfProvider.ResourcesMap["importable_resource"]),
				TFName: "importable_resource",
				Schema: &ResourceInfo{
					Tok: tokens.NewTypeToken("module", "importableResource"),
				},
			},
		},
	}

	{
		urn := resource.NewURN("s", "pr", "pa", "importableResource", "n")
		resp, err := provider.Read(context.TODO(), &pulumirpc.ReadRequest{
			Id:  "MyID",
			Urn: string(urn),
		})

		assert.NoError(t, err)
		assert.Equal(t, &pulumirpc.ReadResponse{}, resp)
	}
}

func makeTestTFProviderV1(schemaMap map[string]*schemav1.Schema, importer schemav1.StateFunc) *schemav1.Provider {
	return &schemav1.Provider{
		ResourcesMap: map[string]*schemav1.Resource{
			"importable_resource": {
				Schema: schemaMap,
				Importer: &schemav1.ResourceImporter{
					State: importer,
				},
				Read: func(d *schemav1.ResourceData, meta interface{}) error {
					return nil
				},
				Create: func(d *schemav1.ResourceData, meta interface{}) error {
					return nil
				},
				Delete: func(d *schemav1.ResourceData, meta interface{}) error {
					return nil
				},
			},
		},
	}
}

func makeTestTFProviderV2(
	schemaMap map[string]*schemav2.Schema,
	importer schemav2.StateContextFunc,
) *schemav2.Provider {
	return &schemav2.Provider{
		ResourcesMap: map[string]*schemav2.Resource{
			"importable_resource": {
				Schema: schemaMap,
				Importer: &schemav2.ResourceImporter{
					StateContext: importer,
				},
				ReadContext: func(context.Context, *schemav2.ResourceData, interface{}) diag.Diagnostics {
					return nil
				},
				CreateContext: func(context.Context, *schemav2.ResourceData, interface{}) diag.Diagnostics {
					return nil
				},
				DeleteContext: func(context.Context, *schemav2.ResourceData, interface{}) diag.Diagnostics {
					return nil
				},
			},
		},
	}
}

func TestStringOutputsWithSchema(t *testing.T) {
	ctx := context.Background()
	result := MakeTerraformOutputs(
		ctx,
		shimv1.NewProvider(testTFProvider),
		map[string]interface{}{
			"bool_property_value":      "false",
			"number_property_value":    "42",
			"float_property_value":     "42.0",
			"no_schema_property_value": "42",
			"not_an_int_value":         "lmao",
			"not_a_bool_value":         "lmao2",
			"not_a_float_value":        "lmao3",
		},
		shimv1.NewSchemaMap(map[string]*schemav1.Schema{
			"bool_property_value":   {Type: schemav1.TypeBool},
			"number_property_value": {Type: schemav1.TypeInt},
			"float_property_value":  {Type: schemav1.TypeFloat},
			"not_an_int_value":      {Type: schemav1.TypeInt},
			"not_a_bool_value":      {Type: schemav1.TypeBool},
			"not_a_float_value":     {Type: schemav1.TypeFloat},
		}),
		map[string]*SchemaInfo{},
		nil,   /* assets */
		false, /* useRawNames */
		true,
	)

	assert.Equal(t, resource.NewPropertyMapFromMap(map[string]interface{}{
		"boolPropertyValue":     false,
		"numberPropertyValue":   42,
		"floatPropertyValue":    42.0,
		"noSchemaPropertyValue": "42",
		"notAnIntValue":         "lmao",
		"notABoolValue":         "lmao2",
		"notAFloatValue":        "lmao3",
	}), result)
}

func TestExtractInputsFromOutputs(t *testing.T) {
	tfProvider := makeTestTFProviderV1(
		map[string]*schemav1.Schema{
			"input_a": {Type: schemav1.TypeString, Required: true},
			"input_b": {Type: schemav1.TypeString, Optional: true},
			"inout_c": {Type: schemav1.TypeString, Optional: true, Computed: true},
			"inout_d": {Type: schemav1.TypeString, Optional: true, Computed: true, Default: "inout_d_default"},
			"input_e": {
				Type: schemav1.TypeList,
				Elem: &schemav1.Resource{
					Schema: map[string]*schemav1.Schema{
						"field_a": {Type: schemav1.TypeString, Optional: true, Default: "field_a_default"},
					},
				},
				MaxItems: 1,
				Optional: true,
			},
			"input_f":  {Type: schemav1.TypeString, Required: true},
			"output_g": {Type: schemav1.TypeString},
			"input_h": {
				Type:     schemav1.TypeString,
				Required: true,
				StateFunc: func(v interface{}) string {
					return strings.ToLower(v.(string))
				},
			},

			// input_i, inout_j, and inout_k test import scenarios where attributes are set to "".
			"input_i": {Type: schemav1.TypeString, Required: true},
			"inout_j": {Type: schemav1.TypeString, Optional: true, Computed: true},
			"inout_k": {Type: schemav1.TypeString, Optional: true, Computed: true, Default: "inout_k_default"},
		},
		func(d *schemav1.ResourceData, meta interface{}) ([]*schemav1.ResourceData, error) {
			return []*schemav1.ResourceData{d}, nil
		})

	set := func(d *schemav1.ResourceData, key string, value interface{}) {
		contract.IgnoreError(d.Set(key, value))
	}

	tfres := tfProvider.ResourcesMap["importable_resource"]
	tfres.Read = func(d *schemav1.ResourceData, meta interface{}) error {
		_, ok := d.GetOk(defaultsKey)
		assert.False(t, ok)

		if _, ok := d.GetOk("input_a"); !ok {
			set(d, "input_a", "input_a_read")
		}
		if _, ok := d.GetOk("inout_c"); !ok {
			set(d, "inout_c", "inout_c_read")
		}
		set(d, "inout_d", "inout_d_read")
		set(d, "output_g", "output_g_read")
		set(d, "input_i", "")
		set(d, "inout_j", "")
		set(d, "inout_k", "")
		return nil
	}
	tfres.Create = func(d *schemav1.ResourceData, meta interface{}) error {
		_, ok := d.GetOk(defaultsKey)
		assert.False(t, ok)

		d.SetId("MyID")
		if _, ok := d.GetOk("inout_c"); !ok {
			set(d, "inout_c", "inout_c_create")
		}
		set(d, "output_g", "output_g_create")
		return nil
	}

	p := &Provider{
		tf: shimv1.NewProvider(tfProvider),
		resources: map[tokens.Type]Resource{
			"importableResource": {
				TF:     shimv1.NewResource(tfProvider.ResourcesMap["importable_resource"]),
				TFName: "importable_resource",
				Schema: &ResourceInfo{
					Tok: tokens.NewTypeToken("module", "importableResource"),
					Fields: map[string]*SchemaInfo{
						"input_f": {
							Default: &DefaultInfo{
								Value: "input_f_default",
							},
						},
						"input_h": {
							Default: &DefaultInfo{
								Value: "Input_H_Default",
							},
						},
					},
				},
			},
		},
	}

	urn := resource.NewURN("s", "pr", "pa", "importableResource", "n")
	id := resource.ID("MyID")

	// Case 1: read a resource that has no old state (this is the read/import case)
	resp, err := p.Read(context.Background(), &pulumirpc.ReadRequest{
		Id:  string(id),
		Urn: string(urn),
	})
	assert.NoError(t, err)

	outs, err := plugin.UnmarshalProperties(resp.GetProperties(), plugin.MarshalOptions{})
	assert.NoError(t, err)
	assert.Equal(t, resource.NewPropertyMapFromMap(map[string]interface{}{
		"id":      "MyID",
		"inputA":  "input_a_read",
		"inoutC":  "inout_c_read",
		"inoutD":  "inout_d_read",
		"outputG": "output_g_read",
		"inputI":  "",
		"inoutJ":  "",
		"inoutK":  "",
	}), outs)

	ins, err := plugin.UnmarshalProperties(resp.GetInputs(), plugin.MarshalOptions{})
	assert.NoError(t, err)
	expected := resource.NewPropertyMapFromMap(map[string]interface{}{
		defaultsKey: []interface{}{},
		"inputA":    "input_a_read",
		"inoutC":    "inout_c_read",
		"inoutD":    "inout_d_read",
		"inputI":    "",
		"inoutK":    "",
	})
	assert.Equal(t, expected, ins)

	// Case 2: read a resource that has old state (this is the refresh case)
	//
	// Though test is fairly verbose, it is conceptually pretty simple: we construct an input bag, pass it through
	// Check, pass the result to Create, and then call Read with the result of Create. We expect the information
	// about defaults and inputs that gets smuggled along in our various property bags to be persisted throughout, with
	// removal of defaults where necessary when calculating the new input set.

	// Step 1: create and check an input bag. We should see information about which properties were populated using
	// defaults in the result.
	pulumiIns, err := plugin.MarshalProperties(resource.NewPropertyMapFromMap(map[string]interface{}{
		"inputA": "input_a_create",
		"inputE": map[string]interface{}{},
	}), plugin.MarshalOptions{})
	assert.NoError(t, err)
	checkResp, err := p.Check(context.Background(), &pulumirpc.CheckRequest{
		Urn:  string(urn),
		News: pulumiIns,
	})
	assert.NoError(t, err)
	checkedIns, err := plugin.UnmarshalProperties(checkResp.GetInputs(), plugin.MarshalOptions{})
	assert.NoError(t, err)
	sortDefaultsList(checkedIns)
	assert.Equal(t, resource.NewPropertyMapFromMap(map[string]interface{}{
		defaultsKey: []interface{}{"inoutD", "inoutK", "inputF", "inputH"},
		"inputA":    "input_a_create",
		"inoutD":    "inout_d_default",
		"inputE": map[string]interface{}{
			defaultsKey: []interface{}{"fieldA"},
			"fieldA":    "field_a_default",
		},
		"inputF": "input_f_default",
		"inputH": "Input_H_Default",
		"inoutK": "inout_k_default",
	}), checkedIns)

	// Step 2: create a resource using the checked input bag. The inputs should be smuggled along with the state.
	createResp, err := p.Create(context.Background(), &pulumirpc.CreateRequest{
		Urn:        string(urn),
		Properties: checkResp.GetInputs(),
	})
	assert.NoError(t, err)

	outs, err = plugin.UnmarshalProperties(createResp.GetProperties(), plugin.MarshalOptions{})
	assert.NoError(t, err)
	assert.Equal(t, resource.NewPropertyMapFromMap(map[string]interface{}{
		"id":     "MyID",
		"inputA": "input_a_create",
		"inoutC": "inout_c_create",
		"inoutD": "inout_d_default",
		"inputE": map[string]interface{}{
			"fieldA": "field_a_default",
		},
		"inputF":  "input_f_default",
		"outputG": "output_g_create",
		"inputH":  "input_h_default",
		"inoutK":  "inout_k_default",
	}), outs)

	// Step 3: read the resource we just created. The read should make the following changes to the inputs:
	// - "inoutC" should now be present in the input map. This is because it has a value in the state and the schema
	//   indicates that it may be an input property. We could probably avoid this by checking to see if the value in
	//   the new state matches the value in the olds state.
	// - "inoutD" should change from "inout_d_default" to "inout_d_read", and should no longer be present in the list
	//   of properties that were populated from defaults.
	resp, err = p.Read(context.Background(), &pulumirpc.ReadRequest{
		Id:         string(id),
		Urn:        string(urn),
		Properties: createResp.GetProperties(),
		Inputs:     checkResp.GetInputs(),
	})
	assert.NoError(t, err)

	outs, err = plugin.UnmarshalProperties(resp.GetProperties(), plugin.MarshalOptions{})
	assert.NoError(t, err)
	assert.Equal(t, resource.NewPropertyMapFromMap(map[string]interface{}{
		"id":     "MyID",
		"inputA": "input_a_create",
		"inoutC": "inout_c_create",
		"inoutD": "inout_d_read",
		"inputE": map[string]interface{}{
			"fieldA": "field_a_default",
		},
		"inputF":  "input_f_default",
		"outputG": "output_g_read",
		"inputH":  "input_h_default",
		"inputI":  "",
		"inoutJ":  "",
		"inoutK":  "",
	}), outs)

	ins, err = plugin.UnmarshalProperties(resp.GetInputs(), plugin.MarshalOptions{})
	assert.NoError(t, err)
	assert.Equal(t, resource.NewPropertyMapFromMap(map[string]interface{}{
		defaultsKey: []interface{}{"inputF", "inputH"},
		"inputA":    "input_a_create",
		"inoutD":    "inout_d_read",
		"inputE": map[string]interface{}{
			defaultsKey: []interface{}{"fieldA"},
			"fieldA":    "field_a_default",
		},
		"inputF": "input_f_default",
		"inputH": "Input_H_Default",
		"inoutK": "",
	}), ins)

	// Step 3a. delete the default annotations from the checked inputs and re-run the read. No default annotations
	// should be present in the result. This is the refresh-after-upgrade case.
	delete(checkedIns, defaultsKey)
	delete(checkedIns["inputE"].ObjectValue(), defaultsKey)
	checkedInsForRefresh, err := plugin.MarshalProperties(checkedIns, plugin.MarshalOptions{})
	assert.NoError(t, err)

	resp, err = p.Read(context.Background(), &pulumirpc.ReadRequest{
		Id:         string(id),
		Urn:        string(urn),
		Properties: createResp.GetProperties(),
		Inputs:     checkedInsForRefresh,
	})
	assert.NoError(t, err)

	outs, err = plugin.UnmarshalProperties(resp.GetProperties(), plugin.MarshalOptions{})
	assert.NoError(t, err)
	assert.Equal(t, resource.NewPropertyMapFromMap(map[string]interface{}{
		"id":     "MyID",
		"inputA": "input_a_create",
		"inoutC": "inout_c_create",
		"inoutD": "inout_d_read",
		"inputE": map[string]interface{}{
			"fieldA": "field_a_default",
		},
		"inputF":  "input_f_default",
		"outputG": "output_g_read",
		"inputH":  "input_h_default",
		"inputI":  "",
		"inoutJ":  "",
		"inoutK":  "",
	}), outs)

	ins, err = plugin.UnmarshalProperties(resp.GetInputs(), plugin.MarshalOptions{})
	assert.NoError(t, err)
	assert.Equal(t, resource.NewPropertyMapFromMap(map[string]interface{}{
		"inputA": "input_a_create",
		"inoutD": "inout_d_read",
		"inputE": map[string]interface{}{
			"fieldA": "field_a_default",
		},
		"inputF": "input_f_default",
		"inputH": "Input_H_Default",
		"inoutK": "",
	}), ins)
}

// This schema replicates the panic behavior of
// pulumi-terraform-bridge/pf/internal/schemashim.objectPseudoResource.
type volatileMap struct{ schema.SchemaMap }

func (s volatileMap) Get(key string) shim.Schema {
	v, ok := s.GetOk(key)
	if !ok {
		panic("Missing key: " + key)
	}
	return v
}

func TestRefreshExtractInputsFromOutputsMaxItemsOne(t *testing.T) {
	t.Parallel()

	ruleSetProps := func() resource.PropertyMap {
		return resource.NewPropertyMapFromMap(map[string]any{
			"rule": map[string]any{
				"action": []any{
					map[string]any{
						"overwritten": map[string]any{
							"from": 299,
							"to":   999,
						},
					},
				},
			},
		})
	}

	ruleSetSchema := func() shim.SchemaMap {
		blockList := func(elem schema.SchemaMap) shim.Schema {
			s := schema.Schema{
				Type: shim.TypeList,
				Elem: (&schema.Resource{
					Schema: volatileMap{elem},
				}).Shim(),
			}
			return s.Shim()
		}

		return schema.SchemaMap{
			"rule": blockList(schema.SchemaMap{
				"action": blockList(schema.SchemaMap{
					"some_effect": blockList(schema.SchemaMap{
						"from": (&schema.Schema{Type: shim.TypeInt}).Shim(),
						"to":   (&schema.Schema{Type: shim.TypeInt}).Shim(),
					}),
					"other": (&schema.Schema{Type: shim.TypeInt}).Shim(),
				}),
			}),
		}
	}

	ruleSetPs := func() map[string]*SchemaInfo {
		list := func(info *SchemaInfo) *SchemaInfo {
			return &SchemaInfo{
				Elem:        info,
				MaxItemsOne: BoolRef(false),
			}
		}
		maxItemsList := func(info *SchemaInfo) *SchemaInfo {
			l := list(info)
			l.MaxItemsOne = BoolRef(true)
			return l
		}

		field := func(name string, elem *SchemaInfo) *SchemaInfo {
			return &SchemaInfo{
				Fields: map[string]*SchemaInfo{
					name: elem,
				},
			}
		}

		return map[string]*SchemaInfo{
			"rule": maxItemsList(field("action", list(field("some_effect", &SchemaInfo{
				Name: "overwritten",
			})))),
		}
	}

	_, err := ExtractInputsFromOutputs(ruleSetProps(), ruleSetProps(),
		ruleSetSchema(), ruleSetPs(), true)
	assert.NoError(t, err)
}

func TestFailureReasonForMissingRequiredFields(t *testing.T) {
	// Define two required inputs
	tfProvider := makeTestTFProviderV1(
		map[string]*schemav1.Schema{
			"input_x": {Type: schemav1.TypeString, Required: true},
			"input_y": {Type: schemav1.TypeString, Required: true},
		},
		func(d *schemav1.ResourceData, meta interface{}) ([]*schemav1.ResourceData, error) {
			return []*schemav1.ResourceData{d}, nil
		})

	// Input Y has a default info pointing to a config key
	p := &Provider{
		tf: shimv1.NewProvider(tfProvider),
		resources: map[tokens.Type]Resource{
			"importableResource": {
				TF:     shimv1.NewResource(tfProvider.ResourcesMap["importable_resource"]),
				TFName: "importable_resource",
				Schema: &ResourceInfo{
					Tok: tokens.NewTypeToken("module", "importableResource"),
					Fields: map[string]*SchemaInfo{
						"input_y": {
							Default: &DefaultInfo{
								Config: "inputYConfig",
							},
						},
					},
				},
			},
		},
	}
	p.module = "test"

	urn := resource.NewURN("s", "pr", "pa", "importableResource", "n")

	// Pass no input values
	pulumiIns, err := plugin.MarshalProperties(
		resource.NewPropertyMapFromMap(map[string]interface{}{}), plugin.MarshalOptions{})
	assert.NoError(t, err)

	// Check the inputs
	checkResp, err := p.Check(context.Background(), &pulumirpc.CheckRequest{
		Urn:  string(urn),
		News: pulumiIns,
	})
	assert.NoError(t, err)

	// Expect two failures: one for each field
	failures := checkResp.Failures
	assert.Equal(t, 2, len(failures))

	x, y := failures[0].Reason, failures[1].Reason
	if strings.Contains(x, "inputY") {
		x, y = y, x
	}

	// Check that Y error reason has been amended with a hint about the config, while X reason is unaffected
	assert.Equal(t, "Missing required property 'inputX'", x)
	assert.Equal(t, "Missing required property 'inputY'. Either set it explicitly or configure it "+
		"with 'pulumi config set test:inputYConfig <value>'", y)
}

func TestAssetRoundtrip(t *testing.T) {
	tfProvider := makeTestTFProviderV1(
		map[string]*schemav1.Schema{
			"input_a": {Type: schemav1.TypeString, Required: true},
		},
		func(d *schemav1.ResourceData, meta interface{}) ([]*schemav1.ResourceData, error) {
			return []*schemav1.ResourceData{d}, nil
		})

	tfres := tfProvider.ResourcesMap["importable_resource"]
	tfres.Create = func(d *schemav1.ResourceData, meta interface{}) error {
		d.SetId("MyID")
		return nil
	}
	tfres.Update = func(d *schemav1.ResourceData, meta interface{}) error {
		return nil
	}

	p := &Provider{
		tf: shimv1.NewProvider(tfProvider),
		resources: map[tokens.Type]Resource{
			"importableResource": {
				TF:     shimv1.NewResource(tfProvider.ResourcesMap["importable_resource"]),
				TFName: "importable_resource",
				Schema: &ResourceInfo{
					Tok: tokens.NewTypeToken("module", "importableResource"),
					Fields: map[string]*SchemaInfo{
						"input_a": {
							Asset: &AssetTranslation{},
						},
					},
				},
			},
		},
	}

	urn := resource.NewURN("s", "pr", "pa", "importableResource", "n")

	asset, err := resource.NewTextAsset("foo")
	assert.NoError(t, err)

	// Step 1: create and check an input bag.
	pulumiIns, err := plugin.MarshalProperties(resource.NewPropertyMapFromMap(map[string]interface{}{
		"inputA": asset,
	}), plugin.MarshalOptions{})
	assert.NoError(t, err)
	checkResp, err := p.Check(context.Background(), &pulumirpc.CheckRequest{
		Urn:  string(urn),
		News: pulumiIns,
	})
	assert.NoError(t, err)

	// Step 2: create a resource using the checked input bag. The inputs should be smuggled along with the state.
	createResp, err := p.Create(context.Background(), &pulumirpc.CreateRequest{
		Urn:        string(urn),
		Properties: checkResp.GetInputs(),
	})
	assert.NoError(t, err)

	outs, err := plugin.UnmarshalProperties(createResp.GetProperties(), plugin.MarshalOptions{})
	assert.NoError(t, err)
	assert.True(t, resource.NewPropertyMapFromMap(map[string]interface{}{
		"id":     "MyID",
		"inputA": asset,
	}).DeepEquals(outs))

	// Step 3: update the resource we just created.
	asset, err = resource.NewTextAsset("bar")
	assert.NoError(t, err)

	pulumiIns, err = plugin.MarshalProperties(resource.NewPropertyMapFromMap(map[string]interface{}{
		"inputA": asset,
	}), plugin.MarshalOptions{})
	assert.NoError(t, err)
	checkResp, err = p.Check(context.Background(), &pulumirpc.CheckRequest{
		Urn:  string(urn),
		News: pulumiIns,
	})
	assert.NoError(t, err)

	// Step 2: create a resource using the checked input bag. The inputs should be smuggled along with the state.
	updateResp, err := p.Update(context.Background(), &pulumirpc.UpdateRequest{
		Id:   "MyID",
		Urn:  string(urn),
		Olds: createResp.GetProperties(),
		News: checkResp.GetInputs(),
	})
	assert.NoError(t, err)

	outs, err = plugin.UnmarshalProperties(updateResp.GetProperties(), plugin.MarshalOptions{})
	assert.NoError(t, err)
	assert.True(t, resource.NewPropertyMapFromMap(map[string]interface{}{
		"id":     "MyID",
		"inputA": asset,
	}).DeepEquals(outs))
}

func TestDeleteBeforeReplaceAutoname(t *testing.T) {
	tfProvider := makeTestTFProviderV1(
		map[string]*schemav1.Schema{
			"input_a": {Type: schemav1.TypeString, Required: true},
			"input_b": {Type: schemav1.TypeString, Required: true, ForceNew: true},
		},
		func(d *schemav1.ResourceData, meta interface{}) ([]*schemav1.ResourceData, error) {
			return []*schemav1.ResourceData{d}, nil
		})

	tfres := tfProvider.ResourcesMap["importable_resource"]
	tfres.Create = func(d *schemav1.ResourceData, meta interface{}) error {
		d.SetId("MyID")
		return nil
	}
	tfres.Update = func(d *schemav1.ResourceData, meta interface{}) error {
		return nil
	}

	p := &Provider{
		tf: shimv1.NewProvider(tfProvider),
		resources: map[tokens.Type]Resource{
			"importableResource": {
				TF:     shimv1.NewResource(tfProvider.ResourcesMap["importable_resource"]),
				TFName: "importable_resource",
				Schema: &ResourceInfo{
					Tok: tokens.NewTypeToken("module", "importableResource"),
					Fields: map[string]*SchemaInfo{
						"input_a": AutoName("inputA", 64, "-"),
					},
				},
			},
		},
	}

	urn := resource.NewURN("s", "pr", "pa", "importableResource", "myResource")

	// Step 1: create and check an input bag. This input bag will use an auto-name.
	pulumiIns, err := plugin.MarshalProperties(resource.NewPropertyMapFromMap(map[string]interface{}{
		"inputB": "foo",
	}), plugin.MarshalOptions{})
	assert.NoError(t, err)
	checkResp, err := p.Check(context.Background(), &pulumirpc.CheckRequest{
		Urn:  string(urn),
		News: pulumiIns,
	})
	assert.NoError(t, err)

	// Step 2: create a resource using the checked input bag. The inputs should be smuggled along with the state.
	createResp, err := p.Create(context.Background(), &pulumirpc.CreateRequest{
		Urn:        string(urn),
		Properties: checkResp.GetInputs(),
	})
	assert.NoError(t, err)

	// Step 3: make a new property bag that changes a force-new property and diff the resource. The result should not
	// be delete-before-create.
	pulumiIns, err = plugin.MarshalProperties(resource.NewPropertyMapFromMap(map[string]interface{}{
		"inputB": "bar",
	}), plugin.MarshalOptions{})
	assert.NoError(t, err)
	checkResp, err = p.Check(context.Background(), &pulumirpc.CheckRequest{
		Urn:  string(urn),
		Olds: createResp.GetProperties(),
		News: pulumiIns,
	})
	assert.NoError(t, err)

	diffResp, err := p.Diff(context.Background(), &pulumirpc.DiffRequest{
		Id:   "MyID",
		Urn:  string(urn),
		Olds: createResp.GetProperties(),
		News: checkResp.GetInputs(),
	})
	assert.NoError(t, err)

	assert.True(t, len(diffResp.GetReplaces()) > 0)
	assert.False(t, diffResp.GetDeleteBeforeReplace())

	// Step 4: make another property bag that sets a value for the name and changes a force-new property and then diff
	// the resource. The result should indicate delete-before-replace.
	pulumiIns, err = plugin.MarshalProperties(resource.NewPropertyMapFromMap(map[string]interface{}{
		"inputA": "myResource",
		"inputB": "bar",
	}), plugin.MarshalOptions{})
	assert.NoError(t, err)
	checkResp, err = p.Check(context.Background(), &pulumirpc.CheckRequest{
		Urn:  string(urn),
		Olds: createResp.GetProperties(),
		News: pulumiIns,
	})
	assert.NoError(t, err)

	diffResp, err = p.Diff(context.Background(), &pulumirpc.DiffRequest{
		Id:   "MyID",
		Urn:  string(urn),
		Olds: createResp.GetProperties(),
		News: checkResp.GetInputs(),
	})
	assert.NoError(t, err)

	assert.True(t, len(diffResp.GetReplaces()) > 0)
	assert.True(t, diffResp.GetDeleteBeforeReplace())

	// Step 5: delete the defaults list from the checked inputs and re-run the diff. The result should not indicate
	// delete-before-replace. This tests the back-compat scenario.
	checkedIns, err := plugin.UnmarshalProperties(checkResp.GetInputs(), plugin.MarshalOptions{})
	assert.NoError(t, err)
	delete(checkedIns, defaultsKey)
	marshaledIns, err := plugin.MarshalProperties(checkedIns, plugin.MarshalOptions{})
	assert.NoError(t, err)

	diffResp, err = p.Diff(context.Background(), &pulumirpc.DiffRequest{
		Id:   "MyID",
		Urn:  string(urn),
		Olds: createResp.GetProperties(),
		News: marshaledIns,
	})
	assert.NoError(t, err)

	assert.True(t, len(diffResp.GetReplaces()) > 0)
	assert.False(t, diffResp.GetDeleteBeforeReplace())

	// Step 6: delete the auto-name default from the schema and re-run the diff. The result should not indicate
	// delete-befer-replace.
	p.resources["importableResource"].Schema.Fields = nil

	pulumiIns, err = plugin.MarshalProperties(resource.NewPropertyMapFromMap(map[string]interface{}{
		"inputA": "myResource",
		"inputB": "bar",
	}), plugin.MarshalOptions{})
	assert.NoError(t, err)
	checkResp, err = p.Check(context.Background(), &pulumirpc.CheckRequest{
		Urn:  string(urn),
		Olds: createResp.GetProperties(),
		News: pulumiIns,
	})
	assert.NoError(t, err)

	diffResp, err = p.Diff(context.Background(), &pulumirpc.DiffRequest{
		Id:   "MyID",
		Urn:  string(urn),
		Olds: createResp.GetProperties(),
		News: checkResp.GetInputs(),
	})
	assert.NoError(t, err)

	assert.True(t, len(diffResp.GetReplaces()) > 0)
	assert.False(t, diffResp.GetDeleteBeforeReplace())
}

func TestExtractDefaultSecretInputs(t *testing.T) {
	tfProvider := makeTestTFProviderV1(
		map[string]*schemav1.Schema{
			"input_a": {Type: schemav1.TypeString, Sensitive: true, Required: true},
			"input_b": {Type: schemav1.TypeString, Sensitive: true, Optional: true},
			"input_c": {Type: schemav1.TypeString, Sensitive: true, Optional: true, Computed: true},
			"input_d": {Type: schemav1.TypeString, Sensitive: true, Optional: true, Default: "input_d_default"},
		},
		func(d *schemav1.ResourceData, meta interface{}) ([]*schemav1.ResourceData, error) {
			return []*schemav1.ResourceData{d}, nil
		})

	set := func(d *schemav1.ResourceData, key string, value interface{}) {
		contract.IgnoreError(d.Set(key, value))
	}

	tfres := tfProvider.ResourcesMap["importable_resource"]
	tfres.Read = func(d *schemav1.ResourceData, meta interface{}) error {
		_, ok := d.GetOk(defaultsKey)
		assert.False(t, ok)

		set(d, "input_a", "input_a_read")
		set(d, "input_c", "input_c_read")
		set(d, "input_d", "input_d_default")
		return nil
	}

	p := &Provider{
		tf: shimv1.NewProvider(tfProvider),
		resources: map[tokens.Type]Resource{
			"importableResource": {
				TF:     shimv1.NewResource(tfProvider.ResourcesMap["importable_resource"]),
				TFName: "importable_resource",
				Schema: &ResourceInfo{
					Tok: tokens.NewTypeToken("module", "importableResource"),
				},
			},
		},
	}

	urn := resource.NewURN("s", "pr", "pa", "importableResource", "n")
	id := resource.ID("MyID")

	resp, err := p.Read(context.Background(), &pulumirpc.ReadRequest{
		Id:  string(id),
		Urn: string(urn),
	})
	assert.NoError(t, err)

	outs, err := plugin.UnmarshalProperties(resp.GetProperties(), plugin.MarshalOptions{})
	assert.NoError(t, err)
	assert.Equal(t, resource.NewPropertyMapFromMap(map[string]interface{}{
		"id":     "MyID",
		"inputA": "input_a_read",
		"inputC": "input_c_read",
		"inputD": "input_d_default",
	}), outs)

	ins, err := plugin.UnmarshalProperties(resp.GetInputs(), plugin.MarshalOptions{})
	assert.NoError(t, err)
	expected := resource.NewPropertyMapFromMap(map[string]interface{}{
		defaultsKey: []interface{}{},
		"inputA":    "input_a_read",
		"inputC":    "input_c_read",
	})
	assert.Equal(t, expected, ins)
}

func TestExtractDefaultIntegerInputs(t *testing.T) {
	// Terrafrom differentiates between Int and Float. Pulumi doesn't so we need to handle both cases for
	// default values.
	tfProvider := makeTestTFProviderV1(
		map[string]*schemav1.Schema{
			"input_a": {Type: schemav1.TypeInt, Optional: true},
			"input_b": {Type: schemav1.TypeFloat, Optional: true},
			"input_c": {Type: schemav1.TypeInt, Optional: true, Default: -1},
			"input_d": {Type: schemav1.TypeFloat, Optional: true, Default: -1},
		},
		func(d *schemav1.ResourceData, meta interface{}) ([]*schemav1.ResourceData, error) {
			return []*schemav1.ResourceData{d}, nil
		})

	set := func(d *schemav1.ResourceData, key string, value interface{}) {
		contract.IgnoreError(d.Set(key, value))
	}

	tfres := tfProvider.ResourcesMap["importable_resource"]
	tfres.Read = func(d *schemav1.ResourceData, meta interface{}) error {
		_, ok := d.GetOk(defaultsKey)
		assert.False(t, ok)

		set(d, "input_a", 0)
		set(d, "input_b", 0)
		set(d, "input_c", -1)
		set(d, "input_d", -1)
		return nil
	}

	p := &Provider{
		tf: shimv1.NewProvider(tfProvider),
		resources: map[tokens.Type]Resource{
			"importableResource": {
				TF:     shimv1.NewResource(tfProvider.ResourcesMap["importable_resource"]),
				TFName: "importable_resource",
				Schema: &ResourceInfo{
					Tok: tokens.NewTypeToken("module", "importableResource"),
				},
			},
		},
	}

	urn := resource.NewURN("s", "pr", "pa", "importableResource", "n")
	id := resource.ID("MyID")

	resp, err := p.Read(context.Background(), &pulumirpc.ReadRequest{
		Id:  string(id),
		Urn: string(urn),
	})
	assert.NoError(t, err)

	outs, err := plugin.UnmarshalProperties(resp.GetProperties(), plugin.MarshalOptions{})
	assert.NoError(t, err)
	assert.Equal(t, resource.NewPropertyMapFromMap(map[string]interface{}{
		"id":     "MyID",
		"inputA": 0,
		"inputB": 0,
		"inputC": -1,
		"inputD": -1,
	}), outs)

	ins, err := plugin.UnmarshalProperties(resp.GetInputs(), plugin.MarshalOptions{})
	assert.NoError(t, err)
	expected := resource.NewPropertyMapFromMap(map[string]interface{}{
		defaultsKey: []interface{}{},
	})
	assert.Equal(t, expected, ins)
}

func TestExtractSchemaInputsNestedMaxItemsOne(t *testing.T) {

	provider := func(info *ResourceInfo) *Provider {
		if info == nil {
			info = &ResourceInfo{}
		}
		if info.Tok == "" {
			info.Tok = tokens.NewTypeToken("module", "importableResource")
		}

		listOfObj := func(maxItems int) *schemav2.Schema {
			return &schemav2.Schema{
				Type: schemav2.TypeList, Optional: true,
				MaxItems: maxItems,
				Elem: &schemav2.Resource{
					Schema: map[string]*schemav2.Schema{
						"field1": {
							Optional: true,
							Type:     schemav2.TypeBool,
						},
						"list_scalar": {
							Type: schemav2.TypeList, Optional: true,
							MaxItems: 1,
							Elem: &schemav2.Schema{
								Type:     schemav2.TypeInt,
								Optional: true,
							},
						},
					},
				},
			}
		}

		tfProvider := makeTestTFProviderV2(
			map[string]*schemav2.Schema{
				"list_object":          listOfObj(0),
				"list_object_maxitems": listOfObj(1),
			},
			func(
				_ context.Context, d *schemav2.ResourceData, meta interface{},
			) ([]*schemav2.ResourceData, error) {
				return []*schemav2.ResourceData{d}, nil
			})

		set := func(d *schemav2.ResourceData, key string, value interface{}) {
			contract.AssertNoErrorf(d.Set(key, value),
				"failed to set %s", key)
		}

		tfres := tfProvider.ResourcesMap["importable_resource"]
		tfres.ReadContext = func(
			_ context.Context, d *schemav2.ResourceData, meta interface{},
		) diag.Diagnostics {
			_, ok := d.GetOk(defaultsKey)
			assert.False(t, ok)

			set(d, "list_object", []any{
				map[string]any{
					"field1":      false,
					"list_scalar": []any{1}},
			})
			set(d, "list_object_maxitems", []any{
				map[string]any{
					"field1":      true,
					"list_scalar": []any{2}},
			})
			return nil
		}

		return &Provider{
			tf: shimv2.NewProvider(tfProvider),
			resources: map[tokens.Type]Resource{
				"importableResource": {
					TF:     shimv2.NewResource(tfProvider.ResourcesMap["importable_resource"]),
					TFName: "importable_resource",
					Schema: info,
				},
			},
		}
	}

	tests := []struct {
		name string

		info map[string]*SchemaInfo

		expectedOutputs resource.PropertyMap
		expectedInputs  resource.PropertyMap
	}{
		{
			name: "no overrides",
			expectedOutputs: resource.PropertyMap{
				"id": resource.NewProperty("MyID"),
				"listObjectMaxitems": resource.NewProperty(resource.PropertyMap{
					"field1":     resource.NewProperty(true),
					"listScalar": resource.NewProperty(2.0),
				}),
				"listObjects": resource.NewProperty([]resource.PropertyValue{
					resource.NewProperty(resource.PropertyMap{
						"field1":     resource.NewProperty(false),
						"listScalar": resource.NewProperty(1.0),
					}),
				}),
			},
			expectedInputs: resource.PropertyMap{
				"__defaults": resource.NewProperty([]resource.PropertyValue{}),
				"listObjectMaxitems": resource.NewProperty(resource.PropertyMap{
					"__defaults": resource.NewProperty([]resource.PropertyValue{}),
					"field1":     resource.NewProperty(true),
					"listScalar": resource.NewProperty(2.0),
				}),
				"listObjects": resource.NewProperty([]resource.PropertyValue{
					resource.NewProperty(resource.PropertyMap{
						"__defaults": resource.NewProperty([]resource.PropertyValue{}),
						"field1":     resource.NewProperty(false),
						"listScalar": resource.NewProperty(1.0),
					}),
				}),
			},
		},
		{
			name: "override `MaxItems: 1` on lists",
			info: map[string]*SchemaInfo{
				"list_object": {
					MaxItemsOne: True(),
					Elem: &SchemaInfo{
						Fields: map[string]*SchemaInfo{
							"list_scalar": {MaxItemsOne: False()},
						},
					},
				},
				"list_object_maxitems": {
					MaxItemsOne: False(),
					Elem: &SchemaInfo{
						Fields: map[string]*SchemaInfo{
							"list_scalar": {Name: "overwritten"},
						},
					},
				},
			},
			expectedOutputs: resource.PropertyMap{
				"id": resource.NewProperty("MyID"),
				"listObject": resource.NewProperty(resource.PropertyMap{
					"field1": resource.NewProperty(false),
					"listScalars": resource.NewProperty([]resource.PropertyValue{
						resource.NewProperty(1.0),
					}),
				}),
				"listObjectMaxitems": resource.NewProperty([]resource.PropertyValue{
					resource.NewProperty(resource.PropertyMap{
						"field1":      resource.NewProperty(true),
						"overwritten": resource.NewProperty(2.0),
					}),
				}),
			},
			expectedInputs: resource.PropertyMap{
				"__defaults": resource.NewProperty([]resource.PropertyValue{}),
				"listObject": resource.NewProperty(resource.PropertyMap{
					"__defaults": resource.NewProperty([]resource.PropertyValue{}),
					"listScalars": resource.NewProperty([]resource.PropertyValue{
						resource.NewProperty(1.0),
					}),
				}),
				"listObjectMaxitems": resource.NewProperty([]resource.PropertyValue{
					resource.NewProperty(resource.PropertyMap{
						"__defaults":  resource.NewProperty([]resource.PropertyValue{}),
						"field1":      resource.NewProperty(true),
						"overwritten": resource.NewProperty(2.0),
					}),
				}),
			},
		},
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			p := provider(&ResourceInfo{Fields: tt.info})
			urn := resource.NewURN("s", "pr", "pa", "importableResource", "n")
			id := resource.ID("MyID")

			resp, err := p.Read(context.Background(), &pulumirpc.ReadRequest{
				Id:  string(id),
				Urn: string(urn),
			})
			assert.NoError(t, err)

			outs, err := plugin.UnmarshalProperties(resp.GetProperties(), plugin.MarshalOptions{})
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedOutputs, outs, "outputs")

			ins, err := plugin.UnmarshalProperties(resp.GetInputs(), plugin.MarshalOptions{})
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedInputs, ins, "inputs")
		})
	}
}

func TestOutputNumberTypes(t *testing.T) {
	ctx := context.Background()
	tfs := shimv1.NewSchemaMap(map[string]*schemav1.Schema{
		"aaa": {Type: schemav1.TypeInt},
		"bbb": {Type: schemav1.TypeInt},
		"ccc": {Type: schemav1.TypeInt},
		"ddd": {Type: schemav1.TypeInt},
		"eee": {Type: schemav1.TypeInt},
		"fff": {Type: schemav1.TypeFloat},
		"ggg": {Type: schemav1.TypeFloat},
	})
	inputs := map[string]interface{}{
		"aaa": int8(50),
		"bbb": int16(50),
		"ccc": int32(50),
		"ddd": int64(50),
		"eee": int(50),
		"fff": float32(50),
		"ggg": float64(50),
	}
	outputs := MakeTerraformOutputs(
		ctx,
		shimv1.NewProvider(testTFProvider),
		inputs,
		tfs,
		map[string]*SchemaInfo{},
		AssetTable{},
		false,
		true,
	)
	assert.Equal(t, resource.PropertyMap{
		"aaa": resource.NewNumberProperty(50),
		"bbb": resource.NewNumberProperty(50),
		"ccc": resource.NewNumberProperty(50),
		"ddd": resource.NewNumberProperty(50),
		"eee": resource.NewNumberProperty(50),
		"fff": resource.NewNumberProperty(50),
		"ggg": resource.NewNumberProperty(50),
	}, outputs)
}

func TestMakeTerraformInputsOnMapNestedObjects(t *testing.T) {
	r := &schemav2.Resource{
		Schema: map[string]*schemav2.Schema{
			"map_prop": {
				Type:     schemav2.TypeMap,
				Optional: true,
				Elem: &schemav2.Schema{
					Type:     schemav2.TypeList,
					Optional: true,
					Elem: &schemav2.Resource{
						Schema: map[string]*schemav2.Schema{
							"x_prop": {
								Optional: true,
								Type:     schemav2.TypeString,
							},
						},
					},
				},
			},
		},
	}

	shimmedR := shimv2.NewResource(r)
	type testCase struct {
		name   string
		ps     map[string]*SchemaInfo
		news   resource.PropertyMap
		olds   resource.PropertyMap
		expect interface{}
	}

	testCases := []testCase{
		{
			name: "translates x_prop",
			news: resource.PropertyMap{
				"mapProp": resource.NewObjectProperty(resource.PropertyMap{
					"elem1": resource.NewArrayProperty([]resource.PropertyValue{
						resource.NewObjectProperty(resource.PropertyMap{
							"xProp": resource.NewStringProperty("xPropValue"),
						}),
					}),
				}),
			},
			expect: map[string]interface{}{
				"__defaults": []interface{}{},
				"map_prop": map[string]interface{}{
					"elem1": []interface{}{
						map[string]interface{}{
							"__defaults": []interface{}{},
							"x_prop":     "xPropValue",
						},
					},
				},
			},
		},
		{
			name: "respects x_prop renames",
			news: resource.PropertyMap{
				"mapProp": resource.NewObjectProperty(resource.PropertyMap{
					"elem1": resource.NewArrayProperty([]resource.PropertyValue{
						resource.NewObjectProperty(resource.PropertyMap{
							"x": resource.NewStringProperty("xPropValue"),
						}),
					}),
				}),
			},
			ps: map[string]*SchemaInfo{
				"map_prop": {
					Elem: &SchemaInfo{
						Elem: &SchemaInfo{
							Fields: map[string]*SchemaInfo{
								"x_prop": {
									Name: "x",
								},
							},
						},
					},
				},
			},
			expect: map[string]interface{}{
				"__defaults": []interface{}{},
				"map_prop": map[string]interface{}{
					"elem1": []interface{}{
						map[string]interface{}{
							"__defaults": []interface{}{},
							"x_prop":     "xPropValue",
						},
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			i, _, err := makeTerraformInputsForConfig(tc.olds, tc.news, shimmedR.Schema(), tc.ps)
			require.NoError(t, err)
			require.Equal(t, tc.expect, i)
		})
	}
}

func TestRegress940(t *testing.T) {
	r := &schemav2.Resource{
		Schema: map[string]*schemav2.Schema{
			"build": {
				Type:     schemav2.TypeSet,
				Optional: true,
				MaxItems: 1,
				Elem: &schemav2.Resource{
					Schema: map[string]*schemav2.Schema{
						"build_arg": {
							Type:     schemav2.TypeMap,
							Optional: true,
							Elem: &schemav2.Schema{
								Type: schemav2.TypeString,
							},
						},
					},
				},
			},
		},
	}
	shimmedR := shimv2.NewResource(r)

	var olds, news resource.PropertyMap

	news = resource.PropertyMap{
		"build": resource.NewObjectProperty(resource.PropertyMap{
			"buildArg": resource.NewObjectProperty(resource.PropertyMap{
				"foo":    resource.NewStringProperty("bar"),
				"":       resource.NewStringProperty("baz"),
				"fooBar": resource.NewStringProperty("foo_bar_value"),
			}),
		}),
	}

	result, _, err := makeTerraformInputsForConfig(olds, news, shimmedR.Schema(), map[string]*SchemaInfo{})

	t.Run("no error with empty keys", func(t *testing.T) {
		assert.NoError(t, err)
	})

	t.Run("map keys are not renamed to Pulumi-style names", func(t *testing.T) {
		// buildArg becomes build_arg  because it is an object property
		// in contrast, fooBar stays the same because it is a map key
		// note also that build becomes array-wrapped because of MaxItems=1 flattening
		assert.Equal(t, "foo_bar_value", result["build"].([]any)[0].(map[string]any)["build_arg"].(map[string]any)["fooBar"])
	})
}

// TestTerraformInputs verifies that we translate Pulumi inputs into Terraform inputs.
func Test_makeTerraformInputsNoDefaults(t *testing.T) {
	type testCase struct {
		testCaseName string
		schemaMap    map[string]*schema.Schema
		schemaInfos  map[string]*SchemaInfo
		propMap      resource.PropertyMap
		expect       autogold.Value
	}

	testCases := []testCase{
		{
			testCaseName: "bool_without_schema",
			propMap: resource.PropertyMap{
				"boolPropertyValue": resource.NewBoolProperty(false),
			},
			expect: autogold.Expect(map[string]interface{}{"bool_property_value": false}),
		},
		{
			testCaseName: "number_without_schema",
			propMap: resource.PropertyMap{
				"numberPropertyValue": resource.NewNumberProperty(42),
			},
			expect: autogold.Expect(map[string]interface{}{"number_property_value": 42}),
		},
		{
			testCaseName: "float",
			schemaMap: map[string]*schema.Schema{
				"float_property_value": {
					Type:     shim.TypeFloat,
					Optional: true,
				},
			},
			propMap: resource.PropertyMap{
				"floatPropertyValue": resource.NewNumberProperty(99.6767932),
			},
			expect: autogold.Expect(map[string]interface{}{"float_property_value": 99.6767932}),
		},
		{
			testCaseName: "string_without_schema_with_rename",
			schemaInfos: map[string]*SchemaInfo{
				"string_property_value": {Name: "stringo"},
			},
			propMap: resource.PropertyMap{
				"stringo": resource.NewStringProperty("ognirts"),
			},
			expect: autogold.Expect(map[string]interface{}{"string_property_value": "ognirts"}),
		},
		{
			testCaseName: "array_without_schema",
			propMap: resource.NewPropertyMapFromMap(map[string]interface{}{
				"arrayPropertyValue": []interface{}{"an array"},
			}),
			expect: autogold.Expect(map[string]interface{}{"array_property_value": []interface{}{"an array"}}),
		},
		{
			testCaseName: "array_unknown_value",
			schemaMap: map[string]*schema.Schema{
				"unknown_array_value": {
					Type:     shim.TypeList,
					Optional: true,
					Elem: (&schema.Schema{
						Type: shim.TypeInt,
					}).Shim(),
				},
			},
			propMap: resource.NewPropertyMapFromMap(map[string]interface{}{
				// The string property inside Computed is irrelevant.
				"unknownArrayValue": resource.Computed{Element: resource.NewStringProperty("")},
			}),
			// NOTE: is this the behavior we would want here? Why is the result [unk] instead of unk?
			//nolint:lll
			expect: autogold.Expect(map[string]interface{}{"unknown_array_value": []interface{}{"74D93920-ED26-11E3-AC10-0800200C9A66"}}),
		},
		{
			testCaseName: "unknown_object_value",
			schemaMap: map[string]*schema.Schema{
				"object_value": {
					Type:     shim.TypeList,
					Optional: true,
					MaxItems: 1,
					Elem: (&schema.Resource{
						Schema: schemaMap(map[string]*schema.Schema{
							"required_property": {
								Type:     shim.TypeString,
								Required: true,
							},
							"conflicts_a": {
								Optional:      true,
								Type:          shim.TypeString,
								ConflictsWith: []string{"object_value.conflicts_b"},
							},
							"conflicts_b": {
								Optional:      true,
								Type:          shim.TypeString,
								ConflictsWith: []string{"object_value.conflicts_a"},
							},
						}),
					}).Shim(),
				},
			},
			propMap: resource.NewPropertyMapFromMap(map[string]interface{}{
				// The string property inside Computed is irrelevant.
				"objectValue": resource.Computed{Element: resource.NewStringProperty("")},
			}),
			// NOTE: is this what we want? Should this be [unk] or unk?
			//nolint:lll
			expect: autogold.Expect(map[string]interface{}{"object_value": []interface{}{"74D93920-ED26-11E3-AC10-0800200C9A66"}}),
		},
		{
			testCaseName: "object",
			schemaMap: map[string]*schema.Schema{
				"object_property_value": {
					Optional: true,
					Type:     shim.TypeList,
					MaxItems: 1,
					Elem: (&schema.Resource{
						Schema: schemaMap(map[string]*schema.Schema{
							"property_a": {
								Type:     shim.TypeString,
								Required: true,
							},
							"property_b": {
								Type:     shim.TypeBool,
								Optional: true,
							},
						}),
					}).Shim(),
				},
			},
			propMap: resource.NewPropertyMapFromMap(map[string]interface{}{
				"objectPropertyValue": map[string]interface{}{
					"propertyA": "a",
					"propertyB": true,
				},
			}),
			//nolint:lll
			expect: autogold.Expect(map[string]interface{}{"object_property_value": []interface{}{map[string]interface{}{"property_a": "a", "property_b": true}}}),
		},
		{
			testCaseName: "map_of_untyped_element",
			schemaMap: map[string]*schema.Schema{
				"map_property_value": {
					Type:     shim.TypeMap,
					Optional: true,
				},
			},
			propMap: resource.NewPropertyMapFromMap(map[string]interface{}{
				"propertyA": "a",
				"propertyB": true,
				"propertyC": map[string]interface{}{
					"nestedPropertyA": true,
				},
			}),
			//nolint:lll
			expect: autogold.Expect(map[string]interface{}{"property_a": "a", "property_b": true, "property_c": map[string]interface{}{"nested_property_a": true}}),
		},
		{
			testCaseName: "list_nested_block",
			schemaMap: map[string]*schema.Schema{
				"nested_resource": {
					Optional: true,
					Type:     shim.TypeList,
					MaxItems: 2,
					// Embed a `*schema.Resource` to validate that type directed walk of the schema
					// successfully walks inside Resources as well as Schemas.
					Elem: (&schema.Resource{
						Schema: schemaMap(map[string]*schema.Schema{
							"configuration": {
								Type:     shim.TypeMap,
								Optional: true,
							},
						}),
					}).Shim(),
				},
			},
			propMap: resource.NewPropertyMapFromMap(map[string]interface{}{
				"nestedResources": []map[string]interface{}{{
					"configuration": map[string]interface{}{
						"configurationValue": true,
					},
				}},
			}),
			//nolint:lll
			expect: autogold.Expect(map[string]interface{}{"nested_resource": []interface{}{map[string]interface{}{"configuration": map[string]interface{}{"configurationValue": true}}}}),
		},
		{
			testCaseName: "optional_config",
			schemaMap: map[string]*schema.Schema{
				"optional_config": {
					Optional: true,
					Type:     shim.TypeList,
					MaxItems: 1,
					Elem: (&schema.Resource{
						Schema: schemaMap(map[string]*schema.Schema{
							"some_value": {
								Optional: true,
								Type:     shim.TypeBool,
							},
							"some_other_value": {
								Optional: true,
								Type:     shim.TypeString,
							},
						}),
					}).Shim(),
				},
			},
			propMap: resource.NewPropertyMapFromMap(map[string]interface{}{
				"optionalConfig": map[string]interface{}{
					"someValue":      true,
					"someOtherValue": "a value",
				},
			}),
			//nolint:lll
			expect: autogold.Expect(map[string]interface{}{"optional_config": []interface{}{map[string]interface{}{"some_other_value": "a value", "some_value": true}}}),
		},
		{
			testCaseName: "optional_config_with_overrides",
			schemaMap: map[string]*schema.Schema{
				"optional_config_other": {
					Type:     shim.TypeList,
					Optional: true,
					Elem: (&schema.Resource{
						Schema: schemaMap(map[string]*schema.Schema{
							"some_value": {
								Type:     shim.TypeBool,
								Optional: true,
							},
							"some_other_value": {
								Optional: true,
								Type:     shim.TypeString,
							},
						}),
					}).Shim(),
				},
			},
			schemaInfos: map[string]*SchemaInfo{
				"optional_config_other": {
					Name:        "optionalConfigOther2",
					MaxItemsOne: boolPointer(true),
				},
			},
			propMap: resource.NewPropertyMapFromMap(map[string]interface{}{
				"optionalConfigOther2": map[string]interface{}{
					"someValue":      true,
					"someOtherValue": "a value",
				},
			}),
			//nolint:lll
			expect: autogold.Expect(map[string]interface{}{"optional_config_other": []interface{}{map[string]interface{}{"some_other_value": "a value", "some_value": true}}}),
		},
		{
			testCaseName: "map_of_int_lists",
			schemaMap: map[string]*schema.Schema{
				"m": {
					Type:     shim.TypeMap,
					Optional: true,
					Elem: (&schema.Schema{
						Type: shim.TypeList,
						Elem: (&schema.Schema{
							Type: shim.TypeInt,
						}).Shim(),
					}).Shim(),
				},
			},
			propMap: resource.NewPropertyMapFromMap(map[string]interface{}{
				"m": map[string]interface{}{
					"ones": []interface{}{1, 10, 100},
					"twos": []interface{}{2, 20, 200},
				},
			}),
			//nolint:lll
			expect: autogold.Expect(map[string]interface{}{"m": map[string]interface{}{"ones": []interface{}{1, 10, 100}, "twos": []interface{}{2, 20, 200}}}),
		},
		{
			testCaseName: "array_with_nested_optional_computed_arrays",
			schemaMap: map[string]*schema.Schema{
				"array_with_nested_optional_computed_arrays": {
					Type:     shim.TypeList,
					Optional: true,
					Computed: true,
					MaxItems: 1,
					Elem: (&schema.Resource{
						Schema: schemaMap(map[string]*schema.Schema{
							"nested_value": {
								Type:     shim.TypeList,
								MaxItems: 1,
								Optional: true,
								Elem: (&schema.Resource{
									Schema: schemaMap(map[string]*schema.Schema{
										"nested_inner_value": {
											Type:     shim.TypeBool,
											Required: true,
										},
									}),
								}).Shim(),
							},
						}),
					}).Shim(),
				},
			},
			propMap: resource.NewPropertyMapFromMap(map[string]interface{}{
				"arrayWithNestedOptionalComputedArrays": []interface{}{
					map[string]interface{}{},
				},
			}),
			expect: autogold.Expect(map[string]interface{}{"array_with_nested_optional_computed_arrays": []interface{}{}}),
			schemaInfos: map[string]*SchemaInfo{
				"array_with_nested_optional_computed_arrays": {
					SuppressEmptyMapElements: boolPointer(true),
				},
			},
		},
		{
			testCaseName: "nil_without_schema",
			propMap: resource.NewPropertyMapFromMap(map[string]interface{}{
				"nilPropertyValue": nil,
			}),
			expect: autogold.Expect(map[string]interface{}{"nil_property_value": nil}),
		},
		// {
		// 	testCaseName: "???",
		// 	schemaMap:    map[string]*schema.Schema{},
		// 	propMap:      resource.NewPropertyMapFromMap(map[string]interface{}{}),
		// 	expect:       autogold.Expect(nil),
		// },
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.testCaseName, func(t *testing.T) {
			results := map[string]any{}

			for _, f := range factories {
				f := f
				sm := f.NewSchemaMap(tc.schemaMap)
				err := sm.Validate()
				require.NoErrorf(t, err, "Invalid test case schema, please fix the testCase")

				result, assetTable, err := makeTerraformInputsNoDefaults(
					nil, /*olds*/
					tc.propMap,
					sm,
					tc.schemaInfos,
				)
				require.NoError(t, err)
				require.Empty(t, assetTable)
				results[f.SDKVersion()] = result
			}

			tc.expect.Equal(t, results[factories[0].SDKVersion()])
			for k, v := range results {
				require.Equalf(t, results[factories[0].SDKVersion()], v, k)
			}
		})
	}
}
