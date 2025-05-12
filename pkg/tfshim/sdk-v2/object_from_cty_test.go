package sdkv2

import (
	"reflect"
	"testing"

	"github.com/hashicorp/go-cty/cty"
	"github.com/stretchr/testify/require"
)

func TestNilCase(t *testing.T) {
	t.Parallel()
	val := cty.NullVal(cty.String)
	result := objectFromCtyValue(val)
	require.Nil(t, result)
}

func TestEncapsulatedTypeFail(t *testing.T) {
	t.Parallel()
	innerVal := "test"
	val := cty.CapsuleVal(cty.Capsule("name", reflect.TypeOf(innerVal)), &innerVal)
	require.Panics(t, func() {
		objectFromCtyValueInner(val)
	})
}

func TestObjectFromCtyValue(t *testing.T) {
	t.Parallel()
	t.Run("Null", func(t *testing.T) {
		val := cty.NullVal(cty.String)
		result := objectFromCtyValueInner(val)
		require.Nil(t, result)
	})

	t.Run("Unknown", func(t *testing.T) {
		val := cty.UnknownVal(cty.String)
		result := objectFromCtyValueInner(val)
		require.Equal(t, terraformUnknownVariableValue, result)
	})

	t.Run("Dynamic Value", func(t *testing.T) {
		val := cty.DynamicVal
		result := objectFromCtyValueInner(val)
		require.Equal(t, terraformUnknownVariableValue, result)
	})

	t.Run("String", func(t *testing.T) {
		val := cty.StringVal("test")
		result := objectFromCtyValueInner(val)
		require.Equal(t, "test", result)
	})

	t.Run("Number", func(t *testing.T) {
		val := cty.NumberIntVal(42)
		result := objectFromCtyValueInner(val)
		require.Equal(t, "42", result)
	})

	t.Run("Bool", func(t *testing.T) {
		val := cty.BoolVal(true)
		result := objectFromCtyValueInner(val)
		require.Equal(t, true, result)
	})

	t.Run("List", func(t *testing.T) {
		val := cty.ListVal([]cty.Value{
			cty.StringVal("one"),
			cty.StringVal("two"),
		})
		result := objectFromCtyValueInner(val)
		expected := []interface{}{"one", "two"}

		require.Equal(t, expected, result)
	})

	t.Run("Set", func(t *testing.T) {
		val := cty.SetVal([]cty.Value{
			cty.StringVal("one"),
			cty.StringVal("two"),
		})
		result := objectFromCtyValueInner(val)
		// Note: sets might have non-deterministic order when converted to list
		elements := result.([]interface{})
		require.Equal(t, 2, len(elements))
		require.Contains(t, elements, "one")
		require.Contains(t, elements, "two")
	})

	t.Run("Map", func(t *testing.T) {
		val := cty.MapVal(map[string]cty.Value{
			"key1": cty.StringVal("value1"),
			"key2": cty.StringVal("value2"),
		})
		result := objectFromCtyValue(val)
		expected := map[string]interface{}{
			"key1": "value1",
			"key2": "value2",
		}

		require.Equal(t, expected, result)
	})

	t.Run("Map with different types", func(t *testing.T) {
		val := cty.ObjectVal(map[string]cty.Value{
			"key1": cty.StringVal("value1"),
			"key2": cty.NumberIntVal(42),
		})
		result := objectFromCtyValue(val)
		expected := map[string]interface{}{
			"key1": "value1",
			"key2": "42",
		}

		require.Equal(t, expected, result)
	})

	t.Run("Object", func(t *testing.T) {
		val := cty.ObjectVal(map[string]cty.Value{
			"name":  cty.StringVal("test"),
			"count": cty.NumberIntVal(5),
			"valid": cty.BoolVal(true),
		})

		result := objectFromCtyValue(val)
		expected := map[string]interface{}{
			"name":  "test",
			"count": "5",
			"valid": true,
		}

		require.Equal(t, expected, result)
	})

	t.Run("Nested", func(t *testing.T) {
		val := cty.ObjectVal(map[string]cty.Value{
			"name": cty.StringVal("parent"),
			"child": cty.ObjectVal(map[string]cty.Value{
				"name": cty.StringVal("child"),
				"list": cty.ListVal([]cty.Value{
					cty.StringVal("item1"),
					cty.StringVal("item2"),
				}),
			}),
		})

		result := objectFromCtyValue(val)
		expected := map[string]interface{}{
			"name": "parent",
			"child": map[string]interface{}{
				"name": "child",
				"list": []interface{}{"item1", "item2"},
			},
		}

		require.Equal(t, expected, result)
	})
}
