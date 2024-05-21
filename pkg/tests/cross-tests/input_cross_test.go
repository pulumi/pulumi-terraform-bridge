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

func TestInputsEmptyCollections(t *testing.T) {
	skipUnlessLinux(t)
	config := tftypes.NewValue(tftypes.Object{}, map[string]tftypes.Value{})

	// signifies a block
	resourceElem := &schema.Resource{
		Schema: map[string]*schema.Schema{
			"x": {Optional: true, Type: schema.TypeString},
		},
	}

	// signifies an attribute
	schemaElem := &schema.Schema{
		Type: schema.TypeMap,
	}

	for _, tc := range []struct {
		name       string
		maxItems   int
		typ        schema.ValueType
		elem       any
		configMode schema.SchemaConfigMode
	}{
		{"list block", 0, schema.TypeList, resourceElem, schema.SchemaConfigModeAuto},
		{"set block", 0, schema.TypeSet, resourceElem, schema.SchemaConfigModeAuto},
		// This isn't quite valid but should work
		{"map block", 0, schema.TypeMap, resourceElem, schema.SchemaConfigModeAuto},
		{"list max items one block", 1, schema.TypeList, resourceElem, schema.SchemaConfigModeAuto},
		{"set max items one block", 1, schema.TypeSet, resourceElem, schema.SchemaConfigModeAuto},
		// This isn't quite valid but should work
		{"map max items one block", 1, schema.TypeMap, resourceElem, schema.SchemaConfigModeAuto},
		{"list attr", 0, schema.TypeList, schemaElem, schema.SchemaConfigModeAuto},
		{"set attr", 0, schema.TypeSet, schemaElem, schema.SchemaConfigModeAuto},
		{"map attr", 0, schema.TypeMap, schemaElem, schema.SchemaConfigModeAuto},
		{"list max items one attr", 1, schema.TypeList, schemaElem, schema.SchemaConfigModeAuto},
		{"set max items one attr", 1, schema.TypeSet, schemaElem, schema.SchemaConfigModeAuto},
		{"map max items one attr", 1, schema.TypeMap, schemaElem, schema.SchemaConfigModeAuto},
		{"list config mode attr", 0, schema.TypeList, resourceElem, schema.SchemaConfigModeAttr},
		{"set config mode attr", 0, schema.TypeSet, resourceElem, schema.SchemaConfigModeAttr},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runCreateInputCheck(t, inputTestCase{
				Resource: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"f0": {
							Type:       tc.typ,
							MaxItems:   tc.maxItems,
							Elem:       tc.elem,
							ConfigMode: tc.configMode,
							Optional:   true,
						},
					},
				},
				Config: config,
			})
		})
	}
}

func TestInputsNestedBlocksEmpty(t *testing.T) {
	skipUnlessLinux(t)

	emptyConfig := tftypes.NewValue(tftypes.Object{}, map[string]tftypes.Value{})

	t3 := tftypes.Object{
		AttributeTypes: map[string]tftypes.Type{
			"x": tftypes.String,
		},
	}
	t2 := tftypes.List{ElementType: t3}
	t1 := tftypes.Object{
		AttributeTypes: map[string]tftypes.Type{
			"f1": t2,
		},
	}
	t0 := tftypes.List{ElementType: t1}

	topLevelNonEmptyConfig := tftypes.NewValue(
		tftypes.Object{
			AttributeTypes: map[string]tftypes.Type{"f0": t0},
		}, map[string]tftypes.Value{
			"f0": tftypes.NewValue(t0, []tftypes.Value{}),
		},
	)

	// TODO: Investigate why this produces the wrong tf program
	// resource "crossprovider_testres" "example" {
	// 	f0 {
 	//  }
	// }

	// it should produce
	// resource "crossprovider_testres" "example" {
	// 	f0 {
	//   f1 {}
 	//  }
	// }
	nestedNonEmptyConfig := tftypes.NewValue(
		tftypes.Object{
			AttributeTypes: map[string]tftypes.Type{"f0": t0},
		}, map[string]tftypes.Value{
			"f0": tftypes.NewValue(t0, []tftypes.Value{
				tftypes.NewValue(t1, map[string]tftypes.Value{
					"f1": tftypes.NewValue(t2, []tftypes.Value{}),
				}),
			}),
		},
	)

	for _, tc := range []struct {
		name   string
		typ1   schema.ValueType
		typ2   schema.ValueType
		config tftypes.Value
	}{
		{"empty list list block", schema.TypeList, schema.TypeList, emptyConfig},
		{"empty set set block", schema.TypeSet, schema.TypeSet, emptyConfig},
		{"empty list set block", schema.TypeList, schema.TypeSet, emptyConfig},
		{"non empty list list block", schema.TypeList, schema.TypeList, topLevelNonEmptyConfig},
		{"nested non empty list list block", schema.TypeList, schema.TypeList, nestedNonEmptyConfig},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.name != "nested non empty list list block" {
				t.SkipNow()
			}
			runCreateInputCheck(t, inputTestCase{
				Resource: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"f0": {
							Type:     tc.typ1,
							Optional: true,
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"f1": {
										Type:     tc.typ2,
										Optional: true,
										Elem: &schema.Resource{
											Schema: map[string]*schema.Schema{
												"x": {Optional: true, Type: schema.TypeString},
											},
										},
									},
								},
							},
						},
					},
				},
				Config: tc.config,
			})
			panic("here!")
		})
	}
}
