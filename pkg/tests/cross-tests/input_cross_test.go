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
			// TODO[pulumi/pulumi-terraform-bridge#1914]
			SkipCompareRaw: true,
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
					// TODO[pulumi/pulumi-terraform-bridge#1915]
					SkipCompareRaw: true,
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
		// TODO[pulumi/pulumi-terraform-bridge#1916]
		SkipCompareRaw: true,
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
		// TODO[pulumi/pulumi-terraform-bridge#1762]
		SkipCompareRaw: true,
	})
}
