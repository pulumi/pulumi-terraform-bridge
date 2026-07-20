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
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/convert"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
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

func TestSchemaFromTypeErrors(t *testing.T) {
	t.Parallel()

	_, err := SchemaFromType(tftypes.Tuple{ElementTypes: []tftypes.Type{tftypes.String}})
	assert.EqualError(t, err, "tuple types are not supported")

	_, err = SchemaMapFromObject(tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"pair": tftypes.Tuple{ElementTypes: []tftypes.Type{tftypes.String}},
	}})
	assert.EqualError(t, err, `attribute "pair": tuple types are not supported`)
}

// Object attribute naming follows the standard bridge rules, including pluralization of
// list-typed attributes.
func TestObjectAttributeNaming(t *testing.T) {
	t.Parallel()

	obj := tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"account_id": tftypes.String,
		"rule":       tftypes.List{ElementType: tftypes.String},
	}}
	m, err := SchemaMapFromObject(obj)
	require.NoError(t, err)

	assert.Equal(t, "accountId", tfbridge.TerraformToPulumiNameV2("account_id", m, nil))
	assert.Equal(t, "rules", tfbridge.TerraformToPulumiNameV2("rule", m, nil))
}

// The synthetic schemas drive the pkg/convert encoders and decoders; values round-trip
// with argument names pinned exactly and nested object attributes renamed.
func TestArgumentsSchemaRoundTrip(t *testing.T) {
	t.Parallel()

	fn := shim.Function{
		Parameters: []shim.FunctionParameter{
			{Name: "value", Type: tftypes.String},
			{
				Name: "config",
				Type: tftypes.Object{AttributeTypes: map[string]tftypes.Type{
					"account_id": tftypes.String,
				}},
			},
		},
		VariadicParameter: &shim.FunctionParameter{Name: "parts", Type: tftypes.Number},
		Return:            tftypes.String,
	}
	argNames := ArgumentNames(fn)
	require.Equal(t, []string{"value", "config", "parts"}, argNames)

	os, err := ArgumentsSchema(fn, argNames)
	require.NoError(t, err)

	enc, err := convert.NewObjectEncoder(convert.ObjectSchema{
		SchemaMap:   os.SchemaMap,
		SchemaInfos: os.SchemaInfos,
		Object:      &os.Type,
	})
	require.NoError(t, err)
	dec, err := convert.NewObjectDecoder(convert.ObjectSchema{
		SchemaMap:   os.SchemaMap,
		SchemaInfos: os.SchemaInfos,
		Object:      &os.Type,
	})
	require.NoError(t, err)

	args := resource.PropertyMap{
		"value": resource.NewStringProperty("v"),
		"config": resource.NewObjectProperty(resource.PropertyMap{
			"accountId": resource.NewStringProperty("12345"),
		}),
		"parts": resource.NewArrayProperty([]resource.PropertyValue{
			resource.NewNumberProperty(1),
			resource.NewNumberProperty(2),
		}),
	}

	encoded, err := convert.EncodePropertyMap(enc, args)
	require.NoError(t, err)
	decoded, err := convert.DecodePropertyMap(context.Background(), dec, encoded)
	require.NoError(t, err)
	assert.Equal(t, args, decoded)
}

// A variadic `dynamic` parameter collects arguments of differing types, so it
// must be modeled as a dynamic attribute rather than a homogeneous list:
// encoding heterogeneous arguments into a List panics on the mismatched element
// types, whereas a dynamic attribute accepts the resulting Tuple.
func TestArgumentsSchemaVariadicDynamic(t *testing.T) {
	t.Parallel()

	fn := shim.Function{
		VariadicParameter: &shim.FunctionParameter{Name: "maps", Type: tftypes.DynamicPseudoType},
		Return:            tftypes.DynamicPseudoType,
	}
	argNames := ArgumentNames(fn)
	require.Equal(t, []string{"maps"}, argNames)

	os, err := ArgumentsSchema(fn, argNames)
	require.NoError(t, err)
	assert.Equal(t, tftypes.DynamicPseudoType, os.Type.AttributeTypes["maps"],
		"a dynamic variadic argument must be a dynamic attribute, not a list")

	enc, err := convert.NewObjectEncoder(convert.ObjectSchema{
		SchemaMap:   os.SchemaMap,
		SchemaInfos: os.SchemaInfos,
		Object:      &os.Type,
	})
	require.NoError(t, err)

	// Two arguments of differing shapes, as in deepmerge's mergo(defaults, {}).
	args := resource.PropertyMap{
		"maps": resource.NewArrayProperty([]resource.PropertyValue{
			resource.NewObjectProperty(resource.PropertyMap{
				"a":      resource.NewNumberProperty(1),
				"nested": resource.NewObjectProperty(resource.PropertyMap{"x": resource.NewStringProperty("y")}),
			}),
			resource.NewObjectProperty(resource.PropertyMap{"b": resource.NewStringProperty("two")}),
		}),
	}

	encoded, err := convert.EncodePropertyMap(enc, args)
	require.NoError(t, err)

	var attrs map[string]tftypes.Value
	require.NoError(t, encoded.As(&attrs))
	assert.True(t, attrs["maps"].Type().Is(tftypes.Tuple{}),
		"heterogeneous variadic arguments must encode to a tuple, got %s", attrs["maps"].Type())
}

func TestResultSchema(t *testing.T) {
	t.Parallel()

	t.Run("object return decodes directly", func(t *testing.T) {
		t.Parallel()
		fn := shim.Function{
			Return: tftypes.Object{AttributeTypes: map[string]tftypes.Type{
				"account_id": tftypes.String,
			}},
		}
		os, isObject, err := ResultSchema(fn, "result")
		require.NoError(t, err)
		assert.True(t, isObject)
		assert.Equal(t, fn.Return, os.Type)
	})

	t.Run("scalar return wraps into a single property", func(t *testing.T) {
		t.Parallel()
		fn := shim.Function{Return: tftypes.List{ElementType: tftypes.String}}
		os, isObject, err := ResultSchema(fn, "result")
		require.NoError(t, err)
		assert.False(t, isObject)

		dec, err := convert.NewObjectDecoder(convert.ObjectSchema{
			SchemaMap:   os.SchemaMap,
			SchemaInfos: os.SchemaInfos,
			Object:      &os.Type,
		})
		require.NoError(t, err)

		value := tftypes.NewValue(os.Type, map[string]tftypes.Value{
			"result": tftypes.NewValue(fn.Return, []tftypes.Value{
				tftypes.NewValue(tftypes.String, "a"),
			}),
		})
		decoded, err := convert.DecodePropertyMap(context.Background(), dec, value)
		require.NoError(t, err)

		// The wrapped property name is pinned: a list-typed result must not
		// pluralize to "results".
		assert.Equal(t, resource.PropertyMap{
			"result": resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewStringProperty("a"),
			}),
		}, decoded)
	})
}
