package tfbridge

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	shimv2 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v2"
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

		require.Error(t, err)
		require.IsType(t, TypeMismatchError{}, err)
	})

	t.Run("both arrays error", func(t *testing.T) {
		val1 := resource.NewArrayProperty([]resource.PropertyValue{
			resource.NewStringProperty("a"),
		})

		val2 := resource.NewArrayProperty([]resource.PropertyValue{
			resource.NewStringProperty("b"),
		})

		err := walkTwoPropertyValues(
			propertyPath{},
			val1,
			val2,
			func(path propertyPath, v1, v2 resource.PropertyValue) error {
				if v1.IsString() || v2.IsString() {
					return errors.New("error")
				}
				return nil
			})

		require.Error(t, err)
	})

	t.Run("both objects error", func(t *testing.T) {
		val1 := resource.NewObjectProperty(resource.PropertyMap{
			"a": resource.NewStringProperty("a"),
		})

		val2 := resource.NewObjectProperty(resource.PropertyMap{
			"b": resource.NewStringProperty("b"),
		})

		err := walkTwoPropertyValues(
			propertyPath{},
			val1,
			val2,
			func(path propertyPath, v1, v2 resource.PropertyValue) error {
				if v1.IsString() || v2.IsString() {
					return errors.New("error")
				}
				return nil
			})

		require.Error(t, err)
	})

	t.Run("mismatched types first array error", func(t *testing.T) {
		val1 := resource.NewArrayProperty([]resource.PropertyValue{
			resource.NewStringProperty("a"),
		})

		val2 := resource.NewObjectProperty(resource.PropertyMap{
			"b": resource.NewStringProperty("b"),
		})

		err := walkTwoPropertyValues(
			propertyPath{},
			val1,
			val2,
			func(path propertyPath, v1, v2 resource.PropertyValue) error {
				if v1.IsString() {
					return errors.New("error")
				}
				return nil
			})

		require.Error(t, err)
		require.IsType(t, TypeMismatchError{}, err)
	})

	t.Run("mismatched types first object error", func(t *testing.T) {
		val1 := resource.NewObjectProperty(resource.PropertyMap{
			"a": resource.NewStringProperty("a"),
		})

		val2 := resource.NewArrayProperty([]resource.PropertyValue{
			resource.NewStringProperty("b"),
		})

		err := walkTwoPropertyValues(
			propertyPath{},
			val1,
			val2,
			func(path propertyPath, v1, v2 resource.PropertyValue) error {
				if v1.IsString() {
					return errors.New("error")
				}
				return nil
			})

		require.Error(t, err)
		require.IsType(t, TypeMismatchError{}, err)
	})

	t.Run("mismatched types second array error", func(t *testing.T) {
		val1 := resource.NewObjectProperty(resource.PropertyMap{
			"a": resource.NewStringProperty("a"),
		})

		val2 := resource.NewArrayProperty([]resource.PropertyValue{
			resource.NewStringProperty("b"),
		})

		err := walkTwoPropertyValues(
			propertyPath{},
			val1,
			val2,
			func(path propertyPath, v1, v2 resource.PropertyValue) error {
				if v2.IsString() {
					return errors.New("error")
				}
				return nil
			})

		require.Error(t, err)
		require.IsType(t, TypeMismatchError{}, err)
	})

	t.Run("mismatched types second object error", func(t *testing.T) {
		val1 := resource.NewArrayProperty([]resource.PropertyValue{
			resource.NewStringProperty("a"),
		})

		val2 := resource.NewObjectProperty(resource.PropertyMap{
			"b": resource.NewStringProperty("b"),
		})

		err := walkTwoPropertyValues(
			propertyPath{},
			val1,
			val2,
			func(path propertyPath, v1, v2 resource.PropertyValue) error {
				if v2.IsString() {
					return errors.New("error")
				}
				return nil
			})

		require.Error(t, err)
		require.IsType(t, TypeMismatchError{}, err)
	})
}

func TestPropertyPath(t *testing.T) {
	t.Parallel()
	require.Equal(t, newPropertyPath("foo").Subpath("bar").Key(), detailedDiffKey("foo.bar"))
}

func TestGetPathRelativeTo(t *testing.T) {
	t.Parallel()
	relativePath, err := newPropertyPath("foo").Subpath("bar").GetPathRelativeTo(newPropertyPath("foo"))
	require.NoError(t, err)
	assert.Equal(t, relativePath, newPropertyPath("bar"))

	relativePath, err = newPropertyPath("foo").Subpath("bar").Index(0).GetPathRelativeTo(newPropertyPath("foo"))
	require.NoError(t, err)
	assert.Equal(t, relativePath, newPropertyPath("bar").Index(0))

	_, err = newPropertyPath("foo").GetPathRelativeTo(newPropertyPath("bar"))
	require.Error(t, err)
}

func TestLookupSchemasPropertyPath(t *testing.T) {
	t.Parallel()

	t.Run("string schema", func(t *testing.T) {
		schemaMap := shimv2.NewSchemaMap(map[string]*schema.Schema{
			"foo": {Type: schema.TypeString},
		})

		sch, _, err := lookupSchemas(newPropertyPath("foo"), schemaMap, nil)
		require.NoError(t, err)
		require.Equal(t, shim.TypeString, sch.Type())
	})

	t.Run("list schema", func(t *testing.T) {
		tfs := shimv2.NewSchemaMap(map[string]*schema.Schema{
			"my_list": {
				Type:     schema.TypeList,
				Optional: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
		})

		sch, _, err := lookupSchemas(newPropertyPath("myList"), tfs, nil)
		require.NoError(t, err)
		require.Equal(t, shim.TypeList, sch.Type())
	})

	t.Run("list element schema", func(t *testing.T) {
		tfs := shimv2.NewSchemaMap(map[string]*schema.Schema{
			"my_list": {
				Type:     schema.TypeList,
				Optional: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
		})

		sch, _, err := lookupSchemas(newPropertyPath("myList").Index(0), tfs, nil)
		require.NoError(t, err)
		require.Equal(t, shim.TypeString, sch.Type())
	})
}

func TestValidInputsFromPlan(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		path        propertyPath
		inputValue  resource.PropertyValue
		planValue   resource.PropertyValue
		sdkv2Schema map[string]*schema.Schema
		want        bool
	}{
		{
			name:       "simple matching values",
			path:       newPropertyPath("foo"),
			inputValue: resource.NewStringProperty("bar"),
			planValue:  resource.NewStringProperty("bar"),
			sdkv2Schema: map[string]*schema.Schema{
				"foo": {
					Type:     schema.TypeString,
					Optional: true,
				},
			},
			want: true,
		},
		{
			name:       "computed property allows missing values",
			path:       newPropertyPath("foo"),
			inputValue: resource.NewNullProperty(),
			planValue:  resource.NewStringProperty("computed"),
			sdkv2Schema: map[string]*schema.Schema{
				"foo": {
					Type:     schema.TypeString,
					Optional: true,
					Computed: true,
				},
			},
			want: true,
		},
		{
			name:       "non-computed property requires matching values",
			path:       newPropertyPath("foo"),
			inputValue: resource.NewStringProperty("bar"),
			planValue:  resource.NewStringProperty("different"),
			sdkv2Schema: map[string]*schema.Schema{
				"foo": {
					Type:     schema.TypeString,
					Optional: true,
				},
			},
			want: false,
		},
		{
			name: "set requires exact match",
			path: newPropertyPath("set"),
			inputValue: resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewStringProperty("a"),
				resource.NewStringProperty("b"),
			}),
			planValue: resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewStringProperty("b"),
				resource.NewStringProperty("a"),
			}),
			sdkv2Schema: map[string]*schema.Schema{
				"set": {
					Type:     schema.TypeSet,
					Optional: true,
					Elem: &schema.Schema{
						Type: schema.TypeString,
					},
				},
			},
			want: false,
		},
		{
			name: "missing non-computed property",
			path: newPropertyPath("obj"),
			inputValue: resource.NewArrayProperty(
				[]resource.PropertyValue{
					resource.NewObjectProperty(resource.PropertyMap{}),
				},
			),
			planValue: resource.NewArrayProperty(
				[]resource.PropertyValue{
					resource.NewObjectProperty(resource.PropertyMap{
						"foo": resource.NewStringProperty("bar"),
					}),
				},
			),
			sdkv2Schema: map[string]*schema.Schema{
				"obj": {
					Type: schema.TypeList,
					Elem: &schema.Resource{
						Schema: map[string]*schema.Schema{
							"foo": {
								Type:     schema.TypeString,
								Optional: true,
							},
						},
					},
				},
			},
			want: false,
		},
		{
			name: "missing computed property",
			path: newPropertyPath("obj"),
			inputValue: resource.NewArrayProperty(
				[]resource.PropertyValue{
					resource.NewObjectProperty(resource.PropertyMap{}),
				},
			),
			planValue: resource.NewArrayProperty(
				[]resource.PropertyValue{
					resource.NewObjectProperty(resource.PropertyMap{
						"foo": resource.NewStringProperty("bar"),
					}),
				},
			),
			sdkv2Schema: map[string]*schema.Schema{
				"obj": {
					Type: schema.TypeList,
					Elem: &schema.Resource{
						Schema: map[string]*schema.Schema{
							"foo": {
								Type:     schema.TypeString,
								Optional: true,
								Computed: true,
							},
						},
					},
				},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tfs := shimv2.NewSchemaMap(tt.sdkv2Schema)
			got := validInputsFromPlan(
				tt.path,
				tt.inputValue,
				tt.planValue,
				tfs,
				nil,
			)
			require.Equal(t, tt.want, got)
		})
	}
}
