package tfbridge

import (
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"
)

func TestDeleteNestedDefaults(t *testing.T) {
	tests := []struct {
		name     string
		inputs   resource.PropertyMap
		expected resource.PropertyMap
	}{
		{
			name: "top level __defaults",
			inputs: resource.NewPropertyMapFromMap(map[string]interface{}{
				"someProp":   "someVal",
				"__defaults": []string{},
			}),
			expected: resource.NewPropertyMapFromMap(map[string]interface{}{
				"someProp": "someVal",
			}),
		},
		{
			name: "nested level __defaults",
			inputs: resource.NewPropertyMapFromMap(map[string]interface{}{
				"someProp": map[string]interface{}{
					"nestedProp": "nestedValue",
					"__defaults": []interface{}{},
				},
				"__defaults": []interface{}{},
			}),
			expected: resource.NewPropertyMapFromMap(map[string]interface{}{
				"someProp": map[string]interface{}{
					"nestedProp": "nestedValue",
				},
			}),
		},
		{
			name: "array __defaults",
			inputs: resource.NewPropertyMapFromMap(map[string]interface{}{
				"someProp": []map[string]interface{}{
					{
						"nestedProp": "nestedValue",
						"__defaults": []interface{}{},
					},
				},
				"__defaults": []interface{}{},
			}),
			expected: resource.NewPropertyMapFromMap(map[string]interface{}{
				"someProp": []map[string]interface{}{
					{
						"nestedProp": "nestedValue",
					},
				},
			}),
		},
		{
			name: "super nested __defaults",
			inputs: resource.NewPropertyMapFromMap(map[string]interface{}{
				"someProp": []map[string]interface{}{
					{
						"nestedProp": map[string]interface{}{
							"__defaults": []interface{}{},
							"doubleNested": []map[string]interface{}{
								{
									"__defaults": []interface{}{},
									"someProp": map[string]interface{}{
										"__defaults": []interface{}{},
										"otherProp":  "value",
									},
								},
							},
						},
						"__defaults": []interface{}{},
					},
				},
				"__defaults": []interface{}{},
			}),
			expected: resource.NewPropertyMapFromMap(map[string]interface{}{
				"someProp": []map[string]interface{}{
					{
						"nestedProp": map[string]interface{}{
							"doubleNested": []map[string]interface{}{
								{
									"someProp": map[string]interface{}{
										"otherProp": "value",
									},
								},
							},
						},
					},
				},
			}),
		},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			deleteDefaultsKey(tc.inputs)
			assert.Equal(t, tc.expected, tc.inputs)
		})
	}
}
