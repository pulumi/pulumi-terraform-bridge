package crosstests

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func TestUpgradeInputsStringBasic(t *testing.T) {
	t.Parallel()
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
	t.Run("same", func(t *testing.T) {
		runUpgradeStateInputCheck(t, upgradeStateTestCase{
			Resource:    res,
			Config1:     configVal("val"),
			Config2:     configVal("val"),
			ExpectEqual: true,
		})
	})

	t.Run("different", func(t *testing.T) {
		runUpgradeStateInputCheck(t, upgradeStateTestCase{
			Resource: res,
			Config1:  configVal("val1"),
			Config2:  configVal("val2"),
		})
	})
}

func TestUpgradeInputsStringBasicNonZeroVersion(t *testing.T) {
	t.Parallel()
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
	t.Run("same", func(t *testing.T) {
		runUpgradeStateInputCheck(t, upgradeStateTestCase{
			Resource:    res,
			Config1:     configVal("val"),
			Config2:     configVal("val"),
			ExpectEqual: true,
		})
	})

	t.Run("different", func(t *testing.T) {
		runUpgradeStateInputCheck(t, upgradeStateTestCase{
			Resource: res,
			Config1:  configVal("val1"),
			Config2:  configVal("val2"),
		})
	})
}

func TestUpgradeInputsObjectBasic(t *testing.T) {
	t.Parallel()
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
	t.Run("same", func(t *testing.T) {
		runUpgradeStateInputCheck(t, upgradeStateTestCase{
			Resource:    res,
			Config1:     configVal("val"),
			Config2:     configVal("val"),
			ExpectEqual: true,
		})
	})

	t.Run("different", func(t *testing.T) {
		runUpgradeStateInputCheck(t, upgradeStateTestCase{
			Resource: res,
			Config1:  configVal("val1"),
			Config2:  configVal("val2"),
		})
	})
}
