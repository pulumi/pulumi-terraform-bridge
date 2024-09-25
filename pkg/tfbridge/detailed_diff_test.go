package tfbridge

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/tokens"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	shimv2 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v2"
)

func computeSchemas(sch map[string]*schema.Schema) (map[string]*info.Schema, shim.SchemaMap) {
	tfp := &schema.Provider{ResourcesMap: map[string]*schema.Resource{
		"prov_res": {Schema: sch},
	}}
	shimProvider := shimv2.NewProvider(tfp)

	provider := ProviderInfo{
		P:                              shimProvider,
		Name:                           "prov",
		Version:                        "0.0.1",
		MetadataInfo:                   &MetadataInfo{},
		EnableZeroDefaultSchemaVersion: true,
	}
	makeToken := func(module, name string) (string, error) {
		return tokens.MakeStandard("prov")(module, name)
	}
	provider.MustComputeTokens(tokens.SingleModule("prov", "index", makeToken))

	return provider.Resources["prov_res"].Fields, provider.P.ResourcesMap().Get("prov_res").Schema()
}

func TestSubPath(t *testing.T) {
	require.Equal(t, getSubPath("foo", "bar"), resource.PropertyKey("foo.bar"))
	require.Equal(t, getSubPath("foo.bar", "baz"), resource.PropertyKey("foo.bar.baz"))
	require.Equal(t, getSubPath("foo", "bar.baz"), resource.PropertyKey(`foo["bar.baz"]`))
}

func TestMakeBaseDiff(t *testing.T) {
	nilVal := resource.NewNullProperty()
	nilArr := resource.NewArrayProperty(nil)
	nilMap := resource.NewObjectProperty(nil)
	nonNilVal := resource.NewStringProperty("foo")
	nonNilVal2 := resource.NewStringProperty("bar")

	require.Equal(t, makeBaseDiff(nilVal, nilVal, true, true), NoDiff)
	require.Equal(t, makeBaseDiff(nilVal, nilVal, false, false), NoDiff)
	require.Equal(t, makeBaseDiff(nilVal, nonNilVal, true, true), Add)
	require.Equal(t, makeBaseDiff(nilVal, nonNilVal, false, true), Add)
	require.Equal(t, makeBaseDiff(nonNilVal, nilVal, true, false), Delete)
	require.Equal(t, makeBaseDiff(nonNilVal, nilArr, true, true), Delete)
	require.Equal(t, makeBaseDiff(nonNilVal, nilMap, true, true), Delete)
	require.Equal(t, makeBaseDiff(nonNilVal, nonNilVal2, true, true), Undecided)
}

func TestMakePropDiff(t *testing.T) {
	tests := []struct {
		name  string
		old   resource.PropertyValue
		new   resource.PropertyValue
		oldOk bool
		newOk bool
		want  *pulumirpc.PropertyDiff
	}{
		{
			name:  "unchanged non-nil",
			old:   resource.NewStringProperty("same"),
			new:   resource.NewStringProperty("same"),
			oldOk: true,
			newOk: true,
			want:  nil,
		},
		{
			name:  "unchanged nil",
			old:   resource.NewNullProperty(),
			new:   resource.NewNullProperty(),
			oldOk: true,
			newOk: true,
			want:  nil,
		},
		{
			name:  "unchanged not present",
			old:   resource.NewNullProperty(),
			new:   resource.NewNullProperty(),
			oldOk: false,
			newOk: false,
			want:  nil,
		},
		{
			name:  "added",
			old:   resource.NewNullProperty(),
			new:   resource.NewStringProperty("new"),
			oldOk: false,
			newOk: true,
			want:  &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_ADD},
		},
		{
			name:  "deleted",
			old:   resource.NewStringProperty("old"),
			new:   resource.NewNullProperty(),
			oldOk: true,
			newOk: false,
			want:  &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_DELETE},
		},
		{
			name:  "changed non-nil",
			old:   resource.NewStringProperty("old"),
			new:   resource.NewStringProperty("new"),
			oldOk: true,
			newOk: true,
			want:  &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_UPDATE},
		},
		{
			name:  "changed from nil",
			old:   resource.NewNullProperty(),
			new:   resource.NewStringProperty("new"),
			oldOk: true,
			newOk: true,
			want:  &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_ADD},
		},
		{
			name:  "changed to nil",
			old:   resource.NewStringProperty("old"),
			new:   resource.NewNullProperty(),
			oldOk: true,
			newOk: true,
			want:  &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_DELETE},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := makeTopPropDiff(tt.old, tt.new, tt.oldOk, tt.newOk)
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

func runDetailedDiffTest(
	t *testing.T,
	old, new resource.PropertyMap,
	tfs shim.SchemaMap,
	ps map[string]*SchemaInfo,
	want map[string]*pulumirpc.PropertyDiff,
) {
	got := makePulumiDetailedDiffV2(context.Background(), tfs, ps, old, new)

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
			emptyValue: []interface{}{},
			value1:     []interface{}{map[string]interface{}{"foo": "bar"}},
			value2:     []interface{}{map[string]interface{}{"foo": "baz"}},
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
			emptyValue: []interface{}{},
			value1:     []interface{}{map[string]interface{}{"foo": "bar"}},
			value2:     []interface{}{map[string]interface{}{"foo": "baz"}},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			for _, optional := range []string{"Optional", "Required", "Computed", "Optional + Computed"} {
				t.Run(optional, func(t *testing.T) {
					optionalValue := optional == "Optional" || optional == "Optional + Computed"
					requiredValue := optional == "Required"
					computedValue := optional == "Computed" || optional == "Optional + Computed"

					sdkv2Schema := map[string]*schema.Schema{
						"foo": &tt.tfs,
					}
					if optionalValue {
						sdkv2Schema["foo"].Optional = true
					}
					if requiredValue {
						sdkv2Schema["foo"].Required = true
					}
					if computedValue {
						sdkv2Schema["foo"].Computed = true
					}

					ps, tfs := computeSchemas(sdkv2Schema)
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

					t.Run("added", func(t *testing.T) {
						runDetailedDiffTest(t, propertyMapNil, propertyMapValue1, tfs, ps, Added)
					})

					if tt.emptyValue != nil {
						t.Run("added empty", func(t *testing.T) {
							runDetailedDiffTest(t, propertyMapNil, propertyMapEmpty, tfs, ps, Added)
						})
					}

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
	ps, tfs := computeSchemas(sdkv2Schema)

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
	ps, tfs := computeSchemas(sdkv2Schema)

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
	ps, tfs := computeSchemas(sdkv2Schema)

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
	ps, tfs := computeSchemas(sdkv2Schema)

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

// TODO: Pulumi-level override tests
