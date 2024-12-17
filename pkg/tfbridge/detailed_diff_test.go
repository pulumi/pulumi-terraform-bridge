package tfbridge

import (
	"bytes"
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/require"
	"gotest.tools/v3/assert"

	"github.com/pulumi/pulumi-terraform-bridge/v3/internal/logging"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	shimschema "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/schema"
	shimv2 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v2"
)

func TestDiffPair(t *testing.T) {
	t.Parallel()
	assert.Equal(t, (newPropertyPath("foo").Subpath("bar")).Key(), detailedDiffKey("foo.bar"))
	assert.Equal(t, newPropertyPath("foo").Subpath("bar").Subpath("baz").Key(), detailedDiffKey("foo.bar.baz"))
	assert.Equal(t, newPropertyPath("foo").Subpath("bar.baz").Key(), detailedDiffKey(`foo["bar.baz"]`))
	assert.Equal(t, newPropertyPath("foo").Index(2).Key(), detailedDiffKey("foo[2]"))
}

func TestReservedKey(t *testing.T) {
	t.Parallel()
	assert.Equal(t, newPropertyPath("foo").Subpath("__meta").IsReservedKey(), true)
	assert.Equal(t, newPropertyPath("foo").Subpath("__defaults").IsReservedKey(), true)
	assert.Equal(t, newPropertyPath("__defaults").IsReservedKey(), true)
	assert.Equal(t, newPropertyPath("foo").Subpath("bar").IsReservedKey(), false)
}

func TestSchemaLookupMaxItemsOnePlain(t *testing.T) {
	t.Parallel()
	sdkv2Schema := map[string]*schema.Schema{
		"string_prop": {
			Type:     schema.TypeList,
			Optional: true,
			MaxItems: 1,
			Elem: &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
			},
		},
	}

	tfs := shimv2.NewSchemaMap(sdkv2Schema)

	sch, _, err := lookupSchemas(newPropertyPath("string_prop"), tfs, nil)
	require.NoError(t, err)
	require.NotNil(t, sch)
	require.Equal(t, sch.Type(), shim.TypeList)
}

func TestSchemaLookupMaxItemsOne(t *testing.T) {
	t.Parallel()
	res := schema.Resource{
		Schema: map[string]*schema.Schema{
			"foo": {
				Type:     schema.TypeList,
				MaxItems: 1,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"bar": {
							Type:     schema.TypeString,
							Optional: true,
						},
					},
				},
			},
		},
	}

	tfs := shimv2.NewSchemaMap(res.Schema)

	sch, _, err := lookupSchemas(newPropertyPath("foo"), tfs, nil)
	require.NoError(t, err)
	require.NotNil(t, sch)
	require.Equal(t, sch.Type(), shim.TypeList)

	sch, _, err = lookupSchemas(newPropertyPath("foo").Subpath("bar"), tfs, nil)
	require.NoError(t, err)
	require.NotNil(t, sch)
	require.Equal(t, sch.Type(), shim.TypeString)
}

func TestSchemaLookupMap(t *testing.T) {
	t.Parallel()
	res := schema.Resource{
		Schema: map[string]*schema.Schema{
			"foo": {
				Type:     schema.TypeMap,
				Optional: true,
				Elem: &schema.Schema{
					Type:     schema.TypeString,
					Optional: true,
				},
			},
		},
	}

	tfs := shimv2.NewSchemaMap(res.Schema)

	sch, _, err := lookupSchemas(newPropertyPath("foo"), tfs, nil)
	require.NoError(t, err)
	require.NotNil(t, sch)
	require.Equal(t, sch.Type(), shim.TypeMap)

	sch, _, err = lookupSchemas(newPropertyPath("foo").Subpath("bar"), tfs, nil)
	require.NoError(t, err)
	require.NotNil(t, sch)
	require.Equal(t, sch.Type(), shim.TypeString)
}

func TestMakeBaseDiff(t *testing.T) {
	t.Parallel()
	nilVal := resource.NewNullProperty()
	nilArr := resource.NewArrayProperty(nil)
	nilMap := resource.NewObjectProperty(nil)
	nonNilVal := resource.NewStringProperty("foo")
	nonNilVal2 := resource.NewStringProperty("bar")

	assert.Equal(t, makeBaseDiff(nilVal, nilVal), noDiff)
	assert.Equal(t, makeBaseDiff(nilVal, nilVal), noDiff)
	assert.Equal(t, makeBaseDiff(nilVal, nonNilVal), addDiff)
	assert.Equal(t, makeBaseDiff(nilVal, nonNilVal), addDiff)
	assert.Equal(t, makeBaseDiff(nonNilVal, nilVal), deleteDiff)
	assert.Equal(t, makeBaseDiff(nonNilVal, nilVal), deleteDiff)
	assert.Equal(t, makeBaseDiff(nonNilVal, nilArr), deleteDiff)
	assert.Equal(t, makeBaseDiff(nonNilVal, nilMap), deleteDiff)
	assert.Equal(t, makeBaseDiff(nonNilVal, nonNilVal2), undecidedDiff)
}

func TestMakePropDiff(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		old      resource.PropertyValue
		new      resource.PropertyValue
		etf      shimschema.Schema
		eps      *SchemaInfo
		expected *pulumirpc.PropertyDiff
	}{
		{
			name:     "unchanged non-nil",
			old:      resource.NewStringProperty("same"),
			new:      resource.NewStringProperty("same"),
			expected: nil,
		},
		{
			name:     "unchanged nil",
			old:      resource.NewNullProperty(),
			new:      resource.NewNullProperty(),
			expected: nil,
		},
		{
			name:     "unchanged not present",
			old:      resource.NewNullProperty(),
			new:      resource.NewNullProperty(),
			expected: nil,
		},
		{
			name:     "added()",
			old:      resource.NewNullProperty(),
			new:      resource.NewStringProperty("new"),
			expected: &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_ADD},
		},
		{
			name:     "deleted()",
			old:      resource.NewStringProperty("old"),
			new:      resource.NewNullProperty(),
			expected: &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_DELETE},
		},
		{
			name:     "changed non-nil",
			old:      resource.NewStringProperty("old"),
			new:      resource.NewStringProperty("new"),
			expected: &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_UPDATE},
		},
		{
			name:     "changed from nil",
			old:      resource.NewNullProperty(),
			new:      resource.NewStringProperty("new"),
			expected: &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_ADD},
		},
		{
			name:     "changed to nil",
			old:      resource.NewStringProperty("old"),
			new:      resource.NewNullProperty(),
			expected: &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_DELETE},
		},
		{
			name:     "tf force new unchanged",
			old:      resource.NewStringProperty("old"),
			new:      resource.NewStringProperty("old"),
			etf:      shimschema.Schema{ForceNew: true},
			expected: nil,
		},
		{
			name:     "tf force new changed non-nil",
			old:      resource.NewStringProperty("old"),
			new:      resource.NewStringProperty("new"),
			etf:      shimschema.Schema{ForceNew: true},
			expected: &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_UPDATE_REPLACE},
		},
		{
			name:     "tf force new changed from nil",
			old:      resource.NewNullProperty(),
			new:      resource.NewStringProperty("new"),
			etf:      shimschema.Schema{ForceNew: true},
			expected: &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_ADD_REPLACE},
		},
		{
			name:     "tf force new changed to nil",
			old:      resource.NewStringProperty("old"),
			new:      resource.NewNullProperty(),
			etf:      shimschema.Schema{ForceNew: true},
			expected: &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_DELETE_REPLACE},
		},
		{
			name:     "ps force new unchanged",
			old:      resource.NewStringProperty("old"),
			new:      resource.NewStringProperty("old"),
			eps:      &SchemaInfo{ForceNew: True()},
			expected: nil,
		},
		{
			name:     "ps force new changed non-nil",
			old:      resource.NewStringProperty("old"),
			new:      resource.NewStringProperty("new"),
			eps:      &SchemaInfo{ForceNew: True()},
			expected: &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_UPDATE_REPLACE},
		},
		{
			name:     "ps force new changed from nil",
			old:      resource.NewNullProperty(),
			new:      resource.NewStringProperty("new"),
			eps:      &SchemaInfo{ForceNew: True()},
			expected: &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_ADD_REPLACE},
		},
		{
			name:     "ps force new changed to nil",
			old:      resource.NewStringProperty("old"),
			new:      resource.NewNullProperty(),
			eps:      &SchemaInfo{ForceNew: True()},
			expected: &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_DELETE_REPLACE},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := detailedDiffer{
				tfs: shimschema.SchemaMap{"foo": tt.etf.Shim()},
				ps:  map[string]*SchemaInfo{"foo": tt.eps},
			}.makePlainPropDiff(newPropertyPath("foo"), tt.old, tt.new)

			var expected map[detailedDiffKey]*pulumirpc.PropertyDiff
			if tt.expected != nil {
				expected = make(map[detailedDiffKey]*pulumirpc.PropertyDiff)
				expected["foo"] = tt.expected
			}

			require.Equal(t, expected, actual)
		})
	}
}

func added() map[string]*pulumirpc.PropertyDiff {
	return map[string]*pulumirpc.PropertyDiff{
		"foo": {Kind: pulumirpc.PropertyDiff_ADD},
	}
}

func updated() map[string]*pulumirpc.PropertyDiff {
	return map[string]*pulumirpc.PropertyDiff{
		"foo": {Kind: pulumirpc.PropertyDiff_UPDATE},
	}
}

func deleted() map[string]*pulumirpc.PropertyDiff {
	return map[string]*pulumirpc.PropertyDiff{
		"foo": {Kind: pulumirpc.PropertyDiff_DELETE},
	}
}

var ComputedVal = resource.NewComputedProperty(resource.Computed{Element: resource.NewStringProperty("")})

func runDetailedDiffTest(
	t *testing.T,
	old, new resource.PropertyMap,
	tfs shim.SchemaMap,
	ps map[string]*SchemaInfo,
	expected map[string]*pulumirpc.PropertyDiff,
) {
	t.Helper()
	actual := MakeDetailedDiffV2(context.Background(), tfs, ps, old, new, new, nil)

	require.Equal(t, expected, actual)
}

func TestBasicDetailedDiff(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name       string
		emptyValue interface{}
		value1     interface{}
		value2     interface{}
		tfs        schema.Schema
		listLike   bool
		objectLike bool
	}{
		{
			name:       "string",
			emptyValue: "",
			value1:     "foo",
			value2:     "bar",
			tfs:        schema.Schema{Type: schema.TypeString},
		},
		{
			name:       "int",
			emptyValue: nil,
			value1:     42,
			value2:     43,
			tfs:        schema.Schema{Type: schema.TypeInt},
		},
		{
			name:       "bool",
			emptyValue: nil,
			value1:     true,
			value2:     false,
			tfs:        schema.Schema{Type: schema.TypeBool},
		},
		{
			name:       "float",
			emptyValue: nil,
			value1:     42.0,
			value2:     43.0,
			tfs:        schema.Schema{Type: schema.TypeFloat},
		},
		{
			name: "list",
			tfs: schema.Schema{
				Type: schema.TypeList,
				Elem: &schema.Schema{Type: schema.TypeString},
			},
			emptyValue: []interface{}{},
			value1:     []interface{}{"foo"},
			value2:     []interface{}{"bar"},
			listLike:   true,
		},
		{
			name: "map",
			tfs: schema.Schema{
				Type: schema.TypeMap,
				Elem: &schema.Schema{Type: schema.TypeString},
			},
			emptyValue: map[string]interface{}{},
			value1:     map[string]interface{}{"foo": "bar"},
			value2:     map[string]interface{}{"foo": "baz"},
			objectLike: true,
		},
		{
			name: "set",
			tfs: schema.Schema{
				Type:     schema.TypeSet,
				Elem:     &schema.Schema{Type: schema.TypeString},
				Optional: true,
			},
			emptyValue: []interface{}{},
			value1:     []interface{}{"foo"},
			value2:     []interface{}{"bar"},
			listLike:   true,
		},
		{
			name: "list block",
			tfs: schema.Schema{
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
			emptyValue: []interface{}{},
			value1:     []interface{}{map[string]interface{}{"foo": "bar"}},
			value2:     []interface{}{map[string]interface{}{"foo": "baz"}},
			listLike:   true,
			objectLike: true,
		},
		{
			name: "max items one list block",
			tfs: schema.Schema{
				Type: schema.TypeList,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"foo": {
							Type:     schema.TypeString,
							Optional: true,
						},
					},
				},
				MaxItems: 1,
			},
			emptyValue: map[string]interface{}{},
			value1:     map[string]interface{}{"foo": "bar"},
			value2:     map[string]interface{}{"foo": "baz"},
			objectLike: true,
		},
		{
			name: "set block",
			tfs: schema.Schema{
				Type: schema.TypeSet,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"foo": {
							Type:     schema.TypeString,
							Optional: true,
						},
					},
				},
			},
			emptyValue: []interface{}{},
			value1:     []interface{}{map[string]interface{}{"foo": "bar"}},
			value2:     []interface{}{map[string]interface{}{"foo": "baz"}},
			listLike:   true,
			objectLike: true,
		},
		{
			name: "max items one set block",
			tfs: schema.Schema{
				Type: schema.TypeSet,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"foo": {
							Type:     schema.TypeString,
							Optional: true,
						},
					},
				},
				MaxItems: 1,
			},
			emptyValue: map[string]interface{}{},
			value1:     map[string]interface{}{"foo": "bar"},
			value2:     map[string]interface{}{"foo": "baz"},
			objectLike: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			sdkv2Schema := map[string]*schema.Schema{
				"foo": &tt.tfs,
			}

			ps, tfs := map[string]*info.Schema{}, shimv2.NewSchemaMap(sdkv2Schema)
			propertyMapNil := resource.NewPropertyMapFromMap(
				map[string]interface{}{},
			)
			propertyMapEmpty := resource.NewPropertyMapFromMap(
				map[string]interface{}{
					"foo": tt.emptyValue,
				},
			)
			propertyMapValue1 := resource.NewPropertyMapFromMap(
				map[string]interface{}{
					"foo": tt.value1,
				},
			)
			propertyMapValue2 := resource.NewPropertyMapFromMap(
				map[string]interface{}{
					"foo": tt.value2,
				},
			)
			propertyMapComputed := resource.NewPropertyMapFromMap(
				map[string]interface{}{
					"foo": ComputedVal,
				},
			)
			propertyMapSecretValue1 := resource.NewPropertyMapFromMap(
				map[string]interface{}{
					"foo": tt.value1,
				},
			)
			propertyMapSecretValue1["foo"] = resource.NewSecretProperty(
				&resource.Secret{Element: propertyMapSecretValue1["foo"]})

			propertyMapSecretValue2 := resource.NewPropertyMapFromMap(
				map[string]interface{}{
					"foo": tt.value2,
				},
			)
			propertyMapSecretValue2["foo"] = resource.NewSecretProperty(
				&resource.Secret{Element: propertyMapSecretValue2["foo"]})

			propertyMapOutputValue1 := resource.NewPropertyMapFromMap(
				map[string]interface{}{
					"foo": tt.value1,
				},
			)
			propertyMapOutputValue1["foo"] = resource.NewOutputProperty(
				resource.Output{Element: propertyMapOutputValue1["foo"], Known: true})

			propertyMapOutputValue2 := resource.NewPropertyMapFromMap(
				map[string]interface{}{
					"foo": tt.value2,
				},
			)
			propertyMapOutputValue2["foo"] = resource.NewOutputProperty(
				resource.Output{Element: propertyMapOutputValue2["foo"], Known: true})

			defaultChangePath := "foo"
			if tt.listLike && tt.objectLike {
				defaultChangePath = "foo[0].foo"
			} else if tt.listLike {
				defaultChangePath = "foo[0]"
			} else if tt.objectLike {
				defaultChangePath = "foo.foo"
			}

			t.Run("unchanged", func(t *testing.T) {
				runDetailedDiffTest(t, propertyMapValue1, propertyMapValue1, tfs, ps, map[string]*pulumirpc.PropertyDiff{})
			})

			t.Run("changed non-empty", func(t *testing.T) {
				expected := map[string]*pulumirpc.PropertyDiff{
					defaultChangePath: {Kind: pulumirpc.PropertyDiff_UPDATE},
				}
				runDetailedDiffTest(t, propertyMapValue1, propertyMapValue2, tfs, ps, expected)
			})

			t.Run("changed non-empty computed", func(t *testing.T) {
				runDetailedDiffTest(t, propertyMapValue1, propertyMapComputed, tfs, ps, updated())
			})

			t.Run("added", func(t *testing.T) {
				runDetailedDiffTest(t, propertyMapNil, propertyMapValue1, tfs, ps, added())
			})

			if tt.emptyValue != nil {
				t.Run("added empty", func(t *testing.T) {
					runDetailedDiffTest(t, propertyMapNil, propertyMapEmpty, tfs, ps, added())
				})
			}

			t.Run("added computed", func(t *testing.T) {
				runDetailedDiffTest(t, propertyMapNil, propertyMapComputed, tfs, ps, added())
			})

			t.Run("deleted", func(t *testing.T) {
				runDetailedDiffTest(t, propertyMapValue1, propertyMapNil, tfs, ps, deleted())
			})

			if tt.emptyValue != nil {
				t.Run("changed from empty", func(t *testing.T) {
					expected := make(map[string]*pulumirpc.PropertyDiff)
					if tt.listLike {
						expected["foo[0]"] = &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_ADD}
					} else if tt.objectLike {
						expected["foo.foo"] = &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_ADD}
					} else {
						expected["foo"] = &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_UPDATE}
					}
					runDetailedDiffTest(t, propertyMapEmpty, propertyMapValue1, tfs, ps, expected)
				})

				t.Run("changed to empty", func(t *testing.T) {
					expected := make(map[string]*pulumirpc.PropertyDiff)
					if tt.listLike {
						expected["foo[0]"] = &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_DELETE}
					} else if tt.objectLike {
						expected["foo.foo"] = &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_DELETE}
					} else {
						expected["foo"] = &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_UPDATE}
					}
					runDetailedDiffTest(t, propertyMapValue1, propertyMapEmpty, tfs, ps, expected)
				})

				t.Run("changed from empty to computed", func(t *testing.T) {
					runDetailedDiffTest(t, propertyMapEmpty, propertyMapComputed, tfs, ps, updated())
				})

				t.Run("unchanged empty", func(t *testing.T) {
					runDetailedDiffTest(t, propertyMapEmpty, propertyMapEmpty, tfs, ps, map[string]*pulumirpc.PropertyDiff{})
				})

				t.Run("deleted() empty", func(t *testing.T) {
					runDetailedDiffTest(t, propertyMapEmpty, propertyMapNil, tfs, ps, deleted())
				})

				t.Run("added() empty", func(t *testing.T) {
					runDetailedDiffTest(t, propertyMapNil, propertyMapEmpty, tfs, ps, added())
				})
			}

			t.Run("secret unchanged", func(t *testing.T) {
				runDetailedDiffTest(
					t, propertyMapSecretValue1, propertyMapSecretValue1, tfs, ps, map[string]*pulumirpc.PropertyDiff{})
			})

			t.Run("value unchanged secretness changed", func(t *testing.T) {
				runDetailedDiffTest(
					t, propertyMapValue1, propertyMapSecretValue1, tfs, ps, map[string]*pulumirpc.PropertyDiff{})
			})

			t.Run("secret added", func(t *testing.T) {
				runDetailedDiffTest(t, propertyMapNil, propertyMapSecretValue1, tfs, ps, added())
			})

			t.Run("secret deleted", func(t *testing.T) {
				runDetailedDiffTest(t, propertyMapSecretValue1, propertyMapNil, tfs, ps, deleted())
			})

			t.Run("secret changed", func(t *testing.T) {
				expected := map[string]*pulumirpc.PropertyDiff{
					defaultChangePath: {Kind: pulumirpc.PropertyDiff_UPDATE},
				}
				runDetailedDiffTest(t, propertyMapSecretValue1, propertyMapSecretValue2, tfs, ps, expected)
			})

			t.Run("output unchanged", func(t *testing.T) {
				runDetailedDiffTest(
					t, propertyMapOutputValue1, propertyMapOutputValue1, tfs, ps, map[string]*pulumirpc.PropertyDiff{})
			})

			t.Run("value unchanged outputness changed", func(t *testing.T) {
				runDetailedDiffTest(
					t, propertyMapValue1, propertyMapOutputValue1, tfs, ps, map[string]*pulumirpc.PropertyDiff{})
			})

			t.Run("output added", func(t *testing.T) {
				runDetailedDiffTest(t, propertyMapNil, propertyMapOutputValue1, tfs, ps, added())
			})

			t.Run("output deleted", func(t *testing.T) {
				runDetailedDiffTest(t, propertyMapOutputValue1, propertyMapNil, tfs, ps, deleted())
			})

			t.Run("output changed", func(t *testing.T) {
				expected := map[string]*pulumirpc.PropertyDiff{
					defaultChangePath: {Kind: pulumirpc.PropertyDiff_UPDATE},
				}
				runDetailedDiffTest(t, propertyMapOutputValue1, propertyMapOutputValue2, tfs, ps, expected)
			})
		})
	}
}

func TestDetailedDiffObject(t *testing.T) {
	t.Parallel()
	sdkv2Schema := map[string]*schema.Schema{
		"foo": {
			Type: schema.TypeList,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"prop1": {Type: schema.TypeString},
					"prop2": {Type: schema.TypeString},
				},
			},
			MaxItems: 1,
		},
	}
	ps, tfs := map[string]*info.Schema{}, shimv2.NewSchemaMap(sdkv2Schema)

	propertyMapEmpty := resource.NewPropertyMapFromMap(
		map[string]interface{}{
			"foo": map[string]interface{}{},
		},
	)
	propertyMapProp1Val1 := resource.NewPropertyMapFromMap(
		map[string]interface{}{
			"foo": map[string]interface{}{"prop1": "val1"},
		},
	)
	propertyMapProp1Val2 := resource.NewPropertyMapFromMap(
		map[string]interface{}{
			"foo": map[string]interface{}{"prop1": "val2"},
		},
	)
	propertyMapProp2 := resource.NewPropertyMapFromMap(
		map[string]interface{}{
			"foo": map[string]interface{}{"prop2": "qux"},
		},
	)
	propertyMapBothProps := resource.NewPropertyMapFromMap(
		map[string]interface{}{
			"foo": map[string]interface{}{"prop1": "val1", "prop2": "qux"},
		},
	)

	propertyMapWithSecrets := resource.PropertyMap{
		resource.PropertyKey("foo"): resource.NewPropertyValue(
			resource.PropertyMap{
				resource.PropertyKey("prop1"): resource.NewSecretProperty(
					&resource.Secret{Element: resource.NewStringProperty("val1")}),
				resource.PropertyKey("prop2"): resource.NewStringProperty("qux"),
			},
		),
	}

	t.Run("unchanged", func(t *testing.T) {
		runDetailedDiffTest(t, propertyMapProp1Val1, propertyMapProp1Val1, tfs, ps, map[string]*pulumirpc.PropertyDiff{})
	})

	t.Run("changed non-empty", func(t *testing.T) {
		runDetailedDiffTest(t, propertyMapProp1Val1, propertyMapProp1Val2, tfs, ps, map[string]*pulumirpc.PropertyDiff{
			"foo.prop1": {Kind: pulumirpc.PropertyDiff_UPDATE},
		})
	})

	t.Run("changed from empty", func(t *testing.T) {
		runDetailedDiffTest(t, propertyMapEmpty, propertyMapProp1Val1, tfs, ps, map[string]*pulumirpc.PropertyDiff{
			"foo.prop1": {Kind: pulumirpc.PropertyDiff_ADD},
		})
	})

	t.Run("changed from empty both", func(t *testing.T) {
		runDetailedDiffTest(t, propertyMapEmpty, propertyMapBothProps, tfs, ps, map[string]*pulumirpc.PropertyDiff{
			"foo.prop1": {Kind: pulumirpc.PropertyDiff_ADD},
			"foo.prop2": {Kind: pulumirpc.PropertyDiff_ADD},
		})
	})

	t.Run("removed", func(t *testing.T) {
		runDetailedDiffTest(t, propertyMapBothProps, propertyMapProp1Val1, tfs, ps, map[string]*pulumirpc.PropertyDiff{
			"foo.prop2": {Kind: pulumirpc.PropertyDiff_DELETE},
		})
	})

	t.Run("one added one removed", func(t *testing.T) {
		runDetailedDiffTest(t, propertyMapProp1Val1, propertyMapProp2, tfs, ps, map[string]*pulumirpc.PropertyDiff{
			"foo.prop1": {Kind: pulumirpc.PropertyDiff_DELETE},
			"foo.prop2": {Kind: pulumirpc.PropertyDiff_ADD},
		})
	})

	t.Run("added non empty", func(t *testing.T) {
		runDetailedDiffTest(t, propertyMapProp1Val1, propertyMapBothProps, tfs, ps, map[string]*pulumirpc.PropertyDiff{
			"foo.prop2": {Kind: pulumirpc.PropertyDiff_ADD},
		})
	})

	t.Run("secret added", func(t *testing.T) {
		runDetailedDiffTest(t, propertyMapProp2, propertyMapWithSecrets, tfs, ps, map[string]*pulumirpc.PropertyDiff{
			"foo.prop1": {Kind: pulumirpc.PropertyDiff_ADD},
		})
	})

	t.Run("secret deleted", func(t *testing.T) {
		runDetailedDiffTest(t, propertyMapWithSecrets, propertyMapProp2, tfs, ps, map[string]*pulumirpc.PropertyDiff{
			"foo.prop1": {Kind: pulumirpc.PropertyDiff_DELETE},
		})
	})
}

func TestDetailedDiffList(t *testing.T) {
	t.Parallel()
	sdkv2Schema := map[string]*schema.Schema{
		"foo": {
			Type: schema.TypeList,
			Elem: &schema.Schema{Type: schema.TypeString},
		},
	}
	ps, tfs := map[string]*info.Schema{}, shimv2.NewSchemaMap(sdkv2Schema)

	propertyMapEmpty := resource.NewPropertyMapFromMap(
		map[string]interface{}{
			"foo": []interface{}{},
		},
	)
	propertyMapVal1 := resource.NewPropertyMapFromMap(
		map[string]interface{}{
			"foo": []interface{}{"val1"},
		},
	)
	propertyMapVal2 := resource.NewPropertyMapFromMap(
		map[string]interface{}{
			"foo": []interface{}{"val2"},
		},
	)
	propertyMapBoth := resource.NewPropertyMapFromMap(
		map[string]interface{}{
			"foo": []interface{}{"val1", "val2"},
		},
	)

	t.Run("unchanged", func(t *testing.T) {
		runDetailedDiffTest(t, propertyMapVal1, propertyMapVal1, tfs, ps, map[string]*pulumirpc.PropertyDiff{})
	})

	t.Run("changed non-empty", func(t *testing.T) {
		runDetailedDiffTest(t, propertyMapVal1, propertyMapVal2, tfs, ps, map[string]*pulumirpc.PropertyDiff{
			"foo[0]": {Kind: pulumirpc.PropertyDiff_UPDATE},
		})
	})

	t.Run("changed from empty", func(t *testing.T) {
		runDetailedDiffTest(t, propertyMapEmpty, propertyMapVal1, tfs, ps, map[string]*pulumirpc.PropertyDiff{
			"foo[0]": {Kind: pulumirpc.PropertyDiff_ADD},
		})
	})

	t.Run("changed from empty to both", func(t *testing.T) {
		runDetailedDiffTest(t, propertyMapEmpty, propertyMapBoth, tfs, ps, map[string]*pulumirpc.PropertyDiff{
			"foo[0]": {Kind: pulumirpc.PropertyDiff_ADD},
			"foo[1]": {Kind: pulumirpc.PropertyDiff_ADD},
		})
	})

	t.Run("removed", func(t *testing.T) {
		runDetailedDiffTest(t, propertyMapBoth, propertyMapVal1, tfs, ps, map[string]*pulumirpc.PropertyDiff{
			"foo[1]": {Kind: pulumirpc.PropertyDiff_DELETE},
		})
	})

	t.Run("removed both", func(t *testing.T) {
		runDetailedDiffTest(t, propertyMapBoth, propertyMapEmpty, tfs, ps, map[string]*pulumirpc.PropertyDiff{
			"foo[0]": {Kind: pulumirpc.PropertyDiff_DELETE},
			"foo[1]": {Kind: pulumirpc.PropertyDiff_DELETE},
		})
	})
}

func TestDetailedDiffMap(t *testing.T) {
	t.Parallel()
	sdkv2Schema := map[string]*schema.Schema{
		"foo": {
			Type: schema.TypeMap,
			Elem: &schema.Schema{Type: schema.TypeString},
		},
	}
	ps, tfs := map[string]*info.Schema{}, shimv2.NewSchemaMap(sdkv2Schema)

	propertyMapEmpty := resource.NewPropertyMapFromMap(
		map[string]interface{}{
			"foo": map[string]interface{}{},
		},
	)
	propertyMapVal1 := resource.NewPropertyMapFromMap(
		map[string]interface{}{
			"foo": map[string]interface{}{"key1": "val1"},
		},
	)
	propertyMapVal2 := resource.NewPropertyMapFromMap(
		map[string]interface{}{
			"foo": map[string]interface{}{"key1": "val2"},
		},
	)
	propertyMapBoth := resource.NewPropertyMapFromMap(
		map[string]interface{}{
			"foo": map[string]interface{}{"key1": "val1", "key2": "val2"},
		},
	)

	t.Run("unchanged", func(t *testing.T) {
		runDetailedDiffTest(t, propertyMapVal1, propertyMapVal1, tfs, ps, map[string]*pulumirpc.PropertyDiff{})
	})

	t.Run("changed non-empty", func(t *testing.T) {
		runDetailedDiffTest(t, propertyMapVal1, propertyMapVal2, tfs, ps, map[string]*pulumirpc.PropertyDiff{
			"foo.key1": {Kind: pulumirpc.PropertyDiff_UPDATE},
		})
	})

	t.Run("changed from empty", func(t *testing.T) {
		runDetailedDiffTest(t, propertyMapEmpty, propertyMapVal1, tfs, ps, map[string]*pulumirpc.PropertyDiff{
			"foo.key1": {Kind: pulumirpc.PropertyDiff_ADD},
		})
	})

	t.Run("changed from empty to both", func(t *testing.T) {
		runDetailedDiffTest(t, propertyMapEmpty, propertyMapBoth, tfs, ps, map[string]*pulumirpc.PropertyDiff{
			"foo.key1": {Kind: pulumirpc.PropertyDiff_ADD},
			"foo.key2": {Kind: pulumirpc.PropertyDiff_ADD},
		})
	})

	t.Run("removed", func(t *testing.T) {
		runDetailedDiffTest(t, propertyMapBoth, propertyMapVal1, tfs, ps, map[string]*pulumirpc.PropertyDiff{
			"foo.key2": {Kind: pulumirpc.PropertyDiff_DELETE},
		})
	})

	t.Run("removed both", func(t *testing.T) {
		runDetailedDiffTest(t, propertyMapBoth, propertyMapEmpty, tfs, ps, map[string]*pulumirpc.PropertyDiff{
			"foo.key1": {Kind: pulumirpc.PropertyDiff_DELETE},
			"foo.key2": {Kind: pulumirpc.PropertyDiff_DELETE},
		})
	})
}

func TestDetailedDiffSet(t *testing.T) {
	t.Parallel()
	sdkv2Schema := map[string]*schema.Schema{
		"foo": {
			Type: schema.TypeSet,
			Elem: &schema.Schema{Type: schema.TypeString},
		},
	}
	ps, tfs := map[string]*info.Schema{}, shimv2.NewSchemaMap(sdkv2Schema)

	propertyMapEmpty := resource.NewPropertyMapFromMap(
		map[string]interface{}{
			"foo": []interface{}{},
		},
	)
	propertyMapVal1 := resource.NewPropertyMapFromMap(
		map[string]interface{}{
			"foo": []interface{}{"val1"},
		},
	)
	propertyMapVal2 := resource.NewPropertyMapFromMap(
		map[string]interface{}{
			"foo": []interface{}{"val2"},
		},
	)
	propertyMapBoth := resource.NewPropertyMapFromMap(
		map[string]interface{}{
			"foo": []interface{}{"val1", "val2"},
		},
	)

	propertyMapWithSecrets := resource.NewPropertyMapFromMap(
		map[string]interface{}{
			"foo": []interface{}{resource.NewSecretProperty(
				&resource.Secret{Element: resource.NewStringProperty("val1")}), "val2"},
		},
	)

	propertyMapWithSecretsAndOutputs := resource.NewPropertyMapFromMap(
		map[string]interface{}{
			"foo": []interface{}{
				resource.NewSecretProperty(&resource.Secret{Element: resource.NewStringProperty("val1")}),
				resource.NewOutputProperty(resource.Output{Element: resource.NewStringProperty("val2")}),
			},
		},
	)

	t.Run("unchanged", func(t *testing.T) {
		runDetailedDiffTest(t, propertyMapVal1, propertyMapVal1, tfs, ps, map[string]*pulumirpc.PropertyDiff{})
	})

	t.Run("changed non-empty", func(t *testing.T) {
		runDetailedDiffTest(t, propertyMapVal1, propertyMapVal2, tfs, ps, map[string]*pulumirpc.PropertyDiff{
			"foo[0]": {Kind: pulumirpc.PropertyDiff_UPDATE},
		})
	})

	t.Run("changed from empty", func(t *testing.T) {
		runDetailedDiffTest(t, propertyMapEmpty, propertyMapVal1, tfs, ps, map[string]*pulumirpc.PropertyDiff{
			"foo[0]": {Kind: pulumirpc.PropertyDiff_ADD},
		})
	})

	t.Run("changed from empty to both", func(t *testing.T) {
		runDetailedDiffTest(t, propertyMapEmpty, propertyMapBoth, tfs, ps, map[string]*pulumirpc.PropertyDiff{
			"foo[0]": {Kind: pulumirpc.PropertyDiff_ADD},
			"foo[1]": {Kind: pulumirpc.PropertyDiff_ADD},
		})
	})

	t.Run("removed", func(t *testing.T) {
		runDetailedDiffTest(t, propertyMapBoth, propertyMapVal1, tfs, ps, map[string]*pulumirpc.PropertyDiff{
			"foo[1]": {Kind: pulumirpc.PropertyDiff_DELETE},
		})
	})

	t.Run("removed both", func(t *testing.T) {
		runDetailedDiffTest(t, propertyMapBoth, propertyMapEmpty, tfs, ps, map[string]*pulumirpc.PropertyDiff{
			"foo[0]": {Kind: pulumirpc.PropertyDiff_DELETE},
			"foo[1]": {Kind: pulumirpc.PropertyDiff_DELETE},
		})
	})

	t.Run("added", func(t *testing.T) {
		runDetailedDiffTest(t, propertyMapVal1, propertyMapBoth, tfs, ps, map[string]*pulumirpc.PropertyDiff{
			"foo[1]": {Kind: pulumirpc.PropertyDiff_ADD},
		})
	})

	t.Run("added both", func(t *testing.T) {
		runDetailedDiffTest(t, propertyMapEmpty, propertyMapBoth, tfs, ps, map[string]*pulumirpc.PropertyDiff{
			"foo[0]": {Kind: pulumirpc.PropertyDiff_ADD},
			"foo[1]": {Kind: pulumirpc.PropertyDiff_ADD},
		})
	})

	t.Run("secret added", func(t *testing.T) {
		runDetailedDiffTest(t, propertyMapVal2, propertyMapWithSecrets, tfs, ps, map[string]*pulumirpc.PropertyDiff{
			"foo[0]": {Kind: pulumirpc.PropertyDiff_ADD},
		})
	})

	t.Run("secret and output added", func(t *testing.T) {
		runDetailedDiffTest(
			t, propertyMapEmpty, propertyMapWithSecretsAndOutputs, tfs, ps, map[string]*pulumirpc.PropertyDiff{
				"foo[0]": {Kind: pulumirpc.PropertyDiff_ADD},
				"foo[1]": {Kind: pulumirpc.PropertyDiff_ADD},
			})
	})

	t.Run("secret removed", func(t *testing.T) {
		runDetailedDiffTest(t, propertyMapWithSecrets, propertyMapVal2, tfs, ps, map[string]*pulumirpc.PropertyDiff{
			"foo[0]": {Kind: pulumirpc.PropertyDiff_DELETE},
		})
	})

	t.Run("output removed", func(t *testing.T) {
		runDetailedDiffTest(t, propertyMapWithSecretsAndOutputs, propertyMapVal1, tfs, ps, map[string]*pulumirpc.PropertyDiff{
			"foo[1]": {Kind: pulumirpc.PropertyDiff_DELETE},
		})
	})

	t.Run("secretness and outputness changed", func(t *testing.T) {
		runDetailedDiffTest(t, propertyMapWithSecretsAndOutputs, propertyMapBoth, tfs, ps, map[string]*pulumirpc.PropertyDiff{
			"foo[1]": {Kind: pulumirpc.PropertyDiff_UPDATE},
		})
	})
}

func TestDetailedDiffTFForceNewPlain(t *testing.T) {
	t.Parallel()
	sdkv2Schema := map[string]*schema.Schema{
		"string_prop": {
			Type:     schema.TypeString,
			Optional: true,
			ForceNew: true,
		},
	}
	ps, tfs := map[string]*info.Schema{}, shimv2.NewSchemaMap(sdkv2Schema)

	propertyMapEmpty := resource.NewPropertyMapFromMap(
		map[string]interface{}{},
	)
	propertyMapVal1 := resource.NewPropertyMapFromMap(
		map[string]interface{}{
			"string_prop": "val1",
		},
	)
	propertyMapVal2 := resource.NewPropertyMapFromMap(
		map[string]interface{}{
			"string_prop": "val2",
		},
	)
	computedPropertyMap := resource.NewPropertyMapFromMap(
		map[string]interface{}{
			"string_prop": ComputedVal,
		},
	)

	t.Run("unchanged", func(t *testing.T) {
		runDetailedDiffTest(t, propertyMapVal1, propertyMapVal1, tfs, ps, map[string]*pulumirpc.PropertyDiff{})
	})

	t.Run("changed non-empty", func(t *testing.T) {
		runDetailedDiffTest(t, propertyMapVal1, propertyMapVal2, tfs, ps, map[string]*pulumirpc.PropertyDiff{
			"string_prop": {Kind: pulumirpc.PropertyDiff_UPDATE_REPLACE},
		})
	})

	t.Run("changed from empty", func(t *testing.T) {
		runDetailedDiffTest(t, propertyMapEmpty, propertyMapVal1, tfs, ps, map[string]*pulumirpc.PropertyDiff{
			"string_prop": {Kind: pulumirpc.PropertyDiff_ADD_REPLACE},
		})
	})

	t.Run("changed to empty", func(t *testing.T) {
		runDetailedDiffTest(t, propertyMapVal1, propertyMapEmpty, tfs, ps, map[string]*pulumirpc.PropertyDiff{
			"string_prop": {Kind: pulumirpc.PropertyDiff_DELETE_REPLACE},
		})
	})

	t.Run("changed to computed", func(t *testing.T) {
		runDetailedDiffTest(t, propertyMapVal1, computedPropertyMap, tfs, ps, map[string]*pulumirpc.PropertyDiff{
			"string_prop": {Kind: pulumirpc.PropertyDiff_UPDATE_REPLACE},
		})
	})

	t.Run("changed empty to computed", func(t *testing.T) {
		runDetailedDiffTest(t, propertyMapEmpty, computedPropertyMap, tfs, ps, map[string]*pulumirpc.PropertyDiff{
			"string_prop": {Kind: pulumirpc.PropertyDiff_ADD_REPLACE},
		})
	})
}

func TestDetailedDiffTFForceNewAttributeCollection(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name               string
		schema             *schema.Schema
		elementIndex       string
		emptyValue         interface{}
		value1             interface{}
		value2             interface{}
		computedCollection interface{}
		computedElem       interface{}
	}{
		{
			name: "list",
			schema: &schema.Schema{
				Type:     schema.TypeList,
				Elem:     &schema.Schema{Type: schema.TypeString},
				Optional: true,
				ForceNew: true,
			},
			elementIndex:       "prop[0]",
			value1:             []interface{}{"val1"},
			value2:             []interface{}{"val2"},
			computedCollection: ComputedVal,
			computedElem:       []interface{}{ComputedVal},
		},
		{
			name: "set",
			schema: &schema.Schema{
				Type:     schema.TypeSet,
				Elem:     &schema.Schema{Type: schema.TypeString},
				Optional: true,
				ForceNew: true,
			},
			elementIndex:       "prop[0]",
			value1:             []interface{}{"val1"},
			value2:             []interface{}{"val2"},
			computedCollection: ComputedVal,
			computedElem:       nil,
		},
		{
			name: "map",
			schema: &schema.Schema{
				Type:     schema.TypeMap,
				Elem:     &schema.Schema{Type: schema.TypeString},
				Optional: true,
				ForceNew: true,
			},
			elementIndex:       "prop.key",
			value1:             map[string]interface{}{"key": "val1"},
			value2:             map[string]interface{}{"key": "val2"},
			computedCollection: ComputedVal,
			computedElem:       map[string]interface{}{"key": ComputedVal},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			sdkv2Schema := map[string]*schema.Schema{
				"prop": tt.schema,
			}
			ps, tfs := map[string]*info.Schema{}, shimv2.NewSchemaMap(sdkv2Schema)

			propertyMapEmpty := resource.NewPropertyMapFromMap(
				map[string]interface{}{},
			)
			propertyMapListVal1 := resource.NewPropertyMapFromMap(
				map[string]interface{}{
					"prop": tt.value1,
				},
			)
			propertyMapListVal2 := resource.NewPropertyMapFromMap(
				map[string]interface{}{
					"prop": tt.value2,
				},
			)
			propertyMapComputedCollection := resource.NewPropertyMapFromMap(
				map[string]interface{}{
					"prop": tt.computedCollection,
				},
			)

			propertyMapComputedElem := resource.NewPropertyMapFromMap(
				map[string]interface{}{
					"prop": tt.computedElem,
				},
			)

			t.Run("unchanged", func(t *testing.T) {
				runDetailedDiffTest(t, propertyMapListVal1, propertyMapListVal1, tfs, ps, map[string]*pulumirpc.PropertyDiff{})
			})

			t.Run("changed non-empty", func(t *testing.T) {
				runDetailedDiffTest(t, propertyMapListVal1, propertyMapListVal2, tfs, ps, map[string]*pulumirpc.PropertyDiff{
					tt.elementIndex: {Kind: pulumirpc.PropertyDiff_UPDATE_REPLACE},
				})
			})

			t.Run("changed from empty", func(t *testing.T) {
				runDetailedDiffTest(t, propertyMapEmpty, propertyMapListVal1, tfs, ps, map[string]*pulumirpc.PropertyDiff{
					"prop": {Kind: pulumirpc.PropertyDiff_ADD_REPLACE},
				})
			})

			t.Run("changed to empty", func(t *testing.T) {
				runDetailedDiffTest(t, propertyMapListVal1, propertyMapEmpty, tfs, ps, map[string]*pulumirpc.PropertyDiff{
					"prop": {Kind: pulumirpc.PropertyDiff_DELETE_REPLACE},
				})
			})

			t.Run("changed to computed collection", func(t *testing.T) {
				runDetailedDiffTest(t, propertyMapListVal1, propertyMapComputedCollection, tfs, ps,
					map[string]*pulumirpc.PropertyDiff{
						"prop": {Kind: pulumirpc.PropertyDiff_UPDATE_REPLACE},
					})
			})

			t.Run("changed from empty to computed collection", func(t *testing.T) {
				runDetailedDiffTest(t, propertyMapEmpty, propertyMapComputedCollection, tfs, ps, map[string]*pulumirpc.PropertyDiff{
					"prop": {Kind: pulumirpc.PropertyDiff_ADD_REPLACE},
				})
			})

			if tt.computedElem != nil {
				t.Run("changed to computed elem", func(t *testing.T) {
					runDetailedDiffTest(t, propertyMapListVal1, propertyMapComputedElem, tfs, ps, map[string]*pulumirpc.PropertyDiff{
						tt.elementIndex: {Kind: pulumirpc.PropertyDiff_UPDATE_REPLACE},
					})
				})

				t.Run("changed from empty to computed elem", func(t *testing.T) {
					runDetailedDiffTest(t, propertyMapEmpty, propertyMapComputedElem, tfs, ps, map[string]*pulumirpc.PropertyDiff{
						"prop": {Kind: pulumirpc.PropertyDiff_ADD_REPLACE},
					})
				})
			}
		})
	}
}

func TestDetailedDiffTFForceNewBlockCollection(t *testing.T) {
	t.Parallel()
	sdkv2Schema := map[string]*schema.Schema{
		"list_prop": {
			ForceNew: true,
			Type:     schema.TypeList,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{"key": {
					Type:     schema.TypeString,
					Optional: true,
				}},
			},
			Optional: true,
		},
	}
	ps, tfs := map[string]*info.Schema{}, shimv2.NewSchemaMap(sdkv2Schema)

	propertyMapEmpty := resource.NewPropertyMapFromMap(
		map[string]interface{}{},
	)

	propertyMapListVal1 := resource.NewPropertyMapFromMap(
		map[string]interface{}{
			"list_prop": []interface{}{map[string]interface{}{"key": "val1"}},
		},
	)

	propertyMapListVal2 := resource.NewPropertyMapFromMap(
		map[string]interface{}{
			"list_prop": []interface{}{map[string]interface{}{"key": "val2"}},
		},
	)
	propertyMapComputedCollection := resource.NewPropertyMapFromMap(
		map[string]interface{}{
			"list_prop": ComputedVal,
		},
	)
	propertyMapComputedElem := resource.NewPropertyMapFromMap(
		map[string]interface{}{
			"list_prop": []interface{}{computedValue},
		},
	)
	propertyMapComputedElemProp := resource.NewPropertyMapFromMap(
		map[string]interface{}{
			"list_prop": []interface{}{map[string]interface{}{"key": ComputedVal}},
		},
	)

	t.Run("unchanged", func(t *testing.T) {
		runDetailedDiffTest(t, propertyMapListVal1, propertyMapListVal1, tfs, ps, map[string]*pulumirpc.PropertyDiff{})
	})

	t.Run("changed non-empty", func(t *testing.T) {
		runDetailedDiffTest(t, propertyMapListVal1, propertyMapListVal2, tfs, ps, map[string]*pulumirpc.PropertyDiff{
			"list_prop[0].key": {Kind: pulumirpc.PropertyDiff_UPDATE},
		})
	})

	t.Run("changed from empty", func(t *testing.T) {
		runDetailedDiffTest(t, propertyMapEmpty, propertyMapListVal1, tfs, ps, map[string]*pulumirpc.PropertyDiff{
			"list_prop": {Kind: pulumirpc.PropertyDiff_ADD_REPLACE},
		})
	})

	t.Run("changed to empty", func(t *testing.T) {
		runDetailedDiffTest(t, propertyMapListVal1, propertyMapEmpty, tfs, ps, map[string]*pulumirpc.PropertyDiff{
			"list_prop": {Kind: pulumirpc.PropertyDiff_DELETE_REPLACE},
		})
	})

	t.Run("changed to computed collection", func(t *testing.T) {
		runDetailedDiffTest(t, propertyMapListVal1, propertyMapComputedCollection, tfs, ps,
			map[string]*pulumirpc.PropertyDiff{
				"list_prop": {Kind: pulumirpc.PropertyDiff_UPDATE_REPLACE},
			})
	})

	t.Run("changed to computed elem", func(t *testing.T) {
		runDetailedDiffTest(t, propertyMapListVal1, propertyMapComputedElem, tfs, ps,
			map[string]*pulumirpc.PropertyDiff{
				"list_prop[0]": {Kind: pulumirpc.PropertyDiff_UPDATE_REPLACE},
			})
	})

	t.Run("changed to computed elem prop", func(t *testing.T) {
		runDetailedDiffTest(t, propertyMapListVal1, propertyMapComputedElemProp, tfs, ps, map[string]*pulumirpc.PropertyDiff{
			"list_prop[0].key": {Kind: pulumirpc.PropertyDiff_UPDATE},
		})
	})

	t.Run("changed from empty to computed collection", func(t *testing.T) {
		runDetailedDiffTest(t, propertyMapEmpty, propertyMapComputedCollection, tfs, ps, map[string]*pulumirpc.PropertyDiff{
			"list_prop": {Kind: pulumirpc.PropertyDiff_ADD_REPLACE},
		})
	})

	t.Run("changed from empty to computed elem", func(t *testing.T) {
		runDetailedDiffTest(t, propertyMapEmpty, propertyMapComputedElem, tfs, ps, map[string]*pulumirpc.PropertyDiff{
			"list_prop": {Kind: pulumirpc.PropertyDiff_ADD_REPLACE},
		})
	})

	t.Run("changed from empty to computed elem prop", func(t *testing.T) {
		runDetailedDiffTest(t, propertyMapEmpty, propertyMapComputedElemProp, tfs, ps, map[string]*pulumirpc.PropertyDiff{
			"list_prop": {Kind: pulumirpc.PropertyDiff_ADD_REPLACE},
		})
	})
}

func TestDetailedDiffTFForceNewElemBlockCollection(t *testing.T) {
	t.Parallel()
	sdkv2Schema := map[string]*schema.Schema{
		"list_prop": {
			Type: schema.TypeList,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{"key": {
					Type:     schema.TypeString,
					Optional: true,
					ForceNew: true,
				}},
			},
			Optional: true,
		},
	}
	ps, tfs := map[string]*info.Schema{}, shimv2.NewSchemaMap(sdkv2Schema)

	propertyMapEmpty := resource.NewPropertyMapFromMap(
		map[string]interface{}{},
	)

	propertyMapListVal1 := resource.NewPropertyMapFromMap(
		map[string]interface{}{
			"list_prop": []interface{}{map[string]interface{}{"key": "val1"}},
		},
	)

	propertyMapListVal2 := resource.NewPropertyMapFromMap(
		map[string]interface{}{
			"list_prop": []interface{}{map[string]interface{}{"key": "val2"}},
		},
	)

	propertyMapComputedCollection := resource.NewPropertyMapFromMap(
		map[string]interface{}{
			"list_prop": ComputedVal,
		},
	)

	propertyMapComputedElem := resource.NewPropertyMapFromMap(
		map[string]interface{}{
			"list_prop": []interface{}{computedValue},
		},
	)

	propertyMapComputedElemProp := resource.NewPropertyMapFromMap(
		map[string]interface{}{
			"list_prop": []interface{}{map[string]interface{}{"key": ComputedVal}},
		},
	)

	t.Run("unchanged", func(t *testing.T) {
		runDetailedDiffTest(t, propertyMapListVal1, propertyMapListVal1, tfs, ps, map[string]*pulumirpc.PropertyDiff{})
	})

	t.Run("changed non-empty", func(t *testing.T) {
		runDetailedDiffTest(t, propertyMapListVal1, propertyMapListVal2, tfs, ps, map[string]*pulumirpc.PropertyDiff{
			"list_prop[0].key": {Kind: pulumirpc.PropertyDiff_UPDATE_REPLACE},
		})
	})

	t.Run("changed from empty", func(t *testing.T) {
		runDetailedDiffTest(t, propertyMapEmpty, propertyMapListVal1, tfs, ps, map[string]*pulumirpc.PropertyDiff{
			"list_prop": {Kind: pulumirpc.PropertyDiff_ADD_REPLACE},
		})
	})

	t.Run("changed to empty", func(t *testing.T) {
		runDetailedDiffTest(t, propertyMapListVal1, propertyMapEmpty, tfs, ps, map[string]*pulumirpc.PropertyDiff{
			"list_prop": {Kind: pulumirpc.PropertyDiff_DELETE_REPLACE},
		})
	})

	t.Run("changed to computed collection", func(t *testing.T) {
		runDetailedDiffTest(t, propertyMapListVal1, propertyMapComputedCollection, tfs, ps,
			map[string]*pulumirpc.PropertyDiff{
				"list_prop": {Kind: pulumirpc.PropertyDiff_UPDATE_REPLACE},
			})
	})

	t.Run("changed to computed elem", func(t *testing.T) {
		runDetailedDiffTest(t, propertyMapListVal1, propertyMapComputedElem, tfs, ps,
			map[string]*pulumirpc.PropertyDiff{
				"list_prop[0]": {Kind: pulumirpc.PropertyDiff_UPDATE_REPLACE},
			})
	})

	t.Run("changed to computed elem prop", func(t *testing.T) {
		runDetailedDiffTest(t, propertyMapListVal1, propertyMapComputedElemProp, tfs, ps, map[string]*pulumirpc.PropertyDiff{
			"list_prop[0].key": {Kind: pulumirpc.PropertyDiff_UPDATE_REPLACE},
		})
	})

	// Note this might actually lead to a replacement, but we don't have enough information to know that.
	t.Run("changed from empty to computed collection", func(t *testing.T) {
		runDetailedDiffTest(t, propertyMapEmpty, propertyMapComputedCollection, tfs, ps, map[string]*pulumirpc.PropertyDiff{
			"list_prop": {Kind: pulumirpc.PropertyDiff_ADD},
		})
	})

	// Note this might actually lead to a replacement, but we don't have enough information to know that.
	t.Run("changed from empty to computed elem", func(t *testing.T) {
		runDetailedDiffTest(t, propertyMapEmpty, propertyMapComputedElem, tfs, ps, map[string]*pulumirpc.PropertyDiff{
			"list_prop": {Kind: pulumirpc.PropertyDiff_ADD},
		})
	})

	t.Run("changed from empty to computed elem prop", func(t *testing.T) {
		runDetailedDiffTest(t, propertyMapEmpty, propertyMapComputedElemProp, tfs, ps, map[string]*pulumirpc.PropertyDiff{
			"list_prop": {Kind: pulumirpc.PropertyDiff_ADD_REPLACE},
		})
	})
}

func TestDetailedDiffMaxItemsOnePlainType(t *testing.T) {
	t.Parallel()
	sdkv2Schema := map[string]*schema.Schema{
		"string_prop": {
			Type:     schema.TypeList,
			Optional: true,
			MaxItems: 1,
			Elem: &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
			},
		},
	}
	ps, tfs := map[string]*info.Schema{}, shimv2.NewSchemaMap(sdkv2Schema)

	propertyMapEmpty := resource.NewPropertyMapFromMap(
		map[string]interface{}{},
	)
	propertyMapVal1 := resource.NewPropertyMapFromMap(
		map[string]interface{}{
			"string_prop": "val1",
		},
	)
	propertyMapVal2 := resource.NewPropertyMapFromMap(
		map[string]interface{}{
			"string_prop": "val2",
		},
	)

	t.Run("unchanged", func(t *testing.T) {
		runDetailedDiffTest(t, propertyMapVal1, propertyMapVal1, tfs, ps, map[string]*pulumirpc.PropertyDiff{})
	})

	t.Run("changed non-empty", func(t *testing.T) {
		runDetailedDiffTest(t, propertyMapVal1, propertyMapVal2, tfs, ps, map[string]*pulumirpc.PropertyDiff{
			"string_prop": {Kind: pulumirpc.PropertyDiff_UPDATE},
		})
	})

	t.Run("changed from empty", func(t *testing.T) {
		runDetailedDiffTest(t, propertyMapEmpty, propertyMapVal1, tfs, ps, map[string]*pulumirpc.PropertyDiff{
			"string_prop": {Kind: pulumirpc.PropertyDiff_ADD},
		})
	})

	t.Run("changed to empty", func(t *testing.T) {
		runDetailedDiffTest(t, propertyMapVal1, propertyMapEmpty, tfs, ps, map[string]*pulumirpc.PropertyDiff{
			"string_prop": {Kind: pulumirpc.PropertyDiff_DELETE},
		})
	})

	t.Run("changed to computed", func(t *testing.T) {
		runDetailedDiffTest(t, propertyMapVal1,
			resource.NewPropertyMapFromMap(map[string]interface{}{"string_prop": ComputedVal}), tfs, ps,
			map[string]*pulumirpc.PropertyDiff{
				"string_prop": {Kind: pulumirpc.PropertyDiff_UPDATE},
			})
	})
}

func TestDetailedDiffNestedMaxItemsOnePlainType(t *testing.T) {
	t.Parallel()
	sdkv2Schema := map[string]*schema.Schema{
		"string_prop": {
			Type:     schema.TypeList,
			Optional: true,
			MaxItems: 1,
			Elem: &schema.Schema{
				Type:     schema.TypeList,
				Optional: true,
				MaxItems: 1,
				Elem: &schema.Schema{
					Type:     schema.TypeString,
					Optional: true,
				},
			},
		},
	}
	ps, tfs := map[string]*info.Schema{}, shimv2.NewSchemaMap(sdkv2Schema)

	propertyMapEmpty := resource.NewPropertyMapFromMap(
		map[string]interface{}{},
	)
	propertyMapVal1 := resource.NewPropertyMapFromMap(
		map[string]interface{}{
			"string_prop": "val1",
		},
	)
	propertyMapVal2 := resource.NewPropertyMapFromMap(
		map[string]interface{}{
			"string_prop": "val2",
		},
	)

	t.Run("unchanged", func(t *testing.T) {
		runDetailedDiffTest(t, propertyMapVal1, propertyMapVal1, tfs, ps, map[string]*pulumirpc.PropertyDiff{})
	})

	t.Run("changed non-empty", func(t *testing.T) {
		runDetailedDiffTest(t, propertyMapVal1, propertyMapVal2, tfs, ps, map[string]*pulumirpc.PropertyDiff{
			"string_prop": {Kind: pulumirpc.PropertyDiff_UPDATE},
		})
	})

	t.Run("changed from empty", func(t *testing.T) {
		runDetailedDiffTest(t, propertyMapEmpty, propertyMapVal1, tfs, ps, map[string]*pulumirpc.PropertyDiff{
			"string_prop": {Kind: pulumirpc.PropertyDiff_ADD},
		})
	})

	t.Run("changed to empty", func(t *testing.T) {
		runDetailedDiffTest(t, propertyMapVal1, propertyMapEmpty, tfs, ps, map[string]*pulumirpc.PropertyDiff{
			"string_prop": {Kind: pulumirpc.PropertyDiff_DELETE},
		})
	})

	t.Run("changed to computed", func(t *testing.T) {
		runDetailedDiffTest(t, propertyMapVal1,
			resource.NewPropertyMapFromMap(map[string]interface{}{"string_prop": ComputedVal}), tfs, ps,
			map[string]*pulumirpc.PropertyDiff{
				"string_prop": {Kind: pulumirpc.PropertyDiff_UPDATE},
			})
	})
}

func TestDetailedDiffTFForceNewObject(t *testing.T) {
	t.Parallel()
	// Note that maxItemsOne flattening means that the PropertyMap values contain no lists
	sdkv2Schema := map[string]*schema.Schema{
		"object_prop": {
			Type: schema.TypeList,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"key": {
						Type:     schema.TypeString,
						Optional: true,
						ForceNew: true,
					},
				},
			},
			Optional: true,
			MaxItems: 1,
		},
	}
	ps, tfs := map[string]*info.Schema{}, shimv2.NewSchemaMap(sdkv2Schema)

	propertyMapEmpty := resource.NewPropertyMapFromMap(
		map[string]interface{}{},
	)
	propertyMapObjectVal1 := resource.NewPropertyMapFromMap(
		map[string]interface{}{
			"object_prop": map[string]interface{}{"key": "val1"},
		},
	)
	propertyMapObjectVal2 := resource.NewPropertyMapFromMap(
		map[string]interface{}{
			"object_prop": map[string]interface{}{"key": "val2"},
		},
	)

	propertyMapComputedObject := resource.NewPropertyMapFromMap(
		map[string]interface{}{
			"object_prop": ComputedVal,
		},
	)

	propertyMapComputedObjectProp := resource.NewPropertyMapFromMap(
		map[string]interface{}{
			"object_prop": map[string]interface{}{"key": ComputedVal},
		},
	)

	t.Run("unchanged", func(t *testing.T) {
		runDetailedDiffTest(t, propertyMapObjectVal1, propertyMapObjectVal1, tfs, ps, map[string]*pulumirpc.PropertyDiff{})
	})

	t.Run("changed non-empty", func(t *testing.T) {
		runDetailedDiffTest(t, propertyMapObjectVal1, propertyMapObjectVal2, tfs, ps, map[string]*pulumirpc.PropertyDiff{
			"object_prop.key": {Kind: pulumirpc.PropertyDiff_UPDATE_REPLACE},
		})
	})

	t.Run("changed from empty", func(t *testing.T) {
		runDetailedDiffTest(t, propertyMapEmpty, propertyMapObjectVal1, tfs, ps, map[string]*pulumirpc.PropertyDiff{
			"object_prop": {Kind: pulumirpc.PropertyDiff_ADD_REPLACE},
		})
	})

	t.Run("changed to empty", func(t *testing.T) {
		runDetailedDiffTest(t, propertyMapObjectVal1, propertyMapEmpty, tfs, ps, map[string]*pulumirpc.PropertyDiff{
			"object_prop": {Kind: pulumirpc.PropertyDiff_DELETE_REPLACE},
		})
	})

	t.Run("changed to computed object", func(t *testing.T) {
		runDetailedDiffTest(t, propertyMapObjectVal1, propertyMapComputedObject, tfs, ps, map[string]*pulumirpc.PropertyDiff{
			"object_prop": {Kind: pulumirpc.PropertyDiff_UPDATE_REPLACE},
		})
	})

	t.Run("changed to computed object prop", func(t *testing.T) {
		runDetailedDiffTest(t, propertyMapObjectVal1, propertyMapComputedObjectProp, tfs, ps,
			map[string]*pulumirpc.PropertyDiff{
				"object_prop.key": {Kind: pulumirpc.PropertyDiff_UPDATE_REPLACE},
			})
	})

	// Note this might actually lead to a replacement, but we don't have enough information to know that.
	t.Run("changed from empty to computed object", func(t *testing.T) {
		runDetailedDiffTest(t, propertyMapEmpty, propertyMapComputedObject, tfs, ps, map[string]*pulumirpc.PropertyDiff{
			"object_prop": {Kind: pulumirpc.PropertyDiff_ADD},
		})
	})

	t.Run("changed from empty to computed object prop", func(t *testing.T) {
		runDetailedDiffTest(t, propertyMapEmpty, propertyMapComputedObjectProp, tfs, ps, map[string]*pulumirpc.PropertyDiff{
			"object_prop": {Kind: pulumirpc.PropertyDiff_ADD_REPLACE},
		})
	})
}

func TestDetailedDiffPulumiSchemaOverride(t *testing.T) {
	t.Parallel()
	sdkv2Schema := map[string]*schema.Schema{
		"foo": {
			Type:     schema.TypeString,
			Optional: true,
		},
	}
	t.Run("renamed property", func(t *testing.T) {
		tfs := shimv2.NewSchemaMap(sdkv2Schema)
		ps := map[string]*SchemaInfo{
			"foo": {
				Name: "bar",
			},
		}

		propertyMapEmpty := resource.NewPropertyMapFromMap(
			map[string]interface{}{},
		)
		propertyMapVal1 := resource.NewPropertyMapFromMap(
			map[string]interface{}{
				"bar": "val1",
			},
		)
		propertyMapVal2 := resource.NewPropertyMapFromMap(
			map[string]interface{}{
				"bar": "val2",
			},
		)

		t.Run("unchanged", func(t *testing.T) {
			runDetailedDiffTest(t, propertyMapVal1, propertyMapVal1, tfs, ps, map[string]*pulumirpc.PropertyDiff{})
		})

		t.Run("changed non-empty", func(t *testing.T) {
			runDetailedDiffTest(t, propertyMapVal1, propertyMapVal2, tfs, ps, map[string]*pulumirpc.PropertyDiff{
				"bar": {Kind: pulumirpc.PropertyDiff_UPDATE},
			})
		})

		t.Run("changed from empty", func(t *testing.T) {
			runDetailedDiffTest(t, propertyMapEmpty, propertyMapVal1, tfs, ps, map[string]*pulumirpc.PropertyDiff{
				"bar": {Kind: pulumirpc.PropertyDiff_ADD},
			})
		})

		t.Run("changed to empty", func(t *testing.T) {
			runDetailedDiffTest(t, propertyMapVal1, propertyMapEmpty, tfs, ps, map[string]*pulumirpc.PropertyDiff{
				"bar": {Kind: pulumirpc.PropertyDiff_DELETE},
			})
		})
	})

	t.Run("force new override property", func(t *testing.T) {
		tfs := shimv2.NewSchemaMap(sdkv2Schema)
		ps := map[string]*SchemaInfo{
			"foo": {
				ForceNew: True(),
			},
		}

		propertyMapEmpty := resource.NewPropertyMapFromMap(
			map[string]interface{}{},
		)
		propertyMapVal1 := resource.NewPropertyMapFromMap(
			map[string]interface{}{
				"foo": "val1",
			},
		)
		propertyMapVal2 := resource.NewPropertyMapFromMap(
			map[string]interface{}{
				"foo": "val2",
			},
		)

		t.Run("unchanged", func(t *testing.T) {
			runDetailedDiffTest(t, propertyMapVal1, propertyMapVal1, tfs, ps, map[string]*pulumirpc.PropertyDiff{})
		})

		t.Run("changed non-empty", func(t *testing.T) {
			runDetailedDiffTest(t, propertyMapVal1, propertyMapVal2, tfs, ps, map[string]*pulumirpc.PropertyDiff{
				"foo": {Kind: pulumirpc.PropertyDiff_UPDATE_REPLACE},
			})
		})

		t.Run("changed from empty", func(t *testing.T) {
			runDetailedDiffTest(t, propertyMapEmpty, propertyMapVal1, tfs, ps, map[string]*pulumirpc.PropertyDiff{
				"foo": {Kind: pulumirpc.PropertyDiff_ADD_REPLACE},
			})
		})

		t.Run("changed to empty", func(t *testing.T) {
			runDetailedDiffTest(t, propertyMapVal1, propertyMapEmpty, tfs, ps, map[string]*pulumirpc.PropertyDiff{
				"foo": {Kind: pulumirpc.PropertyDiff_DELETE_REPLACE},
			})
		})
	})

	t.Run("Type override property", func(t *testing.T) {
		tfs := shimv2.NewSchemaMap(sdkv2Schema)
		ps := map[string]*SchemaInfo{
			"foo": {
				Type: "number",
			},
		}

		propertyMapEmpty := resource.NewPropertyMapFromMap(
			map[string]interface{}{},
		)
		propertyMapVal1 := resource.NewPropertyMapFromMap(
			map[string]interface{}{
				"foo": 1,
			},
		)
		propertyMapVal2 := resource.NewPropertyMapFromMap(
			map[string]interface{}{
				"foo": 2,
			},
		)

		t.Run("unchanged", func(t *testing.T) {
			runDetailedDiffTest(t, propertyMapVal1, propertyMapVal1, tfs, ps, map[string]*pulumirpc.PropertyDiff{})
		})

		t.Run("changed non-empty", func(t *testing.T) {
			runDetailedDiffTest(t, propertyMapVal1, propertyMapVal2, tfs, ps, updated())
		})

		t.Run("changed from empty", func(t *testing.T) {
			runDetailedDiffTest(t, propertyMapEmpty, propertyMapVal1, tfs, ps, added())
		})

		t.Run("changed to empty", func(t *testing.T) {
			runDetailedDiffTest(t, propertyMapVal1, propertyMapEmpty, tfs, ps, deleted())
		})
	})

	t.Run("max items one override property", func(t *testing.T) {
		sdkv2Schema := map[string]*schema.Schema{
			"foo": {
				Type:     schema.TypeList,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"bar": {
							Type:     schema.TypeString,
							Optional: true,
						},
					},
				},
			},
		}
		tfs := shimv2.NewSchemaMap(sdkv2Schema)
		ps := map[string]*SchemaInfo{
			"foo": {
				MaxItemsOne: True(),
			},
		}

		propertyMapEmpty := resource.NewPropertyMapFromMap(
			map[string]interface{}{},
		)
		propertyMapVal1 := resource.NewPropertyMapFromMap(
			map[string]interface{}{
				"foo": map[string]interface{}{"bar": "val1"},
			},
		)
		propertyMapVal2 := resource.NewPropertyMapFromMap(
			map[string]interface{}{
				"foo": map[string]interface{}{"bar": "val2"},
			},
		)

		t.Run("unchanged", func(t *testing.T) {
			runDetailedDiffTest(t, propertyMapVal1, propertyMapVal1, tfs, ps, map[string]*pulumirpc.PropertyDiff{})
		})

		t.Run("changed non-empty", func(t *testing.T) {
			runDetailedDiffTest(t, propertyMapVal1, propertyMapVal2, tfs, ps, map[string]*pulumirpc.PropertyDiff{
				"foo.bar": {Kind: pulumirpc.PropertyDiff_UPDATE},
			})
		})

		t.Run("changed from empty", func(t *testing.T) {
			runDetailedDiffTest(t, propertyMapEmpty, propertyMapVal1, tfs, ps, map[string]*pulumirpc.PropertyDiff{
				"foo": {Kind: pulumirpc.PropertyDiff_ADD},
			})
		})

		t.Run("changed to empty", func(t *testing.T) {
			runDetailedDiffTest(t, propertyMapVal1, propertyMapEmpty, tfs, ps, map[string]*pulumirpc.PropertyDiff{
				"foo": {Kind: pulumirpc.PropertyDiff_DELETE},
			})
		})
	})

	t.Run("max items one removed override property", func(t *testing.T) {
		sdkv2Schema := map[string]*schema.Schema{
			"foo": {
				Type:     schema.TypeList,
				Optional: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"bar": {
							Type:     schema.TypeString,
							Optional: true,
						},
					},
				},
			},
		}
		tfs := shimv2.NewSchemaMap(sdkv2Schema)
		ps := map[string]*SchemaInfo{
			"foo": {
				MaxItemsOne: False(),
			},
		}

		propertyMapEmpty := resource.NewPropertyMapFromMap(
			map[string]interface{}{},
		)
		propertyMapVal1 := resource.NewPropertyMapFromMap(
			map[string]interface{}{
				"foo": []map[string]interface{}{{"bar": "val1"}},
			},
		)
		propertyMapVal2 := resource.NewPropertyMapFromMap(
			map[string]interface{}{
				"foo": []map[string]interface{}{{"bar": "val2"}},
			},
		)
		t.Run("unchanged", func(t *testing.T) {
			runDetailedDiffTest(t, propertyMapVal1, propertyMapVal1, tfs, ps, map[string]*pulumirpc.PropertyDiff{})
		})

		t.Run("changed non-empty", func(t *testing.T) {
			runDetailedDiffTest(t, propertyMapVal1, propertyMapVal2, tfs, ps, map[string]*pulumirpc.PropertyDiff{
				"foo[0].bar": {Kind: pulumirpc.PropertyDiff_UPDATE},
			})
		})

		t.Run("changed from empty", func(t *testing.T) {
			runDetailedDiffTest(t, propertyMapEmpty, propertyMapVal1, tfs, ps, map[string]*pulumirpc.PropertyDiff{
				"foo": {Kind: pulumirpc.PropertyDiff_ADD},
			})
		})

		t.Run("changed to empty", func(t *testing.T) {
			runDetailedDiffTest(t, propertyMapVal1, propertyMapEmpty, tfs, ps, map[string]*pulumirpc.PropertyDiff{
				"foo": {Kind: pulumirpc.PropertyDiff_DELETE},
			})
		})
	})
}

func TestDetailedDiffSetAttribute(t *testing.T) {
	t.Parallel()
	sdkv2Schema := map[string]*schema.Schema{
		"foo": {
			Type: schema.TypeSet,
			Elem: &schema.Schema{Type: schema.TypeString},
		},
	}
	ps, tfs := map[string]*info.Schema{}, shimv2.NewSchemaMap(sdkv2Schema)

	propertyMapElems := func(elems ...interface{}) resource.PropertyMap {
		return resource.NewPropertyMapFromMap(
			map[string]interface{}{
				"foo": elems,
			},
		)
	}

	t.Run("unchanged", func(t *testing.T) {
		runDetailedDiffTest(t, propertyMapElems("val1"), propertyMapElems("val1"), tfs, ps,
			map[string]*pulumirpc.PropertyDiff{})
	})

	t.Run("changed non-empty", func(t *testing.T) {
		runDetailedDiffTest(t,
			propertyMapElems("val1"),
			propertyMapElems("val2"), tfs, ps, map[string]*pulumirpc.PropertyDiff{
				"foo[0]": {Kind: pulumirpc.PropertyDiff_UPDATE},
			})
	})

	t.Run("changed from empty", func(t *testing.T) {
		runDetailedDiffTest(t,
			propertyMapElems(),
			propertyMapElems("val1"), tfs, ps, map[string]*pulumirpc.PropertyDiff{
				"foo[0]": {Kind: pulumirpc.PropertyDiff_ADD},
			})
	})

	t.Run("changed to empty", func(t *testing.T) {
		runDetailedDiffTest(t,
			propertyMapElems("val1"),
			propertyMapElems(), tfs, ps, map[string]*pulumirpc.PropertyDiff{
				"foo[0]": {Kind: pulumirpc.PropertyDiff_DELETE},
			})
	})

	t.Run("removed front", func(t *testing.T) {
		runDetailedDiffTest(t,
			propertyMapElems("val1", "val2", "val3"),
			propertyMapElems("val2", "val3"), tfs, ps, map[string]*pulumirpc.PropertyDiff{
				"foo[0]": {Kind: pulumirpc.PropertyDiff_DELETE},
			})
	})

	t.Run("removed middle", func(t *testing.T) {
		runDetailedDiffTest(t,
			propertyMapElems("val1", "val2", "val3"),
			propertyMapElems("val1", "val3"), tfs, ps, map[string]*pulumirpc.PropertyDiff{
				"foo[1]": {Kind: pulumirpc.PropertyDiff_DELETE},
			})
	})

	t.Run("removed end", func(t *testing.T) {
		runDetailedDiffTest(t,
			propertyMapElems("val1", "val2", "val3"),
			propertyMapElems("val1", "val2"), tfs, ps, map[string]*pulumirpc.PropertyDiff{
				"foo[2]": {Kind: pulumirpc.PropertyDiff_DELETE},
			})
	})

	t.Run("added front", func(t *testing.T) {
		runDetailedDiffTest(t,
			propertyMapElems("val2", "val3"),
			propertyMapElems("val1", "val2", "val3"), tfs, ps, map[string]*pulumirpc.PropertyDiff{
				"foo[0]": {Kind: pulumirpc.PropertyDiff_ADD},
			})
	})

	t.Run("added middle", func(t *testing.T) {
		runDetailedDiffTest(t,
			propertyMapElems("val1", "val3"),
			propertyMapElems("val1", "val2", "val3"), tfs, ps, map[string]*pulumirpc.PropertyDiff{
				"foo[1]": {Kind: pulumirpc.PropertyDiff_ADD},
			})
	})

	t.Run("added end", func(t *testing.T) {
		runDetailedDiffTest(t, propertyMapElems("val1", "val2"),
			propertyMapElems("val1", "val2", "val3"), tfs, ps, map[string]*pulumirpc.PropertyDiff{
				"foo[2]": {Kind: pulumirpc.PropertyDiff_ADD},
			})
	})

	t.Run("same element updated", func(t *testing.T) {
		runDetailedDiffTest(t,
			propertyMapElems("val1", "val2", "val3"),
			propertyMapElems("val1", "val4", "val3"), tfs, ps,
			map[string]*pulumirpc.PropertyDiff{
				"foo[1]": {Kind: pulumirpc.PropertyDiff_UPDATE},
			},
		)
	})

	t.Run("shuffled", func(t *testing.T) {
		runDetailedDiffTest(t,
			propertyMapElems("val1", "val2", "val3"),
			propertyMapElems("val3", "val2", "val1"), tfs, ps, map[string]*pulumirpc.PropertyDiff{})
	})

	t.Run("shuffled with duplicates", func(t *testing.T) {
		runDetailedDiffTest(t,
			propertyMapElems("val1", "val2", "val3"),
			propertyMapElems("val3", "val2", "val1", "val1"), tfs, ps, map[string]*pulumirpc.PropertyDiff{})
	})

	t.Run("shuffled added front", func(t *testing.T) {
		runDetailedDiffTest(t,
			propertyMapElems("val2", "val3"),
			propertyMapElems("val1", "val3", "val2"), tfs, ps, map[string]*pulumirpc.PropertyDiff{
				"foo[0]": {Kind: pulumirpc.PropertyDiff_ADD},
			})
	})

	t.Run("shuffled added middle", func(t *testing.T) {
		runDetailedDiffTest(t,
			propertyMapElems("val1", "val3"),
			propertyMapElems("val3", "val2", "val1"), tfs, ps, map[string]*pulumirpc.PropertyDiff{
				"foo[1]": {Kind: pulumirpc.PropertyDiff_ADD},
			})
	})

	t.Run("shuffled added end", func(t *testing.T) {
		runDetailedDiffTest(t,
			propertyMapElems("val1", "val2"),
			propertyMapElems("val2", "val1", "val3"), tfs, ps, map[string]*pulumirpc.PropertyDiff{
				"foo[2]": {Kind: pulumirpc.PropertyDiff_ADD},
			})
	})

	t.Run("shuffled removed front", func(t *testing.T) {
		runDetailedDiffTest(t,
			propertyMapElems("val1", "val2", "val3"),
			propertyMapElems("val3", "val2"), tfs, ps, map[string]*pulumirpc.PropertyDiff{
				"foo[0]": {Kind: pulumirpc.PropertyDiff_DELETE},
			})
	})

	t.Run("shuffled removed middle", func(t *testing.T) {
		runDetailedDiffTest(t,
			propertyMapElems("val1", "val2", "val3"),
			propertyMapElems("val3", "val1"), tfs, ps, map[string]*pulumirpc.PropertyDiff{
				"foo[1]": {Kind: pulumirpc.PropertyDiff_DELETE},
			})
	})

	t.Run("shuffled removed end", func(t *testing.T) {
		runDetailedDiffTest(t,
			propertyMapElems("val1", "val2", "val3"),
			propertyMapElems("val2", "val1"), tfs, ps, map[string]*pulumirpc.PropertyDiff{
				"foo[2]": {Kind: pulumirpc.PropertyDiff_DELETE},
			})
	})

	t.Run("computed", func(t *testing.T) {
		runDetailedDiffTest(t,
			propertyMapElems("val1"),
			resource.NewPropertyMapFromMap(
				map[string]interface{}{
					"foo": computedValue,
				},
			),
			tfs, ps,
			map[string]*pulumirpc.PropertyDiff{
				"foo": {Kind: pulumirpc.PropertyDiff_UPDATE},
			},
		)
	})

	t.Run("nil to computed", func(t *testing.T) {
		runDetailedDiffTest(t,
			resource.NewPropertyMapFromMap(
				map[string]interface{}{},
			),
			resource.NewPropertyMapFromMap(
				map[string]interface{}{
					"foo": computedValue,
				},
			),
			tfs, ps,
			map[string]*pulumirpc.PropertyDiff{
				"foo": {Kind: pulumirpc.PropertyDiff_ADD},
			},
		)
	})

	t.Run("empty to computed", func(t *testing.T) {
		runDetailedDiffTest(t,
			propertyMapElems(),
			resource.NewPropertyMapFromMap(
				map[string]interface{}{
					"foo": computedValue,
				},
			),
			tfs, ps,
			map[string]*pulumirpc.PropertyDiff{
				"foo": {Kind: pulumirpc.PropertyDiff_UPDATE},
			},
		)
	})

	t.Run("two added, two removed", func(t *testing.T) {
		runDetailedDiffTest(t,
			propertyMapElems("val1", "val2"),
			propertyMapElems("val3", "val4"),
			tfs, ps,
			map[string]*pulumirpc.PropertyDiff{
				"foo[0]": {Kind: pulumirpc.PropertyDiff_UPDATE},
				"foo[1]": {Kind: pulumirpc.PropertyDiff_UPDATE},
			},
		)
	})

	t.Run("two added, two removed, shuffled", func(t *testing.T) {
		runDetailedDiffTest(t,
			propertyMapElems("stable1", "stable2", "val1", "val2"),
			propertyMapElems("val4", "val3", "stable1", "stable2"),
			tfs, ps,
			map[string]*pulumirpc.PropertyDiff{
				"foo[0]": {Kind: pulumirpc.PropertyDiff_ADD},
				"foo[1]": {Kind: pulumirpc.PropertyDiff_ADD},
				"foo[2]": {Kind: pulumirpc.PropertyDiff_DELETE},
				"foo[3]": {Kind: pulumirpc.PropertyDiff_DELETE},
			},
		)
	})
}

func TestDetailedDiffSetBlock(t *testing.T) {
	t.Parallel()
	propertyMapElems := func(elems ...string) resource.PropertyMap {
		var elemMaps []map[string]interface{}
		for _, elem := range elems {
			elemMaps = append(elemMaps, map[string]interface{}{"bar": elem})
		}
		return resource.NewPropertyMapFromMap(
			map[string]interface{}{
				"foo": elemMaps,
			},
		)
	}

	for _, forceNew := range []bool{false, true} {
		sdkv2Schema := map[string]*schema.Schema{
			"foo": {
				Type: schema.TypeSet,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"bar": {
							Type:     schema.TypeString,
							Optional: true,
							ForceNew: forceNew,
						},
					},
				},
			},
		}
		ps, tfs := map[string]*info.Schema{}, shimv2.NewSchemaMap(sdkv2Schema)
		t.Run(fmt.Sprintf("forceNew=%v", forceNew), func(t *testing.T) {
			update := pulumirpc.PropertyDiff_UPDATE
			add := pulumirpc.PropertyDiff_ADD
			delete := pulumirpc.PropertyDiff_DELETE
			if forceNew {
				update = pulumirpc.PropertyDiff_UPDATE_REPLACE
				add = pulumirpc.PropertyDiff_ADD_REPLACE
				delete = pulumirpc.PropertyDiff_DELETE_REPLACE
			}

			t.Run("unchanged", func(t *testing.T) {
				runDetailedDiffTest(t, propertyMapElems("val1"), propertyMapElems("val1"), tfs, ps,
					map[string]*pulumirpc.PropertyDiff{})
			})

			t.Run("changed non-empty", func(t *testing.T) {
				runDetailedDiffTest(t,
					propertyMapElems("val1"),
					propertyMapElems("val2"), tfs, ps, map[string]*pulumirpc.PropertyDiff{
						"foo[0].bar": {Kind: update},
					},
				)
			})

			t.Run("changed from empty", func(t *testing.T) {
				runDetailedDiffTest(t,
					propertyMapElems(),
					propertyMapElems("val1"), tfs, ps, map[string]*pulumirpc.PropertyDiff{
						"foo[0]": {Kind: add},
					},
				)
			})

			t.Run("changed to empty", func(t *testing.T) {
				runDetailedDiffTest(t,
					propertyMapElems("val1"),
					propertyMapElems(), tfs, ps, map[string]*pulumirpc.PropertyDiff{
						"foo[0]": {Kind: delete},
					},
				)
			})

			t.Run("removed front", func(t *testing.T) {
				runDetailedDiffTest(t,
					propertyMapElems("val1", "val2", "val3"),
					propertyMapElems("val2", "val3"), tfs, ps, map[string]*pulumirpc.PropertyDiff{
						"foo[0]": {Kind: delete},
					},
				)
			})

			t.Run("removed middle", func(t *testing.T) {
				runDetailedDiffTest(t,
					propertyMapElems("val1", "val2", "val3"),
					propertyMapElems("val1", "val3"), tfs, ps, map[string]*pulumirpc.PropertyDiff{
						"foo[1]": {Kind: delete},
					},
				)
			})

			t.Run("removed end", func(t *testing.T) {
				runDetailedDiffTest(t,
					propertyMapElems("val1", "val2", "val3"),
					propertyMapElems("val1", "val2"), tfs, ps, map[string]*pulumirpc.PropertyDiff{
						"foo[2]": {Kind: delete},
					},
				)
			})

			t.Run("added front", func(t *testing.T) {
				runDetailedDiffTest(t,
					propertyMapElems("val2", "val3"),
					propertyMapElems("val1", "val2", "val3"), tfs, ps, map[string]*pulumirpc.PropertyDiff{
						"foo[0]": {Kind: add},
					},
				)
			})

			t.Run("added middle", func(t *testing.T) {
				runDetailedDiffTest(t,
					propertyMapElems("val1", "val3"),
					propertyMapElems("val1", "val2", "val3"), tfs, ps, map[string]*pulumirpc.PropertyDiff{
						"foo[1]": {Kind: add},
					},
				)
			})

			t.Run("added end", func(t *testing.T) {
				runDetailedDiffTest(t,
					propertyMapElems("val1", "val2"),
					propertyMapElems("val1", "val2", "val3"), tfs, ps, map[string]*pulumirpc.PropertyDiff{
						"foo[2]": {Kind: add},
					},
				)
			})

			t.Run("same element updated", func(t *testing.T) {
				runDetailedDiffTest(t,
					propertyMapElems("val1", "val2", "val3"),
					propertyMapElems("val1", "val4", "val3"), tfs, ps,
					map[string]*pulumirpc.PropertyDiff{
						"foo[1].bar": {Kind: update},
					},
				)
			})

			t.Run("shuffled", func(t *testing.T) {
				runDetailedDiffTest(t,
					propertyMapElems("val1", "val2", "val3"),
					propertyMapElems("val3", "val2", "val1"), tfs, ps, map[string]*pulumirpc.PropertyDiff{})
			})

			t.Run("shuffled with duplicates", func(t *testing.T) {
				runDetailedDiffTest(t,
					propertyMapElems("val1", "val2", "val3"),
					propertyMapElems("val3", "val2", "val1", "val1"), tfs, ps, map[string]*pulumirpc.PropertyDiff{})
			})

			t.Run("shuffled added front", func(t *testing.T) {
				runDetailedDiffTest(t,
					propertyMapElems("val2", "val3"),
					propertyMapElems("val1", "val3", "val2"), tfs, ps, map[string]*pulumirpc.PropertyDiff{
						"foo[0]": {Kind: add},
					},
				)
			})

			t.Run("shuffled added middle", func(t *testing.T) {
				runDetailedDiffTest(t,
					propertyMapElems("val1", "val3"),
					propertyMapElems("val3", "val2", "val1"), tfs, ps, map[string]*pulumirpc.PropertyDiff{
						"foo[1]": {Kind: add},
					},
				)
			})

			t.Run("shuffled added end", func(t *testing.T) {
				runDetailedDiffTest(t,
					propertyMapElems("val1", "val2"),
					propertyMapElems("val2", "val1", "val3"), tfs, ps, map[string]*pulumirpc.PropertyDiff{
						"foo[2]": {Kind: add},
					},
				)
			})

			t.Run("shuffled removed front", func(t *testing.T) {
				runDetailedDiffTest(t,
					propertyMapElems("val1", "val2", "val3"),
					propertyMapElems("val3", "val2"), tfs, ps, map[string]*pulumirpc.PropertyDiff{
						"foo[0]": {Kind: delete},
					},
				)
			})

			t.Run("shuffled removed middle", func(t *testing.T) {
				runDetailedDiffTest(t,
					propertyMapElems("val1", "val2", "val3"),
					propertyMapElems("val3", "val1"), tfs, ps, map[string]*pulumirpc.PropertyDiff{
						"foo[1]": {Kind: delete},
					},
				)
			})

			t.Run("shuffled removed end", func(t *testing.T) {
				runDetailedDiffTest(t,
					propertyMapElems("val1", "val2", "val3"),
					propertyMapElems("val2", "val1"), tfs, ps, map[string]*pulumirpc.PropertyDiff{
						"foo[2]": {Kind: delete},
					},
				)
			})

			t.Run("computed", func(t *testing.T) {
				runDetailedDiffTest(t,
					propertyMapElems("val1"),
					resource.NewPropertyMapFromMap(
						map[string]interface{}{
							"foo": computedValue,
						},
					),
					tfs, ps,
					map[string]*pulumirpc.PropertyDiff{
						"foo": {Kind: update},
					},
				)
			})

			t.Run("nil to computed", func(t *testing.T) {
				runDetailedDiffTest(t,
					resource.NewPropertyMapFromMap(
						map[string]interface{}{},
					),
					resource.NewPropertyMapFromMap(
						map[string]interface{}{
							"foo": computedValue,
						},
					),
					tfs, ps,
					map[string]*pulumirpc.PropertyDiff{
						"foo": {Kind: pulumirpc.PropertyDiff_ADD},
					},
				)
			})

			t.Run("empty to computed", func(t *testing.T) {
				runDetailedDiffTest(t,
					propertyMapElems(),
					resource.NewPropertyMapFromMap(
						map[string]interface{}{
							"foo": computedValue,
						},
					),
					tfs, ps,
					map[string]*pulumirpc.PropertyDiff{
						"foo": {Kind: pulumirpc.PropertyDiff_UPDATE},
					},
				)
			})
		})
	}
}

func TestDetailedDiffSetBlockNestedMaxItemsOne(t *testing.T) {
	t.Parallel()
	customResponseSchema := func() *schema.Schema {
		return &schema.Schema{
			Type:     schema.TypeList,
			Optional: true,
			MaxItems: 1,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"custom_response_body_key": {
						Type:     schema.TypeString,
						Optional: true,
					},
				},
			},
		}
	}
	blockConfigSchema := func() *schema.Schema {
		return &schema.Schema{
			Type:     schema.TypeList,
			Optional: true,
			MaxItems: 1,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"custom_response": customResponseSchema(),
				},
			},
		}
	}
	ruleElement := &schema.Resource{
		Schema: map[string]*schema.Schema{
			"action": {
				Type:     schema.TypeList,
				Optional: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"block": blockConfigSchema(),
					},
				},
			},
		},
	}

	schMap := map[string]*schema.Schema{
		"rule": {
			Type:     schema.TypeSet,
			Optional: true,
			Elem:     ruleElement,
		},
	}

	ps, tfs := map[string]*info.Schema{}, shimv2.NewSchemaMap(schMap)

	t.Run("unchanged", func(t *testing.T) {
		runDetailedDiffTest(t, resource.NewPropertyMapFromMap(map[string]interface{}{
			"rule": []map[string]interface{}{
				{
					"action": map[string]interface{}{
						"block": map[string]interface{}{
							"custom_response": map[string]interface{}{
								"custom_response_body_key": "val1",
							},
						},
					},
				},
			},
		}), resource.NewPropertyMapFromMap(map[string]interface{}{
			"rule": []map[string]interface{}{
				{
					"action": map[string]interface{}{
						"block": map[string]interface{}{
							"custom_response": map[string]interface{}{
								"custom_response_body_key": "val1",
							},
						},
					},
				},
			},
		}), tfs, ps, map[string]*pulumirpc.PropertyDiff{})
	})
}

func TestDetailedDiffMismatchedSchemas(t *testing.T) {
	t.Parallel()
	stringSchema := map[string]*schema.Schema{
		"foo": {
			Type:     schema.TypeString,
			Optional: true,
		},
	}

	listSchema := map[string]*schema.Schema{
		"foo": {
			Type:     schema.TypeList,
			Optional: true,
			Elem: &schema.Schema{
				Type: schema.TypeString,
			},
		},
	}

	setSchema := map[string]*schema.Schema{
		"foo": {
			Type:     schema.TypeSet,
			Optional: true,
			Elem: &schema.Schema{
				Type: schema.TypeString,
			},
		},
	}

	mapSchema := map[string]*schema.Schema{
		"foo": {
			Type:     schema.TypeMap,
			Optional: true,
			Elem: &schema.Schema{
				Type: schema.TypeString,
			},
		},
	}

	stringValue := resource.NewPropertyMapFromMap(
		map[string]interface{}{
			"foo": "string-value",
		},
	)

	listValue := resource.NewPropertyMapFromMap(
		map[string]interface{}{
			"foo": []interface{}{"list-value"},
		},
	)

	mapValue := resource.NewPropertyMapFromMap(
		map[string]interface{}{
			"foo": map[string]interface{}{"bar": "map-value"},
		},
	)

	t.Run("list schema with string value", func(t *testing.T) {
		tfs := shimv2.NewSchemaMap(listSchema)
		runDetailedDiffTest(t, stringValue, listValue, tfs, nil, map[string]*pulumirpc.PropertyDiff{
			"foo": {Kind: pulumirpc.PropertyDiff_UPDATE},
		})
	})

	t.Run("list schema with map value", func(t *testing.T) {
		tfs := shimv2.NewSchemaMap(listSchema)
		runDetailedDiffTest(t, mapValue, listValue, tfs, nil, map[string]*pulumirpc.PropertyDiff{
			"foo": {Kind: pulumirpc.PropertyDiff_UPDATE},
		})
	})

	t.Run("set schema with string value", func(t *testing.T) {
		tfs := shimv2.NewSchemaMap(setSchema)
		runDetailedDiffTest(t, stringValue, listValue, tfs, nil, map[string]*pulumirpc.PropertyDiff{
			"foo": {Kind: pulumirpc.PropertyDiff_UPDATE},
		})
	})

	t.Run("set schema with map value", func(t *testing.T) {
		tfs := shimv2.NewSchemaMap(setSchema)
		runDetailedDiffTest(t, mapValue, listValue, tfs, nil, map[string]*pulumirpc.PropertyDiff{
			"foo": {Kind: pulumirpc.PropertyDiff_UPDATE},
		})
	})

	t.Run("string schema with list value", func(t *testing.T) {
		tfs := shimv2.NewSchemaMap(stringSchema)
		runDetailedDiffTest(t, listValue, stringValue, tfs, nil, map[string]*pulumirpc.PropertyDiff{
			"foo": {Kind: pulumirpc.PropertyDiff_UPDATE},
		})
	})

	t.Run("string schema with map value", func(t *testing.T) {
		tfs := shimv2.NewSchemaMap(stringSchema)
		runDetailedDiffTest(t, mapValue, stringValue, tfs, nil, map[string]*pulumirpc.PropertyDiff{
			"foo": {Kind: pulumirpc.PropertyDiff_UPDATE},
		})
	})

	t.Run("map schema with string value", func(t *testing.T) {
		tfs := shimv2.NewSchemaMap(mapSchema)
		runDetailedDiffTest(t, stringValue, mapValue, tfs, nil, map[string]*pulumirpc.PropertyDiff{
			"foo": {Kind: pulumirpc.PropertyDiff_UPDATE},
		})
	})

	t.Run("map schema with list value", func(t *testing.T) {
		tfs := shimv2.NewSchemaMap(mapSchema)
		runDetailedDiffTest(t, listValue, mapValue, tfs, nil, map[string]*pulumirpc.PropertyDiff{
			"foo": {Kind: pulumirpc.PropertyDiff_UPDATE},
		})
	})
}

func TestDetailedDiffSetHashChanges(t *testing.T) {
	t.Parallel()
	runTest := func(old, new hashIndexMap, expectedRemoved, expectedAdded []arrayIndex) {
		t.Helper()
		removed, added := computeSetHashChanges(old, new)

		require.Equal(t, removed, expectedRemoved)
		require.Equal(t, added, expectedAdded)
	}

	runTest(hashIndexMap{}, hashIndexMap{}, []arrayIndex{}, []arrayIndex{})
	runTest(hashIndexMap{1: 1}, hashIndexMap{1: 1}, []arrayIndex{}, []arrayIndex{})
	runTest(hashIndexMap{1: 1}, hashIndexMap{}, []arrayIndex{1}, []arrayIndex{})
	runTest(hashIndexMap{1: 1}, hashIndexMap{2: 2}, []arrayIndex{1}, []arrayIndex{2})
}

func TestDetailedDiffSetHashPanicCaught(t *testing.T) {
	t.Parallel()
	tfs := shimv2.NewSchemaMap(map[string]*schema.Schema{
		"foo": {
			Type: schema.TypeSet,
			Elem: &schema.Schema{
				Type: schema.TypeString,
			},
			Set: func(v interface{}) int {
				panic("test")
			},
		},
	})

	buf := &bytes.Buffer{}
	ctx := logging.InitLogging(context.Background(), logging.LogOptions{
		LogSink: &testLogSink{buf: buf},
	})

	differ := detailedDiffer{
		ctx: ctx,
		tfs: tfs,
		ps:  nil,
	}

	differ.calculateSetHashIndexMap(
		newPropertyPath("foo"),
		[]resource.PropertyValue{resource.NewStringProperty("val1")},
	)

	require.Contains(t, buf.String(), "Failed to calculate preview for element in foo")
}

func TestDetailedDiffReplaceOverrideFalse(t *testing.T) {
	t.Parallel()

	old := resource.NewPropertyMapFromMap(map[string]interface{}{
		"foo": "bar",
	})
	new := resource.NewPropertyMapFromMap(map[string]interface{}{
		"foo": "baz",
	})

	tfs := shimv2.NewSchemaMap(map[string]*schema.Schema{
		"foo": {
			Type:     schema.TypeString,
			Optional: true,
			ForceNew: true,
		},
	})

	actual := MakeDetailedDiffV2(context.Background(), tfs, nil, old, new, new, ref(false))
	require.Equal(t, actual, map[string]*pulumirpc.PropertyDiff{
		"foo": {Kind: pulumirpc.PropertyDiff_UPDATE},
	})
}

func TestDetailedDiffReplaceOverrideTrue(t *testing.T) {
	t.Parallel()

	old := resource.NewPropertyMapFromMap(map[string]interface{}{
		"foo": "bar",
	})
	new := resource.NewPropertyMapFromMap(map[string]interface{}{
		"foo": "baz",
	})

	tfs := shimv2.NewSchemaMap(map[string]*schema.Schema{
		"foo": {
			Type:     schema.TypeString,
			Optional: true,
		},
	})

	actual := MakeDetailedDiffV2(context.Background(), tfs, nil, old, new, new, ref(true))
	require.Equal(t, actual, map[string]*pulumirpc.PropertyDiff{
		"foo":    {Kind: pulumirpc.PropertyDiff_UPDATE},
		"__meta": {Kind: pulumirpc.PropertyDiff_UPDATE_REPLACE},
	})
}

func TestDemoteToNoReplace(t *testing.T) {
	t.Parallel()

	diff := &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_ADD_REPLACE}
	require.Equal(t, demoteToNoReplace(diff), &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_ADD})

	diff = &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_DELETE_REPLACE}
	require.Equal(t, demoteToNoReplace(diff), &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_DELETE})

	diff = &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_UPDATE_REPLACE}
	require.Equal(t, demoteToNoReplace(diff), &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_UPDATE})

	diff = &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_ADD}
	require.Equal(t, demoteToNoReplace(diff), &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_ADD})

	diff = &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_DELETE}
	require.Equal(t, demoteToNoReplace(diff), &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_DELETE})

	diff = &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_UPDATE}
	require.Equal(t, demoteToNoReplace(diff), &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_UPDATE})
}

func TestContainsReplace(t *testing.T) {
	t.Parallel()

	require.True(t, containsReplace(map[string]*pulumirpc.PropertyDiff{
		"foo": {Kind: pulumirpc.PropertyDiff_UPDATE_REPLACE},
	}))

	require.True(t, containsReplace(map[string]*pulumirpc.PropertyDiff{
		"foo": {Kind: pulumirpc.PropertyDiff_ADD_REPLACE},
	}))

	require.True(t, containsReplace(map[string]*pulumirpc.PropertyDiff{
		"foo": {Kind: pulumirpc.PropertyDiff_DELETE_REPLACE},
	}))

	require.False(t, containsReplace(map[string]*pulumirpc.PropertyDiff{
		"foo": {Kind: pulumirpc.PropertyDiff_UPDATE},
	}))

	require.False(t, containsReplace(map[string]*pulumirpc.PropertyDiff{}))
}

func TestMatchPlanElementsToInputs(t *testing.T) {
	t.Parallel()
	tfs := shimv2.NewSchemaMap(map[string]*schema.Schema{
		"my_list": {
			Type:     schema.TypeList,
			Optional: true,
			Elem: &schema.Schema{
				Type: schema.TypeString,
			},
		},
	})

	ps := map[string]*SchemaInfo{}
	tests := []struct {
		name            string
		path            propertyPath
		changedIndices  []arrayIndex
		plannedState    []resource.PropertyValue
		newInputs       resource.PropertyMap
		expectedMatches map[arrayIndex]arrayIndex
	}{
		{
			name:           "basic matching",
			path:           newPropertyPath("myList"),
			changedIndices: []arrayIndex{0, 1},
			plannedState: []resource.PropertyValue{
				resource.NewStringProperty("foo"),
				resource.NewStringProperty("bar"),
			},
			newInputs: resource.PropertyMap{
				"myList": resource.NewArrayProperty([]resource.PropertyValue{
					resource.NewStringProperty("foo"),
					resource.NewStringProperty("bar"),
				}),
			},
			expectedMatches: map[arrayIndex]arrayIndex{
				0: 0,
				1: 1,
			},
		},
		{
			name:           "length mismatch returns nil",
			path:           newPropertyPath("myList"),
			changedIndices: []arrayIndex{0},
			plannedState: []resource.PropertyValue{
				resource.NewStringProperty("foo"),
				resource.NewStringProperty("bar"),
			},
			newInputs: resource.PropertyMap{
				"myList": resource.NewArrayProperty([]resource.PropertyValue{
					resource.NewStringProperty("foo"),
				}),
			},
			expectedMatches: nil,
		},
		{
			name:           "no matches returns empty slice",
			path:           newPropertyPath("myList"),
			changedIndices: []arrayIndex{0},
			plannedState: []resource.PropertyValue{
				resource.NewStringProperty("foo"),
				resource.NewStringProperty("bar"),
			},
			newInputs: resource.PropertyMap{
				"myList": resource.NewArrayProperty([]resource.PropertyValue{
					resource.NewStringProperty("baz"),
					resource.NewStringProperty("qux"),
				}),
			},
			expectedMatches: map[arrayIndex]arrayIndex{},
		},
		{
			name:           "missing input path returns empty slice",
			path:           newPropertyPath("nonexistentList"),
			changedIndices: []arrayIndex{0},
			plannedState: []resource.PropertyValue{
				resource.NewStringProperty("foo"),
			},
			newInputs:       resource.PropertyMap{},
			expectedMatches: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			differ := detailedDiffer{
				ctx:       context.Background(),
				tfs:       tfs,
				ps:        ps,
				newInputs: tt.newInputs,
			}

			matches := differ.matchPlanElementsToInputs(tt.path, tt.changedIndices, tt.plannedState)

			if tt.expectedMatches == nil && matches != nil {
				t.Errorf("expected nil matches, got %v", matches)
				return
			}

			if !reflect.DeepEqual(matches, tt.expectedMatches) {
				t.Errorf("expected matches %v, got %v", tt.expectedMatches, matches)
			}
		})
	}
}

func TestMakeSetDiffElementResult(t *testing.T) {
	t.Parallel()

	// Create a basic differ instance for testing
	differ := detailedDiffer{
		ctx: context.Background(),
		tfs: shimv2.NewSchemaMap(map[string]*schema.Schema{
			"test_set": {
				Type: schema.TypeSet,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
		}),
	}

	tests := []struct {
		name     string
		path     propertyPath
		changes  map[arrayIndex]setChange
		oldList  []resource.PropertyValue
		newList  []resource.PropertyValue
		expected map[detailedDiffKey]*pulumirpc.PropertyDiff
	}{
		{
			name: "add element",
			path: newPropertyPath("test_set"),
			changes: map[arrayIndex]setChange{
				0: {
					oldChanged:   false,
					newChanged:   true,
					plannedIndex: 0,
				},
			},
			oldList: []resource.PropertyValue{},
			newList: []resource.PropertyValue{
				resource.NewStringProperty("new_value"),
			},
			expected: map[detailedDiffKey]*pulumirpc.PropertyDiff{
				detailedDiffKey("test_set[0]"): {Kind: pulumirpc.PropertyDiff_ADD},
			},
		},
		{
			name: "delete element",
			path: newPropertyPath("test_set"),
			changes: map[arrayIndex]setChange{
				0: {
					oldChanged:   true,
					newChanged:   false,
					plannedIndex: 0,
				},
			},
			oldList: []resource.PropertyValue{
				resource.NewStringProperty("old_value"),
			},
			newList: []resource.PropertyValue{},
			expected: map[detailedDiffKey]*pulumirpc.PropertyDiff{
				detailedDiffKey("test_set[0]"): {Kind: pulumirpc.PropertyDiff_DELETE},
			},
		},
		{
			name: "update element",
			path: newPropertyPath("test_set"),
			changes: map[arrayIndex]setChange{
				0: {
					oldChanged:   true,
					newChanged:   true,
					plannedIndex: 0,
				},
			},
			oldList: []resource.PropertyValue{
				resource.NewStringProperty("old_value"),
			},
			newList: []resource.PropertyValue{
				resource.NewStringProperty("new_value"),
			},
			expected: map[detailedDiffKey]*pulumirpc.PropertyDiff{
				detailedDiffKey("test_set[0]"): {Kind: pulumirpc.PropertyDiff_UPDATE},
			},
		},
		{
			name: "multiple changes",
			path: newPropertyPath("test_set"),
			changes: map[arrayIndex]setChange{
				0: {
					oldChanged:   true,
					newChanged:   false,
					plannedIndex: 0,
				},
				1: {
					oldChanged:   true,
					newChanged:   true,
					plannedIndex: 1,
				},
				2: {
					oldChanged:   false,
					newChanged:   true,
					plannedIndex: 0,
				},
			},
			oldList: []resource.PropertyValue{
				resource.NewStringProperty("delete_value"),
				resource.NewStringProperty("update_old_value"),
				resource.NewStringProperty("no_change_value"),
			},
			newList: []resource.PropertyValue{
				resource.NewStringProperty("no_change_value"),
				resource.NewStringProperty("update_new_value"),
				resource.NewStringProperty("add_value"),
			},
			expected: map[detailedDiffKey]*pulumirpc.PropertyDiff{
				detailedDiffKey("test_set[0]"): {Kind: pulumirpc.PropertyDiff_DELETE},
				detailedDiffKey("test_set[1]"): {Kind: pulumirpc.PropertyDiff_UPDATE},
				detailedDiffKey("test_set[2]"): {Kind: pulumirpc.PropertyDiff_ADD},
			},
		},
		{
			name:     "no changes",
			path:     newPropertyPath("test_set"),
			changes:  map[arrayIndex]setChange{},
			oldList:  []resource.PropertyValue{},
			newList:  []resource.PropertyValue{},
			expected: map[detailedDiffKey]*pulumirpc.PropertyDiff{},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := differ.makeSetDiffElementResult(tt.path, tt.changes, tt.oldList, tt.newList)
			require.Equal(t, tt.expected, result)
		})
	}
}
