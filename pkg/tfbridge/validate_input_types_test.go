package tfbridge

import (
	"fmt"
	"testing"

	pschema "github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
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
			failures := v.ValidateInputs(tokens.Type("pkg:mod:ResA"), resource.PropertyMap{
				resource.PropertyKey(tc.name): tc.input,
			})
			if failures != nil && len(failures) != len(tc.failures) {
				t.Fatalf("%d failures, got %d: %v", len(tc.failures), len(failures), failures)
			}
			if len(tc.failures) > 0 {
				if failures == nil {
					t.Fatalf("expected failures, got none")
				} else {
					assert.Equal(t, tc.failures, failures)
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
			failures := v.ValidateInputs(tokens.Type("pkg:mod:ResA"), resource.PropertyMap{
				resource.PropertyKey(tc.name): tc.input,
			})
			if failures != nil && len(failures) != len(tc.failures) {
				t.Fatalf("%d failures, got %d: %v", len(tc.failures), len(failures), failures)
			}
			if len(tc.failures) > 0 {
				if failures == nil {
					t.Fatalf("expected failures, got none")
				} else {
					assert.Equal(t, tc.failures, failures)
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
			name:  "array_type_string_coalesce_success",
			input: resource.NewArrayProperty([]resource.PropertyValue{resource.NewStringProperty("foo")}),
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
				resource.NewPropertyMapFromMap(map[string]interface{}{"foo": []string{"bar"}}),
			),
			failures: []TypeFailure{
				{Reason: "expected boolean type, got [] type", ResourcePath: "object_type_failure.foo"},
			},
			inputProperties: map[string]pschema.PropertySpec{
				"object_type_failure": {
					TypeSpec: pschema.TypeSpec{
						Type: "object",
						AdditionalProperties: &pschema.TypeSpec{
							Type: "bool",
						},
					},
				},
			},
		},
		{
			name:  "extraneous_property_success",
			input: resource.NewNumberProperty(1),
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
			name:  "secret_type_string_coalesce_success",
			input: resource.NewSecretProperty(&resource.Secret{Element: resource.NewStringProperty("foo")}),
			inputProperties: map[string]pschema.PropertySpec{
				"secret_type_string_coalesce_success": {
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
			name:  "output_type_coalesce_success",
			input: resource.NewOutputProperty(resource.Output{Element: resource.NewStringProperty("foo"), Known: true}),
			inputProperties: map[string]pschema.PropertySpec{
				"output_type_coalesce_success": {
					TypeSpec: pschema.TypeSpec{
						Type: "number",
					},
				},
			},
		},
		{
			name:  "null_type_success",
			input: resource.NewNullProperty(),
			inputProperties: map[string]pschema.PropertySpec{
				"null_type_success": {
					TypeSpec: pschema.TypeSpec{
						Type: "string",
					},
				},
			},
		},
		{
			name:  "default_required_success",
			input: resource.NewNullProperty(),
			inputProperties: map[string]pschema.PropertySpec{
				"default_required_success": {
					Default: "foo",
					TypeSpec: pschema.TypeSpec{
						Type: "string",
					},
				},
			},
		},
		{
			name:  "oneof_default_type_success",
			input: resource.NewStringProperty("foo"),
			inputProperties: map[string]pschema.PropertySpec{
				"oneof_default_type_success": {
					TypeSpec: pschema.TypeSpec{
						Type: "string",
						OneOf: []pschema.TypeSpec{
							{
								Type: "array",
								Items: &pschema.TypeSpec{
									Type: "string",
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
			failures := v.ValidateInputs(tokens.Type("pkg:mod:ResA"), resource.PropertyMap{
				resource.PropertyKey(tc.name): tc.input,
			})
			if failures != nil && len(failures) != len(tc.failures) {
				t.Fatalf("%d failures, got %d: %v", len(tc.failures), len(failures), failures)
			}
			if len(tc.failures) > 0 {
				if failures == nil {
					t.Fatalf("expected failures, got none")
				} else {
					assert.Equal(t, tc.failures, failures)
				}
			}
		})
	}
}

func TestValidateConfigType(t *testing.T) {
	testCases := []struct {
		name      string
		inputName string
		input     resource.PropertyValue
		failures  []TypeFailure
	}{
		{
			name:      "unexpected_argument",
			inputName: "endpoints",
			failures: []TypeFailure{{
				Reason:       "an unexpected argument \"wxyz\" was provided",
				ResourcePath: "endpoints[0]",
			}},
			input: resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewObjectProperty(resource.NewPropertyMapFromMap(map[string]interface{}{
					"wxyz": "foo",
				})),
			}),
		},
		{
			name:      "no_error_for_extra_inputs",
			inputName: "doesnt_exist",
			failures:  []TypeFailure{},
			input: resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewObjectProperty(resource.NewPropertyMapFromMap(map[string]interface{}{
					"wxyz": "foo",
				})),
			}),
		},
	}
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			pspec := pschema.PackageSpec{
				Name: "test",
				Types: map[string]pschema.ComplexTypeSpec{
					"aws:config/endpoints:endpoints": {
						ObjectTypeSpec: pschema.ObjectTypeSpec{
							Type: "object",
							Properties: map[string]pschema.PropertySpec{
								"abcd": {
									TypeSpec: pschema.TypeSpec{
										Type: "string",
									},
								},
							},
						},
					},
				},
				Config: pschema.ConfigSpec{
					Variables: map[string]pschema.PropertySpec{
						"endpoints": {
							TypeSpec: pschema.TypeSpec{
								Type: "array",
								Items: &pschema.TypeSpec{
									Ref: "#/types/aws:config/endpoints:endpoints",
								},
							},
						},
					},
				},
			}
			urn := resource.CreateURN(
				"testResource",
				"pkg:mod:ResA",
				"",
				"stack",
				"project",
			)

			v := &PulumiInputValidator{
				urn:                  urn,
				schema:               pspec,
				validateUnknownTypes: true,
			}
			failures := v.ValidateConfig(resource.PropertyMap{
				resource.PropertyKey(tc.inputName): tc.input,
			})
			assert.Equal(t, tc.failures, failures)
		})
	}
}
