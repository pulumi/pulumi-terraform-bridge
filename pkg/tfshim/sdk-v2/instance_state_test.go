package sdkv2

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/stretchr/testify/assert"
)

func TestToInstanceState(t *testing.T) {
	res := NewResource(&schema.Resource{
		Schema: map[string]*schema.Schema{
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
		},
	})

	state, err := res.InstanceState("id", map[string]interface{}{
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
	}, nil)

	assert.NoError(t, err)
	assert.Equal(t, state.(v2InstanceState).tf.Attributes, map[string]string{
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
	})

	// MapFieldWriter has issues with values of TypeMap. Build a schema without such values s.t. we can test
	// MakeTerraformState against the output of MapFieldWriter.
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
	cfg := terraform.NewResourceConfigRaw(sharedInputs)
	reader := &schema.ConfigFieldReader{Config: cfg, Schema: sharedSchema}
	writer := &schema.MapFieldWriter{Schema: sharedSchema}
	for k := range sharedInputs {
		f, ferr := reader.ReadField([]string{k})
		assert.NoError(t, ferr)

		err = writer.WriteField([]string{k}, f.Value)
		assert.NoError(t, err)
	}
	expected := writer.Map()

	// Build the same using makeTerraformAttributesFromInputs.
	res = NewResource(&schema.Resource{Schema: sharedSchema})
	state, err = res.InstanceState("id", sharedInputs, nil)
	assert.NoError(t, err)
	assert.Equal(t, expected, state.(v2InstanceState).tf.Attributes)
}

// Test that an unset list still generates a length attribute.
func TestEmptyListAttribute(t *testing.T) {
	res := NewResource(&schema.Resource{
		Schema: map[string]*schema.Schema{
			"list_property": {Type: schema.TypeList, Optional: true},
		},
	})

	state, err := res.InstanceState("id", map[string]interface{}{}, nil)
	assert.NoError(t, err)
	assert.Equal(t, state.(v2InstanceState).tf.Attributes, map[string]string{
		"list_property.#": "0",
	})
}

func TestObjectFromInstanceDiff(t *testing.T) {
	res := NewResource(&schema.Resource{
		Schema: map[string]*schema.Schema{
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
				Type: schema.TypeString,
			},
		},
	})

	state, err := res.InstanceState("id", map[string]interface{}{
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
	}, nil)
	assert.NoError(t, err)

	s := state.(v2InstanceState)
	s.diff = &terraform.InstanceDiff{
		Attributes: map[string]*terraform.ResourceAttrDiff{
			"number_property_value": {
				Old:         "42",
				New:         UnknownVariableValue,
				NewComputed: true,
			},
			"object_property_value.property_a": {
				Old:         "a",
				New:         UnknownVariableValue,
				NewComputed: true,
			},
			"map_property_value.%": {
				Old:         "3",
				New:         UnknownVariableValue,
				NewComputed: true,
			},
			"nested_resources.0.configuration.configurationValue": {
				Old:         "true",
				New:         UnknownVariableValue,
				NewComputed: true,
			},
			"set_property_value.1234": {
				New:         UnknownVariableValue,
				NewComputed: true,
			},
		},
	}

	obj, err := s.Object(res.Schema())
	assert.NoError(t, err)

	assert.Equal(t, map[string]interface{}{
		"id":                    "",
		"bool_property_value":   false,
		"float_property_value":  99.6767932,
		"string_property_value": "ognirts",
		"array_property_value":  []interface{}{"an array"},
		"object_property_value": map[string]interface{}{
			"property_a": UnknownVariableValue,
			"property_b": "true",
		},
		"nested_resources": []interface{}{
			map[string]interface{}{
				"configuration": map[string]interface{}{
					"configurationValue": UnknownVariableValue,
				},
			},
		},
		"string_with_bad_interpolation": "some ${interpolated:value} with syntax errors",
		"removed_property_value":        "a removed property",
	}, obj)
}
