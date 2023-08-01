// Copyright 2016-2018, Pulumi Corporation.
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

	structpb "github.com/golang/protobuf/ptypes/struct"
	schemav1 "github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	schemav2 "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-terraform-bridge/v3/internal/testprovider"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/schema"
	shimv1 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v1"
	shimv2 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v2"
)

func makeTerraformInputs(olds, news resource.PropertyMap,
	tfs shim.SchemaMap, ps map[string]*SchemaInfo) (map[string]interface{}, AssetTable, error) {

	ctx := &conversionContext{Assets: AssetTable{}}
	inputs, err := ctx.MakeTerraformInputs(olds, news, tfs, ps, false, resource.PropertyPath{})
	if err != nil {
		return nil, nil, err
	}
	return inputs, ctx.Assets, err
}

func makeTerraformInputsWithDefaults(olds, news resource.PropertyMap,
	tfs shim.SchemaMap, ps map[string]*SchemaInfo) (map[string]interface{}, AssetTable, error) {

	ctx := &conversionContext{
		Assets:        AssetTable{},
		ApplyDefaults: true,
	}
	inputs, err := ctx.MakeTerraformInputs(olds, news, tfs, ps, false, resource.PropertyPath{})
	if err != nil {
		return nil, nil, err
	}
	return inputs, ctx.Assets, err
}

func makeTerraformInput(v resource.PropertyValue, tfs shim.Schema, ps *SchemaInfo) (interface{}, error) {
	ctx := &conversionContext{}
	return ctx.MakeTerraformInput("v", resource.PropertyValue{}, v, tfs, ps, false,
		resource.PropertyPath{})
}

// TestTerraformInputs verifies that we translate Pulumi inputs into Terraform inputs.
func TestTerraformInputs(t *testing.T) {
	for _, f := range factories {
		t.Run(f.SDKVersion(), func(t *testing.T) {
			result, _, err := makeTerraformInputs(
				nil, /*olds*/
				resource.NewPropertyMapFromMap(map[string]interface{}{
					"boolPropertyValue":   false,
					"numberPropertyValue": 42,
					"floatPropertyValue":  99.6767932,
					"stringo":             "ognirts",
					"arrayPropertyValue":  []interface{}{"an array"},
					"unknownArrayValue":   resource.Computed{Element: resource.NewStringProperty("")},
					"unknownArrayValue2":  resource.Computed{Element: resource.NewStringProperty("")},
					"objectPropertyValue": map[string]interface{}{
						"propertyA": "a",
						"propertyB": true,
					},
					"mapPropertyValue": map[string]interface{}{
						"propertyA": "a",
						"propertyB": true,
						"propertyC": map[string]interface{}{
							"nestedPropertyA": true,
						},
					},
					"nestedResources": []map[string]interface{}{{
						"configuration": map[string]interface{}{
							"configurationValue": true,
						},
					}},
					"optionalConfig": map[string]interface{}{
						"someValue":      true,
						"someOtherValue": "a value",
					},
					"optionalConfigOther": map[string]interface{}{
						"someValue":      true,
						"someOtherValue": "a value",
					},
					"mapWithResourceElem": map[string]interface{}{
						"someValue": "a value",
					},
					"arrayWithNestedOptionalComputedArrays": []interface{}{
						map[string]interface{}{},
					},
				}),
				f.NewSchemaMap(map[string]*schema.Schema{
					// Type mapPropertyValue as a map so that keys aren't mangled in the usual way.
					"float_property_value": {Type: shim.TypeFloat},
					"unknown_array_value":  {Type: shim.TypeList},
					"unknown_array_value2": {
						Type:     shim.TypeList,
						MinItems: 1,
						Elem: (&schema.Resource{
							Schema: schemaMap(map[string]*schema.Schema{
								"required_property": {Type: shim.TypeString, Required: true},
								"conflicts_a":       {Type: shim.TypeString, ConflictsWith: []string{"conflicts_b"}},
								"conflicts_b":       {Type: shim.TypeString, ConflictsWith: []string{"conflicts_a"}},
							}),
						}).Shim(),
					},
					"map_property_value": {Type: shim.TypeMap},
					"nested_resource": {
						Type:     shim.TypeList,
						MaxItems: 2,
						// Embed a `*schema.Resource` to validate that type directed
						// walk of the schema successfully walks inside Resources as well
						// as Schemas.
						Elem: (&schema.Resource{
							Schema: schemaMap(map[string]*schema.Schema{
								"configuration": {Type: shim.TypeMap},
							}),
						}).Shim(),
					},
					"optional_config": {
						Type:     shim.TypeList,
						MaxItems: 1,
						Elem: (&schema.Resource{
							Schema: schemaMap(map[string]*schema.Schema{
								"some_value":       {Type: shim.TypeBool},
								"some_other_value": {Type: shim.TypeString},
							}),
						}).Shim(),
					},
					"optional_config_other": {
						Type: shim.TypeList,
						Elem: (&schema.Resource{
							Schema: schemaMap(map[string]*schema.Schema{
								"some_value":       {Type: shim.TypeBool},
								"some_other_value": {Type: shim.TypeString},
							}),
						}).Shim(),
					},
					"map_with_resource_elem": {
						Type: shim.TypeMap,
						Elem: (&schema.Resource{
							Schema: schemaMap(map[string]*schema.Schema{
								"some_value": {Type: shim.TypeString},
							}),
						}).Shim(),
					},
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
				}),
				map[string]*SchemaInfo{
					// Reverse map string_property_value to the stringo property.
					"string_property_value": {
						Name: "stringo",
					},
					"optional_config_other": {
						Name:        "optionalConfigOther",
						MaxItemsOne: boolPointer(true),
					},
					"array_with_nested_optional_computed_arrays": {
						SuppressEmptyMapElements: boolPointer(true),
					},
				})
			assert.Nil(t, err)

			var nilInterfaceSlice []interface{}
			assert.Equal(t, map[string]interface{}{
				"bool_property_value":   false,
				"number_property_value": 42,
				"float_property_value":  99.6767932,
				"string_property_value": "ognirts",
				"array_property_value":  []interface{}{"an array"},
				"unknown_array_value":   []interface{}{TerraformUnknownVariableValue},
				"unknown_array_value2": []interface{}{
					map[string]interface{}{
						"required_property": TerraformUnknownVariableValue,
					},
				},
				"object_property_value": map[string]interface{}{
					"property_a": "a",
					"property_b": true,
				},
				"map_property_value": map[string]interface{}{
					"propertyA": "a",
					"propertyB": true,
					"propertyC": map[string]interface{}{
						"nestedPropertyA": true,
					},
				},
				"nested_resource": []interface{}{
					map[string]interface{}{
						"configuration": map[string]interface{}{
							"configurationValue": true,
						},
					},
				},
				"optional_config": []interface{}{
					map[string]interface{}{
						"some_value":       true,
						"some_other_value": "a value",
					},
				},
				"optional_config_other": []interface{}{
					map[string]interface{}{
						"some_value":       true,
						"some_other_value": "a value",
					},
				},
				"map_with_resource_elem": []interface{}{
					map[string]interface{}{
						"some_value": "a value",
					},
				},
				"array_with_nested_optional_computed_arrays": nilInterfaceSlice,
			}, result)

			_, _, err = makeTerraformInputs(
				nil, /*olds*/
				resource.NewPropertyMapFromMap(map[string]interface{}{
					"nilPropertyValue": nil,
				}),
				nil, /* tfs */
				nil, /* ps */
			)
			assert.NoError(t, err)
		})
	}
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
				})}),
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
			result, _, err := makeTerraformInputs(
				olds, news, tfs, nil /* ps */)
			require.NoError(t, err)
			assert.Equal(t, map[string]interface{}{
				"element": tt.tfValue,
			}, result)
		})
	}
}

type MyString string

// TestTerraformOutputsWithSecretsSupported verifies that we translate Terraform outputs into Pulumi outputs and
// treating sensitive outputs as secrets
func TestTerraformOutputsWithSecretsSupported(t *testing.T) {
	for _, f := range factories {
		t.Run(f.SDKVersion(), func(t *testing.T) {
			result := MakeTerraformOutputs(
				f.NewTestProvider(),
				map[string]interface{}{
					"nil_property_value":       nil,
					"bool_property_value":      false,
					"number_property_value":    42,
					"float_property_value":     99.6767932,
					"string_property_value":    "ognirts",
					"my_string_property_value": MyString("ognirts"),
					"array_property_value":     []interface{}{"an array"},
					"object_property_value": map[string]interface{}{
						"property_a": "a",
						"property_b": true,
					},
					"map_property_value": map[string]interface{}{
						"propertyA": "a",
						"propertyB": true,
						"propertyC": map[string]interface{}{
							"nestedPropertyA": true,
						},
					},
					"nested_resource": []interface{}{
						map[string]interface{}{
							"configuration": map[string]interface{}{
								"configurationValue": true,
							},
						},
					},
					"optional_config": []interface{}{
						map[string]interface{}{
							"some_value":       true,
							"some_other_value": "a value",
						},
					},
					"optional_config_other": []interface{}{
						map[string]interface{}{
							"some_value":       true,
							"some_other_value": "a value",
						},
					},
					"secret_value": "MyPassword",
					"nested_secret_value": []interface{}{
						map[string]interface{}{
							"secret_value": "MyPassword",
						},
					},
				},
				f.NewSchemaMap(map[string]*schema.Schema{
					// Type mapPropertyValue as a map so that keys aren't mangled in the usual way.
					"float_property_value":     {Type: shim.TypeFloat},
					"my_string_property_value": {Type: shim.TypeString},
					"map_property_value":       {Type: shim.TypeMap},
					"nested_resource": {
						Type:     shim.TypeList,
						MaxItems: 2,
						// Embed a `*schema.Resource` to validate that type directed
						// walk of the schema successfully walks inside Resources as well
						// as Schemas.
						Elem: (&schema.Resource{
							Schema: schemaMap(map[string]*schema.Schema{
								"configuration": {Type: shim.TypeMap},
							}),
						}).Shim(),
					},
					"optional_config": {
						Type:     shim.TypeList,
						MaxItems: 1,
						Elem: (&schema.Resource{
							Schema: schemaMap(map[string]*schema.Schema{
								"some_value":       {Type: shim.TypeBool},
								"some_other_value": {Type: shim.TypeString},
							}),
						}).Shim(),
					},
					"optional_config_other": {
						Type: shim.TypeList,
						Elem: (&schema.Resource{
							Schema: schemaMap(map[string]*schema.Schema{
								"some_value":       {Type: shim.TypeBool},
								"some_other_value": {Type: shim.TypeString},
							}),
						}).Shim(),
					},
					"secret_value": {
						Type:      shim.TypeString,
						Optional:  true,
						Sensitive: true,
					},
					"nested_secret_value": {
						Type:     shim.TypeList,
						MaxItems: 1,
						Elem: (&schema.Resource{
							Schema: schemaMap(map[string]*schema.Schema{
								"secret_value": {
									Type:      shim.TypeString,
									Sensitive: true,
								},
							}),
						}).Shim(),
					},
				}),
				map[string]*SchemaInfo{
					// Reverse map string_property_value to the stringo property.
					"string_property_value": {
						Name: "stringo",
					},
					"optional_config_other": {
						Name:        "optionalConfigOther",
						MaxItemsOne: boolPointer(true),
					},
				},
				nil,   /* assets */
				false, /* useRawNames */
				true,  /* supportsSecrets */
			)
			assert.Equal(t, resource.NewPropertyMapFromMap(map[string]interface{}{
				"nilPropertyValue":      nil,
				"boolPropertyValue":     false,
				"numberPropertyValue":   42,
				"floatPropertyValue":    99.6767932,
				"stringo":               "ognirts",
				"myStringPropertyValue": "ognirts",
				"arrayPropertyValue":    []interface{}{"an array"},
				"objectPropertyValue": map[string]interface{}{
					"propertyA": "a",
					"propertyB": true,
				},
				"mapPropertyValue": map[string]interface{}{
					"propertyA": "a",
					"propertyB": true,
					"propertyC": map[string]interface{}{
						"nestedPropertyA": true,
					},
				},
				"nestedResources": []map[string]interface{}{{
					"configuration": map[string]interface{}{
						"configurationValue": true,
					},
				}},
				"optionalConfig": map[string]interface{}{
					"someValue":      true,
					"someOtherValue": "a value",
				},
				"optionalConfigOther": map[string]interface{}{
					"someValue":      true,
					"someOtherValue": "a value",
				},
				"secretValue": &resource.Secret{
					Element: resource.PropertyValue{
						V: "MyPassword",
					},
				},
				"nestedSecretValue": map[string]interface{}{
					"secretValue": &resource.Secret{
						Element: resource.PropertyValue{
							V: "MyPassword",
						},
					},
				},
			}), result)
		})
	}
}

// TestTerraformOutputsWithSecretsUnsupported verifies that we translate Terraform outputs into Pulumi outputs without
// treating sensitive outputs as secrets
func TestTerraformOutputsWithSecretsUnsupported(t *testing.T) {
	for _, f := range factories {
		t.Run(f.SDKVersion(), func(t *testing.T) {
			result := MakeTerraformOutputs(
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

			const resName = "example_resource"
			res := prov.ResourcesMap().Get(resName)

			state := f.NewInstanceState("0")
			read, err := prov.Refresh(resName, state)
			assert.NoError(t, err)
			assert.NotNil(t, read)

			props, err := MakeTerraformResult(prov, read, res.Schema(), nil, nil, true)
			assert.NoError(t, err)
			assert.NotNil(t, props)

			state, err = MakeTerraformState(Resource{TF: res, Schema: &ResourceInfo{}}, state.ID(), props)
			assert.NoError(t, err)
			assert.NotNil(t, state)

			assert.Equal(t, strconv.Itoa(res.SchemaVersion()), state.Meta()["schema_version"])

			read2, err := prov.Refresh(resName, state)
			assert.NoError(t, err)
			assert.NotNil(t, read2)
			assert.Equal(t, read, read2)

			// Delete the resource's meta-property and ensure that we re-populate its schema version.
			delete(props, metaKey)

			state, err = MakeTerraformState(Resource{TF: res, Schema: &ResourceInfo{}}, state.ID(), props)
			assert.NoError(t, err)
			assert.NotNil(t, state)

			assert.Equal(t, strconv.Itoa(res.SchemaVersion()), state.Meta()["schema_version"])

			// Remove the resource's meta-attributes and ensure that we do not include them in the result.
			ok := clearMeta(read2)
			assert.True(t, ok)
			props, err = MakeTerraformResult(prov, read2, res.Schema(), nil, nil, true)
			assert.NoError(t, err)
			assert.NotNil(t, props)
			assert.NotContains(t, props, metaKey)

			// Ensure that timeouts are populated and preserved.
			ok = clearID(state)
			assert.True(t, ok)
			cfg := prov.NewResourceConfig(map[string]interface{}{})
			diff, err := prov.Diff(resName, state, cfg)
			assert.NoError(t, err)

			// To populate default timeouts, we take the timeouts from the resource schema and insert them into the diff
			timeouts, err := res.DecodeTimeouts(cfg)
			assert.NoError(t, err)
			err = diff.EncodeTimeouts(timeouts)
			assert.NoError(t, err)

			assert.NoError(t, err)
			create, err := prov.Apply(resName, state, diff)
			assert.NoError(t, err)

			props, err = MakeTerraformResult(prov, create, res.Schema(), nil, nil, true)
			assert.NoError(t, err)
			assert.NotNil(t, props)

			state, err = MakeTerraformState(Resource{TF: res, Schema: &ResourceInfo{}}, state.ID(), props)
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

			const resName = "second_resource"
			res := prov.ResourcesMap().Get(resName)

			state := f.NewInstanceState("0")
			read, err := prov.Refresh(resName, state)
			assert.NoError(t, err)
			assert.NotNil(t, read)

			props, err := MakeTerraformResult(prov, read, res.Schema(), nil, nil, true)
			assert.NoError(t, err)
			assert.NotNil(t, props)

			state, err = MakeTerraformState(Resource{TF: res, Schema: &ResourceInfo{}}, state.ID(), props)
			assert.NoError(t, err)
			assert.NotNil(t, state)

			assert.Equal(t, strconv.Itoa(res.SchemaVersion()), state.Meta()["schema_version"])

			read2, err := prov.Refresh(resName, state)
			assert.NoError(t, err)
			assert.NotNil(t, read2)
			assert.Equal(t, read, read2)

			// Delete the resource's meta-property and ensure that we re-populate its schema version.
			delete(props, metaKey)

			state, err = MakeTerraformState(Resource{TF: res, Schema: &ResourceInfo{}}, state.ID(), props)
			assert.NoError(t, err)
			assert.NotNil(t, state)

			assert.Equal(t, strconv.Itoa(res.SchemaVersion()), state.Meta()["schema_version"])

			// Remove the resource's meta-attributes and ensure that we do not include them in the result.
			ok := clearMeta(read2)
			assert.True(t, ok)
			props, err = MakeTerraformResult(prov, read2, res.Schema(), nil, nil, true)
			assert.NoError(t, err)
			assert.NotNil(t, props)
			assert.NotContains(t, props, metaKey)

			// Ensure that timeouts are populated and preserved.
			ok = clearID(state)
			assert.True(t, ok)
			cfg := prov.NewResourceConfig(map[string]interface{}{})
			diff, err := prov.Diff(resName, state, cfg)
			assert.NoError(t, err)

			// To populate default timeouts, we take the timeouts from the resource schema and insert them into the diff
			resourceTimeouts, err := res.DecodeTimeouts(cfg)
			assert.NoError(t, err)
			err = diff.EncodeTimeouts(resourceTimeouts)
			assert.NoError(t, err)

			diff.SetTimeout(300, schemav1.TimeoutCreate)

			assert.NoError(t, err)
			create, err := prov.Apply(resName, state, diff)
			assert.NoError(t, err)

			props, err = MakeTerraformResult(prov, create, res.Schema(), nil, nil, true)
			assert.NoError(t, err)
			assert.NotNil(t, props)

			state, err = MakeTerraformState(Resource{TF: res, Schema: &ResourceInfo{}}, state.ID(), props)
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

			const resName = "example_resource"
			res := prov.ResourcesMap().Get("example_resource")

			state := f.NewInstanceState("0")
			read, err := prov.Refresh(resName, state)
			assert.NoError(t, err)
			assert.NotNil(t, read)

			props, err := MakeTerraformResult(prov, read, res.Schema(), nil, nil, true)
			assert.NoError(t, err)
			assert.NotNil(t, props)

			state, err = MakeTerraformState(Resource{TF: res, Schema: &ResourceInfo{}}, state.ID(), props)
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
			//     - ll2: old input "OLL", TF default "TFL", no input => "TL2"
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

			x1ofN := []string{"x1of1", "x1of2", "x1of3"}
			tfs := f.NewSchemaMap(map[string]*schema.Schema{
				"xyz": {Type: shim.TypeString, ExactlyOneOf: []string{"xyz", "abc"}},
				"abc": {Type: shim.TypeString, Default: "ABC", ExactlyOneOf: []string{"xyz", "abc"}},
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
				"nnn": {Type: shim.TypeString, ConflictsWith: []string{"nn2"}, Default: "NNN"},
				"nn2": {Type: shim.TypeString, ConflictsWith: []string{"nnn"}, Default: "NN2"},
				"ooo": {Type: shim.TypeString, ConflictsWith: []string{"oo2"}, Default: "OOO"},
				"oo2": {Type: shim.TypeString, ConflictsWith: []string{"ooo"}},
				"oo3": {Type: shim.TypeString, ConflictsWith: []string{"nonexisting"}},
				"sss": {Type: shim.TypeString, Removed: "removed"},
				"ttt": {Type: shim.TypeString, Removed: "removed", Default: "TFD"},
				"uuu": {Type: shim.TypeString},
				"vvv": {Type: shim.TypeInt},
				"www": {Type: shim.TypeString, Deprecated: "deprecated", Required: true},
				"xxx": {Type: shim.TypeString, Deprecated: "deprecated", Optional: true},
				"yyy": {Type: shim.TypeString, Default: "TLY", Deprecated: "deprecated", Optional: true},
				"zzz": {Type: shim.TypeString},

				// Test exactly one of behavior with default funcs:
				"x1of1": {Type: shim.TypeString, ExactlyOneOf: x1ofN, DefaultFunc: fixedDefault("x1of1-value")},
				"x1of2": {Type: shim.TypeString, ExactlyOneOf: x1ofN, DefaultFunc: fixedDefault(nil)},
				"x1of3": {Type: shim.TypeString, ExactlyOneOf: x1ofN, DefaultFunc: fixedDefault(nil)},
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
				"oo2": {Default: &DefaultInfo{Value: "PO2"}},
				"sss": {Default: &DefaultInfo{Value: "PSS"}, Removed: true},
				"uuu": {Default: &DefaultInfo{Value: "PSU", EnvVars: []string{"PTFU", "PTFU2"}}},
				"vvv": {Default: &DefaultInfo{Value: 42, EnvVars: []string{"PTFV", "PTFV2"}}},
				"www": {Default: &DefaultInfo{Value: "PSW"}},
				"zzz": {Asset: &AssetTranslation{Kind: FileAsset}},
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
			inputs, assets, err := makeTerraformInputsWithDefaults(olds, props, tfs, ps)
			assert.NoError(t, err)
			outputs := MakeTerraformOutputs(f.NewTestProvider(), inputs, tfs, ps, assets, false, true)

			// sort the defaults list before the equality test below.
			sortDefaultsList(outputs)

			assert.Equal(t, resource.NewPropertyMapFromMap(map[string]interface{}{
				defaultsKey: []interface{}{
					"abc", "cc2", "ccc", "ee2", "eee", "ggg", "iii", "ll2", "lll", "mm2", "mmm", "oo2", "uuu", "vvv", "www",
					"x1of1",
				},
				"abc": "ABC",
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
				"oo2": "PO2",
				"uuu": "PSU",
				"vvv": 1337,
				"www": "OLW",
				// xzy is NOT set as it's either that or abc
				"zzz": asset,

				// x1of1 is set as it UNIQUELY has a default value in its ExactlyOneOf set (x1of1, x1of2, x1of3)
				"x1of1": "x1of1-value",
			}), outputs)

			// Now delete the defaults list from the olds and re-run. This will affect the values for "ll2" and "mm2", which
			// will be pulled from the old inputs instead of regenerated.
			delete(olds, defaultsKey)
			inputs, assets, err = makeTerraformInputsWithDefaults(olds, props, tfs, ps)
			assert.NoError(t, err)
			outputs = MakeTerraformOutputs(f.NewTestProvider(), inputs, tfs, ps, assets, false, true)

			//sort the defaults list before the equality test below.
			sortDefaultsList(outputs)
			assert.Equal(t, resource.NewPropertyMapFromMap(map[string]interface{}{
				defaultsKey: []interface{}{
					"abc", "cc2", "ccc", "ee2", "eee", "ggg", "iii", "ll2", "lll", "mm2", "mmm", "oo2", "uuu", "vvv", "www",
					"x1of1",
				},
				"abc": "ABC",
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
				// nnn/nn2 are NOT set as they conflict with each other
				// ooo is NOT set as it conflicts with oo2
				"oo2": "PO2",
				"uuu": "PSU",
				"vvv": 1337,
				"www": "OLW",
				// xyz is NOT set as it has ExactlyOneOf with abc
				"zzz": asset,

				// x1of1 is set as it UNIQUELY has a default value in its ExactlyOneOf set (x1of1, x1of2, x1of3)
				"x1of1": "x1of1-value",
			}), outputs)
		})
	}
}

func TestComputedAsset(t *testing.T) {
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
	inputs, assets, err := makeTerraformInputs(olds, props, tfs, ps)
	assert.NoError(t, err)
	outputs := MakeTerraformOutputs(shimv1.NewProvider(testTFProvider), inputs, tfs, ps, assets, false, true)
	assert.Equal(t, resource.PropertyMap{
		"zzz": resource.PropertyValue{V: resource.Computed{Element: resource.PropertyValue{V: ""}}},
	}, outputs)
}

func TestInvalidAsset(t *testing.T) {
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
	inputs, assets, err := makeTerraformInputs(olds, props, tfs, ps)
	assert.NoError(t, err)
	outputs := MakeTerraformOutputs(shimv1.NewProvider(testTFProvider), inputs, tfs, ps, assets, false, true)
	assert.Equal(t, resource.PropertyMap{
		"zzz": resource.NewStringProperty("invalid"),
	}, outputs)
}

func TestOverridingTFSchema(t *testing.T) {
	result := MakeTerraformOutputs(
		shimv1.NewProvider(testTFProvider),
		map[string]interface{}{
			"pulumi_override_tf_string_to_boolean":    MyString("true"),
			"pulumi_override_tf_string_to_bool":       MyString("true"),
			"pulumi_empty_tf_override":                MyString("true"),
			"pulumi_override_tf_string_to_int":        MyString("1"),
			"pulumi_override_tf_string_to_integer":    MyString("1"),
			"tf_empty_string_to_pulumi_bool_override": MyString(""),
		},
		shimv1.NewSchemaMap(map[string]*schemav1.Schema{
			"pulumi_override_tf_string_to_boolean":    {Type: schemav1.TypeString},
			"pulumi_override_tf_string_to_bool":       {Type: schemav1.TypeString},
			"pulumi_empty_tf_override":                {Type: schemav1.TypeString},
			"pulumi_override_tf_string_to_int":        {Type: schemav1.TypeString},
			"pulumi_override_tf_string_to_integer":    {Type: schemav1.TypeString},
			"tf_empty_string_to_pulumi_bool_override": {Type: schemav1.TypeString},
		}),
		map[string]*SchemaInfo{
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
		},
		nil,   /* assets */
		false, /*useRawNames*/
		true,
	)
	assert.Equal(t, resource.NewPropertyMapFromMap(map[string]interface{}{
		"pulumiOverrideTfStringToBoolean":   true,
		"pulumiOverrideTfStringToBool":      true,
		"pulumiEmptyTfOverride":             "true",
		"pulumiOverrideTfStringToInt":       1,
		"pulumiOverrideTfStringToInteger":   1,
		"tfEmptyStringToPulumiBoolOverride": nil,
	}), result)
}

func TestArchiveAsAsset(t *testing.T) {
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
	inputs, assets, err := makeTerraformInputs(olds, props, tfs, ps)
	assert.NoError(t, err)
	outputs := MakeTerraformOutputs(shimv1.NewProvider(testTFProvider), inputs, tfs, ps, assets, false, true)
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
	tfProvider := makeTestTFProvider(
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
	tfProvider := makeTestTFProvider(
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
	tfProvider := makeTestTFProvider(
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
	tfProvider := makeTestTFProvider(
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
	tfProvider := makeTestTFProvider(
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

	tfProvider := makeTestTFProvider(map[string]*schemav1.Schema{},
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

func makeTestTFProvider(schemaMap map[string]*schemav1.Schema, importer schemav1.StateFunc) *schemav1.Provider {
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

func TestStringOutputsWithSchema(t *testing.T) {
	result := MakeTerraformOutputs(
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
	tfProvider := makeTestTFProvider(
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
	tfProvider := makeTestTFProvider(
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
	tfProvider := makeTestTFProvider(
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
	tfProvider := makeTestTFProvider(
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
	tfProvider := makeTestTFProvider(
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
	// Terrafrom differentiates between Int and Float. Pulumi doesn't so we need to handle both cases for default values.
	tfProvider := makeTestTFProvider(
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
