package crosstests

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func TestInputsEqualStringBasic(t *testing.T) {
	skipUnlessLinux(t)
	// Test both config representations.
	for _, tc := range []struct {
		name   string
		config any
	}{
		{"any", map[string]any{"f0": "val"}},
		{
			"tftype",
			tftypes.NewValue(tftypes.Object{
				AttributeTypes: map[string]tftypes.Type{
					"f0": tftypes.String,
				},
			}, map[string]tftypes.Value{
				"f0": tftypes.NewValue(tftypes.String, "val"),
			}),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runCreateInputCheck(t, inputTestCase{
				Resource: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"f0": {
							Type:     schema.TypeString,
							Optional: true,
						},
					},
				},
				Config: tc.config,
			})
		})
	}
}

func TestInputsEqualObjectBasic(t *testing.T) {
	skipUnlessLinux(t)
	t1 := tftypes.Object{
		AttributeTypes: map[string]tftypes.Type{
			"x": tftypes.String,
		},
	}
	t0 := tftypes.List{ElementType: t1}

	// Test both config representations.
	for _, tc := range []struct {
		name   string
		config any
	}{
		{"any", map[string]any{"f0": []any{map[string]any{"x": "ok"}}}},
		{
			"tftype",
			tftypes.NewValue(
				tftypes.Object{
					AttributeTypes: map[string]tftypes.Type{
						"f0": t0,
					},
				},
				map[string]tftypes.Value{
					"f0": tftypes.NewValue(t0,
						[]tftypes.Value{
							tftypes.NewValue(t1,
								map[string]tftypes.Value{
									"x": tftypes.NewValue(tftypes.String, "ok"),
								}),
						},
					),
				},
			),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runCreateInputCheck(t, inputTestCase{
				Resource: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"f0": {
							Required: true,
							Type:     schema.TypeList,
							MaxItems: 1,
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"x": {Optional: true, Type: schema.TypeString},
								},
							},
						},
					},
				},
				Config: tc.config,
			})
		})
	}
}

// Isolated from rapid-generated tests
func TestInputsEmptySchema(t *testing.T) {
	skipUnlessLinux(t)
	runCreateInputCheck(
		t, inputTestCase{
			Resource: &schema.Resource{
				Schema: map[string]*schema.Schema{},
			},
			Config: tftypes.NewValue(tftypes.Object{}, map[string]tftypes.Value{}),
		},
	)
}

func TestInputsEqualEmptyList(t *testing.T) {
	skipUnlessLinux(t)
	for _, maxItems := range []int{0, 1} {
		for _, configMode := range []schema.SchemaConfigMode{schema.SchemaConfigModeAuto, schema.SchemaConfigModeBlock, schema.SchemaConfigModeAttr} {
			name := fmt.Sprintf("MaxItems: %v, ConfigMode: %v", maxItems, configMode)
			t.Run(name, func(t *testing.T) {
				t1 := tftypes.List{ElementType: tftypes.String}
				t0 := tftypes.Object{
					AttributeTypes: map[string]tftypes.Type{
						"f0": t1,
					},
				}
				runCreateInputCheck(t, inputTestCase{
					Resource: &schema.Resource{
						Schema: map[string]*schema.Schema{
							"f0": {
								Optional:   true,
								Type:       schema.TypeList,
								MaxItems:   maxItems,
								ConfigMode: configMode,
								Elem: &schema.Resource{
									Schema: map[string]*schema.Schema{
										"x": {Optional: true, Type: schema.TypeString},
									},
								},
							},
						},
					},
					Config: tftypes.NewValue(
						t0,
						map[string]tftypes.Value{
							"f0": tftypes.NewValue(t1, []tftypes.Value{}),
						},
					),
				})
			})
		}
	}
}

// Isolated from rapid-generated tests
func TestInputsEmptyString(t *testing.T) {
	skipUnlessLinux(t)
	runCreateInputCheck(t, inputTestCase{
		Resource: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"f0": {
					Type:     schema.TypeString,
					Required: true,
				},
			},
		},
		Config: tftypes.NewValue(
			tftypes.Object{
				AttributeTypes: map[string]tftypes.Type{
					"f0": tftypes.String,
				},
			},
			map[string]tftypes.Value{
				"f0": tftypes.NewValue(tftypes.String, ""),
			},
		),
	})
}

// Isolated from rapid-generated tests
func TestInputsEmptyConfigModeAttrSet(t *testing.T) {
	skipUnlessLinux(t)
	t2 := tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"f0": tftypes.String,
	}}
	t0 := tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"f1": tftypes.Set{ElementType: t2},
	}}

	runCreateInputCheck(t, inputTestCase{
		Resource: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"f1": {
					Type:       schema.TypeSet,
					ConfigMode: schema.SchemaConfigMode(1),
					Optional:   true,
					Elem: &schema.Resource{Schema: map[string]*schema.Schema{
						"f0": {
							Type:     schema.TypeString,
							Optional: true,
							ForceNew: true,
						},
					}},
					MaxItems: 1,
				},
			},
		},
		Config: tftypes.NewValue(t0, map[string]tftypes.Value{
			"f1": tftypes.NewValue(tftypes.Set{ElementType: t2}, []tftypes.Value{}),
		}),
	})
}

func TestOptionalSetNotSpecified(t *testing.T) {
	skipUnlessLinux(t)
	runCreateInputCheck(t, inputTestCase{
		Resource: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"f0": {
					Optional: true,
					Type:     schema.TypeSet,
					Elem: &schema.Resource{
						Schema: map[string]*schema.Schema{
							"x": {Optional: true, Type: schema.TypeString},
						},
					},
				},
			},
		},
		Config: tftypes.NewValue(tftypes.Object{}, map[string]tftypes.Value{}),
	})
}

func TestExplicitNilList(t *testing.T) {
	skipUnlessLinux(t)
	t.Skipf("We've regressed on explicit nills")
	// TODO: fix.
	runCreateInputCheck(t, inputTestCase{
		Resource: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"f0": {
					Optional: true,
					Type:     schema.TypeList,
					Elem: &schema.Schema{
						Type:     schema.TypeMap,
						Optional: true,
						Computed: true,
						Elem: &schema.Schema{
							Type:      schema.TypeInt,
							Optional:  true,
							Sensitive: true,
						},
					},
				},
			},
		},
		// TODO: How does one express this with tftypes?
		// tftypes.NewValue(tftypes.DynamicPseudoType, nil)?
		Config: map[string]interface{}{"f0": nil},
	})
}

func TestRequiredEmptyListOfObjects(t *testing.T) {
	skipUnlessLinux(t)

	t1 := tftypes.Object{}
	t0 := tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"d3f0": tftypes.List{ElementType: t1},
	}}
	config := tftypes.NewValue(t0, map[string]tftypes.Value{
		"d3f0": tftypes.NewValue(tftypes.List{ElementType: t1}, []tftypes.Value{}),
	})

	runCreateInputCheck(t, inputTestCase{
		Resource: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"d3f0": {
					Type:     schema.TypeList,
					Required: true,
					Elem:     &schema.Resource{Schema: map[string]*schema.Schema{}},
					MaxItems: 1,
				},
			},
		},
		Config: config,
	})
}

func TestRequiredEmptyListOfLists(t *testing.T) {
	skipUnlessLinux(t)
	t.Skipf("Fix - returns []interface {}{interface {}(nil)} instead of []interface {}{}")

	t1 := tftypes.List{ElementType: tftypes.String}
	t0 := tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"d3f0": tftypes.List{ElementType: t1},
	}}
	config := tftypes.NewValue(t0, map[string]tftypes.Value{
		"d3f0": tftypes.NewValue(tftypes.List{ElementType: t1}, []tftypes.Value{}),
	})

	runCreateInputCheck(t, inputTestCase{
		Resource: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"d3f0": {
					Type:     schema.TypeList,
					Required: true,
					Elem: &schema.Schema{
						Type: schema.TypeList, Elem: &schema.Schema{Type: schema.TypeString},
					},
					MaxItems: 1,
				},
			},
		},
		Config: config,
	})
}

func TestRequiredEmptyListOfSets(t *testing.T) {
	skipUnlessLinux(t)
	t.Skipf("Fix - returns []interface {}{interface {}(nil)} instead of []interface {}{}")

	t1 := tftypes.Set{ElementType: tftypes.String}
	t0 := tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"d3f0": tftypes.List{ElementType: t1},
	}}
	config := tftypes.NewValue(t0, map[string]tftypes.Value{
		"d3f0": tftypes.NewValue(tftypes.List{ElementType: t1}, []tftypes.Value{}),
	})

	runCreateInputCheck(t, inputTestCase{
		Resource: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"d3f0": {
					Type:     schema.TypeList,
					Required: true,
					Elem: &schema.Schema{
						Type: schema.TypeSet, Elem: &schema.Schema{Type: schema.TypeString},
					},
					MaxItems: 1,
				},
			},
		},
		Config: config,
	})
}

func TestNonEmptyNestedMaxItemsOnes(t *testing.T) {
	skipUnlessLinux(t)

	t1 := tftypes.List{ElementType: tftypes.String}
	t0 := tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"d3f0": tftypes.List{ElementType: t1},
	}}
	// Non-empty
	config1 := tftypes.NewValue(t0, map[string]tftypes.Value{
		"d3f0": tftypes.NewValue(tftypes.List{ElementType: t1}, []tftypes.Value{
			tftypes.NewValue(t1, []tftypes.Value{
				tftypes.NewValue(tftypes.String, "foo"),
			}),
		}),
	})
	// first level empty
	config2 := tftypes.NewValue(t0, map[string]tftypes.Value{
		"d3f0": tftypes.NewValue(tftypes.List{ElementType: t1}, []tftypes.Value{}),
	})
	// second level empty - this is impossible to handle because of flattening
	_ = tftypes.NewValue(t0, map[string]tftypes.Value{
		"d3f0": tftypes.NewValue(tftypes.List{ElementType: t1}, []tftypes.Value{
			tftypes.NewValue(t1, []tftypes.Value{}),
		}),
	})

	for _, tc := range []struct {
		name   string
		config tftypes.Value
	}{
		{"non-empty", config1},
		{"first-level-empty", config2},
		// {"second-level-empty", config3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runCreateInputCheck(t, inputTestCase{
				Resource: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"d3f0": {
							Type:     schema.TypeList,
							Required: true,
							Elem: &schema.Schema{
								Type:     schema.TypeList,
								MaxItems: 1,
								Elem:     &schema.Schema{Type: schema.TypeString},
							},
							MaxItems: 1,
						},
					},
				},
				Config: tc.config,
			})
		})
	}
}
