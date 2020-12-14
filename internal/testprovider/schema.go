package testprovider

import (
	"context"
	"time"

	schemav1 "github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	terraformv1 "github.com/hashicorp/terraform-plugin-sdk/terraform"
	schemav2 "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	terraformv2 "github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

type Settable interface {
	Set(key string, value interface{}) error
}

type ResourceData interface {
	Settable

	GetOk(key string) (interface{}, bool)
}

func MustSet(data Settable, key string, value interface{}) {
	err := data.Set(key, value)
	if err != nil {
		panic(err)
	}
}

func MustSetIfUnset(data ResourceData, key string, value interface{}) {
	if _, ok := data.GetOk(key); !ok {
		MustSet(data, key, value)
	}
}

func Timeout(d time.Duration) *time.Duration {
	return &d
}

func ProviderV1() *schemav1.Provider {
	return &schemav1.Provider{
		Schema: map[string]*schemav1.Schema{
			"config_value": {Type: schemav1.TypeString, Optional: true},
		},
		ResourcesMap: map[string]*schemav1.Resource{
			"example_resource": {
				Schema: map[string]*schemav1.Schema{
					"nil_property_value":    {Type: schemav1.TypeMap, Optional: true},
					"bool_property_value":   {Type: schemav1.TypeBool, Optional: true},
					"number_property_value": {Type: schemav1.TypeInt, Optional: true},
					"float_property_value":  {Type: schemav1.TypeFloat, Optional: true},
					"string_property_value": {Type: schemav1.TypeString, Optional: true},
					"array_property_value": {
						Type:     schemav1.TypeList,
						Elem:     &schemav1.Schema{Type: schemav1.TypeString},
						Required: true,
					},
					"object_property_value": {Type: schemav1.TypeMap, Optional: true},
					"nested_resources": {
						Type:     schemav1.TypeList,
						MaxItems: 1,
						// Embed a `*schemav1.Resource` to validate that type directed
						// walk of the schema successfully walks inside Resources as well
						// as Schemas.
						Elem: &schemav1.Resource{
							Schema: map[string]*schemav1.Schema{
								"kind":          {Type: schemav1.TypeString, Optional: true},
								"configuration": {Type: schemav1.TypeMap, Required: true},
							},
						},
						Optional: true,
					},
					"set_property_value": {
						Type:     schemav1.TypeSet,
						Elem:     &schemav1.Schema{Type: schemav1.TypeString},
						Optional: true,
						ForceNew: true,
					},
					"string_with_bad_interpolation": {Type: schemav1.TypeString, Optional: true},
				},
				SchemaVersion: 1,
				MigrateState: func(v int, is *terraformv1.InstanceState, p interface{}) (*terraformv1.InstanceState, error) {
					return is, nil
				},
				Create: func(data *schemav1.ResourceData, p interface{}) error {
					data.SetId("0")
					MustSetIfUnset(data, "bool_property_value", false)
					MustSetIfUnset(data, "number_property_value", 42)
					MustSetIfUnset(data, "float_property_value", 99.6767932)
					MustSetIfUnset(data, "string_property_value", "ognirts")
					MustSetIfUnset(data, "array_property_value", []interface{}{"an array"})
					MustSetIfUnset(data, "object_property_value", map[string]interface{}{
						"property_a": "a",
						"property_b": "true",
						"property.c": "some.value",
					})
					MustSetIfUnset(data, "nested_resources", []interface{}{
						map[string]interface{}{
							"configuration": map[string]interface{}{
								"configurationValue": "true",
							},
						},
					})
					MustSetIfUnset(data, "set_property_value", []interface{}{"set member 1", "set member 2"})
					MustSetIfUnset(data, "string_with_bad_interpolation", "some ${interpolated:value} with syntax errors")
					return nil
				},
				Read: func(data *schemav1.ResourceData, p interface{}) error {
					MustSetIfUnset(data, "bool_property_value", false)
					MustSetIfUnset(data, "number_property_value", 42)
					MustSetIfUnset(data, "float_property_value", 99.6767932)
					MustSetIfUnset(data, "string_property_value", "ognirts")
					MustSetIfUnset(data, "array_property_value", []interface{}{"an array"})
					MustSetIfUnset(data, "object_property_value", map[string]interface{}{
						"property_a": "a",
						"property_b": "true",
						"property.c": "some.value",
					})
					MustSetIfUnset(data, "nested_resources", []interface{}{
						map[string]interface{}{
							"configuration": map[string]interface{}{
								"configurationValue": "true",
							},
						},
					})
					MustSetIfUnset(data, "set_property_value", []interface{}{"set member 1", "set member 2"})
					MustSetIfUnset(data, "string_with_bad_interpolation", "some ${interpolated:value} with syntax errors")
					return nil
				},
				Update: func(data *schemav1.ResourceData, p interface{}) error {
					MustSetIfUnset(data, "bool_property_value", false)
					MustSetIfUnset(data, "number_property_value", 42)
					MustSetIfUnset(data, "float_property_value", 99.6767932)
					MustSetIfUnset(data, "string_property_value", "ognirts")
					MustSetIfUnset(data, "array_property_value", []interface{}{"an array"})
					MustSetIfUnset(data, "object_property_value", map[string]interface{}{
						"property_a": "a",
						"property_b": "true",
						"property.c": "some.value",
					})
					MustSetIfUnset(data, "nested_resources", []interface{}{
						map[string]interface{}{
							"configuration": map[string]interface{}{
								"configurationValue": "true",
							},
						},
					})
					MustSetIfUnset(data, "set_property_value", []interface{}{"set member 1", "set member 2"})
					MustSetIfUnset(data, "string_with_bad_interpolation", "some ${interpolated:value} with syntax errors")
					return nil
				},
				Delete: func(data *schemav1.ResourceData, p interface{}) error {
					return nil
				},
				Timeouts: &schemav1.ResourceTimeout{
					Create: Timeout(time.Second * 120),
				},
				Importer: &schemav1.ResourceImporter{
					State: func(state *schemav1.ResourceData, _ interface{}) ([]*schemav1.ResourceData, error) {
						return []*schemav1.ResourceData{state}, nil
					},
				},
			},
			"second_resource": {
				Schema: map[string]*schemav1.Schema{
					"nil_property_value":    {Type: schemav1.TypeMap, Optional: true},
					"bool_property_value":   {Type: schemav1.TypeBool, Optional: true},
					"number_property_value": {Type: schemav1.TypeInt, Optional: true},
					"float_property_value":  {Type: schemav1.TypeFloat, Optional: true},
					"string_property_value": {Type: schemav1.TypeString, Optional: true},
					"array_property_value": {
						Type:     schemav1.TypeList,
						Elem:     &schemav1.Schema{Type: schemav1.TypeString},
						Required: true,
					},
					"object_property_value": {Type: schemav1.TypeMap, Optional: true},
					"nested_resources": {
						Type:     schemav1.TypeList,
						MaxItems: 1,
						// Embed a `*schemav1.Resource` to validate that type directed
						// walk of the schema successfully walks inside Resources as well
						// as Schemas.
						Elem: &schemav1.Resource{
							Schema: map[string]*schemav1.Schema{
								"configuration": {Type: schemav1.TypeMap, Required: true},
							},
						},
						Optional: true,
					},
					"set_property_value": {
						Type:     schemav1.TypeSet,
						Elem:     &schemav1.Schema{Type: schemav1.TypeString},
						Optional: true,
						ForceNew: true,
					},
					"string_with_bad_interpolation": {Type: schemav1.TypeString, Optional: true},
				},
				SchemaVersion: 1,
				MigrateState: func(v int, is *terraformv1.InstanceState, p interface{}) (*terraformv1.InstanceState, error) {
					return is, nil
				},
				Create: func(data *schemav1.ResourceData, p interface{}) error {
					data.SetId("0")
					MustSetIfUnset(data, "bool_property_value", false)
					MustSetIfUnset(data, "number_property_value", 42)
					MustSetIfUnset(data, "float_property_value", 99.6767932)
					MustSetIfUnset(data, "string_property_value", "ognirts")
					MustSetIfUnset(data, "array_property_value", []interface{}{"an array"})
					MustSetIfUnset(data, "object_property_value", map[string]interface{}{
						"property_a": "a",
						"property_b": "true",
						"property.c": "some.value",
					})
					MustSetIfUnset(data, "nested_resources", []interface{}{
						map[string]interface{}{
							"configuration": map[string]interface{}{
								"configurationValue": "true",
							},
						},
					})
					MustSetIfUnset(data, "set_property_value", []interface{}{"set member 1", "set member 2"})
					MustSetIfUnset(data, "string_with_bad_interpolation", "some ${interpolated:value} with syntax errors")
					return nil
				},
				Read: func(data *schemav1.ResourceData, p interface{}) error {
					MustSetIfUnset(data, "bool_property_value", false)
					MustSetIfUnset(data, "number_property_value", 42)
					MustSetIfUnset(data, "float_property_value", 99.6767932)
					MustSetIfUnset(data, "string_property_value", "ognirts")
					MustSetIfUnset(data, "array_property_value", []interface{}{"an array"})
					MustSetIfUnset(data, "object_property_value", map[string]interface{}{
						"property_a": "a",
						"property_b": "true",
						"property.c": "some.value",
					})
					MustSetIfUnset(data, "nested_resources", []interface{}{
						map[string]interface{}{
							"configuration": map[string]interface{}{
								"configurationValue": "true",
							},
						},
					})
					MustSetIfUnset(data, "set_property_value", []interface{}{"set member 1", "set member 2"})
					MustSetIfUnset(data, "string_with_bad_interpolation", "some ${interpolated:value} with syntax errors")
					return nil
				},
				Update: func(data *schemav1.ResourceData, p interface{}) error {
					MustSetIfUnset(data, "bool_property_value", false)
					MustSetIfUnset(data, "number_property_value", 42)
					MustSetIfUnset(data, "float_property_value", 99.6767932)
					MustSetIfUnset(data, "string_property_value", "ognirts")
					MustSetIfUnset(data, "array_property_value", []interface{}{"an array"})
					MustSetIfUnset(data, "object_property_value", map[string]interface{}{
						"property_a": "a",
						"property_b": "true",
						"property.c": "some.value",
					})
					MustSetIfUnset(data, "nested_resources", []interface{}{
						map[string]interface{}{
							"configuration": map[string]interface{}{
								"configurationValue": "true",
							},
						},
					})
					MustSetIfUnset(data, "set_property_value", []interface{}{"set member 1", "set member 2"})
					MustSetIfUnset(data, "string_with_bad_interpolation", "some ${interpolated:value} with syntax errors")
					return nil
				},
				Delete: func(data *schemav1.ResourceData, p interface{}) error {
					return nil
				},
				Timeouts: &schemav1.ResourceTimeout{
					Create: Timeout(time.Second * 120),
					Update: Timeout(time.Second * 120),
				},
			},
		},
		DataSourcesMap: map[string]*schemav1.Resource{
			"example_resource": {
				Schema: map[string]*schemav1.Schema{
					"nil_property_value":    {Type: schemav1.TypeMap, Optional: true},
					"bool_property_value":   {Type: schemav1.TypeBool, Optional: true},
					"number_property_value": {Type: schemav1.TypeInt, Optional: true},
					"float_property_value":  {Type: schemav1.TypeFloat, Optional: true},
					"string_property_value": {Type: schemav1.TypeString, Optional: true},
					"array_property_value": {
						Type:     schemav1.TypeList,
						Elem:     &schemav1.Schema{Type: schemav1.TypeString},
						Required: true,
					},
					"object_property_value": {Type: schemav1.TypeMap, Optional: true},
					"map_property_value":    {Type: schemav1.TypeMap, Optional: true},
					"nested_resources": {
						Type:     schemav1.TypeList,
						MaxItems: 1,
						// Embed a `*schemav1.Resource` to validate that type directed
						// walk of the schema successfully walks inside Resources as well
						// as Schemas.
						Elem: &schemav1.Resource{
							Schema: map[string]*schemav1.Schema{
								"configuration": {Type: schemav1.TypeMap, Required: true},
							},
						},
						Optional: true,
					},
					"set_property_value": {
						Type:     schemav1.TypeSet,
						Elem:     &schemav1.Schema{Type: schemav1.TypeString},
						Optional: true,
					},
					"string_with_bad_interpolation": {Type: schemav1.TypeString, Optional: true},
				},
				SchemaVersion: 1,
				Read: func(data *schemav1.ResourceData, p interface{}) error {
					data.SetId("0")
					MustSetIfUnset(data, "bool_property_value", false)
					MustSetIfUnset(data, "number_property_value", 42)
					MustSetIfUnset(data, "float_property_value", 99.6767932)
					MustSetIfUnset(data, "string_property_value", "ognirts")
					MustSetIfUnset(data, "array_property_value", []interface{}{"an array"})
					MustSetIfUnset(data, "object_property_value", map[string]interface{}{
						"property_a": "a",
						"property_b": "true",
						"property.c": "some.value",
					})
					MustSetIfUnset(data, "nested_resources", []interface{}{
						map[string]interface{}{
							"configuration": map[string]interface{}{
								"configurationValue": "true",
							},
						},
					})
					MustSetIfUnset(data, "set_property_value", []interface{}{"set member 1", "set member 2"})
					MustSetIfUnset(data, "string_with_bad_interpolation", "some ${interpolated:value} with syntax errors")
					return nil
				},
			},
		},
		ConfigureFunc: func(data *schemav1.ResourceData) (interface{}, error) {
			return nil, nil
		},
	}
}

func ProviderV2() *schemav2.Provider {
	return &schemav2.Provider{
		Schema: map[string]*schemav2.Schema{
			"config_value": {Type: schemav2.TypeString, Optional: true},
		},
		ResourcesMap: map[string]*schemav2.Resource{
			"example_resource": {
				Schema: map[string]*schemav2.Schema{
					"nil_property_value":    {Type: schemav2.TypeMap, Optional: true},
					"bool_property_value":   {Type: schemav2.TypeBool, Optional: true},
					"number_property_value": {Type: schemav2.TypeInt, Optional: true},
					"float_property_value":  {Type: schemav2.TypeFloat, Optional: true},
					"string_property_value": {Type: schemav2.TypeString, Optional: true},
					"array_property_value": {
						Type:     schemav2.TypeList,
						Elem:     &schemav2.Schema{Type: schemav2.TypeString},
						Required: true,
					},
					"object_property_value": {Type: schemav2.TypeMap, Optional: true},
					"nested_resources": {
						Type:     schemav2.TypeList,
						MaxItems: 1,
						// Embed a `*schemav2.Resource` to validate that type directed
						// walk of the schema successfully walks inside Resources as well
						// as Schemas.
						Elem: &schemav2.Resource{
							Schema: map[string]*schemav2.Schema{
								"kind":          {Type: schemav2.TypeString, Optional: true},
								"configuration": {Type: schemav2.TypeMap, Required: true},
							},
						},
						Optional: true,
					},
					"set_property_value": {
						Type:     schemav2.TypeSet,
						Elem:     &schemav2.Schema{Type: schemav2.TypeString},
						Optional: true,
						ForceNew: true,
					},
					"string_with_bad_interpolation": {Type: schemav2.TypeString, Optional: true},
				},
				SchemaVersion: 1,
				MigrateState: func(v int, is *terraformv2.InstanceState, p interface{}) (*terraformv2.InstanceState, error) {
					return is, nil
				},
				Create: func(data *schemav2.ResourceData, p interface{}) error {
					data.SetId("0")
					MustSetIfUnset(data, "bool_property_value", false)
					MustSetIfUnset(data, "number_property_value", 42)
					MustSetIfUnset(data, "float_property_value", 99.6767932)
					MustSetIfUnset(data, "string_property_value", "ognirts")
					MustSetIfUnset(data, "array_property_value", []interface{}{"an array"})
					MustSetIfUnset(data, "object_property_value", map[string]interface{}{
						"property_a": "a",
						"property_b": "true",
						"property.c": "some.value",
					})
					MustSetIfUnset(data, "nested_resources", []interface{}{
						map[string]interface{}{
							"configuration": map[string]interface{}{
								"configurationValue": "true",
							},
						},
					})
					MustSetIfUnset(data, "set_property_value", []interface{}{"set member 1", "set member 2"})
					MustSetIfUnset(data, "string_with_bad_interpolation", "some ${interpolated:value} with syntax errors")
					return nil
				},
				Read: func(data *schemav2.ResourceData, p interface{}) error {
					MustSetIfUnset(data, "bool_property_value", false)
					MustSetIfUnset(data, "number_property_value", 42)
					MustSetIfUnset(data, "float_property_value", 99.6767932)
					MustSetIfUnset(data, "string_property_value", "ognirts")
					MustSetIfUnset(data, "array_property_value", []interface{}{"an array"})
					MustSetIfUnset(data, "object_property_value", map[string]interface{}{
						"property_a": "a",
						"property_b": "true",
						"property.c": "some.value",
					})
					MustSetIfUnset(data, "nested_resources", []interface{}{
						map[string]interface{}{
							"configuration": map[string]interface{}{
								"configurationValue": "true",
							},
						},
					})
					MustSetIfUnset(data, "set_property_value", []interface{}{"set member 1", "set member 2"})
					MustSetIfUnset(data, "string_with_bad_interpolation", "some ${interpolated:value} with syntax errors")
					return nil
				},
				Update: func(data *schemav2.ResourceData, p interface{}) error {
					MustSetIfUnset(data, "bool_property_value", false)
					MustSetIfUnset(data, "number_property_value", 42)
					MustSetIfUnset(data, "float_property_value", 99.6767932)
					MustSetIfUnset(data, "string_property_value", "ognirts")
					MustSetIfUnset(data, "array_property_value", []interface{}{"an array"})
					MustSetIfUnset(data, "object_property_value", map[string]interface{}{
						"property_a": "a",
						"property_b": "true",
						"property.c": "some.value",
					})
					MustSetIfUnset(data, "nested_resources", []interface{}{
						map[string]interface{}{
							"configuration": map[string]interface{}{
								"configurationValue": "true",
							},
						},
					})
					MustSetIfUnset(data, "set_property_value", []interface{}{"set member 1", "set member 2"})
					MustSetIfUnset(data, "string_with_bad_interpolation", "some ${interpolated:value} with syntax errors")
					return nil
				},
				Delete: func(data *schemav2.ResourceData, p interface{}) error {
					return nil
				},
				Timeouts: &schemav2.ResourceTimeout{
					Create: Timeout(time.Second * 120),
				},
				Importer: &schemav2.ResourceImporter{
					StateContext: func(_ context.Context, state *schemav2.ResourceData,
						_ interface{}) ([]*schemav2.ResourceData, error) {

						return []*schemav2.ResourceData{state}, nil
					},
				},
			},
			"second_resource": {
				Schema: map[string]*schemav2.Schema{
					"nil_property_value":    {Type: schemav2.TypeMap, Optional: true},
					"bool_property_value":   {Type: schemav2.TypeBool, Optional: true},
					"number_property_value": {Type: schemav2.TypeInt, Optional: true},
					"float_property_value":  {Type: schemav2.TypeFloat, Optional: true},
					"string_property_value": {Type: schemav2.TypeString, Optional: true},
					"array_property_value": {
						Type:     schemav2.TypeList,
						Elem:     &schemav2.Schema{Type: schemav2.TypeString},
						Required: true,
					},
					"object_property_value": {Type: schemav2.TypeMap, Optional: true},
					"nested_resources": {
						Type:     schemav2.TypeList,
						MaxItems: 1,
						// Embed a `*schemav2.Resource` to validate that type directed
						// walk of the schema successfully walks inside Resources as well
						// as Schemas.
						Elem: &schemav2.Resource{
							Schema: map[string]*schemav2.Schema{
								"configuration": {Type: schemav2.TypeMap, Required: true},
							},
						},
						Optional: true,
					},
					"set_property_value": {
						Type:     schemav2.TypeSet,
						Elem:     &schemav2.Schema{Type: schemav2.TypeString},
						Optional: true,
						ForceNew: true,
					},
					"string_with_bad_interpolation": {Type: schemav2.TypeString, Optional: true},
				},
				SchemaVersion: 1,
				MigrateState: func(v int, is *terraformv2.InstanceState, p interface{}) (*terraformv2.InstanceState, error) {
					return is, nil
				},
				Create: func(data *schemav2.ResourceData, p interface{}) error {
					data.SetId("0")
					MustSetIfUnset(data, "bool_property_value", false)
					MustSetIfUnset(data, "number_property_value", 42)
					MustSetIfUnset(data, "float_property_value", 99.6767932)
					MustSetIfUnset(data, "string_property_value", "ognirts")
					MustSetIfUnset(data, "array_property_value", []interface{}{"an array"})
					MustSetIfUnset(data, "object_property_value", map[string]interface{}{
						"property_a": "a",
						"property_b": "true",
						"property.c": "some.value",
					})
					MustSetIfUnset(data, "nested_resources", []interface{}{
						map[string]interface{}{
							"configuration": map[string]interface{}{
								"configurationValue": "true",
							},
						},
					})
					MustSetIfUnset(data, "set_property_value", []interface{}{"set member 1", "set member 2"})
					MustSetIfUnset(data, "string_with_bad_interpolation", "some ${interpolated:value} with syntax errors")
					return nil
				},
				Read: func(data *schemav2.ResourceData, p interface{}) error {
					MustSetIfUnset(data, "bool_property_value", false)
					MustSetIfUnset(data, "number_property_value", 42)
					MustSetIfUnset(data, "float_property_value", 99.6767932)
					MustSetIfUnset(data, "string_property_value", "ognirts")
					MustSetIfUnset(data, "array_property_value", []interface{}{"an array"})
					MustSetIfUnset(data, "object_property_value", map[string]interface{}{
						"property_a": "a",
						"property_b": "true",
						"property.c": "some.value",
					})
					MustSetIfUnset(data, "nested_resources", []interface{}{
						map[string]interface{}{
							"configuration": map[string]interface{}{
								"configurationValue": "true",
							},
						},
					})
					MustSetIfUnset(data, "set_property_value", []interface{}{"set member 1", "set member 2"})
					MustSetIfUnset(data, "string_with_bad_interpolation", "some ${interpolated:value} with syntax errors")
					return nil
				},
				Update: func(data *schemav2.ResourceData, p interface{}) error {
					MustSetIfUnset(data, "bool_property_value", false)
					MustSetIfUnset(data, "number_property_value", 42)
					MustSetIfUnset(data, "float_property_value", 99.6767932)
					MustSetIfUnset(data, "string_property_value", "ognirts")
					MustSetIfUnset(data, "array_property_value", []interface{}{"an array"})
					MustSetIfUnset(data, "object_property_value", map[string]interface{}{
						"property_a": "a",
						"property_b": "true",
						"property.c": "some.value",
					})
					MustSetIfUnset(data, "nested_resources", []interface{}{
						map[string]interface{}{
							"configuration": map[string]interface{}{
								"configurationValue": "true",
							},
						},
					})
					MustSetIfUnset(data, "set_property_value", []interface{}{"set member 1", "set member 2"})
					MustSetIfUnset(data, "string_with_bad_interpolation", "some ${interpolated:value} with syntax errors")
					return nil
				},
				Delete: func(data *schemav2.ResourceData, p interface{}) error {
					return nil
				},
				Timeouts: &schemav2.ResourceTimeout{
					Create: Timeout(time.Second * 120),
					Update: Timeout(time.Second * 120),
				},
			},
		},
		DataSourcesMap: map[string]*schemav2.Resource{
			"example_resource": {
				Schema: map[string]*schemav2.Schema{
					"nil_property_value":    {Type: schemav2.TypeMap, Optional: true},
					"bool_property_value":   {Type: schemav2.TypeBool, Optional: true},
					"number_property_value": {Type: schemav2.TypeInt, Optional: true},
					"float_property_value":  {Type: schemav2.TypeFloat, Optional: true},
					"string_property_value": {Type: schemav2.TypeString, Optional: true},
					"array_property_value": {
						Type:     schemav2.TypeList,
						Elem:     &schemav2.Schema{Type: schemav2.TypeString},
						Required: true,
					},
					"object_property_value": {Type: schemav2.TypeMap, Optional: true},
					"map_property_value":    {Type: schemav2.TypeMap, Optional: true},
					"nested_resources": {
						Type:     schemav2.TypeList,
						MaxItems: 1,
						// Embed a `*schemav2.Resource` to validate that type directed
						// walk of the schema successfully walks inside Resources as well
						// as Schemas.
						Elem: &schemav2.Resource{
							Schema: map[string]*schemav2.Schema{
								"configuration": {Type: schemav2.TypeMap, Required: true},
							},
						},
						Optional: true,
					},
					"set_property_value": {
						Type:     schemav2.TypeSet,
						Elem:     &schemav2.Schema{Type: schemav2.TypeString},
						Optional: true,
					},
					"string_with_bad_interpolation": {Type: schemav2.TypeString, Optional: true},
				},
				SchemaVersion: 1,
				Read: func(data *schemav2.ResourceData, p interface{}) error {
					data.SetId("0")
					MustSetIfUnset(data, "bool_property_value", false)
					MustSetIfUnset(data, "number_property_value", 42)
					MustSetIfUnset(data, "float_property_value", 99.6767932)
					MustSetIfUnset(data, "string_property_value", "ognirts")
					MustSetIfUnset(data, "array_property_value", []interface{}{"an array"})
					MustSetIfUnset(data, "object_property_value", map[string]interface{}{
						"property_a": "a",
						"property_b": "true",
						"property.c": "some.value",
					})
					MustSetIfUnset(data, "nested_resources", []interface{}{
						map[string]interface{}{
							"configuration": map[string]interface{}{
								"configurationValue": "true",
							},
						},
					})
					MustSetIfUnset(data, "set_property_value", []interface{}{"set member 1", "set member 2"})
					MustSetIfUnset(data, "string_with_bad_interpolation", "some ${interpolated:value} with syntax errors")
					return nil
				},
			},
		},
		ConfigureFunc: func(data *schemav2.ResourceData) (interface{}, error) {
			return nil, nil
		},
	}
}
