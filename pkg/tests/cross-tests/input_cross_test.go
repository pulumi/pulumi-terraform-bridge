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
	t.Skip("We represent resources with empty schemas wrong.")
	// TODO[pulumi/pulumi-terraform-bridge#1914]
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
	t.Skip("We misrepresent empty lists")
	// TODO[pulumi/pulumi-terraform-bridge#1915]
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
	t.Skip("Empty strings are misrepresented")
	// TODO[pulumi/pulumi-terraform-bridge#1916]
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
			}),
	})
}

// Isolated from rapid-generated tests
func TestInputsEmptyConfigModeAttrSet(t *testing.T) {
	skipUnlessLinux(t)
	t.Skip("Our handling of ConfigModeAttr is wrong.")
	// TODO[pulumi/pulumi-terraform-bridge#1762]
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
	//skipUnlessLinux(t)

	//t.Skip("We misrepresent empty sets")
	// TODO[pulumi/pulumi-terraform-bridge#1970]

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

// input_check.go:40: RawConfig not equal!
// input_check.go:41: TF value cty.ObjectVal(map[string]cty.Value{"f0":cty.SetValEmpty(cty.Object(map[string]cty.Type{"x":cty.String})), "id":cty.NullVal(cty.String)})
// input_check.go:42: PU value cty.ObjectVal(map[string]cty.Value{"f0":cty.NullVal(cty.Set(cty.Object(map[string]cty.Type{"x":cty.String}))), "id":cty.NullVal(cty.String)})
// input_check.go:40: RawPlan not equal!
// input_check.go:41: TF value cty.ObjectVal(map[string]cty.Value{"f0":cty.SetValEmpty(cty.Object(map[string]cty.Type{"x":cty.String})), "id":cty.UnknownVal(cty.String)})
// input_check.go:42: PU value cty.ObjectVal(map[string]cty.Value{"f0":cty.NullVal(cty.Set(cty.Object(map[string]cty.Type{"x":cty.String}))), "id":cty.NullVal(cty.String)})
