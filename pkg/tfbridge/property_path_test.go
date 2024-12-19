package tfbridge

import (
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/require"
)

func TestWalkTwoPropertyValues(t *testing.T) {
	t.Parallel()
	t.Run("simple values", func(t *testing.T) {
		val1 := resource.NewStringProperty("hello")
		val2 := resource.NewStringProperty("world")
		visited := make(map[string]bool)

		err := walkTwoPropertyValues(
			propertyPath{},
			val1,
			val2,
			func(path propertyPath, v1, v2 resource.PropertyValue) error {
				visited[path.String()] = true
				require.Equal(t, "hello", v1.StringValue())
				require.Equal(t, "world", v2.StringValue())
				return nil
			})

		require.NoError(t, err)
		require.True(t, visited[""])
	})

	t.Run("arrays", func(t *testing.T) {
		val1 := resource.NewArrayProperty([]resource.PropertyValue{
			resource.NewStringProperty("a"),
			resource.NewStringProperty("b"),
		})
		val2 := resource.NewArrayProperty([]resource.PropertyValue{
			resource.NewStringProperty("x"),
			resource.NewStringProperty("y"),
			resource.NewStringProperty("z"),
		})
		visited := make([]string, 0)

		err := walkTwoPropertyValues(
			propertyPath{},
			val1,
			val2,
			func(path propertyPath, v1, v2 resource.PropertyValue) error {
				visited = append(visited, path.String())
				return nil
			})

		require.NoError(t, err)
		require.Equal(t, []string{"", "[0]", "[1]", "[2]"}, visited)
	})

	t.Run("objects", func(t *testing.T) {
		val1 := resource.NewObjectProperty(resource.PropertyMap{
			"a": resource.NewStringProperty("value1"),
		})
		val2 := resource.NewObjectProperty(resource.PropertyMap{
			"b": resource.NewStringProperty("value2"),
		})
		visited := make([]string, 0)

		err := walkTwoPropertyValues(
			propertyPath{},
			val1,
			val2,
			func(path propertyPath, v1, v2 resource.PropertyValue) error {
				visited = append(visited, path.String())
				return nil
			})

		require.NoError(t, err)
		require.Len(t, visited, 3)
		require.Contains(t, visited, "")
		require.Contains(t, visited, "a")
		require.Contains(t, visited, "b")
	})

	t.Run("skip children", func(t *testing.T) {
		val1 := resource.NewArrayProperty([]resource.PropertyValue{
			resource.NewStringProperty("a"),
			resource.NewStringProperty("b"),
		})
		val2 := resource.NewArrayProperty([]resource.PropertyValue{
			resource.NewStringProperty("x"),
			resource.NewStringProperty("y"),
		})
		visited := make([]string, 0)

		err := walkTwoPropertyValues(
			propertyPath{},
			val1,
			val2,
			func(path propertyPath, v1, v2 resource.PropertyValue) error {
				visited = append(visited, path.String())
				return SkipChildrenError{}
			})

		require.NoError(t, err)
		require.Equal(t, []string{""}, visited)
	})

	t.Run("nested structure", func(t *testing.T) {
		val1 := resource.NewObjectProperty(resource.PropertyMap{
			"array": resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewObjectProperty(resource.PropertyMap{
					"key": resource.NewStringProperty("value1"),
				}),
			}),
		})
		val2 := resource.NewObjectProperty(resource.PropertyMap{
			"array": resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewObjectProperty(resource.PropertyMap{
					"key": resource.NewStringProperty("value2"),
				}),
			}),
		})
		visited := make([]string, 0)

		err := walkTwoPropertyValues(
			propertyPath{},
			val1,
			val2,
			func(path propertyPath, v1, v2 resource.PropertyValue) error {
				visited = append(visited, path.String())
				return nil
			})

		require.NoError(t, err)
		require.Equal(t, []string{"", "array", "array[0]", "array[0].key"}, visited)
	})

	t.Run("mismatched types", func(t *testing.T) {
		val1 := resource.NewObjectProperty(resource.PropertyMap{
			"a": resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewStringProperty("hello"),
			}),
		})
		val2 := resource.NewObjectProperty(resource.PropertyMap{
			"a": resource.NewObjectProperty(resource.PropertyMap{
				"b": resource.NewStringProperty("world"),
			}),
		})

		visited := make([]string, 0)

		err := walkTwoPropertyValues(
			propertyPath{},
			val1,
			val2,
			func(path propertyPath, v1, v2 resource.PropertyValue) error {
				visited = append(visited, path.String())
				return nil
			})

		require.NoError(t, err)
		require.Equal(t, []string{"", "a", "a[0]", "a.b"}, visited)
	})
}
