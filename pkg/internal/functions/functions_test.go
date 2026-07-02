// Copyright 2016-2026, Pulumi Corporation.
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

package functions

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

func TestArgumentNames(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		fn       shim.Function
		expected []string
	}{
		{
			name: "named parameters",
			fn: shim.Function{
				Parameters: []shim.FunctionParameter{
					{Name: "arn", Type: tftypes.String},
					{Name: "account_id", Type: tftypes.String},
				},
			},
			expected: []string{"arn", "accountId"},
		},
		{
			name: "empty names",
			fn: shim.Function{
				Parameters: []shim.FunctionParameter{
					{Type: tftypes.String},
					{Type: tftypes.String},
				},
			},
			expected: []string{"arg1", "arg2"},
		},
		{
			name: "duplicate names",
			fn: shim.Function{
				Parameters: []shim.FunctionParameter{
					{Name: "value", Type: tftypes.String},
					{Name: "value", Type: tftypes.String},
					{Name: "value", Type: tftypes.String},
				},
			},
			expected: []string{"value", "value2", "value3"},
		},
		{
			name: "variadic parameter",
			fn: shim.Function{
				Parameters: []shim.FunctionParameter{
					{Name: "separator", Type: tftypes.String},
				},
				VariadicParameter: &shim.FunctionParameter{Name: "parts", Type: tftypes.String},
			},
			expected: []string{"separator", "parts"},
		},
		{
			name: "unnamed variadic parameter",
			fn: shim.Function{
				Parameters: []shim.FunctionParameter{
					{Name: "separator", Type: tftypes.String},
				},
				VariadicParameter: &shim.FunctionParameter{Type: tftypes.String},
			},
			expected: []string{"separator", "arg2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, ArgumentNames(tt.fn))
		})
	}
}

func TestEncodeDecodeRoundTrip(t *testing.T) {
	t.Parallel()

	objType := tftypes.Object{
		AttributeTypes: map[string]tftypes.Type{
			"account_id": tftypes.String,
			"port":       tftypes.Number,
			"tags":       tftypes.Map{ElementType: tftypes.String},
		},
		OptionalAttributes: map[string]struct{}{
			"tags": {},
		},
	}

	tests := []struct {
		name  string
		typ   tftypes.Type
		value resource.PropertyValue
	}{
		{"string", tftypes.String, resource.NewStringProperty("hello")},
		{"number", tftypes.Number, resource.NewNumberProperty(42.5)},
		{"bool", tftypes.Bool, resource.NewBoolProperty(true)},
		{"null string", tftypes.String, resource.NewNullProperty()},
		{
			"list of strings",
			tftypes.List{ElementType: tftypes.String},
			resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewStringProperty("a"),
				resource.NewStringProperty("b"),
			}),
		},
		{
			"set of numbers",
			tftypes.Set{ElementType: tftypes.Number},
			resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewNumberProperty(1),
				resource.NewNumberProperty(2),
			}),
		},
		{
			"map of strings",
			tftypes.Map{ElementType: tftypes.String},
			resource.NewObjectProperty(resource.PropertyMap{
				"key_with_underscores": resource.NewStringProperty("v"),
			}),
		},
		{
			"object with renamed attributes",
			objType,
			resource.NewObjectProperty(resource.PropertyMap{
				"accountId": resource.NewStringProperty("12345"),
				"port":      resource.NewNumberProperty(443),
				"tags": resource.NewObjectProperty(resource.PropertyMap{
					"env": resource.NewStringProperty("prod"),
				}),
			}),
		},
		{"dynamic string", tftypes.DynamicPseudoType, resource.NewStringProperty("s")},
		{"dynamic bool", tftypes.DynamicPseudoType, resource.NewBoolProperty(false)},
		{
			"dynamic list",
			tftypes.DynamicPseudoType,
			resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewNumberProperty(1),
				resource.NewNumberProperty(2),
			}),
		},
		{
			"dynamic object",
			tftypes.DynamicPseudoType,
			resource.NewObjectProperty(resource.PropertyMap{
				"snake_key": resource.NewStringProperty("not renamed"),
			}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			encoded, err := EncodeValue(tt.typ, tt.value)
			require.NoError(t, err)
			decoded, err := DecodeValue(tt.typ, encoded)
			require.NoError(t, err)
			assert.Equal(t, tt.value, decoded)
		})
	}
}

func TestEncodeObjectFillsMissingAttributes(t *testing.T) {
	t.Parallel()

	typ := tftypes.Object{
		AttributeTypes: map[string]tftypes.Type{
			"required_attr": tftypes.String,
			"optional_attr": tftypes.String,
		},
		OptionalAttributes: map[string]struct{}{"optional_attr": {}},
	}
	encoded, err := EncodeValue(typ, resource.NewObjectProperty(resource.PropertyMap{
		"requiredAttr": resource.NewStringProperty("v"),
	}))
	require.NoError(t, err)

	assert.Equal(t, tftypes.NewValue(typ, map[string]tftypes.Value{
		"required_attr": tftypes.NewValue(tftypes.String, "v"),
		"optional_attr": tftypes.NewValue(tftypes.String, nil),
	}), encoded)
}

func TestEncodeErrors(t *testing.T) {
	t.Parallel()

	t.Run("type mismatch", func(t *testing.T) {
		t.Parallel()
		_, err := EncodeValue(tftypes.String, resource.NewNumberProperty(1))
		assert.EqualError(t, err, "expected a string, got number")
	})

	t.Run("unexpected object property", func(t *testing.T) {
		t.Parallel()
		typ := tftypes.Object{AttributeTypes: map[string]tftypes.Type{"a": tftypes.String}}
		_, err := EncodeValue(typ, resource.NewObjectProperty(resource.PropertyMap{
			"a": resource.NewStringProperty("v"),
			"b": resource.NewStringProperty("v"),
		}))
		assert.EqualError(t, err, `unexpected property "b"`)
	})

	t.Run("tuple unsupported", func(t *testing.T) {
		t.Parallel()
		typ := tftypes.Tuple{ElementTypes: []tftypes.Type{tftypes.String}}
		_, err := EncodeValue(typ, resource.NewArrayProperty([]resource.PropertyValue{
			resource.NewStringProperty("v"),
		}))
		assert.EqualError(t, err, "tuple types are not supported")
	})

	t.Run("unknown value", func(t *testing.T) {
		t.Parallel()
		_, err := EncodeValue(tftypes.String, resource.MakeComputed(resource.NewStringProperty("")))
		assert.EqualError(t, err, "unexpected unknown value")
	})
}

func TestEncodeSecretUnwraps(t *testing.T) {
	t.Parallel()

	encoded, err := EncodeValue(tftypes.String, resource.MakeSecret(resource.NewStringProperty("s3cret")))
	require.NoError(t, err)
	assert.Equal(t, tftypes.NewValue(tftypes.String, "s3cret"), encoded)
}
