package sdkv2

import (
	"encoding/json"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hexops/autogold/v2"
	"github.com/stretchr/testify/assert"
)

func TestToInstanceState(t *testing.T) {
	t.Parallel()
	res := newElemResource(&schema.Resource{
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
	autogold.Expect(map[string]interface{}{
		"array_property_value": []interface{}{"an array"},
		"bool_property_value":  false,
		"float_property_value": json.Number("99.6767932"),
		"id":                   "id",
		"map_property_value": map[string]interface{}{
			"propertyA": "a",
			"propertyB": true,
			"propertyC": map[string]interface{}{"nestedPropertyA": true},
		},
		"nested_resources": []interface{}{map[string]interface{}{
			"configuration": map[string]interface{}{"configurationValue": true},
		}},
		"nil_property_value":    nil,
		"number_property_value": json.Number("42"),
		"object_property_value": map[string]interface{}{
			"property_a": "a",
			"property_b": true,
		},
		"set_property_value": []interface{}{
			"set member 1",
			"set member 2",
		},
		"string_property_value":         "ognirts",
		"string_with_bad_interpolation": "some ${interpolated:value} with syntax errors",
	}).Equal(t, objectFromCtyValue(state.(*v2InstanceState2).stateValue))
}
