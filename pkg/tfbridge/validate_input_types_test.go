package tfbridge

import (
	"fmt"
	"testing"

	pschema "github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"
)

// TestValidateInputTypesV2 tests the validateInputTypesV2 function
func TestValidateInputType_objects(t *testing.T) {
	testCases := []struct {
		name     string
		typeRef  string
		typeName string
		input    resource.PropertyValue
		types    map[string]pschema.ComplexTypeSpec
		failures []TypeFailure
	}{
		{
			name:     "enum_string_success",
			typeRef:  "EnumType",
			typeName: "string",
			input:    resource.NewStringProperty("Value1"),
			types: map[string]pschema.ComplexTypeSpec{
				"pkg:index/type:EnumType": {
					ObjectTypeSpec: pschema.ObjectTypeSpec{
						Type: "string",
					},
					Enum: []pschema.EnumValueSpec{
						{Name: "Value1", Value: "v1"},
						{Name: "Value2", Value: "v2"},
					},
				},
			},
		},
		{
			name:     "enum_string_no_failure",
			typeRef:  "EnumType",
			typeName: "string",
			input:    resource.NewStringProperty("Value3"),
			types: map[string]pschema.ComplexTypeSpec{
				"pkg:index/type:EnumType": {
					ObjectTypeSpec: pschema.ObjectTypeSpec{
						Type: "string",
					},
					Enum: []pschema.EnumValueSpec{
						{Name: "Value1", Value: "v1"},
						{Name: "Value2", Value: "v2"},
					},
				},
			},
		},
		{
			name:     "enum_number_success",
			typeRef:  "NumberEnumType",
			typeName: "number",
			input:    resource.NewStringProperty("Value1"),
			types: map[string]pschema.ComplexTypeSpec{
				"pkg:index/type:NumberEnumType": {
					ObjectTypeSpec: pschema.ObjectTypeSpec{
						Type: "number",
					},
					Enum: []pschema.EnumValueSpec{
						{Name: "Value1", Value: 1},
						{Name: "Value2", Value: 2},
					},
				},
			},
		},
		{
			name:     "enum_number_no_failure",
			typeRef:  "NumberEnumType",
			typeName: "number",
			input:    resource.NewNumberProperty(3),
			types: map[string]pschema.ComplexTypeSpec{
				"pkg:index/type:NumberEnumType": {
					ObjectTypeSpec: pschema.ObjectTypeSpec{
						Type: "number",
					},
					Enum: []pschema.EnumValueSpec{
						{Name: "Value1", Value: 1},
						{Name: "Value2", Value: 2},
					},
				},
			},
		},
		{
			name:     "object_multi_type_success",
			typeRef:  "ObjectMultiType",
			typeName: "object",
			input: resource.NewObjectProperty(resource.NewPropertyMapFromMap(map[string]interface{}{
				"prop": "foo",
			})),
			types: map[string]pschema.ComplexTypeSpec{
				"pkg:index/type:ObjectMultiType": {
					ObjectTypeSpec: pschema.ObjectTypeSpec{
						Type: "object",
						Properties: map[string]pschema.PropertySpec{
							"prop": {
								TypeSpec: pschema.TypeSpec{
									OneOf: []pschema.TypeSpec{
										{
											Type: "string",
										},
										{
											Type: "number",
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name:     "object_multi_type_enum_success",
			typeRef:  "ObjectMultiType",
			typeName: "object",
			input: resource.NewObjectProperty(resource.NewPropertyMapFromMap(map[string]interface{}{
				"prop": "Value1",
			})),
			types: map[string]pschema.ComplexTypeSpec{
				"pkg:index/type:EnumType": {
					ObjectTypeSpec: pschema.ObjectTypeSpec{
						Type: "string",
					},
					Enum: []pschema.EnumValueSpec{
						{Name: "Value1", Value: "v1"},
						{Name: "Value2", Value: "v2"},
					},
				},
				"pkg:index/type:ObjectMultiType": {
					ObjectTypeSpec: pschema.ObjectTypeSpec{
						Type: "object",
						Properties: map[string]pschema.PropertySpec{
							"prop": {
								TypeSpec: pschema.TypeSpec{
									OneOf: []pschema.TypeSpec{
										{
											Type: "object",
											Ref:  "#/types/pkg:index/type:EnumType",
										},
										{
											Type: "number",
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name:     "object_multi_type_enum_no_failure",
			typeRef:  "ObjectMultiType",
			typeName: "object",
			input: resource.NewObjectProperty(resource.NewPropertyMapFromMap(map[string]interface{}{
				"prop": "foo",
			})),
			types: map[string]pschema.ComplexTypeSpec{
				"pkg:index/type:EnumType": {
					ObjectTypeSpec: pschema.ObjectTypeSpec{
						Type: "string",
					},
					Enum: []pschema.EnumValueSpec{
						{Name: "Value1", Value: "v1"},
						{Name: "Value2", Value: "v2"},
					},
				},
				"pkg:index/type:ObjectMultiType": {
					ObjectTypeSpec: pschema.ObjectTypeSpec{
						Type: "object",
						Properties: map[string]pschema.PropertySpec{
							"prop": {
								TypeSpec: pschema.TypeSpec{
									OneOf: []pschema.TypeSpec{
										{
											Type: "object",
											Ref:  "#/types/pkg:index/type:EnumType",
										},
										{
											Type: "number",
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name:     "object_multi_type_success_number",
			typeRef:  "ObjectMultiType",
			typeName: "object",
			input: resource.NewObjectProperty(resource.NewPropertyMapFromMap(map[string]interface{}{
				"prop": 1,
			})),
			types: map[string]pschema.ComplexTypeSpec{
				"pkg:index/type:ObjectMultiType": {
					ObjectTypeSpec: pschema.ObjectTypeSpec{
						Type: "object",
						Properties: map[string]pschema.PropertySpec{
							"prop": {
								TypeSpec: pschema.TypeSpec{
									OneOf: []pschema.TypeSpec{
										{
											Type: "string",
										},
										{
											Type: "number",
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name:     "object_multi_type_success_object",
			typeRef:  "ObjectMultiType",
			typeName: "object",
			input: resource.NewObjectProperty(resource.NewPropertyMapFromMap(map[string]interface{}{
				"prop": map[string]string{"foo": "bar"},
			})),
			types: map[string]pschema.ComplexTypeSpec{
				"pkg:index/type:ObjectMultiType": {
					ObjectTypeSpec: pschema.ObjectTypeSpec{
						Type: "object",
						Properties: map[string]pschema.PropertySpec{
							"prop": {
								TypeSpec: pschema.TypeSpec{
									OneOf: []pschema.TypeSpec{
										{
											Type: "string",
										},
										{
											Type: "object",
											AdditionalProperties: &pschema.TypeSpec{
												Type: "string",
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name:     "object_multi_type_failure_object",
			typeRef:  "ObjectMultiType",
			typeName: "object",
			input: resource.NewObjectProperty(resource.NewPropertyMapFromMap(map[string]interface{}{
				"prop": []interface{}{"foo"},
			})),
			failures: []TypeFailure{
				{Reason: "expected string type, got [] type", ResourcePath: "object_multi_type_failure_object.prop"},
				{Reason: "expected object type, got [] type", ResourcePath: "object_multi_type_failure_object.prop"},
			},
			types: map[string]pschema.ComplexTypeSpec{
				"pkg:index/type:ObjectMultiType": {
					ObjectTypeSpec: pschema.ObjectTypeSpec{
						Type: "object",
						Properties: map[string]pschema.PropertySpec{
							"prop": {
								TypeSpec: pschema.TypeSpec{
									OneOf: []pschema.TypeSpec{
										{
											Type: "string",
										},
										{
											Type: "object",
											AdditionalProperties: &pschema.TypeSpec{
												Type: "string",
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name:     "top_level_type_failure",
			typeRef:  "ObjectMultiType",
			typeName: "object",
			input: resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewObjectProperty(resource.NewPropertyMapFromMap(map[string]interface{}{
					"prop": []string{"foo"},
				})),
			}),
			failures: []TypeFailure{
				{Reason: "expected object type, got [] type", ResourcePath: "top_level_type_failure"},
			},
			types: map[string]pschema.ComplexTypeSpec{
				"pkg:index/type:ObjectMultiType": {
					ObjectTypeSpec: pschema.ObjectTypeSpec{
						Type: "object",
						Properties: map[string]pschema.PropertySpec{
							"prop": {
								TypeSpec: pschema.TypeSpec{
									OneOf: []pschema.TypeSpec{
										{
											Type: "string",
										},
										{
											Type: "number",
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name:     "object_multi_type_failure",
			typeRef:  "ObjectMultiType",
			typeName: "object",
			input: resource.NewObjectProperty(resource.NewPropertyMapFromMap(map[string]interface{}{
				"prop": []string{"foo"},
			})),
			failures: []TypeFailure{
				{
					Reason:       "expected string type, got [] type",
					ResourcePath: "object_multi_type_failure.prop",
				},
				{
					Reason:       "expected number type, got [] type",
					ResourcePath: "object_multi_type_failure.prop",
				},
			},
			types: map[string]pschema.ComplexTypeSpec{
				"pkg:index/type:ObjectMultiType": {
					ObjectTypeSpec: pschema.ObjectTypeSpec{
						Type: "object",
						Properties: map[string]pschema.PropertySpec{
							"prop": {
								TypeSpec: pschema.TypeSpec{
									OneOf: []pschema.TypeSpec{
										{
											Type: "string",
										},
										{
											Type: "number",
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name:     "object_string_type_success",
			typeRef:  "ObjectStringType",
			typeName: "object",
			input: resource.NewObjectProperty(resource.NewPropertyMapFromMap(map[string]interface{}{
				"objectStringProp": "foo",
			})),
			types: map[string]pschema.ComplexTypeSpec{
				"pkg:index/type:ObjectStringType": {
					ObjectTypeSpec: pschema.ObjectTypeSpec{
						Type: "object",
						Properties: map[string]pschema.PropertySpec{
							"objectStringProp": {
								TypeSpec: pschema.TypeSpec{
									Type: "string",
								},
							},
						},
					},
				},
			},
		},
		{
			name:     "object_string_type_failure",
			typeRef:  "ObjectStringType",
			typeName: "object",
			input: resource.NewObjectProperty(resource.NewPropertyMapFromMap(map[string]interface{}{
				"objectStringProp": map[string]string{"foo": "bar"},
			})),
			failures: []TypeFailure{
				{
					Reason:       "expected string type, got object type",
					ResourcePath: "object_string_type_failure.objectStringProp",
				},
			},
			types: map[string]pschema.ComplexTypeSpec{
				"pkg:index/type:ObjectStringType": {
					ObjectTypeSpec: pschema.ObjectTypeSpec{
						Type: "object",
						Properties: map[string]pschema.PropertySpec{
							"objectStringProp": {
								TypeSpec: pschema.TypeSpec{
									Type: "string",
								},
							},
						},
					},
				},
			},
		},
		{
			name:     "object_string_type_failure_name",
			typeRef:  "ObjectStringType",
			typeName: "object",
			input: resource.NewObjectProperty(resource.NewPropertyMapFromMap(map[string]interface{}{
				"objectStringProp1": "foo",
			})),
			failures: []TypeFailure{
				{
					Reason:       "property objectStringProp1 is not defined in the schema",
					ResourcePath: "object_string_type_failure_name",
				},
			},
			types: map[string]pschema.ComplexTypeSpec{
				"pkg:index/type:ObjectStringType": {
					ObjectTypeSpec: pschema.ObjectTypeSpec{
						Type: "object",
						Properties: map[string]pschema.PropertySpec{
							"objectStringProp": {
								TypeSpec: pschema.TypeSpec{
									Type: "string",
								},
							},
						},
					},
				},
			},
		},
		{
			name:     "object_nested_object_type_success",
			typeRef:  "ObjectNestedObjectType",
			typeName: "object",
			input: resource.NewObjectProperty(resource.NewPropertyMapFromMap(map[string]interface{}{
				"prop": map[string]interface{}{
					"foo": "foo",
				},
			})),
			types: map[string]pschema.ComplexTypeSpec{
				"pkg:index/type:ObjectNestedObjectType": {
					ObjectTypeSpec: pschema.ObjectTypeSpec{
						Type: "object",
						Properties: map[string]pschema.PropertySpec{
							"prop": {
								TypeSpec: pschema.TypeSpec{
									Type: "object",
									// not using ref to test arbitrary object keys
									AdditionalProperties: &pschema.TypeSpec{
										Type: "string",
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name:     "object_nested_object_type_failure",
			typeRef:  "ObjectNestedObjectType",
			typeName: "object",
			input: resource.NewObjectProperty(resource.NewPropertyMapFromMap(map[string]interface{}{
				"prop": []map[string]interface{}{
					{"foo": "bar"},
				},
			})),
			failures: []TypeFailure{
				{
					Reason:       "expected object type, got [] type",
					ResourcePath: "object_nested_object_type_failure.prop",
				},
			},
			types: map[string]pschema.ComplexTypeSpec{
				"pkg:index/type:ObjectNestedObjectType": {
					ObjectTypeSpec: pschema.ObjectTypeSpec{
						Type: "object",
						Properties: map[string]pschema.PropertySpec{
							"prop": {
								TypeSpec: pschema.TypeSpec{
									Type: "object",
									// not using ref to test arbitrary object keys
									AdditionalProperties: &pschema.TypeSpec{
										Type: "string",
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name:     "object_double_nested_object_type_required_failure",
			typeRef:  "ObjectDoubleNestedObjectType",
			typeName: "object",
			input: resource.NewObjectProperty(resource.NewPropertyMapFromMap(map[string]interface{}{
				"prop": map[string]interface{}{},
			})),
			failures: []TypeFailure{
				{
					Reason:       "object_double_nested_object_type_required_failure.prop object is missing required properties: objectStringProp",
					ResourcePath: "object_double_nested_object_type_required_failure.prop",
				},
			},
			types: map[string]pschema.ComplexTypeSpec{
				"pkg:index/type:ObjectStringType": {
					ObjectTypeSpec: pschema.ObjectTypeSpec{
						Type:     "object",
						Required: []string{"objectStringProp"},
						Properties: map[string]pschema.PropertySpec{
							"objectStringProp": {
								TypeSpec: pschema.TypeSpec{
									Type: "object",
									AdditionalProperties: &pschema.TypeSpec{
										Type: "string",
									},
								},
							},
						},
					},
				},
				"pkg:index/type:ObjectDoubleNestedObjectType": {
					ObjectTypeSpec: pschema.ObjectTypeSpec{
						Type: "object",
						Properties: map[string]pschema.PropertySpec{
							"prop": {
								TypeSpec: pschema.TypeSpec{
									Type: "object",
									// not using ref to test arbitrary object keys
									AdditionalProperties: &pschema.TypeSpec{
										Type: "object",
										Ref:  "#/types/pkg:index/type:ObjectStringType",
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name:     "object_double_nested_object_type_required_failure_2",
			typeRef:  "ObjectDoubleNestedObjectType",
			typeName: "object",
			input: resource.NewObjectProperty(resource.NewPropertyMapFromMap(map[string]interface{}{
				"prop": []map[string]interface{}{
					{"objectStringProp": "foo", "foo": "bar"},
					{"foo": "bar"},
					{"bar": "baz"},
				},
			})),
			failures: []TypeFailure{
				{
					Reason:       "object_double_nested_object_type_required_failure_2.prop[1] object is missing required property: objectStringProp",
					ResourcePath: "object_double_nested_object_type_required_failure_2.prop[1]",
				},
				{
					Reason:       "object_double_nested_object_type_required_failure_2.prop[2] object is missing required property: objectStringProp",
					ResourcePath: "object_double_nested_object_type_required_failure_2.prop[2]",
				},
				{
					Reason:       "object_double_nested_object_type_required_failure_2.prop[2] object is missing required property: foo",
					ResourcePath: "object_double_nested_object_type_required_failure_2.prop[2]",
				},
			},
			types: map[string]pschema.ComplexTypeSpec{
				"pkg:index/type:ObjectStringType": {
					ObjectTypeSpec: pschema.ObjectTypeSpec{
						Type:     "object",
						Required: []string{"objectStringProp", "foo"},
						Properties: map[string]pschema.PropertySpec{
							"objectStringProp": {
								TypeSpec: pschema.TypeSpec{
									Type: "string",
								},
							},
							"foo": {
								TypeSpec: pschema.TypeSpec{
									Type: "string",
								},
							},
						},
					},
				},
				"pkg:index/type:ObjectDoubleNestedObjectType": {
					ObjectTypeSpec: pschema.ObjectTypeSpec{
						Type: "object",
						Properties: map[string]pschema.PropertySpec{
							"prop": {
								TypeSpec: pschema.TypeSpec{
									Type: "array",
									Items: &pschema.TypeSpec{
										Type: "object",
										// not using ref to test arbitrary object keys
										AdditionalProperties: &pschema.TypeSpec{
											Type: "object",
											Ref:  "#/types/pkg:index/type:ObjectStringType",
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name:     "object_double_nested_object_type_success",
			typeRef:  "ObjectDoubleNestedObjectType",
			typeName: "object",
			input: resource.NewObjectProperty(resource.NewPropertyMapFromMap(map[string]interface{}{
				"prop": map[string]interface{}{
					"objectStringProp": map[string]interface{}{
						"bar": "baz",
					},
				},
			})),
			types: map[string]pschema.ComplexTypeSpec{
				"pkg:index/type:ObjectStringType": {
					ObjectTypeSpec: pschema.ObjectTypeSpec{
						Type: "object",
						Properties: map[string]pschema.PropertySpec{
							"objectStringProp": {
								TypeSpec: pschema.TypeSpec{
									Type: "object",
									AdditionalProperties: &pschema.TypeSpec{
										Type: "string",
									},
								},
							},
						},
					},
				},
				"pkg:index/type:ObjectDoubleNestedObjectType": {
					ObjectTypeSpec: pschema.ObjectTypeSpec{
						Type: "object",
						Properties: map[string]pschema.PropertySpec{
							"prop": {
								TypeSpec: pschema.TypeSpec{
									Type: "object",
									// not using ref to test arbitrary object keys
									AdditionalProperties: &pschema.TypeSpec{
										Type: "object",
										Ref:  "#/types/pkg:index/type:ObjectStringType",
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name:     "object_double_nested_object_type_failure_name",
			typeRef:  "ObjectDoubleNestedObjectType",
			typeName: "object",
			input: resource.NewObjectProperty(resource.NewPropertyMapFromMap(map[string]interface{}{
				"prop": map[string]interface{}{
					"foo": map[string]interface{}{
						"bar": "baz",
					},
				},
			})),
			failures: []TypeFailure{
				{
					Reason:       "property foo is not defined in the schema",
					ResourcePath: "object_double_nested_object_type_failure_name.prop",
				},
			},
			types: map[string]pschema.ComplexTypeSpec{
				"pkg:index/type:ObjectStringType": {
					ObjectTypeSpec: pschema.ObjectTypeSpec{
						Type: "object",
						Properties: map[string]pschema.PropertySpec{
							"objectStringProp": {
								TypeSpec: pschema.TypeSpec{
									Type: "object",
									AdditionalProperties: &pschema.TypeSpec{
										Type: "string",
									},
								},
							},
						},
					},
				},
				"pkg:index/type:ObjectDoubleNestedObjectType": {
					ObjectTypeSpec: pschema.ObjectTypeSpec{
						Type: "object",
						Properties: map[string]pschema.PropertySpec{
							"prop": {
								TypeSpec: pschema.TypeSpec{
									Type: "object",
									// not using ref to test arbitrary object keys
									AdditionalProperties: &pschema.TypeSpec{
										Type: "object",
										Ref:  "#/types/pkg:index/type:ObjectStringType",
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name:     "object_double_nested_object_type_failure",
			typeRef:  "ObjectDoubleNestedObjectType",
			typeName: "object",
			input: resource.NewObjectProperty(resource.NewPropertyMapFromMap(map[string]interface{}{
				"prop": map[string]interface{}{
					"objectStringProp": "foo",
				},
			})),
			failures: []TypeFailure{
				{
					Reason:       "expected object type, got string type",
					ResourcePath: "object_double_nested_object_type_failure.prop.objectStringProp",
				},
			},
			types: map[string]pschema.ComplexTypeSpec{
				"pkg:index/type:ObjectStringType": {
					ObjectTypeSpec: pschema.ObjectTypeSpec{
						Type: "object",
						Properties: map[string]pschema.PropertySpec{
							"objectStringProp": {
								TypeSpec: pschema.TypeSpec{
									Type: "object",
									AdditionalProperties: &pschema.TypeSpec{
										Type: "string",
									},
								},
							},
						},
					},
				},
				"pkg:index/type:ObjectDoubleNestedObjectType": {
					ObjectTypeSpec: pschema.ObjectTypeSpec{
						Type: "object",
						Properties: map[string]pschema.PropertySpec{
							"prop": {
								TypeSpec: pschema.TypeSpec{
									Type: "object",
									// not using ref to test arbitrary object keys
									AdditionalProperties: &pschema.TypeSpec{
										Type: "object",
										Ref:  "#/types/pkg:index/type:ObjectStringType",
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name:     "object_nested_array_type_success",
			typeRef:  "ObjectNestedArrayType",
			typeName: "object",
			input: resource.NewObjectProperty(resource.NewPropertyMapFromMap(map[string]interface{}{
				"prop": []string{
					"foo",
				},
			})),
			types: map[string]pschema.ComplexTypeSpec{
				"pkg:index/type:ObjectNestedArrayType": {
					ObjectTypeSpec: pschema.ObjectTypeSpec{
						Type: "object",
						Properties: map[string]pschema.PropertySpec{
							"prop": {
								TypeSpec: pschema.TypeSpec{
									Type: "array",
									Items: &pschema.TypeSpec{
										Type: "string",
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name:     "object_nested_array_type_failure",
			typeRef:  "ObjectNestedArrayType",
			typeName: "object",
			input: resource.NewObjectProperty(resource.NewPropertyMapFromMap(map[string]interface{}{
				"prop": map[string]string{"foo": "bar"},
			})),
			failures: []TypeFailure{
				{
					Reason:       "expected array type, got object type",
					ResourcePath: "object_nested_array_type_failure.prop",
				},
			},
			types: map[string]pschema.ComplexTypeSpec{
				"pkg:index/type:ObjectNestedArrayType": {
					ObjectTypeSpec: pschema.ObjectTypeSpec{
						Type: "object",
						Properties: map[string]pschema.PropertySpec{
							"prop": {
								TypeSpec: pschema.TypeSpec{
									Type: "array",
									Items: &pschema.TypeSpec{
										Type: "string",
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name:     "object_nested_array_object_type_success",
			typeRef:  "ObjectNestedArrayObjectType",
			typeName: "object",
			input: resource.NewObjectProperty(resource.NewPropertyMapFromMap(map[string]interface{}{
				"prop": []resource.PropertyValue{
					resource.NewObjectProperty(resource.NewPropertyMapFromMap(map[string]interface{}{
						"objectStringProp": "foo",
					})),
				},
			})),
			types: map[string]pschema.ComplexTypeSpec{
				"pkg:index/type:ObjectStringType": {
					ObjectTypeSpec: pschema.ObjectTypeSpec{
						Type: "object",
						Properties: map[string]pschema.PropertySpec{
							"objectStringProp": {
								TypeSpec: pschema.TypeSpec{
									Type: "string",
								},
							},
						},
					},
				},
				"pkg:index/type:ObjectNestedArrayObjectType": {
					ObjectTypeSpec: pschema.ObjectTypeSpec{
						Type: "object",
						Properties: map[string]pschema.PropertySpec{
							"prop": {
								TypeSpec: pschema.TypeSpec{
									Type: "array",
									Items: &pschema.TypeSpec{
										Type: "object",
										// using ref to test specific object keys
										Ref: "#/types/pkg:index/type:ObjectStringType",
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name:     "object_nested_array_object_type_failure",
			typeRef:  "ObjectNestedArrayObjectType",
			typeName: "object",
			input: resource.NewObjectProperty(resource.NewPropertyMapFromMap(map[string]interface{}{
				"prop": []map[string]interface{}{
					{"objectStringProp": []string{"foo"}},
				},
			})),
			failures: []TypeFailure{
				{
					Reason:       "expected string type, got [] type",
					ResourcePath: "object_nested_array_object_type_failure.prop[0].objectStringProp",
				},
			},
			types: map[string]pschema.ComplexTypeSpec{
				"pkg:index/type:ObjectStringType": {
					ObjectTypeSpec: pschema.ObjectTypeSpec{
						Type: "object",
						Properties: map[string]pschema.PropertySpec{
							"objectStringProp": {
								TypeSpec: pschema.TypeSpec{
									Type: "string",
								},
							},
						},
					},
				},
				"pkg:index/type:ObjectNestedArrayObjectType": {
					ObjectTypeSpec: pschema.ObjectTypeSpec{
						Type: "object",
						Properties: map[string]pschema.PropertySpec{
							"prop": {
								TypeSpec: pschema.TypeSpec{
									Type: "array",
									Items: &pschema.TypeSpec{
										Type: "object",
										// using ref to test specific object keys
										Ref: "#/types/pkg:index/type:ObjectStringType",
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name:     "object_multi_type_nested_success",
			typeRef:  "ObjectNestedArrayObjectType",
			typeName: "object",
			input: resource.NewObjectProperty(resource.NewPropertyMapFromMap(map[string]interface{}{
				"prop": []map[string]interface{}{
					{"objectStringProp": "foo"},
				},
			})),
			types: map[string]pschema.ComplexTypeSpec{
				"pkg:index/type:ObjectStringType": {
					ObjectTypeSpec: pschema.ObjectTypeSpec{
						Type: "object",
						Properties: map[string]pschema.PropertySpec{
							"objectStringProp": {
								TypeSpec: pschema.TypeSpec{
									Type: "string",
								},
							},
						},
					},
				},
				"pkg:index/type:ObjectNestedArrayObjectType": {
					ObjectTypeSpec: pschema.ObjectTypeSpec{
						Type: "object",
						Properties: map[string]pschema.PropertySpec{
							"prop": {
								TypeSpec: pschema.TypeSpec{
									OneOf: []pschema.TypeSpec{
										{
											Type: "array",
											Items: &pschema.TypeSpec{
												Type: "object",
												// using ref to test specific object keys
												Ref: "#/types/pkg:index/type:ObjectStringType",
											},
										},
										{
											Type: "object",
											AdditionalProperties: &pschema.TypeSpec{
												OneOf: []pschema.TypeSpec{
													{
														Type: "string",
													},
													{
														Type: "object",
														AdditionalProperties: &pschema.TypeSpec{
															Type: "string",
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name:     "object_multi_type_nested_success2",
			typeRef:  "ObjectNestedArrayObjectType",
			typeName: "object",
			input: resource.NewObjectProperty(resource.NewPropertyMapFromMap(map[string]interface{}{
				"prop": map[string]interface{}{
					"objectStringProp": "foo",
					"foo":              map[string]interface{}{"bar": "baz"},
				},
			})),
			types: map[string]pschema.ComplexTypeSpec{
				"pkg:index/type:ObjectStringType": {
					ObjectTypeSpec: pschema.ObjectTypeSpec{
						Type: "object",
						Properties: map[string]pschema.PropertySpec{
							"objectStringProp": {
								TypeSpec: pschema.TypeSpec{
									Type: "string",
								},
							},
						},
					},
				},
				"pkg:index/type:ObjectNestedArrayObjectType": {
					ObjectTypeSpec: pschema.ObjectTypeSpec{
						Type: "object",
						Properties: map[string]pschema.PropertySpec{
							"prop": {
								TypeSpec: pschema.TypeSpec{
									OneOf: []pschema.TypeSpec{
										{
											Type: "array",
											Items: &pschema.TypeSpec{
												Type: "object",
												// using ref to test specific object keys
												Ref: "#/types/pkg:index/type:ObjectStringType",
											},
										},
										{
											Type: "object",
											AdditionalProperties: &pschema.TypeSpec{
												OneOf: []pschema.TypeSpec{
													{
														Type: "string",
													},
													{
														Type: "object",
														AdditionalProperties: &pschema.TypeSpec{
															Type: "string",
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name:     "object_multi_type_nested_failure2",
			typeRef:  "ObjectNestedArrayObjectType",
			typeName: "object",
			input: resource.NewObjectProperty(resource.NewPropertyMapFromMap(map[string]interface{}{
				"prop": map[string]interface{}{
					"objectStringProp": "foo",
					"foo":              map[string]interface{}{"bar": "baz"},
					"bar":              []interface{}{1},
				},
			})),
			failures: []TypeFailure{
				{
					Reason:       "expected array type, got object type",
					ResourcePath: "object_multi_type_nested_failure2.prop",
				},
				{
					Reason:       "expected string type, got [] type",
					ResourcePath: "object_multi_type_nested_failure2.prop.bar",
				},
				{
					Reason:       "expected object type, got [] type",
					ResourcePath: "object_multi_type_nested_failure2.prop.bar",
				},
			},
			types: map[string]pschema.ComplexTypeSpec{
				"pkg:index/type:ObjectStringType": {
					ObjectTypeSpec: pschema.ObjectTypeSpec{
						Type: "object",
						Properties: map[string]pschema.PropertySpec{
							"objectStringProp": {
								TypeSpec: pschema.TypeSpec{
									Type: "string",
								},
							},
						},
					},
				},
				"pkg:index/type:ObjectNestedArrayObjectType": {
					ObjectTypeSpec: pschema.ObjectTypeSpec{
						Type: "object",
						Properties: map[string]pschema.PropertySpec{
							"prop": {
								TypeSpec: pschema.TypeSpec{
									OneOf: []pschema.TypeSpec{
										{
											Type: "array",
											Items: &pschema.TypeSpec{
												Type: "object",
												// using ref to test specific object keys
												Ref: "#/types/pkg:index/type:ObjectStringType",
											},
										},
										{
											Type: "object",
											AdditionalProperties: &pschema.TypeSpec{
												OneOf: []pschema.TypeSpec{
													{
														Type: "string",
													},
													{
														Type: "object",
														AdditionalProperties: &pschema.TypeSpec{
															Type: "string",
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name:     "object_multi_type_nested_failure",
			typeRef:  "ObjectNestedArrayObjectType",
			typeName: "object",
			input: resource.NewObjectProperty(resource.NewPropertyMapFromMap(map[string]interface{}{
				"prop": []map[string]interface{}{
					{"objectStringProp": []string{"foo"}},
				},
			})),
			failures: []TypeFailure{
				{
					Reason:       "expected string type, got [] type",
					ResourcePath: "object_multi_type_nested_failure.prop[0].objectStringProp",
				},
				{
					Reason:       "expected object type, got [] type",
					ResourcePath: "object_multi_type_nested_failure.prop",
				},
			},
			types: map[string]pschema.ComplexTypeSpec{
				"pkg:index/type:ObjectStringType": {
					ObjectTypeSpec: pschema.ObjectTypeSpec{
						Type: "object",
						Properties: map[string]pschema.PropertySpec{
							"objectStringProp": {
								TypeSpec: pschema.TypeSpec{
									Type: "string",
								},
							},
						},
					},
				},
				"pkg:index/type:ObjectNestedArrayObjectType": {
					ObjectTypeSpec: pschema.ObjectTypeSpec{
						Type: "object",
						Properties: map[string]pschema.PropertySpec{
							"prop": {
								TypeSpec: pschema.TypeSpec{
									OneOf: []pschema.TypeSpec{
										{
											Type: "array",
											Items: &pschema.TypeSpec{
												Type: "object",
												// using ref to test specific object keys
												Ref: "#/types/pkg:index/type:ObjectStringType",
											},
										},
										{
											Type: "object",
											AdditionalProperties: &pschema.TypeSpec{
												Type: "string",
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			pspec := pschema.PackageSpec{
				Name:  "test",
				Types: tc.types,
				Resources: map[string]pschema.ResourceSpec{
					"pkg:mod:ResA": {
						InputProperties: map[string]pschema.PropertySpec{
							tc.name: {
								TypeSpec: pschema.TypeSpec{
									Type: tc.typeName,
									Ref:  fmt.Sprintf("#/types/pkg:index/type:%s", tc.typeRef),
								},
							},
						},
					},
				},
			}
			pathBuilder := resource.PropertyPath{}
			urn := resource.CreateURN(
				"testResource",
				"pkg:mod:ResA",
				"",
				"stack",
				"project",
			)

			v := &PulumiInputValidator{
				urn:    urn,
				schema: pspec,
			}
			failures := v.validateResourceInputType(tc.name, tc.input, pathBuilder)
			if failures != nil && len(*failures) != len(tc.failures) {
				t.Fatalf("%d failures, got %d: %v", len(tc.failures), len(*failures), *failures)
			}
			if len(tc.failures) > 0 {
				if failures == nil {
					t.Fatalf("expected failures, got none")
				} else {
					assert.Equal(t, tc.failures, *failures)
				}
			}
		})
	}
}

func TestValidateInputType_arrays(t *testing.T) {
	testCases := []struct {
		name     string
		typeRef  string
		typeName string
		input    resource.PropertyValue
		failures []TypeFailure
		types    map[string]pschema.ComplexTypeSpec
	}{
		{
			name:     "enum_string_success",
			typeRef:  "EnumType",
			typeName: "string",
			input: resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewStringProperty("Value1"),
			}),
			types: map[string]pschema.ComplexTypeSpec{
				"pkg:index/type:EnumType": {
					ObjectTypeSpec: pschema.ObjectTypeSpec{
						Type: "string",
					},
					Enum: []pschema.EnumValueSpec{
						{Name: "Value1", Value: "v1"},
						{Name: "Value2", Value: "v2"},
					},
				},
			},
		},
		{
			name:     "enum_number_success",
			typeRef:  "NumberEnumType",
			typeName: "number",
			input: resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewStringProperty("Value1"),
			}),
			types: map[string]pschema.ComplexTypeSpec{
				"pkg:index/type:NumberEnumType": {
					ObjectTypeSpec: pschema.ObjectTypeSpec{
						Type: "number",
					},
					Enum: []pschema.EnumValueSpec{
						{Name: "Value1", Value: 1},
						{Name: "Value2", Value: 2},
					},
				},
			},
		},
		{
			name:     "object_multi_type_success",
			typeRef:  "ObjectMultiType",
			typeName: "object",
			input: resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewObjectProperty(resource.NewPropertyMapFromMap(map[string]interface{}{
					"prop": "foo",
				})),
			}),
			types: map[string]pschema.ComplexTypeSpec{
				"pkg:index/type:ObjectMultiType": {
					ObjectTypeSpec: pschema.ObjectTypeSpec{
						Type: "object",
						Properties: map[string]pschema.PropertySpec{
							"prop": {
								TypeSpec: pschema.TypeSpec{
									OneOf: []pschema.TypeSpec{
										{
											Type: "string",
										},
										{
											Type: "number",
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name:     "object_multi_type_success_number",
			typeRef:  "ObjectMultiType",
			typeName: "object",
			input: resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewObjectProperty(resource.NewPropertyMapFromMap(map[string]interface{}{
					"prop": 1,
				})),
			}),
			types: map[string]pschema.ComplexTypeSpec{
				"pkg:index/type:ObjectMultiType": {
					ObjectTypeSpec: pschema.ObjectTypeSpec{
						Type: "object",
						Properties: map[string]pschema.PropertySpec{
							"prop": {
								TypeSpec: pschema.TypeSpec{
									OneOf: []pschema.TypeSpec{
										{
											Type: "string",
										},
										{
											Type: "number",
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name:     "object_string_type_success",
			typeRef:  "ObjectStringType",
			typeName: "object",
			input: resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewObjectProperty(resource.NewPropertyMapFromMap(map[string]interface{}{
					"objectStringProp": "foo",
				})),
			}),
			types: map[string]pschema.ComplexTypeSpec{
				"pkg:index/type:ObjectStringType": {
					ObjectTypeSpec: pschema.ObjectTypeSpec{
						Type: "object",
						Properties: map[string]pschema.PropertySpec{
							"objectStringProp": {
								TypeSpec: pschema.TypeSpec{
									Type: "string",
								},
							},
						},
					},
				},
			},
		},
		{
			name:     "object_string_type_failure",
			typeRef:  "ObjectStringType",
			typeName: "object",
			input: resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewObjectProperty(resource.NewPropertyMapFromMap(map[string]interface{}{
					"objectStringProp": "foo",
				})),
				resource.NewObjectProperty(resource.NewPropertyMapFromMap(map[string]interface{}{
					"objectStringProp": []interface{}{1},
				})),
				resource.NewStringProperty("foo"),
			}),
			failures: []TypeFailure{
				{
					Reason:       "expected string type, got [] type",
					ResourcePath: "object_string_type_failure[1].objectStringProp",
				},
				{
					Reason:       "expected object type, got string type",
					ResourcePath: "object_string_type_failure[2]",
				},
			},
			types: map[string]pschema.ComplexTypeSpec{
				"pkg:index/type:ObjectStringType": {
					ObjectTypeSpec: pschema.ObjectTypeSpec{
						Type: "object",
						Properties: map[string]pschema.PropertySpec{
							"objectStringProp": {
								TypeSpec: pschema.TypeSpec{
									Type: "string",
								},
							},
						},
					},
				},
			},
		},
		{
			name:     "object_nested_object_type_success",
			typeRef:  "ObjectNestedObjectType",
			typeName: "object",
			input: resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewObjectProperty(resource.NewPropertyMapFromMap(map[string]interface{}{
					"prop": map[string]interface{}{
						"foo": "foo",
					},
				})),
			}),
			types: map[string]pschema.ComplexTypeSpec{
				"pkg:index/type:ObjectNestedObjectType": {
					ObjectTypeSpec: pschema.ObjectTypeSpec{
						Type: "object",
						Properties: map[string]pschema.PropertySpec{
							"prop": {
								TypeSpec: pschema.TypeSpec{
									Type: "object",
									// not using ref to test arbitrary object keys
									AdditionalProperties: &pschema.TypeSpec{
										Type: "string",
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name:     "object_nested_object_type_failure",
			typeRef:  "ObjectNestedObjectType",
			typeName: "object",
			input: resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewObjectProperty(resource.NewPropertyMapFromMap(map[string]interface{}{
					"prop": map[string]interface{}{
						"foo": "foo",
					},
				})),
				resource.NewObjectProperty(resource.NewPropertyMapFromMap(map[string]interface{}{
					"prop": map[string]interface{}{
						"foo": []interface{}{1},
					},
				})),
			}),
			failures: []TypeFailure{
				{
					Reason:       "expected string type, got [] type",
					ResourcePath: "object_nested_object_type_failure[1].prop.foo",
				},
			},
			types: map[string]pschema.ComplexTypeSpec{
				"pkg:index/type:ObjectNestedObjectType": {
					ObjectTypeSpec: pschema.ObjectTypeSpec{
						Type: "object",
						Properties: map[string]pschema.PropertySpec{
							"prop": {
								TypeSpec: pschema.TypeSpec{
									Type: "object",
									// not using ref to test arbitrary object keys
									AdditionalProperties: &pschema.TypeSpec{
										Type: "string",
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name:     "object_double_nested_object_type_success",
			typeRef:  "ObjectDoubleNestedObjectType",
			typeName: "object",
			input: resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewObjectProperty(resource.NewPropertyMapFromMap(map[string]interface{}{
					"prop": map[string]interface{}{
						"objectStringProp": "foo",
					},
				})),
			}),
			types: map[string]pschema.ComplexTypeSpec{
				"pkg:index/type:ObjectStringType": {
					ObjectTypeSpec: pschema.ObjectTypeSpec{
						Type: "object",
						Properties: map[string]pschema.PropertySpec{
							"objectStringProp": {
								TypeSpec: pschema.TypeSpec{
									Type: "string",
								},
							},
						},
					},
				},
				"pkg:index/type:ObjectDoubleNestedObjectType": {
					ObjectTypeSpec: pschema.ObjectTypeSpec{
						Type: "object",
						Properties: map[string]pschema.PropertySpec{
							"prop": {
								TypeSpec: pschema.TypeSpec{
									Type: "object",
									// not using ref to test arbitrary object keys
									AdditionalProperties: &pschema.TypeSpec{
										Type: "object",
										Ref:  "#/types/pkg:index/type:ObjectStringType",
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name:     "object_double_nested_object_type_failure_name",
			typeRef:  "ObjectDoubleNestedObjectType",
			typeName: "object",
			input: resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewObjectProperty(resource.NewPropertyMapFromMap(map[string]interface{}{
					"prop": map[string]interface{}{
						"foo": map[string]interface{}{
							"bar": "baz",
						},
					},
				})),
			}),
			failures: []TypeFailure{
				{
					Reason:       "property foo is not defined in the schema",
					ResourcePath: "object_double_nested_object_type_failure_name[0].prop",
				},
			},
			types: map[string]pschema.ComplexTypeSpec{
				"pkg:index/type:ObjectStringType": {
					ObjectTypeSpec: pschema.ObjectTypeSpec{
						Type: "object",
						Properties: map[string]pschema.PropertySpec{
							"objectStringProp": {
								TypeSpec: pschema.TypeSpec{
									Type: "string",
								},
							},
						},
					},
				},
				"pkg:index/type:ObjectDoubleNestedObjectType": {
					ObjectTypeSpec: pschema.ObjectTypeSpec{
						Type: "object",
						Properties: map[string]pschema.PropertySpec{
							"prop": {
								TypeSpec: pschema.TypeSpec{
									Type: "object",
									// not using ref to test arbitrary object keys
									AdditionalProperties: &pschema.TypeSpec{
										Type: "object",
										Ref:  "#/types/pkg:index/type:ObjectStringType",
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name:     "object_double_nested_object_type_no_ref_success",
			typeRef:  "ObjectDoubleNestedObjectType",
			typeName: "object",
			input: resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewObjectProperty(resource.NewPropertyMapFromMap(map[string]interface{}{
					"prop": map[string]interface{}{
						"foo": map[string]interface{}{
							"bar": "baz",
						},
					},
				})),
			}),
			types: map[string]pschema.ComplexTypeSpec{
				"pkg:index/type:ObjectDoubleNestedObjectType": {
					ObjectTypeSpec: pschema.ObjectTypeSpec{
						Type: "object",
						Properties: map[string]pschema.PropertySpec{
							"prop": {
								TypeSpec: pschema.TypeSpec{
									Type: "object",
									// not using ref to test arbitrary object keys
									AdditionalProperties: &pschema.TypeSpec{
										Type: "object",
										Ref:  "#/types/pkg:index/type:ObjectStringType",
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name:     "object_nested_array_type_success",
			typeRef:  "ObjectNestedArrayType",
			typeName: "object",
			input: resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewObjectProperty(resource.NewPropertyMapFromMap(map[string]interface{}{
					"prop": []string{
						"foo",
					},
				})),
			}),
			types: map[string]pschema.ComplexTypeSpec{
				"pkg:index/type:ObjectNestedArrayType": {
					ObjectTypeSpec: pschema.ObjectTypeSpec{
						Type: "object",
						Properties: map[string]pschema.PropertySpec{
							"prop": {
								TypeSpec: pschema.TypeSpec{
									Type: "array",
									Items: &pschema.TypeSpec{
										Type: "string",
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name:     "object_nested_array_type_failure",
			typeRef:  "ObjectNestedArrayType",
			typeName: "object",
			input: resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewObjectProperty(resource.NewPropertyMapFromMap(map[string]interface{}{
					"prop": []interface{}{
						"foo",
						map[string]interface{}{"foo": "bar"},
						[]string{"bar"},
					},
				})),
			}),
			failures: []TypeFailure{
				{
					Reason:       "expected string type, got object type",
					ResourcePath: "object_nested_array_type_failure[0].prop[1]",
				},
				{
					Reason:       "expected string type, got [] type",
					ResourcePath: "object_nested_array_type_failure[0].prop[2]",
				},
			},
			types: map[string]pschema.ComplexTypeSpec{
				"pkg:index/type:ObjectNestedArrayType": {
					ObjectTypeSpec: pschema.ObjectTypeSpec{
						Type: "object",
						Properties: map[string]pschema.PropertySpec{
							"prop": {
								TypeSpec: pschema.TypeSpec{
									Type: "array",
									Items: &pschema.TypeSpec{
										Type: "string",
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name:     "object_nested_array_object_type_success",
			typeRef:  "ObjectNestedArrayObjectType",
			typeName: "object",
			input: resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewObjectProperty(resource.NewPropertyMapFromMap(map[string]interface{}{
					"prop": []map[string]interface{}{
						{
							"objectStringProp": "foo",
						},
					},
				})),
			}),
			types: map[string]pschema.ComplexTypeSpec{
				"pkg:index/type:ObjectStringType": {
					ObjectTypeSpec: pschema.ObjectTypeSpec{
						Type: "object",
						Properties: map[string]pschema.PropertySpec{
							"objectStringProp": {
								TypeSpec: pschema.TypeSpec{
									Type: "string",
								},
							},
						},
					},
				},
				"pkg:index/type:ObjectNestedArrayObjectType": {
					ObjectTypeSpec: pschema.ObjectTypeSpec{
						Type: "object",
						Properties: map[string]pschema.PropertySpec{
							"prop": {
								TypeSpec: pschema.TypeSpec{
									Type: "array",
									Items: &pschema.TypeSpec{
										Type: "object",
										// using ref to test specific object keys
										Ref: "#/types/pkg:index/type:ObjectStringType",
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name:     "object_nested_array_object_type_failure",
			typeRef:  "ObjectNestedArrayObjectType",
			typeName: "object",
			input: resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewObjectProperty(resource.NewPropertyMapFromMap(map[string]interface{}{
					"prop": []map[string]interface{}{
						{
							"objectStringProp": "foo",
						},
						{
							"objectStringProp": []string{"foo"},
						},
						{
							"foo": "bar",
						},
					},
				})),
			}),
			failures: []TypeFailure{
				{
					Reason:       "expected string type, got [] type",
					ResourcePath: "object_nested_array_object_type_failure[0].prop[1].objectStringProp",
				},
				{
					Reason:       "property foo is not defined in the schema",
					ResourcePath: "object_nested_array_object_type_failure[0].prop[2]",
				},
			},
			types: map[string]pschema.ComplexTypeSpec{
				"pkg:index/type:ObjectStringType": {
					ObjectTypeSpec: pschema.ObjectTypeSpec{
						Type: "object",
						Properties: map[string]pschema.PropertySpec{
							"objectStringProp": {
								TypeSpec: pschema.TypeSpec{
									Type: "string",
								},
							},
						},
					},
				},
				"pkg:index/type:ObjectNestedArrayObjectType": {
					ObjectTypeSpec: pschema.ObjectTypeSpec{
						Type: "object",
						Properties: map[string]pschema.PropertySpec{
							"prop": {
								TypeSpec: pschema.TypeSpec{
									Type: "array",
									Items: &pschema.TypeSpec{
										Type: "object",
										// using ref to test specific object keys
										Ref: "#/types/pkg:index/type:ObjectStringType",
									},
								},
							},
						},
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			pspec := pschema.PackageSpec{
				Name:  "test",
				Types: tc.types,
				Resources: map[string]pschema.ResourceSpec{
					"pkg:mod:ResA": {
						RequiredInputs: []string{tc.name},
						InputProperties: map[string]pschema.PropertySpec{
							tc.name: {
								TypeSpec: pschema.TypeSpec{
									Type: "array",
									Items: &pschema.TypeSpec{
										Ref: fmt.Sprintf("#/types/pkg:index/type:%s", tc.typeRef),
									},
								},
							},
						},
					},
				},
			}
			pathBuilder := resource.PropertyPath{}
			urn := resource.CreateURN(
				"testResource",
				"pkg:mod:ResA",
				"",
				"stack",
				"project",
			)

			v := &PulumiInputValidator{
				urn:    urn,
				schema: pspec,
			}
			failures := v.validateResourceInputType(tc.name, tc.input, pathBuilder)
			if failures != nil && len(*failures) != len(tc.failures) {
				t.Fatalf("%d failures, got %d: %v", len(tc.failures), len(*failures), *failures)
			}
			if len(tc.failures) > 0 {
				if failures == nil {
					t.Fatalf("expected failures, got none")
				} else {
					assert.Equal(t, tc.failures, *failures)
				}
			}
		})
	}
}

func TestValidateInputType_toplevel(t *testing.T) {
	testCases := []struct {
		name            string
		inputProperties map[string]pschema.PropertySpec
		input           resource.PropertyValue
		failures        []TypeFailure
	}{
		{
			name:  "string_type_success",
			input: resource.NewStringProperty("foo"),
			inputProperties: map[string]pschema.PropertySpec{
				"string_type_success": {
					TypeSpec: pschema.TypeSpec{
						Type: "string",
					},
				},
			},
		},
		{
			name:  "integer_type_success",
			input: resource.NewNumberProperty(1),
			inputProperties: map[string]pschema.PropertySpec{
				"integer_type_success": {
					TypeSpec: pschema.TypeSpec{
						Type: "integer",
					},
				},
			},
		},
		{
			name:  "integer_type_success_string",
			input: resource.NewNumberProperty(1),
			inputProperties: map[string]pschema.PropertySpec{
				"integer_type_success_string": {
					TypeSpec: pschema.TypeSpec{
						Type: "string",
					},
				},
			},
		},
		{
			name:  "bool_type_success",
			input: resource.NewBoolProperty(true),
			inputProperties: map[string]pschema.PropertySpec{
				"bool_type_success": {
					TypeSpec: pschema.TypeSpec{
						Type: "boolean",
					},
				},
			},
		},
		{
			name:  "bool_type_success_string",
			input: resource.PropertyValue{V: true},
			inputProperties: map[string]pschema.PropertySpec{
				"bool_type_success_string": {
					TypeSpec: pschema.TypeSpec{
						Type: "string",
					},
				},
			},
		},
		{
			name:  "string_type_failure",
			input: resource.NewArrayProperty([]resource.PropertyValue{resource.NewNumberProperty(1)}),
			failures: []TypeFailure{
				{Reason: "expected string type, got [] type", ResourcePath: "string_type_failure"},
			},
			inputProperties: map[string]pschema.PropertySpec{
				"string_type_failure": {
					TypeSpec: pschema.TypeSpec{
						Type: "string",
					},
				},
			},
		},
		{
			name:  "array_type_success",
			input: resource.NewArrayProperty([]resource.PropertyValue{resource.NewStringProperty("foo")}),
			inputProperties: map[string]pschema.PropertySpec{
				"array_type_success": {
					TypeSpec: pschema.TypeSpec{
						Type: "array",
						Items: &pschema.TypeSpec{
							Type: "string",
						},
					},
				},
			},
		},
		{
			name: "array_type_success_oneof",
			input: resource.NewArrayProperty(
				[]resource.PropertyValue{
					resource.NewStringProperty("foo"),
					resource.NewNumberProperty(1),
				},
			),
			inputProperties: map[string]pschema.PropertySpec{
				"array_type_success_oneof": {
					TypeSpec: pschema.TypeSpec{
						Type: "array",
						Items: &pschema.TypeSpec{
							OneOf: []pschema.TypeSpec{
								{Type: "string"},
								{Type: "number"},
							},
						},
					},
				},
			},
		},
		{
			name:  "array_type_failure",
			input: resource.NewArrayProperty([]resource.PropertyValue{resource.NewStringProperty("foo")}),
			failures: []TypeFailure{
				{Reason: "expected number type, got string type", ResourcePath: "array_type_failure[0]"},
			},
			inputProperties: map[string]pschema.PropertySpec{
				"array_type_failure": {
					TypeSpec: pschema.TypeSpec{
						Type: "array",
						Items: &pschema.TypeSpec{
							Type: "number",
						},
					},
				},
			},
		},
		{
			name: "object_type_success",
			input: resource.NewObjectProperty(
				resource.NewPropertyMapFromMap(map[string]interface{}{"foo": "bar"}),
			),
			inputProperties: map[string]pschema.PropertySpec{
				"object_type_success": {
					TypeSpec: pschema.TypeSpec{
						Type: "object",
						AdditionalProperties: &pschema.TypeSpec{
							Type: "string",
						},
					},
				},
			},
		},
		{
			name: "object_type_failure",
			input: resource.NewObjectProperty(
				resource.NewPropertyMapFromMap(map[string]interface{}{"foo": "bar"}),
			),
			failures: []TypeFailure{
				{Reason: "expected number type, got string type", ResourcePath: "object_type_failure.foo"},
			},
			inputProperties: map[string]pschema.PropertySpec{
				"object_type_failure": {
					TypeSpec: pschema.TypeSpec{
						Type: "object",
						AdditionalProperties: &pschema.TypeSpec{
							Type: "number",
						},
					},
				},
			},
		},
		{
			name:  "string_type_failure_name",
			input: resource.NewNumberProperty(1),
			failures: []TypeFailure{
				{Reason: "property string_type_failure_name is not defined in the schema", ResourcePath: ""},
			},
			inputProperties: map[string]pschema.PropertySpec{
				"prop": {
					TypeSpec: pschema.TypeSpec{
						Type: "string",
					},
				},
			},
		},
		{
			name:  "secret_type_success",
			input: resource.NewSecretProperty(&resource.Secret{Element: resource.NewStringProperty("foo")}),
			inputProperties: map[string]pschema.PropertySpec{
				"secret_type_success": {
					TypeSpec: pschema.TypeSpec{
						Type: "string",
					},
				},
			},
		},
		{
			name:  "secret_type_failure",
			input: resource.NewSecretProperty(&resource.Secret{Element: resource.NewStringProperty("foo")}),
			failures: []TypeFailure{
				{Reason: "expected number type, got secret<string> type", ResourcePath: "secret_type_failure"},
			},
			inputProperties: map[string]pschema.PropertySpec{
				"secret_type_failure": {
					TypeSpec: pschema.TypeSpec{
						Type: "number",
					},
				},
			},
		},
		{
			name:  "output_type_success",
			input: resource.NewOutputProperty(resource.Output{Element: resource.NewStringProperty("foo")}),
			inputProperties: map[string]pschema.PropertySpec{
				"output_type_success": {
					TypeSpec: pschema.TypeSpec{
						Type: "string",
					},
				},
			},
		},
		{
			name: "output_computed_type_success",
			input: resource.NewOutputProperty(resource.Output{
				Element: resource.NewComputedProperty(
					resource.Computed{Element: resource.NewStringProperty("foo")},
				),
			}),
			inputProperties: map[string]pschema.PropertySpec{
				"output_computed_type_success": {
					TypeSpec: pschema.TypeSpec{
						Type: "string",
					},
				},
			},
		},
		{
			name: "output_computed_type_failure",
			input: resource.NewOutputProperty(resource.Output{
				Element: resource.NewComputedProperty(resource.Computed{
					Element: resource.NewStringProperty("foo"),
				}),
			}),
			failures: []TypeFailure{
				{Reason: "expected number type, got output<output<string>> type", ResourcePath: "output_computed_type_failure"},
			},
			inputProperties: map[string]pschema.PropertySpec{
				"output_computed_type_failure": {
					TypeSpec: pschema.TypeSpec{
						Type: "number",
					},
				},
			},
		},
		{
			name:  "output_type_failure",
			input: resource.NewOutputProperty(resource.Output{Element: resource.NewStringProperty("foo")}),
			failures: []TypeFailure{
				{Reason: "expected number type, got output<string> type", ResourcePath: "output_type_failure"},
			},
			inputProperties: map[string]pschema.PropertySpec{
				"output_type_failure": {
					TypeSpec: pschema.TypeSpec{
						Type: "number",
					},
				},
			},
		},
		{
			name:  "null_type_failure",
			input: resource.NewNullProperty(),
			failures: []TypeFailure{
				{Reason: "property null_type_failure is required", ResourcePath: ""},
			},
			inputProperties: map[string]pschema.PropertySpec{
				"null_type_failure": {
					TypeSpec: pschema.TypeSpec{
						Type: "string",
					},
				},
			},
		},
	}
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			pspec := pschema.PackageSpec{
				Name: "test",
				Resources: map[string]pschema.ResourceSpec{
					"pkg:mod:ResA": {
						RequiredInputs:  []string{tc.name},
						InputProperties: tc.inputProperties,
					},
				},
			}
			pathBuilder := resource.PropertyPath{}
			urn := resource.CreateURN(
				"testResource",
				"pkg:mod:ResA",
				"",
				"stack",
				"project",
			)

			v := &PulumiInputValidator{
				urn:    urn,
				schema: pspec,
			}
			failures := v.validateResourceInputType(tc.name, tc.input, pathBuilder)
			if failures != nil && len(*failures) != len(tc.failures) {
				t.Fatalf("%d failures, got %d: %v", len(tc.failures), len(*failures), *failures)
			}
			if len(tc.failures) > 0 {
				if failures == nil {
					t.Fatalf("expected failures, got none")
				} else {
					assert.Equal(t, tc.failures, *failures)
				}
			}
		})
	}
}
