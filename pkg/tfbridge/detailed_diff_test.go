package tfbridge

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	shimschema "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/schema"
	shimv2 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v2"
)

func TestDiffPair(t *testing.T) {
	require.Equal(t, (newPropertyPath("foo").SubKey("bar")).Key(), detailedDiffKey("foo.bar"))
	require.Equal(t, newPropertyPath("foo").SubKey("bar").SubKey("baz").Key(), detailedDiffKey("foo.bar.baz"))
	require.Equal(t, newPropertyPath("foo").SubKey("bar.baz").Key(), detailedDiffKey(`foo["bar.baz"]`))
	require.Equal(t, newPropertyPath("foo").Index(2).Key(), detailedDiffKey("foo[2]"))

	require.Equal(t, newPropertyPath("foo").SubKey("__meta").IsReservedKey(), true)
	require.Equal(t, newPropertyPath("foo").SubKey("__defaults").IsReservedKey(), true)
	require.Equal(t, newPropertyPath("__defaults").IsReservedKey(), true)
	require.Equal(t, newPropertyPath("foo").SubKey("bar").IsReservedKey(), false)
}

func TestSchemaLookupMaxItemsOne(t *testing.T) {
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

	differ := detailedDiffer{
		tfs: shimv2.NewSchemaMap(res.Schema),
	}

	sch, _, err := differ.lookupSchemas(newPropertyPath("foo"))
	require.NoError(t, err)
	require.NotNil(t, sch)
	require.Equal(t, sch.Type(), shim.TypeList)

	sch, _, err = differ.lookupSchemas(newPropertyPath("foo").SubKey("bar"))
	require.NoError(t, err)
	require.NotNil(t, sch)
	require.Equal(t, sch.Type(), shim.TypeString)
}

func TestSchemaLookupMap(t *testing.T) {
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

	differ := detailedDiffer{
		tfs: shimv2.NewSchemaMap(res.Schema),
	}

	sch, _, err := differ.lookupSchemas(newPropertyPath("foo"))
	require.NoError(t, err)
	require.NotNil(t, sch)
	require.Equal(t, sch.Type(), shim.TypeMap)

	sch, _, err = differ.lookupSchemas(propertyPath{"foo", "bar"})
	require.NoError(t, err)
	require.NotNil(t, sch)
	require.Equal(t, sch.Type(), shim.TypeString)
}

func TestMakeBaseDiff(t *testing.T) {
	nilVal := resource.NewNullProperty()
	nilArr := resource.NewArrayProperty(nil)
	nilMap := resource.NewObjectProperty(nil)
	nonNilVal := resource.NewStringProperty("foo")
	nonNilVal2 := resource.NewStringProperty("bar")

	require.Equal(t, makeBaseDiff(nilVal, nilVal), NoDiff)
	require.Equal(t, makeBaseDiff(nilVal, nilVal), NoDiff)
	require.Equal(t, makeBaseDiff(nilVal, nonNilVal), Add)
	require.Equal(t, makeBaseDiff(nilVal, nonNilVal), Add)
	require.Equal(t, makeBaseDiff(nonNilVal, nilVal), Delete)
	require.Equal(t, makeBaseDiff(nonNilVal, nilVal), Delete)
	require.Equal(t, makeBaseDiff(nonNilVal, nilArr), Delete)
	require.Equal(t, makeBaseDiff(nonNilVal, nilMap), Delete)
	require.Equal(t, makeBaseDiff(nonNilVal, nonNilVal2), Undecided)
}

func TestMakePropDiff(t *testing.T) {
	tests := []struct {
		name string
		old  resource.PropertyValue
		new  resource.PropertyValue
		etf  shimschema.Schema
		eps  *SchemaInfo
		want *pulumirpc.PropertyDiff
	}{
		{
			name: "unchanged non-nil",
			old:  resource.NewStringProperty("same"),
			new:  resource.NewStringProperty("same"),
			want: nil,
		},
		{
			name: "unchanged nil",
			old:  resource.NewNullProperty(),
			new:  resource.NewNullProperty(),
			want: nil,
		},
		{
			name: "unchanged not present",
			old:  resource.NewNullProperty(),
			new:  resource.NewNullProperty(),
			want: nil,
		},
		{
			name: "added",
			old:  resource.NewNullProperty(),
			new:  resource.NewStringProperty("new"),
			want: &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_ADD},
		},
		{
			name: "deleted",
			old:  resource.NewStringProperty("old"),
			new:  resource.NewNullProperty(),
			want: &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_DELETE},
		},
		{
			name: "changed non-nil",
			old:  resource.NewStringProperty("old"),
			new:  resource.NewStringProperty("new"),
			want: &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_UPDATE},
		},
		{
			name: "changed from nil",
			old:  resource.NewNullProperty(),
			new:  resource.NewStringProperty("new"),
			want: &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_ADD},
		},
		{
			name: "changed to nil",
			old:  resource.NewStringProperty("old"),
			new:  resource.NewNullProperty(),
			want: &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_DELETE},
		},
		{
			name: "tf force new unchanged",
			old:  resource.NewStringProperty("old"),
			new:  resource.NewStringProperty("old"),
			etf:  shimschema.Schema{ForceNew: true},
			want: nil,
		},
		{
			name: "tf force new changed non-nil",
			old:  resource.NewStringProperty("old"),
			new:  resource.NewStringProperty("new"),
			etf:  shimschema.Schema{ForceNew: true},
			want: &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_UPDATE_REPLACE},
		},
		{
			name: "tf force new changed from nil",
			old:  resource.NewNullProperty(),
			new:  resource.NewStringProperty("new"),
			etf:  shimschema.Schema{ForceNew: true},
			want: &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_ADD_REPLACE},
		},
		{
			name: "tf force new changed to nil",
			old:  resource.NewStringProperty("old"),
			new:  resource.NewNullProperty(),
			etf:  shimschema.Schema{ForceNew: true},
			want: &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_DELETE_REPLACE},
		},
		{
			name: "ps force new unchanged",
			old:  resource.NewStringProperty("old"),
			new:  resource.NewStringProperty("old"),
			eps:  &SchemaInfo{ForceNew: True()},
			want: nil,
		},
		{
			name: "ps force new changed non-nil",
			old:  resource.NewStringProperty("old"),
			new:  resource.NewStringProperty("new"),
			eps:  &SchemaInfo{ForceNew: True()},
			want: &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_UPDATE_REPLACE},
		},
		{
			name: "ps force new changed from nil",
			old:  resource.NewNullProperty(),
			new:  resource.NewStringProperty("new"),
			eps:  &SchemaInfo{ForceNew: True()},
			want: &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_ADD_REPLACE},
		},
		{
			name: "ps force new changed to nil",
			old:  resource.NewStringProperty("old"),
			new:  resource.NewNullProperty(),
			eps:  &SchemaInfo{ForceNew: True()},
			want: &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_DELETE_REPLACE},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detailedDiffer{
				tfs: shimschema.SchemaMap{"foo": tt.etf.Shim()},
				ps:  map[string]*SchemaInfo{"foo": tt.eps},
			}.makeTopPropDiff(tt.old, tt.new, newPropertyPath("foo"))
			if got == nil && tt.want == nil {
				return
			}
			if got == nil || tt.want == nil {
				t.Errorf("makeTopPropDiff() = %v, want %v", got, tt.want)
				return
			}
			if got.Kind != tt.want.Kind {
				t.Errorf("makeTopPropDiff() = %v, want %v", got.String(), tt.want.String())
			}
		})
	}
}

var Added = map[string]*pulumirpc.PropertyDiff{
	"foo": {Kind: pulumirpc.PropertyDiff_ADD},
}

var Updated = map[string]*pulumirpc.PropertyDiff{
	"foo": {Kind: pulumirpc.PropertyDiff_UPDATE},
}

var Deleted = map[string]*pulumirpc.PropertyDiff{
	"foo": {Kind: pulumirpc.PropertyDiff_DELETE},
}

var ComputedVal = resource.NewComputedProperty(resource.Computed{Element: resource.NewStringProperty("")})

func runDetailedDiffTest(
	t *testing.T,
	old, new resource.PropertyMap,
	tfs shim.SchemaMap,
	ps map[string]*SchemaInfo,
	want map[string]*pulumirpc.PropertyDiff,
) {
	t.Helper()
	differ := detailedDiffer{tfs: tfs, ps: ps}
	got := differ.makeDetailedDiffPropertyMap(old, new)

	if len(got) != len(want) {
		t.Logf("got %d diffs, want %d", len(got), len(want))
		t.Logf("got: %v", got)
		t.Logf("want: %v", want)
		t.Fatalf("unexpected diff count")
	}

	for k, v := range got {
		wantV, ok := want[k]
		if !ok {
			t.Logf("got: %v", got)
			t.Logf("want: %v", want)
			t.Fatalf("unexpected diff %s", k)
		}
		if v.Kind != wantV.Kind {
			t.Logf("got: %v", got)
			t.Logf("want: %v", want)
			t.Errorf("got diff %s = %v, want %v", k, v.Kind, wantV.Kind)
		}
	}
}

func TestBasicDetailedDiff(t *testing.T) {
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

			t.Run("unchanged", func(t *testing.T) {
				runDetailedDiffTest(t, propertyMapValue1, propertyMapValue1, tfs, ps, nil)
			})

			t.Run("changed non-empty", func(t *testing.T) {
				expected := make(map[string]*pulumirpc.PropertyDiff)
				if tt.listLike && tt.objectLike {
					expected["foo[0].foo"] = &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_UPDATE}
				} else if tt.listLike {
					expected["foo[0]"] = &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_UPDATE}
				} else if tt.objectLike {
					expected["foo.foo"] = &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_UPDATE}
				} else {
					expected["foo"] = &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_UPDATE}
				}
				runDetailedDiffTest(t, propertyMapValue1, propertyMapValue2, tfs, ps, expected)
			})

			t.Run("changed non-empty computed", func(t *testing.T) {
				expected := make(map[string]*pulumirpc.PropertyDiff)
				expected["foo"] = &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_UPDATE}
				runDetailedDiffTest(t, propertyMapValue1, propertyMapComputed, tfs, ps, expected)
			})

			t.Run("added", func(t *testing.T) {
				runDetailedDiffTest(t, propertyMapNil, propertyMapValue1, tfs, ps, Added)
			})

			if tt.emptyValue != nil {
				t.Run("added empty", func(t *testing.T) {
					runDetailedDiffTest(t, propertyMapNil, propertyMapEmpty, tfs, ps, Added)
				})
			}

			t.Run("added computed", func(t *testing.T) {
				runDetailedDiffTest(t, propertyMapNil, propertyMapComputed, tfs, ps, Added)
			})

			t.Run("deleted", func(t *testing.T) {
				runDetailedDiffTest(t, propertyMapValue1, propertyMapNil, tfs, ps, Deleted)
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
					expected := make(map[string]*pulumirpc.PropertyDiff)
					expected["foo"] = &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_UPDATE}
					runDetailedDiffTest(t, propertyMapEmpty, propertyMapComputed, tfs, ps, expected)
				})

				t.Run("unchanged empty", func(t *testing.T) {
					runDetailedDiffTest(t, propertyMapEmpty, propertyMapEmpty, tfs, ps, nil)
				})

				t.Run("deleted empty", func(t *testing.T) {
					runDetailedDiffTest(t, propertyMapEmpty, propertyMapNil, tfs, ps, Deleted)
				})

				t.Run("added empty", func(t *testing.T) {
					runDetailedDiffTest(t, propertyMapNil, propertyMapEmpty, tfs, ps, Added)
				})
			}
		})
	}
}

func TestDetailedDiffObject(t *testing.T) {
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

	t.Run("unchanged", func(t *testing.T) {
		runDetailedDiffTest(t, propertyMapProp1Val1, propertyMapProp1Val1, tfs, ps, nil)
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
}

func TestDetailedDiffList(t *testing.T) {
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
		runDetailedDiffTest(t, propertyMapVal1, propertyMapVal1, tfs, ps, nil)
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
		runDetailedDiffTest(t, propertyMapVal1, propertyMapVal1, tfs, ps, nil)
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

	t.Run("unchanged", func(t *testing.T) {
		runDetailedDiffTest(t, propertyMapVal1, propertyMapVal1, tfs, ps, nil)
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
}

func TestDetailedDiffTFForceNewPlain(t *testing.T) {
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
		runDetailedDiffTest(t, propertyMapVal1, propertyMapVal1, tfs, ps, nil)
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
			computedElem:       []interface{}{ComputedVal},
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
				runDetailedDiffTest(t, propertyMapListVal1, propertyMapListVal1, tfs, ps, nil)
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

			t.Run("changed to computed elem", func(t *testing.T) {
				runDetailedDiffTest(t, propertyMapListVal1, propertyMapComputedElem, tfs, ps, map[string]*pulumirpc.PropertyDiff{
					tt.elementIndex: {Kind: pulumirpc.PropertyDiff_UPDATE_REPLACE},
				})
			})

			t.Run("changed from empty to computed collection", func(t *testing.T) {
				runDetailedDiffTest(t, propertyMapEmpty, propertyMapComputedCollection, tfs, ps, map[string]*pulumirpc.PropertyDiff{
					"prop": {Kind: pulumirpc.PropertyDiff_ADD_REPLACE},
				})
			})

			t.Run("changed from empty to computed elem", func(t *testing.T) {
				runDetailedDiffTest(t, propertyMapEmpty, propertyMapComputedElem, tfs, ps, map[string]*pulumirpc.PropertyDiff{
					"prop": {Kind: pulumirpc.PropertyDiff_ADD_REPLACE},
				})
			})
		})
	}
}

func TestDetailedDiffTFForceNewBlockCollection(t *testing.T) {
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
		runDetailedDiffTest(t, propertyMapListVal1, propertyMapListVal1, tfs, ps, nil)
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
		runDetailedDiffTest(t, propertyMapListVal1, propertyMapListVal1, tfs, ps, nil)
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

func TestDetailedDiffTFForceNewObject(t *testing.T) {
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
		runDetailedDiffTest(t, propertyMapObjectVal1, propertyMapObjectVal1, tfs, ps, nil)
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
			runDetailedDiffTest(t, propertyMapVal1, propertyMapVal1, tfs, ps, nil)
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
			runDetailedDiffTest(t, propertyMapVal1, propertyMapVal1, tfs, ps, nil)
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
			runDetailedDiffTest(t, propertyMapVal1, propertyMapVal1, tfs, ps, nil)
		})

		t.Run("changed non-empty", func(t *testing.T) {
			runDetailedDiffTest(t, propertyMapVal1, propertyMapVal2, tfs, ps, map[string]*pulumirpc.PropertyDiff{
				"foo": {Kind: pulumirpc.PropertyDiff_UPDATE},
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
			runDetailedDiffTest(t, propertyMapVal1, propertyMapVal1, tfs, ps, nil)
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
			runDetailedDiffTest(t, propertyMapVal1, propertyMapVal1, tfs, ps, nil)
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
