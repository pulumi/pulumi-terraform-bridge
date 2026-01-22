// Copyright 2016-2024, Pulumi Corporation.
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

package tfbridge

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
)

func TestConvertTfTypeToCtyType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		tfType  tftypes.Type
		want    cty.Type
		wantErr bool
	}{
		{
			name:   "String type",
			tfType: tftypes.String,
			want:   cty.String,
		},
		{
			name:   "Number type",
			tfType: tftypes.Number,
			want:   cty.Number,
		},
		{
			name:   "Bool type",
			tfType: tftypes.Bool,
			want:   cty.Bool,
		},
		{
			name:   "Object type",
			tfType: tftypes.Object{AttributeTypes: map[string]tftypes.Type{"id": tftypes.String}},
			want:   cty.Object(map[string]cty.Type{"id": cty.String}),
		},
		{
			name:   "List type",
			tfType: tftypes.List{ElementType: tftypes.String},
			want:   cty.List(cty.String),
		},
		{
			name:   "Set type",
			tfType: tftypes.Set{ElementType: tftypes.String},
			want:   cty.Set(cty.String),
		},
		{
			name:   "Map type",
			tfType: tftypes.Map{ElementType: tftypes.String},
			want:   cty.Map(cty.String),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := convertTfTypeToCtyType(tt.tfType)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestPfConversionToCtyValue(t *testing.T) {
	t.Parallel()

	t.Run("nil DynamicValue returns null", func(t *testing.T) {
		conv := &pfConversion{
			tfType:  tftypes.String,
			ctyType: cty.String,
		}

		result, err := conv.toCtyValue(nil)
		require.NoError(t, err)
		require.True(t, result.IsNull())
	})

	t.Run("DynamicValue with MsgPack data", func(t *testing.T) {
		tfType := tftypes.String
		strValue := tftypes.NewValue(tfType, "hello")

		dv, err := makeDynamicValue(strValue)
		require.NoError(t, err)

		conv := &pfConversion{
			tfType:  tfType,
			ctyType: cty.String,
		}

		result, err := conv.toCtyValue(&dv)
		require.NoError(t, err)
		require.Equal(t, "hello", result.AsString())
	})

	t.Run("DynamicValue prefers MsgPack when both available", func(t *testing.T) {
		tfType := tftypes.String
		strValue := tftypes.NewValue(tfType, "world")

		dv, err := makeDynamicValue(strValue)
		require.NoError(t, err)

		// Both MsgPack and JSON should be available, but MsgPack is preferred
		require.NotNil(t, dv.MsgPack)

		conv := &pfConversion{
			tfType:  tfType,
			ctyType: cty.String,
		}

		result, err := conv.toCtyValue(&dv)
		require.NoError(t, err)
		require.Equal(t, "world", result.AsString())
	})

	t.Run("DynamicValue with neither MsgPack nor JSON returns error", func(t *testing.T) {
		conv := &pfConversion{
			tfType:  tftypes.String,
			ctyType: cty.String,
		}

		dv := &tfprotov6.DynamicValue{}

		result, err := conv.toCtyValue(dv)
		require.Error(t, err)
		require.True(t, result.IsNull() && result.Type() == cty.NilType)
		require.Contains(t, err.Error(), "DynamicValue has neither MsgPack nor JSON data")
	})

	t.Run("Object type conversion", func(t *testing.T) {
		objType := tftypes.Object{
			AttributeTypes: map[string]tftypes.Type{
				"id":   tftypes.String,
				"name": tftypes.String,
			},
		}

		objValue := tftypes.NewValue(objType, map[string]tftypes.Value{
			"id":   tftypes.NewValue(tftypes.String, "123"),
			"name": tftypes.NewValue(tftypes.String, "test"),
		})

		dv, err := makeDynamicValue(objValue)
		require.NoError(t, err)

		ctyType := cty.Object(map[string]cty.Type{
			"id":   cty.String,
			"name": cty.String,
		})

		conv := &pfConversion{
			tfType:  objType,
			ctyType: ctyType,
		}

		result, err := conv.toCtyValue(&dv)
		require.NoError(t, err)
		require.Equal(t, "123", result.GetAttr("id").AsString())
		require.Equal(t, "test", result.GetAttr("name").AsString())
	})

	t.Run("List type conversion", func(t *testing.T) {
		listType := tftypes.List{ElementType: tftypes.String}
		listValue := tftypes.NewValue(listType, []tftypes.Value{
			tftypes.NewValue(tftypes.String, "a"),
			tftypes.NewValue(tftypes.String, "b"),
		})

		dv, err := makeDynamicValue(listValue)
		require.NoError(t, err)

		ctyType := cty.List(cty.String)

		conv := &pfConversion{
			tfType:  listType,
			ctyType: ctyType,
		}

		result, err := conv.toCtyValue(&dv)
		require.NoError(t, err)

		elements := result.AsValueSlice()
		require.Len(t, elements, 2)
		require.Equal(t, "a", elements[0].AsString())
		require.Equal(t, "b", elements[1].AsString())
	})

	t.Run("Number type conversion", func(t *testing.T) {
		numType := tftypes.Number
		numValue := tftypes.NewValue(numType, 42)

		dv, err := makeDynamicValue(numValue)
		require.NoError(t, err)

		conv := &pfConversion{
			tfType:  numType,
			ctyType: cty.Number,
		}

		result, err := conv.toCtyValue(&dv)
		require.NoError(t, err)

		num := result.AsBigFloat()
		require.NotNil(t, num)
	})

	t.Run("Bool type conversion", func(t *testing.T) {
		boolType := tftypes.Bool
		boolValue := tftypes.NewValue(boolType, true)

		dv, err := makeDynamicValue(boolValue)
		require.NoError(t, err)

		conv := &pfConversion{
			tfType:  boolType,
			ctyType: cty.Bool,
		}

		result, err := conv.toCtyValue(&dv)
		require.NoError(t, err)
		require.True(t, result.True())
	})
}

func TestPfConversionToCtyValueEdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("null string value", func(t *testing.T) {
		tfType := tftypes.String
		strValue := tftypes.NewValue(tfType, nil)

		dv, err := makeDynamicValue(strValue)
		require.NoError(t, err)

		conv := &pfConversion{
			tfType:  tfType,
			ctyType: cty.String,
		}

		result, err := conv.toCtyValue(&dv)
		require.NoError(t, err)
		require.True(t, result.IsNull())
	})

	t.Run("unknown value", func(t *testing.T) {
		tfType := tftypes.String
		strValue := tftypes.NewValue(tfType, tftypes.UnknownValue)

		dv, err := makeDynamicValue(strValue)
		require.NoError(t, err)

		conv := &pfConversion{
			tfType:  tfType,
			ctyType: cty.String,
		}

		result, err := conv.toCtyValue(&dv)
		require.NoError(t, err)
		require.True(t, result.IsMarked() || !result.IsKnown())
	})
}

func TestConvertTfTypeToCtyTypeComplexTypes(t *testing.T) {
	t.Parallel()

	t.Run("nested object type", func(t *testing.T) {
		innerObj := tftypes.Object{
			AttributeTypes: map[string]tftypes.Type{
				"nested": tftypes.String,
			},
		}

		outerObj := tftypes.Object{
			AttributeTypes: map[string]tftypes.Type{
				"inner": innerObj,
			},
		}

		ctyType, err := convertTfTypeToCtyType(outerObj)
		require.NoError(t, err)

		expectedType := cty.Object(map[string]cty.Type{
			"inner": cty.Object(map[string]cty.Type{
				"nested": cty.String,
			}),
		})

		require.Equal(t, expectedType, ctyType)
	})

	t.Run("list of objects", func(t *testing.T) {
		objType := tftypes.Object{
			AttributeTypes: map[string]tftypes.Type{
				"id": tftypes.String,
			},
		}

		listType := tftypes.List{ElementType: objType}

		ctyType, err := convertTfTypeToCtyType(listType)
		require.NoError(t, err)

		expectedType := cty.List(cty.Object(map[string]cty.Type{
			"id": cty.String,
		}))

		require.Equal(t, expectedType, ctyType)
	})

	t.Run("set of numbers", func(t *testing.T) {
		setType := tftypes.Set{ElementType: tftypes.Number}

		ctyType, err := convertTfTypeToCtyType(setType)
		require.NoError(t, err)

		expectedType := cty.Set(cty.Number)
		require.Equal(t, expectedType, ctyType)
	})

	t.Run("map of strings", func(t *testing.T) {
		mapType := tftypes.Map{ElementType: tftypes.String}

		ctyType, err := convertTfTypeToCtyType(mapType)
		require.NoError(t, err)

		expectedType := cty.Map(cty.String)
		require.Equal(t, expectedType, ctyType)
	})
}

func TestPfConversionRoundTrip(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		tfValue  tftypes.Value
		tfType   tftypes.Type
		ctyType  cty.Type
		validate func(*testing.T, cty.Value)
	}{
		{
			name:    "string roundtrip",
			tfValue: tftypes.NewValue(tftypes.String, "test"),
			tfType:  tftypes.String,
			ctyType: cty.String,
			validate: func(t *testing.T, val cty.Value) {
				require.Equal(t, "test", val.AsString())
			},
		},
		{
			name:    "number roundtrip",
			tfValue: tftypes.NewValue(tftypes.Number, 123),
			tfType:  tftypes.Number,
			ctyType: cty.Number,
			validate: func(t *testing.T, val cty.Value) {
				require.NotNil(t, val.AsBigFloat())
			},
		},
		{
			name:    "bool roundtrip",
			tfValue: tftypes.NewValue(tftypes.Bool, false),
			tfType:  tftypes.Bool,
			ctyType: cty.Bool,
			validate: func(t *testing.T, val cty.Value) {
				require.False(t, val.True())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dv, err := makeDynamicValue(tt.tfValue)
			require.NoError(t, err)

			conv := &pfConversion{
				tfType:  tt.tfType,
				ctyType: tt.ctyType,
			}

			result, err := conv.toCtyValue(&dv)
			require.NoError(t, err)

			tt.validate(t, result)
		})
	}
}

func TestConversionErrorHandling(t *testing.T) {
	t.Parallel()

	t.Run("invalid msgpack data", func(t *testing.T) {
		dv := &tfprotov6.DynamicValue{
			MsgPack: []byte("invalid"),
		}

		conv := &pfConversion{
			tfType:  tftypes.String,
			ctyType: cty.String,
		}

		_, err := conv.toCtyValue(dv)
		require.Error(t, err)
	})

	t.Run("invalid JSON data", func(t *testing.T) {
		dv := &tfprotov6.DynamicValue{
			MsgPack: nil,
			JSON:    []byte("invalid json"),
		}

		conv := &pfConversion{
			tfType:  tftypes.String,
			ctyType: cty.String,
		}

		_, err := conv.toCtyValue(dv)
		require.Error(t, err)
	})
}

func TestConversionTypeMatching(t *testing.T) {
	t.Parallel()

	t.Run("type mismatch msgpack", func(t *testing.T) {
		tfType := tftypes.String
		intValue := tftypes.NewValue(tftypes.Number, 42)

		dv, err := makeDynamicValue(intValue)
		require.NoError(t, err)

		conv := &pfConversion{
			tfType:  tfType,
			ctyType: cty.String,
		}

		_, err = conv.toCtyValue(&dv)
		require.Error(t, err)
	})
}

func TestPfConversionComplexObjects(t *testing.T) {
	t.Parallel()

	t.Run("deeply nested object", func(t *testing.T) {
		innermost := tftypes.Object{
			AttributeTypes: map[string]tftypes.Type{
				"value": tftypes.String,
			},
		}

		middle := tftypes.Object{
			AttributeTypes: map[string]tftypes.Type{
				"inner": innermost,
			},
		}

		outer := tftypes.Object{
			AttributeTypes: map[string]tftypes.Type{
				"middle": middle,
			},
		}

		ctyType, err := convertTfTypeToCtyType(outer)
		require.NoError(t, err)

		expectedType := cty.Object(map[string]cty.Type{
			"middle": cty.Object(map[string]cty.Type{
				"inner": cty.Object(map[string]cty.Type{
					"value": cty.String,
				}),
			}),
		})

		require.Equal(t, expectedType, ctyType)
	})

	t.Run("object with multiple field types", func(t *testing.T) {
		objType := tftypes.Object{
			AttributeTypes: map[string]tftypes.Type{
				"id":      tftypes.String,
				"count":   tftypes.Number,
				"enabled": tftypes.Bool,
				"tags":    tftypes.Map{ElementType: tftypes.String},
				"items":   tftypes.List{ElementType: tftypes.String},
			},
		}

		tagsVal := map[string]tftypes.Value{
			"env": tftypes.NewValue(tftypes.String, "test"),
		}
		itemsVal := []tftypes.Value{
			tftypes.NewValue(tftypes.String, "a"),
		}

		objValue := tftypes.NewValue(objType, map[string]tftypes.Value{
			"id":      tftypes.NewValue(tftypes.String, "123"),
			"count":   tftypes.NewValue(tftypes.Number, 5),
			"enabled": tftypes.NewValue(tftypes.Bool, true),
			"tags": tftypes.NewValue(
				tftypes.Map{ElementType: tftypes.String},
				tagsVal,
			),
			"items": tftypes.NewValue(
				tftypes.List{ElementType: tftypes.String},
				itemsVal,
			),
		})

		dv, err := makeDynamicValue(objValue)
		require.NoError(t, err)

		ctyType, err := convertTfTypeToCtyType(objType)
		require.NoError(t, err)

		conv := &pfConversion{
			tfType:  objType,
			ctyType: ctyType,
		}

		result, err := conv.toCtyValue(&dv)
		require.NoError(t, err)

		require.Equal(t, "123", result.GetAttr("id").AsString())
	})
}
