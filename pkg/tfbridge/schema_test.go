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
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/terraform"
	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/resource/plugin"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/contract"
	pulumirpc "github.com/pulumi/pulumi/sdk/proto/go"
)

// TestTerraformInputs verifies that we translate Pulumi inputs into Terraform inputs.
func TestTerraformInputs(t *testing.T) {
	result, err := MakeTerraformInputs(
		nil, /*res*/
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
		map[string]*schema.Schema{
			// Type mapPropertyValue as a map so that keys aren't mangled in the usual way.
			"float_property_value": {Type: schema.TypeFloat},
			"unknown_array_value":  {Type: schema.TypeList},
			"unknown_array_value2": {
				Type:     schema.TypeList,
				MinItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"required_property": {Type: schema.TypeString, Required: true},
						"conflicts_a":       {Type: schema.TypeString, ConflictsWith: []string{"conflicts_b"}},
						"conflicts_b":       {Type: schema.TypeString, ConflictsWith: []string{"conflicts_a"}},
					},
				},
			},
			"map_property_value": {Type: schema.TypeMap},
			"nested_resource": {
				Type:     schema.TypeList,
				MaxItems: 2,
				// Embed a `*schema.Resource` to validate that type directed
				// walk of the schema successfully walks inside Resources as well
				// as Schemas.
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"configuration": {Type: schema.TypeMap},
					},
				},
			},
			"optional_config": {
				Type:     schema.TypeList,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"some_value":       {Type: schema.TypeBool},
						"some_other_value": {Type: schema.TypeString},
					},
				},
			},
			"optional_config_other": {
				Type: schema.TypeList,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"some_value":       {Type: schema.TypeBool},
						"some_other_value": {Type: schema.TypeString},
					},
				},
			},
			"map_with_resource_elem": {
				Type: schema.TypeMap,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"some_value": {Type: schema.TypeString},
					},
				},
			},
			"array_with_nested_optional_computed_arrays": {
				Type:     schema.TypeList,
				Optional: true,
				Computed: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"nested_value": {
							Type:     schema.TypeList,
							MaxItems: 1,
							Optional: true,
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"nested_inner_value": {
										Type:     schema.TypeBool,
										Required: true,
									},
								},
							},
						},
					},
				},
			},
		},
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
		},
		nil,   /* assets */
		nil,   /* config */
		false, /*defaults*/
		false, /*useRawNames*/
	)
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

	_, err = MakeTerraformInputs(
		nil, /*res*/
		nil, /*olds*/
		resource.NewPropertyMapFromMap(map[string]interface{}{
			"nilPropertyValue": nil,
		}),
		nil,   /* tfs */
		nil,   /* ps */
		nil,   /* assets */
		nil,   /* config */
		false, /*defaults*/
		false, /*useRawNames*/
	)
	assert.NoError(t, err)
}

type MyString string

// TestTerraformOutputsWithSecretsSupported verifies that we translate Terraform outputs into Pulumi outputs and
// treating sensitive outputs as secrets
func TestTerraformOutputsWithSecretsSupported(t *testing.T) {
	result := MakeTerraformOutputs(
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
		map[string]*schema.Schema{
			// Type mapPropertyValue as a map so that keys aren't mangled in the usual way.
			"float_property_value":     {Type: schema.TypeFloat},
			"my_string_property_value": {Type: schema.TypeString},
			"map_property_value":       {Type: schema.TypeMap},
			"nested_resource": {
				Type:     schema.TypeList,
				MaxItems: 2,
				// Embed a `*schema.Resource` to validate that type directed
				// walk of the schema successfully walks inside Resources as well
				// as Schemas.
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"configuration": {Type: schema.TypeMap},
					},
				},
			},
			"optional_config": {
				Type:     schema.TypeList,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"some_value":       {Type: schema.TypeBool},
						"some_other_value": {Type: schema.TypeString},
					},
				},
			},
			"optional_config_other": {
				Type: schema.TypeList,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"some_value":       {Type: schema.TypeBool},
						"some_other_value": {Type: schema.TypeString},
					},
				},
			},
			"secret_value": {
				Type:      schema.TypeString,
				Optional:  true,
				Sensitive: true,
			},
			"nested_secret_value": {
				Type:     schema.TypeList,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"secret_value": {
							Type:      schema.TypeString,
							Sensitive: true,
						},
					},
				},
			},
		},
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
		false, /*useRawNames*/
		true,
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
}

// TestTerraformOutputsWithSecretsUnsupported verifies that we translate Terraform outputs into Pulumi outputs without
// treating sensitive outputs as secrets
func TestTerraformOutputsWithSecretsUnsupported(t *testing.T) {
	result := MakeTerraformOutputs(
		map[string]interface{}{
			"secret_value": "MyPassword",
		},
		map[string]*schema.Schema{
			"secret_value": {
				Type:      schema.TypeString,
				Optional:  true,
				Sensitive: true,
			},
		},
		map[string]*SchemaInfo{},
		nil,   /* assets */
		false, /*useRawNames*/
		false,
	)
	assert.Equal(t, resource.NewPropertyMapFromMap(map[string]interface{}{
		"secretValue": "MyPassword",
	}), result)
}

func TestTerraformAttributes(t *testing.T) {
	result, err := MakeTerraformAttributesFromInputs(
		map[string]interface{}{
			"nil_property_value":    nil,
			"bool_property_value":   false,
			"number_property_value": 42,
			"float_property_value":  99.6767932,
			"string_property_value": "ognirts",
			"array_property_value":  []interface{}{"an array"},
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
			"nested_resources": []interface{}{
				map[string]interface{}{
					"configuration": map[string]interface{}{
						"configurationValue": true,
					},
				},
			},
			"set_property_value":            []interface{}{"set member 1", "set member 2"},
			"string_with_bad_interpolation": "some ${interpolated:value} with syntax errors",
			"removed_property_value":        "a removed property",
		},
		map[string]*schema.Schema{
			"nil_property_value":    {Type: schema.TypeMap},
			"bool_property_value":   {Type: schema.TypeBool},
			"number_property_value": {Type: schema.TypeInt},
			"float_property_value":  {Type: schema.TypeFloat},
			"string_property_value": {Type: schema.TypeString},
			"array_property_value": {
				Type: schema.TypeList,
				Elem: &schema.Schema{Type: schema.TypeString},
			},
			"object_property_value": {Type: schema.TypeMap},
			"map_property_value":    {Type: schema.TypeMap},
			"nested_resources": {
				Type:     schema.TypeList,
				MaxItems: 1,
				// Embed a `*schema.Resource` to validate that type directed
				// walk of the schema successfully walks inside Resources as well
				// as Schemas.
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"configuration": {Type: schema.TypeMap},
					},
				},
			},
			"set_property_value": {
				Type: schema.TypeSet,
				Elem: &schema.Schema{Type: schema.TypeString},
			},
			"string_with_bad_interpolation": {Type: schema.TypeString},
			"removed_property_value": {
				Type:    schema.TypeString,
				Removed: "Removed in the Terraform provider",
			},
		})

	assert.NoError(t, err)
	assert.Equal(t, result, map[string]string{
		"array_property_value.#":                              "1",
		"array_property_value.0":                              "an array",
		"bool_property_value":                                 "false",
		"float_property_value":                                "99.6767932",
		"map_property_value.%":                                "3",
		"map_property_value.propertyA":                        "a",
		"map_property_value.propertyB":                        "true",
		"map_property_value.propertyC.%":                      "1",
		"map_property_value.propertyC.nestedPropertyA":        "true",
		"nested_resources.#":                                  "1",
		"nested_resources.0.%":                                "1",
		"nested_resources.0.configuration.%":                  "1",
		"nested_resources.0.configuration.configurationValue": "true",
		"number_property_value":                               "42",
		"object_property_value.%":                             "2",
		"object_property_value.property_a":                    "a",
		"object_property_value.property_b":                    "true",
		"set_property_value.#":                                "2",
		"set_property_value.3618983862":                       "set member 2",
		"set_property_value.4237827189":                       "set member 1",
		"string_property_value":                               "ognirts",
		"string_with_bad_interpolation":                       "some ${interpolated:value} with syntax errors",
		"removed_property_value":                              "a removed property",
	})

	// MapFieldWriter has issues with values of TypeMap. Build a schema without such values s.t. we can test
	// MakeTerraformAttributes against the output of MapFieldWriter.
	sharedSchema := map[string]*schema.Schema{
		"bool_property_value":   {Type: schema.TypeBool},
		"number_property_value": {Type: schema.TypeInt},
		"float_property_value":  {Type: schema.TypeFloat},
		"string_property_value": {Type: schema.TypeString},
		"array_property_value": {
			Type: schema.TypeList,
			Elem: &schema.Schema{Type: schema.TypeString},
		},
		"nested_resource_value": {
			Type:     schema.TypeList,
			MaxItems: 1,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"nested_set_property": {
						Type: schema.TypeSet,
						Elem: &schema.Schema{Type: schema.TypeString},
					},
					"nested_string_property": {Type: schema.TypeString},
				},
			},
		},
		"set_property_value": {
			Type: schema.TypeSet,
			Elem: &schema.Schema{Type: schema.TypeString},
		},
		"string_with_bad_interpolation": {Type: schema.TypeString},
	}
	sharedInputs := map[string]interface{}{
		"bool_property_value":   false,
		"number_property_value": 42,
		"float_property_value":  99.6767932,
		"string_property_value": "ognirts",
		"array_property_value":  []interface{}{"an array"},
		"nested_resource_value": map[string]interface{}{
			"nested_set_property":    []interface{}{"nested set member"},
			"nested_string_property": "value",
		},
		"set_property_value":            []interface{}{"set member 1", "set member 2"},
		"string_with_bad_interpolation": "some ${interpolated:value} with syntax errors",
	}

	// Build a TF attribute map using schema.MapFieldWriter.
	cfg, err := MakeTerraformConfigFromInputs(sharedInputs)

	assert.NoError(t, err)
	reader := &schema.ConfigFieldReader{Config: cfg, Schema: sharedSchema}
	writer := &schema.MapFieldWriter{Schema: sharedSchema}
	for k := range sharedInputs {
		f, ferr := reader.ReadField([]string{k})
		assert.NoError(t, ferr)

		err = writer.WriteField([]string{k}, f.Value)
		assert.NoError(t, err)
	}
	expected := writer.Map()

	// Build the same using MakeTerraformAttributesFromInputs.
	result, err = MakeTerraformAttributesFromInputs(sharedInputs, sharedSchema)
	assert.NoError(t, err)
	assert.Equal(t, expected, result)
}

// Test that meta-properties are correctly produced.
func TestMetaProperties(t *testing.T) {
	const resName = "example_resource"
	res := testTFProvider.ResourcesMap["example_resource"]

	info := &terraform.InstanceInfo{Type: resName}
	state := &terraform.InstanceState{ID: "0", Attributes: map[string]string{}, Meta: map[string]interface{}{}}
	read, err := testTFProvider.Refresh(info, state)
	assert.NoError(t, err)
	assert.NotNil(t, read)

	props, err := MakeTerraformResult(read, res.Schema, nil, nil, true)
	assert.NoError(t, err)
	assert.NotNil(t, props)

	attrs, meta, err := MakeTerraformAttributes(res, props, res.Schema, nil, nil, false)
	assert.NoError(t, err)
	assert.NotNil(t, attrs)
	assert.NotNil(t, meta)

	assert.Equal(t, strconv.Itoa(res.SchemaVersion), meta["schema_version"])

	state.Attributes, state.Meta = attrs, meta
	read2, err := testTFProvider.Refresh(info, state)
	assert.NoError(t, err)
	assert.NotNil(t, read2)
	assert.Equal(t, read, read2)

	// Delete the resource's meta-property and ensure that we re-populate its schema version.
	delete(props, metaKey)

	attrs, meta, err = MakeTerraformAttributes(res, props, res.Schema, nil, nil, false)
	assert.NoError(t, err)
	assert.NotNil(t, attrs)
	assert.NotNil(t, meta)

	assert.Equal(t, strconv.Itoa(res.SchemaVersion), meta["schema_version"])

	// Remove the resource's meta-attributes and ensure that we do not include them in the result.
	read2.Meta = map[string]interface{}{}
	props, err = MakeTerraformResult(read2, res.Schema, nil, nil, true)
	assert.NoError(t, err)
	assert.NotNil(t, props)
	assert.NotContains(t, props, metaKey)

	// Ensure that timeouts are populated and preserved.
	state.ID = ""
	cfg := terraform.NewResourceConfigRaw(map[string]interface{}{})
	diff, err := testTFProvider.Diff(info, state, cfg)

	assert.NoError(t, err)
	create, err := testTFProvider.Apply(info, state, diff)
	assert.NoError(t, err)

	props, err = MakeTerraformResult(create, res.Schema, nil, nil, true)
	assert.NoError(t, err)
	assert.NotNil(t, props)

	attrs, meta, err = MakeTerraformAttributes(res, props, res.Schema, nil, nil, false)
	assert.NoError(t, err)
	assert.NotNil(t, attrs)
	assert.NotNil(t, meta)

	assert.Contains(t, meta, schema.TimeoutKey)
}

func TestInjectingCustomTimeouts(t *testing.T) {
	const resName = "second_resource"
	res := testTFProvider.ResourcesMap["second_resource"]

	info := &terraform.InstanceInfo{Type: resName}
	state := &terraform.InstanceState{ID: "0", Attributes: map[string]string{}, Meta: map[string]interface{}{}}
	read, err := testTFProvider.Refresh(info, state)
	assert.NoError(t, err)
	assert.NotNil(t, read)

	props, err := MakeTerraformResult(read, res.Schema, nil, nil, true)
	assert.NoError(t, err)
	assert.NotNil(t, props)

	attrs, meta, err := MakeTerraformAttributes(res, props, res.Schema, nil, nil, false)
	assert.NoError(t, err)
	assert.NotNil(t, attrs)
	assert.NotNil(t, meta)

	assert.Equal(t, strconv.Itoa(res.SchemaVersion), meta["schema_version"])

	state.Attributes, state.Meta = attrs, meta
	read2, err := testTFProvider.Refresh(info, state)
	assert.NoError(t, err)
	assert.NotNil(t, read2)
	assert.Equal(t, read, read2)

	// Delete the resource's meta-property and ensure that we re-populate its schema version.
	delete(props, metaKey)

	attrs, meta, err = MakeTerraformAttributes(res, props, res.Schema, nil, nil, false)
	assert.NoError(t, err)
	assert.NotNil(t, attrs)
	assert.NotNil(t, meta)

	assert.Equal(t, strconv.Itoa(res.SchemaVersion), meta["schema_version"])

	// Remove the resource's meta-attributes and ensure that we do not include them in the result.
	read2.Meta = map[string]interface{}{}
	props, err = MakeTerraformResult(read2, res.Schema, nil, nil, true)
	assert.NoError(t, err)
	assert.NotNil(t, props)
	assert.NotContains(t, props, metaKey)

	// Ensure that timeouts are populated and preserved.
	state.ID = ""
	cfg := terraform.NewResourceConfigRaw(map[string]interface{}{})
	diff, err := testTFProvider.Diff(info, state, cfg)
	assert.NoError(t, err)

	setTimeout(diff, float64(300), schema.TimeoutCreate)

	create, err := testTFProvider.Apply(info, state, diff)
	assert.NoError(t, err)

	props, err = MakeTerraformResult(create, res.Schema, nil, nil, true)
	assert.NoError(t, err)
	assert.NotNil(t, props)

	attrs, meta, err = MakeTerraformAttributes(res, props, res.Schema, nil, nil, false)
	assert.NoError(t, err)
	assert.NotNil(t, attrs)
	assert.NotNil(t, meta)

	timeouts := meta[schema.TimeoutKey]
	assert.NotNil(t, timeouts)
	assert.Contains(t, timeouts, schema.TimeoutCreate)
	assert.NotContains(t, timeouts, schema.TimeoutDelete)
	assert.NotContains(t, timeouts, schema.TimeoutUpdate)
}

// Test that MakeTerraformResult reads property values appropriately.
func TestResultAttributesRoundTrip(t *testing.T) {
	const resName = "example_resource"
	res := testTFProvider.ResourcesMap["example_resource"]

	info := &terraform.InstanceInfo{Type: resName}
	state := &terraform.InstanceState{ID: "0", Attributes: map[string]string{}, Meta: map[string]interface{}{}}

	read, err := testTFProvider.Refresh(info, state)
	assert.NoError(t, err)
	assert.NotNil(t, read)

	props, err := MakeTerraformResult(read, res.Schema, nil, nil, true)
	assert.NoError(t, err)
	assert.NotNil(t, props)

	attrs, _, err := MakeTerraformAttributes(res, props, res.Schema, nil, nil, false)
	assert.NoError(t, err)
	assert.NotNil(t, attrs)

	// We may add extra "%" fields to represent map counts. These diffs are innocuous. If we only see them in the
	// attributes produced by MakeTerraformResult, ignore them.
	for k, v := range attrs {
		expected, ok := read.Attributes[k]
		if !ok {
			assert.True(t, strings.HasSuffix(k, ".%"))
		} else {
			assert.Equal(t, expected, v)
		}
	}
}

// Test that an unset list still generates a length attribute.
func TestEmptyListAttribute(t *testing.T) {
	result, err := MakeTerraformAttributesFromInputs(
		map[string]interface{}{},
		map[string]*schema.Schema{
			"list_property": {Type: schema.TypeList, Optional: true},
		})

	assert.NoError(t, err)
	assert.Equal(t, result, map[string]string{
		"list_property.#": "0",
	})
}

func sortDefaultsList(m resource.PropertyMap) {
	defs := m[defaultsKey].ArrayValue()
	sort.Slice(defs, func(i, j int) bool { return defs[i].StringValue() < defs[j].StringValue() })
	m[defaultsKey] = resource.NewArrayProperty(defs)
}

func TestDefaults(t *testing.T) {
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
	assets := make(AssetTable)
	tfs := map[string]*schema.Schema{
		"ccc": {Type: schema.TypeString, Default: "CCC"},
		"cc2": {Type: schema.TypeString, DefaultFunc: func() (interface{}, error) { return "CC2", nil }},
		"ddd": {Type: schema.TypeString, Default: "TFD"},
		"dd2": {Type: schema.TypeString, DefaultFunc: func() (interface{}, error) { return "TD2", nil }},
		"ggg": {Type: schema.TypeString, Default: "TFG"},
		"hhh": {Type: schema.TypeString, Default: "TFH"},
		"iii": {Type: schema.TypeString, Default: "TFI"},
		"jjj": {Type: schema.TypeString},
		"lll": {Type: schema.TypeString, Default: "TFL"},
		"ll2": {Type: schema.TypeString, Default: "TL2"},
		"mmm": {Type: schema.TypeString},
		"mm2": {Type: schema.TypeString},
		"nnn": {Type: schema.TypeString, ConflictsWith: []string{"nn2"}, Default: "NNN"},
		"nn2": {Type: schema.TypeString, ConflictsWith: []string{"nnn"}, Default: "NN2"},
		"ooo": {Type: schema.TypeString, ConflictsWith: []string{"oo2"}, Default: "OOO"},
		"oo2": {Type: schema.TypeString, ConflictsWith: []string{"ooo"}},
		"sss": {Type: schema.TypeString, Removed: "removed"},
		"ttt": {Type: schema.TypeString, Removed: "removed", Default: "TFD"},
		"uuu": {Type: schema.TypeString},
		"vvv": {Type: schema.TypeInt},
		"www": {Type: schema.TypeString, Deprecated: "deprecated", Required: true},
		"xxx": {Type: schema.TypeString, Deprecated: "deprecated", Optional: true},
		"yyy": {Type: schema.TypeString, Default: "TLY", Deprecated: "deprecated", Optional: true},
		"zzz": {Type: schema.TypeString},
	}
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
		"sss": {Default: &DefaultInfo{Value: "PSS"}},
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
	inputs, err := MakeTerraformInputs(nil, olds, props, tfs, ps, assets, nil, true, false)
	assert.NoError(t, err)
	outputs := MakeTerraformOutputs(inputs, tfs, ps, assets, false, true)

	// sort the defaults list before the equality test below.
	sortDefaultsList(outputs)
	assert.Equal(t, resource.NewPropertyMapFromMap(map[string]interface{}{
		defaultsKey: []interface{}{
			"cc2", "ccc", "ee2", "eee", "ggg", "iii", "ll2", "lll", "mm2", "mmm", "oo2", "uuu", "vvv", "www",
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
		"oo2": "PO2",
		"uuu": "PSU",
		"vvv": 1337,
		"www": "OLW",
		"zzz": asset,
	}), outputs)

	// Now delete the defaults list from the olds and re-run. This will affect the values for "ll2" and "mm2", which
	// will be pulled from the old inputs instead of regenerated.
	delete(olds, defaultsKey)
	assets = make(AssetTable)
	inputs, err = MakeTerraformInputs(nil, olds, props, tfs, ps, assets, nil, true, false)
	assert.NoError(t, err)
	outputs = MakeTerraformOutputs(inputs, tfs, ps, assets, false, true)

	// sort the defaults list before the equality test below.
	sortDefaultsList(outputs)
	assert.Equal(t, resource.NewPropertyMapFromMap(map[string]interface{}{
		defaultsKey: []interface{}{
			"cc2", "ccc", "ee2", "eee", "ggg", "iii", "ll2", "lll", "mm2", "mmm", "oo2", "uuu", "vvv", "www",
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
		// nnn/nn2 are NOT set as they conflict with each other
		// ooo is NOT set as it conflicts with oo2
		"oo2": "PO2",
		"uuu": "PSU",
		"vvv": 1337,
		"www": "OLW",
		"zzz": asset,
	}), outputs)
}

func TestComputedAsset(t *testing.T) {
	assets := make(AssetTable)
	tfs := map[string]*schema.Schema{
		"zzz": {Type: schema.TypeString},
	}
	ps := map[string]*SchemaInfo{
		"zzz": {Asset: &AssetTranslation{Kind: FileAsset}},
	}
	olds := resource.PropertyMap{}
	props := resource.PropertyMap{
		"zzz": resource.NewStringProperty(TerraformUnknownVariableValue),
	}
	inputs, err := MakeTerraformInputs(nil, olds, props, tfs, ps, assets, nil, false, false)
	assert.NoError(t, err)
	outputs := MakeTerraformOutputs(inputs, tfs, ps, assets, false, true)
	assert.Equal(t, resource.PropertyMap{
		"zzz": resource.PropertyValue{V: resource.Computed{Element: resource.PropertyValue{V: ""}}},
	}, outputs)
}

func TestInvalidAsset(t *testing.T) {
	assets := make(AssetTable)
	tfs := map[string]*schema.Schema{
		"zzz": {Type: schema.TypeString},
	}
	ps := map[string]*SchemaInfo{
		"zzz": {Asset: &AssetTranslation{Kind: FileAsset}},
	}
	olds := resource.PropertyMap{}
	props := resource.PropertyMap{
		"zzz": resource.NewStringProperty("invalid"),
	}
	inputs, err := MakeTerraformInputs(nil, olds, props, tfs, ps, assets, nil, false, false)
	assert.NoError(t, err)
	outputs := MakeTerraformOutputs(inputs, tfs, ps, assets, false, true)
	assert.Equal(t, resource.PropertyMap{
		"zzz": resource.NewStringProperty("invalid"),
	}, outputs)
}

func TestArchiveAsAsset(t *testing.T) {
	assets := make(AssetTable)
	tfs := map[string]*schema.Schema{
		"zzz": {Type: schema.TypeString},
	}
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
	inputs, err := MakeTerraformInputs(nil, olds, props, tfs, ps, assets, nil, false, false)
	assert.NoError(t, err)
	outputs := MakeTerraformOutputs(inputs, tfs, ps, assets, false, true)
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
	tfs := &schema.Schema{Type: schema.TypeString}
	psi := &SchemaInfo{Transform: TransformJSONDocument}

	v1, err := MakeTerraformInput(
		nil, "v", resource.PropertyValue{}, resource.NewObjectProperty(resource.NewPropertyMapFromMap(doc)),
		tfs, psi, nil, nil, false, false)
	assert.NoError(t, err)
	assert.Equal(t, `{"a":99,"b":false}`, v1)

	array := []resource.PropertyValue{resource.NewObjectProperty(resource.NewPropertyMapFromMap(doc))}
	v1Array, err := MakeTerraformInput(
		nil, "v", resource.PropertyValue{}, resource.NewArrayProperty(array),
		tfs, psi, nil, nil, false, false)
	assert.NoError(t, err)
	assert.Equal(t, `[{"a":99,"b":false}]`, v1Array)

	v2, err := MakeTerraformInput(
		nil, "v", resource.PropertyValue{}, resource.NewStringProperty(`{"a":99,"b":false}`),
		tfs, psi, nil, nil, false, false)
	assert.NoError(t, err)
	assert.Equal(t, `{"a":99,"b":false}`, v2)

	doc["c"] = resource.Computed{Element: resource.PropertyValue{V: ""}}
	v3, err := MakeTerraformInput(
		nil, "v", resource.PropertyValue{}, resource.NewObjectProperty(resource.NewPropertyMapFromMap(doc)),
		tfs, psi, nil, nil, false, false)
	assert.NoError(t, err)
	assert.Equal(t, TerraformUnknownVariableValue, v3)

	v4, err := MakeTerraformInput(
		nil, "v", resource.PropertyValue{}, resource.MakeComputed(resource.NewStringProperty("")),
		tfs, psi, nil, nil, false, false)
	assert.NoError(t, err)
	assert.Equal(t, TerraformUnknownVariableValue, v4)
}

func TestImporterOnRead(t *testing.T) {
	tfProvider := makeTestTFProvider(
		map[string]*schema.Schema{
			"required_for_import": {
				Type: schema.TypeString,
			},
		},
		func(d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
			importValue := d.Id() + "-imported"
			mustSet(d, "required_for_import", importValue)

			return []*schema.ResourceData{d}, nil
		})

	provider := &Provider{
		tf: tfProvider,
		resources: map[tokens.Type]Resource{
			"importableResource": {
				TF:     tfProvider.ResourcesMap["importable_resource"],
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
		map[string]*schema.Schema{
			"required_for_import": {
				Type: schema.TypeString,
			},
		},
		func(d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
			d.SetId(d.Id() + "-imported")
			return []*schema.ResourceData{d}, nil
		})

	provider := &Provider{
		tf: tfProvider,
		resources: map[tokens.Type]Resource{
			"importableResource": {
				TF:     tfProvider.ResourcesMap["importable_resource"],
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
		map[string]*schema.Schema{
			"required_for_import": {
				Type: schema.TypeString,
			},
		},
		func(d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
			d.SetId(d.Id() + "-imported")

			d2 := &schema.ResourceData{}
			d2.SetType("other_importable_resource")
			d2.SetId(d.Id())

			return []*schema.ResourceData{d, d2}, nil
		})

	provider := &Provider{
		tf: tfProvider,
		resources: map[tokens.Type]Resource{
			"importableResource": {
				TF:     tfProvider.ResourcesMap["importable_resource"],
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
		map[string]*schema.Schema{
			"required_for_import": {
				Type: schema.TypeString,
			},
		},
		func(d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
			d.SetId(d.Id())

			d2 := &schema.ResourceData{}
			d2.SetType(d.State().Ephemeral.Type)
			d2.SetId(d.Id() + "-imported")

			return []*schema.ResourceData{d, d2}, nil
		})

	provider := &Provider{
		tf: tfProvider,
		resources: map[tokens.Type]Resource{
			"importableResource": {
				TF:     tfProvider.ResourcesMap["importable_resource"],
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
		map[string]*schema.Schema{
			"required_for_import": {
				Type: schema.TypeString,
			},
		},
		func(d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
			d.SetId(d.Id() + "-imported")

			d2 := &schema.ResourceData{}
			d2.SetType(d.State().Ephemeral.Type)
			d2.SetId(d.Id() + "-2")

			return []*schema.ResourceData{d, d2}, nil
		})

	provider := &Provider{
		tf: tfProvider,
		resources: map[tokens.Type]Resource{
			"importableResource": {
				TF:     tfProvider.ResourcesMap["importable_resource"],
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

	tfProvider := makeTestTFProvider(map[string]*schema.Schema{},
		func(d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
			// Return nothing
			return []*schema.ResourceData{}, nil
		})

	provider := &Provider{
		tf: tfProvider,
		resources: map[tokens.Type]Resource{
			"importableResource": {
				TF:     tfProvider.ResourcesMap["importable_resource"],
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

func makeTestTFProvider(schemaMap map[string]*schema.Schema, importer schema.StateFunc) *schema.Provider {
	return &schema.Provider{
		ResourcesMap: map[string]*schema.Resource{
			"importable_resource": {
				Schema: schemaMap,
				Importer: &schema.ResourceImporter{
					State: importer,
				},
				Read: func(d *schema.ResourceData, meta interface{}) error {
					return nil
				},
				Create: func(d *schema.ResourceData, meta interface{}) error {
					return nil
				},
				Delete: func(d *schema.ResourceData, meta interface{}) error {
					return nil
				},
			},
		},
	}
}

func TestStringOutputsWithSchema(t *testing.T) {
	result := MakeTerraformOutputs(
		map[string]interface{}{
			"bool_property_value":      "false",
			"number_property_value":    "42",
			"float_property_value":     "42.0",
			"no_schema_property_value": "42",
			"not_an_int_value":         "lmao",
			"not_a_bool_value":         "lmao2",
			"not_a_float_value":        "lmao3",
		},
		map[string]*schema.Schema{
			"bool_property_value":   {Type: schema.TypeBool},
			"number_property_value": {Type: schema.TypeInt},
			"float_property_value":  {Type: schema.TypeFloat},
			"not_an_int_value":      {Type: schema.TypeInt},
			"not_a_bool_value":      {Type: schema.TypeBool},
			"not_a_float_value":     {Type: schema.TypeFloat},
		},
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
		map[string]*schema.Schema{
			"input_a": {Type: schema.TypeString, Required: true},
			"input_b": {Type: schema.TypeString, Optional: true},
			"inout_c": {Type: schema.TypeString, Optional: true, Computed: true},
			"inout_d": {Type: schema.TypeString, Optional: true, Computed: true, Default: "inout_d_default"},
			"input_e": {
				Type: schema.TypeList,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"field_a": {Type: schema.TypeString, Optional: true, Default: "field_a_default"},
					},
				},
				MaxItems: 1,
				Optional: true,
			},
			"input_f":  {Type: schema.TypeString, Required: true},
			"output_g": {Type: schema.TypeString},
			"input_h": {
				Type:     schema.TypeString,
				Required: true,
				StateFunc: func(v interface{}) string {
					return strings.ToLower(v.(string))
				},
			},
		},
		func(d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
			return []*schema.ResourceData{d}, nil
		})

	set := func(d *schema.ResourceData, key string, value interface{}) {
		contract.IgnoreError(d.Set(key, value))
	}

	tfres := tfProvider.ResourcesMap["importable_resource"]
	tfres.Read = func(d *schema.ResourceData, meta interface{}) error {
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
		return nil
	}
	tfres.Create = func(d *schema.ResourceData, meta interface{}) error {
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
		tf: tfProvider,
		resources: map[tokens.Type]Resource{
			"importableResource": {
				TF:     tfProvider.ResourcesMap["importable_resource"],
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
	}), outs)

	ins, err := plugin.UnmarshalProperties(resp.GetInputs(), plugin.MarshalOptions{})
	assert.NoError(t, err)
	expected := resource.NewPropertyMapFromMap(map[string]interface{}{
		defaultsKey: []interface{}{},
		"inputA":    "input_a_read",
		"inoutC":    "inout_c_read",
		"inoutD":    "inout_d_read",
	})
	assert.True(t, expected.DeepEquals(ins))

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
		defaultsKey: []interface{}{"inoutD", "inputF", "inputH"},
		"inputA":    "input_a_create",
		"inoutD":    "inout_d_default",
		"inputE": map[string]interface{}{
			defaultsKey: []interface{}{"fieldA"},
			"fieldA":    "field_a_default",
		},
		"inputF": "input_f_default",
		"inputH": "Input_H_Default",
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
	}), outs)

	ins, err = plugin.UnmarshalProperties(resp.GetInputs(), plugin.MarshalOptions{})
	assert.NoError(t, err)
	assert.Equal(t, resource.NewPropertyMapFromMap(map[string]interface{}{
		defaultsKey: []interface{}{"inputF", "inputH"},
		"inputA":    "input_a_create",
		"inoutC":    "inout_c_create",
		"inoutD":    "inout_d_read",
		"inputE": map[string]interface{}{
			defaultsKey: []interface{}{"fieldA"},
			"fieldA":    "field_a_default",
		},
		"inputF": "input_f_default",
		"inputH": "Input_H_Default",
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
	}), outs)

	ins, err = plugin.UnmarshalProperties(resp.GetInputs(), plugin.MarshalOptions{})
	assert.NoError(t, err)
	assert.Equal(t, resource.NewPropertyMapFromMap(map[string]interface{}{
		"inputA": "input_a_create",
		"inoutC": "inout_c_create",
		"inoutD": "inout_d_read",
		"inputE": map[string]interface{}{
			"fieldA": "field_a_default",
		},
		"inputF": "input_f_default",
		"inputH": "Input_H_Default",
	}), ins)

}

func TestFailureReasonForMissingRequiredFields(t *testing.T) {
	// Define two required inputs
	tfProvider := makeTestTFProvider(
		map[string]*schema.Schema{
			"input_x": {Type: schema.TypeString, Required: true},
			"input_y": {Type: schema.TypeString, Required: true},
		},
		func(d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
			return []*schema.ResourceData{d}, nil
		})

	// Input Y has a default info pointing to a config key
	p := &Provider{
		tf: tfProvider,
		resources: map[tokens.Type]Resource{
			"importableResource": {
				TF:     tfProvider.ResourcesMap["importable_resource"],
				TFName: "importable_resource",
				Schema: &ResourceInfo{
					Tok: tokens.NewTypeToken("module", "importableResource"),
					Fields: map[string]*SchemaInfo{
						"inputY": {
							Default: &DefaultInfo{
								Config: "input_y_config",
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
		"with 'pulumi config set test:input_y_config <value>'.", y)
}

func TestAssetRoundtrip(t *testing.T) {
	tfProvider := makeTestTFProvider(
		map[string]*schema.Schema{
			"input_a": {Type: schema.TypeString, Required: true},
		},
		func(d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
			return []*schema.ResourceData{d}, nil
		})

	tfres := tfProvider.ResourcesMap["importable_resource"]
	tfres.Create = func(d *schema.ResourceData, meta interface{}) error {
		d.SetId("MyID")
		return nil
	}
	tfres.Update = func(d *schema.ResourceData, meta interface{}) error {
		return nil
	}

	p := &Provider{
		tf: tfProvider,
		resources: map[tokens.Type]Resource{
			"importableResource": {
				TF:     tfProvider.ResourcesMap["importable_resource"],
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
		map[string]*schema.Schema{
			"input_a": {Type: schema.TypeString, Required: true},
			"input_b": {Type: schema.TypeString, Required: true, ForceNew: true},
		},
		func(d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
			return []*schema.ResourceData{d}, nil
		})

	tfres := tfProvider.ResourcesMap["importable_resource"]
	tfres.Create = func(d *schema.ResourceData, meta interface{}) error {
		d.SetId("MyID")
		return nil
	}
	tfres.Update = func(d *schema.ResourceData, meta interface{}) error {
		return nil
	}

	p := &Provider{
		tf: tfProvider,
		resources: map[tokens.Type]Resource{
			"importableResource": {
				TF:     tfProvider.ResourcesMap["importable_resource"],
				TFName: "importable_resource",
				Schema: &ResourceInfo{
					Tok: tokens.NewTypeToken("module", "importableResource"),
					Fields: map[string]*SchemaInfo{
						"input_a": AutoName("inputA", 64),
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
