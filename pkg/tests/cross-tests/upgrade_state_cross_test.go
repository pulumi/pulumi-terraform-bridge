package crosstests

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func TestUpgradeInputsStringBasicSame(t *testing.T) {
	skipUnlessLinux(t)
	res := &schema.Resource{
		Schema: map[string]*schema.Schema{
			"f0": {
				Type:     schema.TypeString,
				Optional: true,
			},
		},
	}

	configVal := func(val string) tftypes.Value {
		return tftypes.NewValue(tftypes.Object{
			AttributeTypes: map[string]tftypes.Type{
				"f0": tftypes.String,
			},
		}, map[string]tftypes.Value{
			"f0": tftypes.NewValue(tftypes.String, val),
		})
	}
	for _, PRC := range []bool{true, false} {
		t.Run(fmt.Sprintf("PRC=%v", PRC), func(t *testing.T) {
			t.Run("same", func(t *testing.T) {
				runUpgradeStateInputCheck(t, upgradeStateTestCase{
					Resource:                  res,
					Config1:                   configVal("val"),
					Config2:                   configVal("val"),
					DisablePlanResourceChange: !PRC,
					ExpectEqual:               true,
				})
			})

			t.Run("different", func(t *testing.T) {
				runUpgradeStateInputCheck(t, upgradeStateTestCase{
					Resource:                  res,
					Config1:                   configVal("val1"),
					Config2:                   configVal("val2"),
					DisablePlanResourceChange: !PRC,
				})
			})
		})
	}
}

func TestUpgradeInputsStringBasicNonZeroVersionSame(t *testing.T) {
	skipUnlessLinux(t)

	res := &schema.Resource{
		Schema: map[string]*schema.Schema{
			"f0": {
				Type:     schema.TypeString,
				Optional: true,
			},
		},
		SchemaVersion: 1,
	}

	configVal := func(val string) tftypes.Value {
		return tftypes.NewValue(tftypes.Object{
			AttributeTypes: map[string]tftypes.Type{
				"f0": tftypes.String,
			},
		}, map[string]tftypes.Value{
			"f0": tftypes.NewValue(tftypes.String, val),
		})
	}
	for _, PRC := range []bool{true, false} {
		t.Run(fmt.Sprintf("PRC=%v", PRC), func(t *testing.T) {
			t.Run("same", func(t *testing.T) {
				runUpgradeStateInputCheck(t, upgradeStateTestCase{
					Resource:                  res,
					Config1:                   configVal("val"),
					Config2:                   configVal("val"),
					DisablePlanResourceChange: !PRC,
					ExpectEqual:               true,
				})
			})

			t.Run("different", func(t *testing.T) {
				runUpgradeStateInputCheck(t, upgradeStateTestCase{
					Resource:                  res,
					Config1:                   configVal("val1"),
					Config2:                   configVal("val2"),
					DisablePlanResourceChange: !PRC,
				})
			})
		})
	}
}

func TestUpgradeInputsObjectBasicSame(t *testing.T) {
	skipUnlessLinux(t)

	res := &schema.Resource{
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
	}

	t1 := tftypes.Object{
		AttributeTypes: map[string]tftypes.Type{
			"x": tftypes.String,
		},
	}
	t0 := tftypes.List{ElementType: t1}

	configVal := func(val string) tftypes.Value {
		return tftypes.NewValue(
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
								"x": tftypes.NewValue(tftypes.String, val),
							}),
					},
				),
			},
		)
	}
	for _, PRC := range []bool{true, false} {
		t.Run(fmt.Sprintf("PRC=%v", PRC), func(t *testing.T) {
			t.Run("same", func(t *testing.T) {
				runUpgradeStateInputCheck(t, upgradeStateTestCase{
					Resource:                  res,
					Config1:                   configVal("val"),
					Config2:                   configVal("val"),
					DisablePlanResourceChange: !PRC,
					ExpectEqual:               true,
				})
			})

			t.Run("different", func(t *testing.T) {
				runUpgradeStateInputCheck(t, upgradeStateTestCase{
					Resource:                  res,
					Config1:                   configVal("val1"),
					Config2:                   configVal("val2"),
					DisablePlanResourceChange: !PRC,
				})
			})
		})
	}
}
