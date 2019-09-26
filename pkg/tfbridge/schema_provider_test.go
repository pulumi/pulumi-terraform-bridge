package tfbridge

import (
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/terraform"
)

func mustSet(data *schema.ResourceData, key string, value interface{}) {
	err := data.Set(key, value)
	if err != nil {
		panic(err)
	}
}

func timeout(d time.Duration) *time.Duration {
	return &d
}

var testTFProvider = &schema.Provider{
	Schema: map[string]*schema.Schema{
		"config_value": {},
	},
	ResourcesMap: map[string]*schema.Resource{
		"example_resource": {
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
			SchemaVersion: 1,
			MigrateState: func(v int, is *terraform.InstanceState, p interface{}) (*terraform.InstanceState, error) {
				return is, nil
			},
			Create: func(data *schema.ResourceData, p interface{}) error {
				data.SetId("0")
				mustSet(data, "bool_property_value", false)
				mustSet(data, "number_property_value", 42)
				mustSet(data, "float_property_value", 99.6767932)
				mustSet(data, "string_property_value", "ognirts")
				mustSet(data, "array_property_value", []interface{}{"an array"})
				mustSet(data, "object_property_value", map[string]interface{}{
					"property_a": "a",
					"property_b": "true",
					"property.c": "some.value",
				})
				mustSet(data, "nested_resources", []interface{}{
					map[string]interface{}{
						"configuration": map[string]interface{}{
							"configurationValue": "true",
						},
					},
				})
				mustSet(data, "set_property_value", []interface{}{"set member 1", "set member 2"})
				mustSet(data, "string_with_bad_interpolation", "some ${interpolated:value} with syntax errors")
				return nil
			},
			Read: func(data *schema.ResourceData, p interface{}) error {
				mustSet(data, "bool_property_value", false)
				mustSet(data, "number_property_value", 42)
				mustSet(data, "float_property_value", 99.6767932)
				mustSet(data, "string_property_value", "ognirts")
				mustSet(data, "array_property_value", []interface{}{"an array"})
				mustSet(data, "object_property_value", map[string]interface{}{
					"property_a": "a",
					"property_b": "true",
					"property.c": "some.value",
				})
				mustSet(data, "nested_resources", []interface{}{
					map[string]interface{}{
						"configuration": map[string]interface{}{
							"configurationValue": "true",
						},
					},
				})
				mustSet(data, "set_property_value", []interface{}{"set member 1", "set member 2"})
				mustSet(data, "string_with_bad_interpolation", "some ${interpolated:value} with syntax errors")
				return nil
			},
			Update: func(data *schema.ResourceData, p interface{}) error {
				mustSet(data, "bool_property_value", false)
				mustSet(data, "number_property_value", 42)
				mustSet(data, "float_property_value", 99.6767932)
				mustSet(data, "string_property_value", "ognirts")
				mustSet(data, "array_property_value", []interface{}{"an array"})
				mustSet(data, "object_property_value", map[string]interface{}{
					"property_a": "a",
					"property_b": "true",
					"property.c": "some.value",
				})
				mustSet(data, "nested_resources", []interface{}{
					map[string]interface{}{
						"configuration": map[string]interface{}{
							"configurationValue": "true",
						},
					},
				})
				mustSet(data, "set_property_value", []interface{}{"set member 1", "set member 2"})
				mustSet(data, "string_with_bad_interpolation", "some ${interpolated:value} with syntax errors")
				return nil
			},
			Delete: func(data *schema.ResourceData, p interface{}) error {
				return nil
			},
			Timeouts: &schema.ResourceTimeout{
				Create: timeout(time.Second * 120),
			},
		},
		"second_resource": {
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
			SchemaVersion: 1,
			MigrateState: func(v int, is *terraform.InstanceState, p interface{}) (*terraform.InstanceState, error) {
				return is, nil
			},
			Create: func(data *schema.ResourceData, p interface{}) error {
				data.SetId("0")
				mustSet(data, "bool_property_value", false)
				mustSet(data, "number_property_value", 42)
				mustSet(data, "float_property_value", 99.6767932)
				mustSet(data, "string_property_value", "ognirts")
				mustSet(data, "array_property_value", []interface{}{"an array"})
				mustSet(data, "object_property_value", map[string]interface{}{
					"property_a": "a",
					"property_b": "true",
					"property.c": "some.value",
				})
				mustSet(data, "nested_resources", []interface{}{
					map[string]interface{}{
						"configuration": map[string]interface{}{
							"configurationValue": "true",
						},
					},
				})
				mustSet(data, "set_property_value", []interface{}{"set member 1", "set member 2"})
				mustSet(data, "string_with_bad_interpolation", "some ${interpolated:value} with syntax errors")
				return nil
			},
			Read: func(data *schema.ResourceData, p interface{}) error {
				mustSet(data, "bool_property_value", false)
				mustSet(data, "number_property_value", 42)
				mustSet(data, "float_property_value", 99.6767932)
				mustSet(data, "string_property_value", "ognirts")
				mustSet(data, "array_property_value", []interface{}{"an array"})
				mustSet(data, "object_property_value", map[string]interface{}{
					"property_a": "a",
					"property_b": "true",
					"property.c": "some.value",
				})
				mustSet(data, "nested_resources", []interface{}{
					map[string]interface{}{
						"configuration": map[string]interface{}{
							"configurationValue": "true",
						},
					},
				})
				mustSet(data, "set_property_value", []interface{}{"set member 1", "set member 2"})
				mustSet(data, "string_with_bad_interpolation", "some ${interpolated:value} with syntax errors")
				return nil
			},
			Update: func(data *schema.ResourceData, p interface{}) error {
				mustSet(data, "bool_property_value", false)
				mustSet(data, "number_property_value", 42)
				mustSet(data, "float_property_value", 99.6767932)
				mustSet(data, "string_property_value", "ognirts")
				mustSet(data, "array_property_value", []interface{}{"an array"})
				mustSet(data, "object_property_value", map[string]interface{}{
					"property_a": "a",
					"property_b": "true",
					"property.c": "some.value",
				})
				mustSet(data, "nested_resources", []interface{}{
					map[string]interface{}{
						"configuration": map[string]interface{}{
							"configurationValue": "true",
						},
					},
				})
				mustSet(data, "set_property_value", []interface{}{"set member 1", "set member 2"})
				mustSet(data, "string_with_bad_interpolation", "some ${interpolated:value} with syntax errors")
				return nil
			},
			Delete: func(data *schema.ResourceData, p interface{}) error {
				return nil
			},
		},
	},
	DataSourcesMap: map[string]*schema.Resource{
		"example_resource": {
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
			SchemaVersion: 1,
			Read: func(data *schema.ResourceData, p interface{}) error {
				mustSet(data, "bool_property_value", false)
				mustSet(data, "number_property_value", 42)
				mustSet(data, "float_property_value", 99.6767932)
				mustSet(data, "string_property_value", "ognirts")
				mustSet(data, "array_property_value", []interface{}{"an array"})
				mustSet(data, "object_property_value", map[string]interface{}{
					"property_a": "a",
					"property_b": "true",
					"property.c": "some.value",
				})
				mustSet(data, "nested_resources", []interface{}{
					map[string]interface{}{
						"configuration": map[string]interface{}{
							"configurationValue": "true",
						},
					},
				})
				mustSet(data, "set_property_value", []interface{}{"set member 1", "set member 2"})
				mustSet(data, "string_with_bad_interpolation", "some ${interpolated:value} with syntax errors")
				return nil
			},
		},
	},
	ConfigureFunc: func(data *schema.ResourceData) (interface{}, error) {
		return nil, nil
	},
}
