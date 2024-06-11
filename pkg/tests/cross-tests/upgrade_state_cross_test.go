package crosstests

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func TestUpgradeInputsStringBasic(t *testing.T) {
	for _, PRC := range []bool{true, false} {
		t.Run(fmt.Sprintf("PRC=%v", PRC), func(t *testing.T) {
			skipUnlessLinux(t)
			runUpgradeStateInputCheck(t, inputTestCase{
				Resource: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"f0": {
							Type:     schema.TypeString,
							Optional: true,
						},
					},
				},
				Config: tftypes.NewValue(tftypes.Object{
					AttributeTypes: map[string]tftypes.Type{
						"f0": tftypes.String,
					},
				}, map[string]tftypes.Value{
					"f0": tftypes.NewValue(tftypes.String, "val"),
				}),
				DisablePlanResourceChange: !PRC,
			})
		})
	}
}

func TestUpgradeInputsStringBasicNonZeroVersion(t *testing.T) {
	for _, PRC := range []bool{true, false} {
		t.Run(fmt.Sprintf("PRC=%v", PRC), func(t *testing.T) {
			skipUnlessLinux(t)
			runUpgradeStateInputCheck(t, inputTestCase{
				Resource: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"f0": {
							Type:     schema.TypeString,
							Optional: true,
						},
					},
					SchemaVersion: 1,
				},
				Config: tftypes.NewValue(tftypes.Object{
					AttributeTypes: map[string]tftypes.Type{
						"f0": tftypes.String,
					},
				}, map[string]tftypes.Value{
					"f0": tftypes.NewValue(tftypes.String, "val"),
				}),
			})
		})
	}
}

func TestUpgradeInputsObjectBasic(t *testing.T) {
	for _, PRC := range []bool{true, false} {
		t.Run(fmt.Sprintf("PRC=%v", PRC), func(t *testing.T) {
			skipUnlessLinux(t)
			t1 := tftypes.Object{
				AttributeTypes: map[string]tftypes.Type{
					"x": tftypes.String,
				},
			}
			t0 := tftypes.List{ElementType: t1}
			runUpgradeStateInputCheck(t, inputTestCase{
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
				Config: tftypes.NewValue(
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
			})
		})
	}
}
