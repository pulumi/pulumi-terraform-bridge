// Copyright 2016-2025, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package valueshim

import (
	"encoding/json"
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/hexops/autogold/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValueToJSON(t *testing.T) {
	t.Parallel()
	type testCase struct {
		name        string
		typ         tftypes.Type
		value       tftypes.Value
		expectJSON  autogold.Value
		expectError string
	}

	testCases := []testCase{
		// String types
		{
			name:       "string null",
			typ:        tftypes.String,
			value:      tftypes.NewValue(tftypes.String, nil),
			expectJSON: autogold.Expect("null"),
		},
		{
			name:       "string value",
			typ:        tftypes.String,
			value:      tftypes.NewValue(tftypes.String, "hello world"),
			expectJSON: autogold.Expect(`"hello world"`),
		},
		{
			name:       "string empty",
			typ:        tftypes.String,
			value:      tftypes.NewValue(tftypes.String, ""),
			expectJSON: autogold.Expect(`""`),
		},
		{
			name:       "string with special characters",
			typ:        tftypes.String,
			value:      tftypes.NewValue(tftypes.String, "hello\nworld\t\"quoted\""),
			expectJSON: autogold.Expect(`"hello\nworld\t\"quoted\""`),
		},

		// Number types
		{
			name:       "number null",
			typ:        tftypes.Number,
			value:      tftypes.NewValue(tftypes.Number, nil),
			expectJSON: autogold.Expect("null"),
		},
		{
			name:       "number integer",
			typ:        tftypes.Number,
			value:      tftypes.NewValue(tftypes.Number, 42),
			expectJSON: autogold.Expect("42"),
		},
		{
			name:       "number float",
			typ:        tftypes.Number,
			value:      tftypes.NewValue(tftypes.Number, 3.14159),
			expectJSON: autogold.Expect("3.14159"),
		},
		{
			name:       "number zero",
			typ:        tftypes.Number,
			value:      tftypes.NewValue(tftypes.Number, 0),
			expectJSON: autogold.Expect("0"),
		},
		{
			name:       "number negative",
			typ:        tftypes.Number,
			value:      tftypes.NewValue(tftypes.Number, -123.45),
			expectJSON: autogold.Expect("-123.45"),
		},

		// Bool types
		{
			name:       "bool null",
			typ:        tftypes.Bool,
			value:      tftypes.NewValue(tftypes.Bool, nil),
			expectJSON: autogold.Expect("null"),
		},
		{
			name:       "bool true",
			typ:        tftypes.Bool,
			value:      tftypes.NewValue(tftypes.Bool, true),
			expectJSON: autogold.Expect("true"),
		},
		{
			name:       "bool false",
			typ:        tftypes.Bool,
			value:      tftypes.NewValue(tftypes.Bool, false),
			expectJSON: autogold.Expect("false"),
		},

		// List types
		{
			name:       "list null",
			typ:        tftypes.List{ElementType: tftypes.String},
			value:      tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, nil),
			expectJSON: autogold.Expect("null"),
		},
		{
			name:       "list empty",
			typ:        tftypes.List{ElementType: tftypes.String},
			value:      tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, []tftypes.Value{}),
			expectJSON: autogold.Expect("[]"),
		},
		{
			name: "list of strings",
			typ:  tftypes.List{ElementType: tftypes.String},
			value: tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, []tftypes.Value{
				tftypes.NewValue(tftypes.String, "first"),
				tftypes.NewValue(tftypes.String, "second"),
				tftypes.NewValue(tftypes.String, "third"),
			}),
			expectJSON: autogold.Expect(`["first","second","third"]`),
		},
		{
			name: "list of numbers",
			typ:  tftypes.List{ElementType: tftypes.Number},
			value: tftypes.NewValue(tftypes.List{ElementType: tftypes.Number}, []tftypes.Value{
				tftypes.NewValue(tftypes.Number, 1),
				tftypes.NewValue(tftypes.Number, 2),
				tftypes.NewValue(tftypes.Number, 3),
			}),
			expectJSON: autogold.Expect(`[1,2,3]`),
		},
		{
			name: "list with null element",
			typ:  tftypes.List{ElementType: tftypes.String},
			value: tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, []tftypes.Value{
				tftypes.NewValue(tftypes.String, "first"),
				tftypes.NewValue(tftypes.String, nil),
				tftypes.NewValue(tftypes.String, "third"),
			}),
			expectJSON: autogold.Expect(`["first",null,"third"]`),
		},

		// Set types
		{
			name:       "set null",
			typ:        tftypes.Set{ElementType: tftypes.String},
			value:      tftypes.NewValue(tftypes.Set{ElementType: tftypes.String}, nil),
			expectJSON: autogold.Expect("null"),
		},
		{
			name:       "set empty",
			typ:        tftypes.Set{ElementType: tftypes.String},
			value:      tftypes.NewValue(tftypes.Set{ElementType: tftypes.String}, []tftypes.Value{}),
			expectJSON: autogold.Expect("[]"),
		},
		{
			name: "set of strings",
			typ:  tftypes.Set{ElementType: tftypes.String},
			value: tftypes.NewValue(tftypes.Set{ElementType: tftypes.String}, []tftypes.Value{
				tftypes.NewValue(tftypes.String, "alpha"),
				tftypes.NewValue(tftypes.String, "beta"),
			}),
			expectJSON: autogold.Expect(`["alpha","beta"]`),
		},

		// Map types
		{
			name:       "map null",
			typ:        tftypes.Map{ElementType: tftypes.String},
			value:      tftypes.NewValue(tftypes.Map{ElementType: tftypes.String}, nil),
			expectJSON: autogold.Expect("null"),
		},
		{
			name:       "map empty",
			typ:        tftypes.Map{ElementType: tftypes.String},
			value:      tftypes.NewValue(tftypes.Map{ElementType: tftypes.String}, map[string]tftypes.Value{}),
			expectJSON: autogold.Expect("{}"),
		},
		{
			name: "map of strings",
			typ:  tftypes.Map{ElementType: tftypes.String},
			value: tftypes.NewValue(tftypes.Map{ElementType: tftypes.String}, map[string]tftypes.Value{
				"key1": tftypes.NewValue(tftypes.String, "value1"),
				"key2": tftypes.NewValue(tftypes.String, "value2"),
			}),
			expectJSON: autogold.Expect(`{"key1":"value1","key2":"value2"}`),
		},
		{
			name: "map with null value",
			typ:  tftypes.Map{ElementType: tftypes.String},
			value: tftypes.NewValue(tftypes.Map{ElementType: tftypes.String}, map[string]tftypes.Value{
				"key1": tftypes.NewValue(tftypes.String, "value1"),
				"key2": tftypes.NewValue(tftypes.String, nil),
			}),
			expectJSON: autogold.Expect(`{"key1":"value1","key2":null}`),
		},

		// Object types
		{
			name: "object null",
			typ: tftypes.Object{AttributeTypes: map[string]tftypes.Type{
				"name": tftypes.String,
				"age":  tftypes.Number,
			}},
			value: tftypes.NewValue(tftypes.Object{AttributeTypes: map[string]tftypes.Type{
				"name": tftypes.String,
				"age":  tftypes.Number,
			}}, nil),
			expectJSON: autogold.Expect("null"),
		},
		{
			name: "object simple",
			typ: tftypes.Object{AttributeTypes: map[string]tftypes.Type{
				"name": tftypes.String,
				"age":  tftypes.Number,
			}},
			value: tftypes.NewValue(tftypes.Object{AttributeTypes: map[string]tftypes.Type{
				"name": tftypes.String,
				"age":  tftypes.Number,
			}}, map[string]tftypes.Value{
				"name": tftypes.NewValue(tftypes.String, "John"),
				"age":  tftypes.NewValue(tftypes.Number, 30),
			}),
			expectJSON: autogold.Expect(`{"age":30,"name":"John"}`),
		},
		{
			name: "object with null attribute",
			typ: tftypes.Object{AttributeTypes: map[string]tftypes.Type{
				"name":  tftypes.String,
				"email": tftypes.String,
			}},
			value: tftypes.NewValue(tftypes.Object{AttributeTypes: map[string]tftypes.Type{
				"name":  tftypes.String,
				"email": tftypes.String,
			}}, map[string]tftypes.Value{
				"name":  tftypes.NewValue(tftypes.String, "John"),
				"email": tftypes.NewValue(tftypes.String, nil),
			}),
			expectJSON: autogold.Expect(`{"email":null,"name":"John"}`),
		},

		// Tuple types
		{
			name: "tuple null",
			typ:  tftypes.Tuple{ElementTypes: []tftypes.Type{tftypes.String, tftypes.Number}},
			value: tftypes.NewValue(tftypes.Tuple{ElementTypes: []tftypes.Type{
				tftypes.String,
				tftypes.Number,
			}}, nil),
			expectJSON: autogold.Expect("null"),
		},
		{
			name: "tuple simple",
			typ:  tftypes.Tuple{ElementTypes: []tftypes.Type{tftypes.String, tftypes.Number, tftypes.Bool}},
			value: tftypes.NewValue(tftypes.Tuple{ElementTypes: []tftypes.Type{
				tftypes.String,
				tftypes.Number,
				tftypes.Bool,
			}}, []tftypes.Value{
				tftypes.NewValue(tftypes.String, "hello"),
				tftypes.NewValue(tftypes.Number, 42),
				tftypes.NewValue(tftypes.Bool, true),
			}),
			expectJSON: autogold.Expect(`["hello",42,true]`),
		},
		{
			name: "tuple with null element",
			typ:  tftypes.Tuple{ElementTypes: []tftypes.Type{tftypes.String, tftypes.Number}},
			value: tftypes.NewValue(tftypes.Tuple{ElementTypes: []tftypes.Type{
				tftypes.String,
				tftypes.Number,
			}}, []tftypes.Value{
				tftypes.NewValue(tftypes.String, "hello"),
				tftypes.NewValue(tftypes.Number, nil),
			}),
			expectJSON: autogold.Expect(`["hello",null]`),
		},

		// Nested structures
		{
			name: "nested list of objects",
			typ:  tftypes.List{ElementType: tftypes.Object{AttributeTypes: map[string]tftypes.Type{"name": tftypes.String}}},
			value: tftypes.NewValue(
				tftypes.List{ElementType: tftypes.Object{AttributeTypes: map[string]tftypes.Type{"name": tftypes.String}}},
				[]tftypes.Value{
					tftypes.NewValue(tftypes.Object{AttributeTypes: map[string]tftypes.Type{"name": tftypes.String}},
						map[string]tftypes.Value{"name": tftypes.NewValue(tftypes.String, "Alice")}),
					tftypes.NewValue(tftypes.Object{AttributeTypes: map[string]tftypes.Type{"name": tftypes.String}},
						map[string]tftypes.Value{"name": tftypes.NewValue(tftypes.String, "Bob")}),
				},
			),
			expectJSON: autogold.Expect(`[{"name":"Alice"},{"name":"Bob"}]`),
		},
		{
			name: "object with nested list",
			typ: tftypes.Object{AttributeTypes: map[string]tftypes.Type{
				"name":  tftypes.String,
				"items": tftypes.List{ElementType: tftypes.String},
			}},
			value: tftypes.NewValue(tftypes.Object{AttributeTypes: map[string]tftypes.Type{
				"name":  tftypes.String,
				"items": tftypes.List{ElementType: tftypes.String},
			}}, map[string]tftypes.Value{
				"name": tftypes.NewValue(tftypes.String, "shopping"),
				"items": tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, []tftypes.Value{
					tftypes.NewValue(tftypes.String, "milk"),
					tftypes.NewValue(tftypes.String, "bread"),
				}),
			}),
			expectJSON: autogold.Expect(`{"items":["milk","bread"],"name":"shopping"}`),
		},

		// Error cases
		{
			name:        "unknown value",
			typ:         tftypes.String,
			value:       tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
			expectError: "unknown values cannot be serialized to JSON",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result, err := tftypeValueToJSON(tc.typ, tc.value)

			if tc.expectError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectError)
			} else {
				require.NoError(t, err)
				tc.expectJSON.Equal(t, string(result))
			}
		})
	}
}

func TestValueToJSON_DynamicPseudoType(t *testing.T) {
	t.Parallel()

	// Test dynamic pseudo type which requires special handling
	// DynamicPseudoType is used to wrap values of any concrete type
	// We create a string value but tell the marshal function to treat it as dynamic
	stringValue := tftypes.NewValue(tftypes.String, "dynamic content")

	// When marshaling with DynamicPseudoType as the schema type,
	// it should produce {"type": "...", "value": "..."}
	result, err := tftypeValueToJSON(tftypes.DynamicPseudoType, stringValue)
	require.NoError(t, err)

	// The result should be a JSON object with type and value fields
	expected := `{"type":"string","value":"dynamic content"}`
	assert.JSONEq(t, expected, string(result))

	// Also test with other types
	numberValue := tftypes.NewValue(tftypes.Number, 42)
	result, err = tftypeValueToJSON(tftypes.DynamicPseudoType, numberValue)
	require.NoError(t, err)

	expected = `{"type":"number","value":42}`
	assert.JSONEq(t, expected, string(result))

	boolValue := tftypes.NewValue(tftypes.Bool, true)
	result, err = tftypeValueToJSON(tftypes.DynamicPseudoType, boolValue)
	require.NoError(t, err)

	expected = `{"type":"bool","value":true}`
	assert.JSONEq(t, expected, string(result))
}

func TestValueToJSON_ComplexNested(t *testing.T) {
	t.Parallel()

	// Test a complex nested structure
	complexType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"metadata": tftypes.Object{AttributeTypes: map[string]tftypes.Type{
			"name":    tftypes.String,
			"version": tftypes.Number,
		}},
		"items": tftypes.List{ElementType: tftypes.Object{AttributeTypes: map[string]tftypes.Type{
			"id":     tftypes.String,
			"active": tftypes.Bool,
			"tags":   tftypes.Set{ElementType: tftypes.String},
		}}},
		"config": tftypes.Map{ElementType: tftypes.String},
	}}

	complexValue := tftypes.NewValue(complexType, map[string]tftypes.Value{
		"metadata": tftypes.NewValue(tftypes.Object{AttributeTypes: map[string]tftypes.Type{
			"name":    tftypes.String,
			"version": tftypes.Number,
		}}, map[string]tftypes.Value{
			"name":    tftypes.NewValue(tftypes.String, "test-app"),
			"version": tftypes.NewValue(tftypes.Number, 1.5),
		}),
		"items": tftypes.NewValue(tftypes.List{ElementType: tftypes.Object{AttributeTypes: map[string]tftypes.Type{
			"id":     tftypes.String,
			"active": tftypes.Bool,
			"tags":   tftypes.Set{ElementType: tftypes.String},
		}}}, []tftypes.Value{
			tftypes.NewValue(tftypes.Object{AttributeTypes: map[string]tftypes.Type{
				"id":     tftypes.String,
				"active": tftypes.Bool,
				"tags":   tftypes.Set{ElementType: tftypes.String},
			}}, map[string]tftypes.Value{
				"id":     tftypes.NewValue(tftypes.String, "item1"),
				"active": tftypes.NewValue(tftypes.Bool, true),
				"tags": tftypes.NewValue(tftypes.Set{ElementType: tftypes.String}, []tftypes.Value{
					tftypes.NewValue(tftypes.String, "prod"),
					tftypes.NewValue(tftypes.String, "web"),
				}),
			}),
		}),
		"config": tftypes.NewValue(tftypes.Map{ElementType: tftypes.String}, map[string]tftypes.Value{
			"env":      tftypes.NewValue(tftypes.String, "production"),
			"debug":    tftypes.NewValue(tftypes.String, "false"),
			"optional": tftypes.NewValue(tftypes.String, nil),
		}),
	})

	result, err := tftypeValueToJSON(complexType, complexValue)
	require.NoError(t, err)

	// Verify it's valid JSON
	var parsed interface{}
	err = json.Unmarshal(result, &parsed)
	require.NoError(t, err)

	// Convert back to pretty JSON for readability in test output
	prettyResult, err := json.MarshalIndent(parsed, "", "  ")
	require.NoError(t, err)

	autogold.Expect(`{
  "config": {
    "debug": "false",
    "env": "production",
    "optional": null
  },
  "items": [
    {
      "active": true,
      "id": "item1",
      "tags": [
        "prod",
        "web"
      ]
    }
  ],
  "metadata": {
    "name": "test-app",
    "version": 1.5
  }
}`).Equal(t, string(prettyResult))
}

func TestValueToJSON_EdgeCases(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name        string
		typ         tftypes.Type
		value       tftypes.Value
		expectError string
	}

	testCases := []testCase{
		{
			name:        "unknown string value",
			typ:         tftypes.String,
			value:       tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
			expectError: "unknown values cannot be serialized to JSON",
		},
		{
			name:        "unknown number value",
			typ:         tftypes.Number,
			value:       tftypes.NewValue(tftypes.Number, tftypes.UnknownValue),
			expectError: "unknown values cannot be serialized to JSON",
		},
		{
			name:        "unknown bool value",
			typ:         tftypes.Bool,
			value:       tftypes.NewValue(tftypes.Bool, tftypes.UnknownValue),
			expectError: "unknown values cannot be serialized to JSON",
		},
		{
			name: "list with unknown element",
			typ:  tftypes.List{ElementType: tftypes.String},
			value: tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, []tftypes.Value{
				tftypes.NewValue(tftypes.String, "known"),
				tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
			}),
			expectError: "unknown values cannot be serialized to JSON",
		},
		{
			name: "object with unknown attribute",
			typ: tftypes.Object{AttributeTypes: map[string]tftypes.Type{
				"known":   tftypes.String,
				"unknown": tftypes.String,
			}},
			value: tftypes.NewValue(tftypes.Object{AttributeTypes: map[string]tftypes.Type{
				"known":   tftypes.String,
				"unknown": tftypes.String,
			}}, map[string]tftypes.Value{
				"known":   tftypes.NewValue(tftypes.String, "value"),
				"unknown": tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
			}),
			expectError: "unknown values cannot be serialized to JSON",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result, err := tftypeValueToJSON(tc.typ, tc.value)

			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.expectError)
			assert.Nil(t, result)
		})
	}
}
