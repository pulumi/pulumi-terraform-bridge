package crosstests

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// TestInputsEqualStringBasic validates that [runCreateInputCheck] works across both input
// types.
func TestInputsEqualStringBasic(t *testing.T) {
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
func TestInputsEmptyString(t *testing.T) {
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

func TestInputsUnspecifiedMaxItemsOne(t *testing.T) {
	// Regression test for [pulumi/pulumi-terraform-bridge#1767]
	runCreateInputCheck(t, inputTestCase{
		Resource: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"f0": {
					Type:     schema.TypeList,
					MaxItems: 1,
					Optional: true,
					Elem: &schema.Resource{
						Schema: map[string]*schema.Schema{
							"x": {Optional: true, Type: schema.TypeString},
						},
					},
				},
			},
		},
		Config: tftypes.NewValue(
			tftypes.Object{},
			map[string]tftypes.Value{},
		),
	})
}

func TestOptionalSetNotSpecified(t *testing.T) {
	// Regression test for [pulumi/pulumi-terraform-bridge#1970] and [pulumi/pulumi-terraform-bridge#1964]
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

func TestInputsEqualEmptyList(t *testing.T) {
	// Regression test for [pulumi/pulumi-terraform-bridge#1915]
	for _, maxItems := range []int{0, 1} {
		name := fmt.Sprintf("MaxItems: %v", maxItems)
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
							Optional: true,
							Type:     schema.TypeList,
							MaxItems: maxItems,
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

func TestExplicitNilList(t *testing.T) {
	t0 := tftypes.Map{ElementType: tftypes.Number}
	t1 := tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"f0": tftypes.List{ElementType: t0},
	}}

	// This is an explicit null on the tf side:
	// resource "crossprovider_testres" "example" {
	//     f0 = null
	// }
	runCreateInputCheck(t, inputTestCase{
		Resource: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"f0": {
					Optional: true,
					Type:     schema.TypeList,
					Elem: &schema.Schema{
						Type: schema.TypeMap,
						Elem: &schema.Schema{
							Type: schema.TypeInt,
						},
					},
				},
			},
		},
		Config: tftypes.NewValue(t1, map[string]tftypes.Value{
			"f0": tftypes.NewValue(tftypes.List{ElementType: t0}, nil),
		}),
	})
}

func TestInputsEmptyCollections(t *testing.T) {
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
		Elem: &schema.Schema{Type: schema.TypeString},
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
		// TypeMap with Elem *Resource not supported
		// {"map block", 0, schema.TypeMap, resourceElem, schema.SchemaConfigModeAuto},
		{"list max items one block", 1, schema.TypeList, resourceElem, schema.SchemaConfigModeAuto},
		{"set max items one block", 1, schema.TypeSet, resourceElem, schema.SchemaConfigModeAuto},
		// MaxItems is only valid on lists and sets
		// {"map max items one block", 1, schema.TypeMap, resourceElem, schema.SchemaConfigModeAuto},
		{"list attr", 0, schema.TypeList, schemaElem, schema.SchemaConfigModeAuto},
		{"set attr", 0, schema.TypeSet, schemaElem, schema.SchemaConfigModeAuto},
		{"map attr", 0, schema.TypeMap, schemaElem, schema.SchemaConfigModeAuto},
		{"list max items one attr", 1, schema.TypeList, schemaElem, schema.SchemaConfigModeAuto},
		{"set max items one attr", 1, schema.TypeSet, schemaElem, schema.SchemaConfigModeAuto},
		// MaxItems is only valid on lists and sets
		// {"map max items one attr", 1, schema.TypeMap, schemaElem, schema.SchemaConfigModeAuto},
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

	emptyConfig := tftypes.NewValue(tftypes.Object{}, map[string]tftypes.Value{})

	t1 := tftypes.Object{
		AttributeTypes: map[string]tftypes.Type{
			"f2": tftypes.String,
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

	nestedListListConfig := tftypes.NewValue(
		tftypes.Object{
			AttributeTypes: map[string]tftypes.Type{"f0": t0},
		}, map[string]tftypes.Value{
			"f0": tftypes.NewValue(t0, []tftypes.Value{
				tftypes.NewValue(t1, map[string]tftypes.Value{
					"f2": tftypes.NewValue(tftypes.String, "val"),
				}),
			}),
		},
	)

	t2 := tftypes.Set{ElementType: t1}

	nestedSetSetConfig := tftypes.NewValue(
		tftypes.Object{
			AttributeTypes: map[string]tftypes.Type{"f0": t2},
		}, map[string]tftypes.Value{
			"f0": tftypes.NewValue(t2, []tftypes.Value{
				tftypes.NewValue(t1, map[string]tftypes.Value{
					"f2": tftypes.NewValue(tftypes.String, "val"),
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
		{"nested non empty list list block", schema.TypeList, schema.TypeList, nestedListListConfig},
		{"nested non empty set set block", schema.TypeSet, schema.TypeSet, nestedSetSetConfig},
	} {
		t.Run(tc.name, func(t *testing.T) {
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
									// This allows us to specify non-empty f0s with an empty f1
									"f2": {
										Type:     schema.TypeString,
										Optional: true,
									},
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

func TestEmptySetOfEmptyObjects(t *testing.T) {
	t1 := tftypes.Object{}
	t0 := tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"d3f0": tftypes.Set{ElementType: t1},
	}}
	config := tftypes.NewValue(t0, map[string]tftypes.Value{
		"d3f0": tftypes.NewValue(tftypes.Set{ElementType: t1}, []tftypes.Value{}),
	})

	runCreateInputCheck(t, inputTestCase{
		Resource: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"d3f0": {
					Type:     schema.TypeSet,
					Optional: true,
					Elem:     &schema.Resource{Schema: map[string]*schema.Schema{}},
				},
			},
		},
		Config: config,
	})
}

func TestMap(t *testing.T) {
	t0 := tftypes.Map{ElementType: tftypes.String}
	t1 := tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"tags": t0,
	}}
	mapVal := tftypes.NewValue(t0, map[string]tftypes.Value{
		"key":  tftypes.NewValue(tftypes.String, "val"),
		"key2": tftypes.NewValue(tftypes.String, "val2"),
	})
	config := tftypes.NewValue(t1, map[string]tftypes.Value{
		"tags": mapVal,
	})

	runCreateInputCheck(t, inputTestCase{
		Resource: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"tags": {
					Type:     schema.TypeMap,
					Optional: true,
					Elem: &schema.Schema{
						Optional: true,
						Type:     schema.TypeString,
					},
				},
			},
		},
		Config: config,
	})
}

func TestTimeouts(t *testing.T) {
	emptyConfig := tftypes.NewValue(tftypes.Object{}, map[string]tftypes.Value{})
	runCreateInputCheck(t, inputTestCase{
		Resource: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"tags": {
					Type:     schema.TypeMap,
					Optional: true,
					Elem: &schema.Schema{
						Optional: true,
						Type:     schema.TypeString,
					},
				},
			},
			Timeouts: &schema.ResourceTimeout{
				Create: schema.DefaultTimeout(time.Duration(120)),
			},
		},
		Config: emptyConfig,
	})
}

// TestAccCloudWatch failed with PlanResourceChange to do a simple Create preview because the state upgrade was
// unexpectedly called with nil state. Emulate this here to test it does not fail.
func TestCreateDoesNotPanicWithStateUpgraders(t *testing.T) {

	resourceRuleV0 := func() *schema.Resource {
		return &schema.Resource{
			Schema: map[string]*schema.Schema{
				"event_bus_name": {
					Type:     schema.TypeString,
					Optional: true,
				},
				"is_enabled": {
					Type:     schema.TypeBool,
					Optional: true,
				},
			},
		}
	}

	resourceRuleUpgradeV0 := func(ctx context.Context, rawState map[string]any, meta any) (map[string]any, error) {
		if rawState == nil {
			rawState = map[string]any{}
		}

		if rawState["is_enabled"].(bool) { // used to panic here
			t.Logf("enabled")
		} else {
			t.Logf("disabled")
		}

		return rawState, nil
	}

	runCreateInputCheck(t, inputTestCase{
		Resource: &schema.Resource{
			SchemaVersion: 1,
			StateUpgraders: []schema.StateUpgrader{
				{
					Type:    resourceRuleV0().CoreConfigSchema().ImpliedType(),
					Upgrade: resourceRuleUpgradeV0,
					Version: 0,
				},
			},
			Schema: resourceRuleV0().Schema,
		},
		Config: map[string]any{
			"event_bus_name": "default",
		},
	})
}

// TestStateFunc ensures that resources with a StateFunc set on their schema are correctly
// handled. This includes ensuring that the PlannedPrivate blob is passed from
// PlanResourceChange to ApplyResourceChange. If this is passed correctly, the provider
// will see the original value of the field, rather than the value that was produced by
// the StateFunc.
func TestStateFunc(t *testing.T) {
	input := "hello"

	runCreateInputCheck(t, inputTestCase{
		Resource: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"test": {
					Type:     schema.TypeString,
					Optional: true,
					ForceNew: true,
					StateFunc: func(v interface{}) string {
						return v.(string) + " world"
					},
				},
			},
		},
		Config: map[string]any{
			"test": input,
		},
	})
}
