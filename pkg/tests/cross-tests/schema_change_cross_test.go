package crosstests

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func TestForceNewAdded(t *testing.T) {
	res1 := &schema.Resource{
		Schema: map[string]*schema.Schema{
			"foo": {
				Type:     schema.TypeString,
				Optional: true,
			},
		},
	}

	res2 := &schema.Resource{
		Schema: map[string]*schema.Schema{
			"foo": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"default_outbound_access_enabled": {
				Type:     schema.TypeBool,
				Optional: true,
				ForceNew: true,
				Default:  true,
			},
		},
	}

	configVal := func(val string) tftypes.Value {
		return tftypes.NewValue(tftypes.Object{
			AttributeTypes: map[string]tftypes.Type{
				"foo": tftypes.String,
			},
		}, map[string]tftypes.Value{
			"foo": tftypes.NewValue(tftypes.String, val),
		})
	}

	for _, PRC := range []bool{true, false} {
		t.Run(fmt.Sprintf("PRC=%v", PRC), func(t *testing.T) {
			t.Run("same", func(t *testing.T) {
				runSchemaChangeCheck(t, schemaChangeTestCase{
					Resource1: res1,
					Resource2: res2,
					Config1:   configVal("val"),
					Config2:   configVal("val"),
					DisablePlanResourceChange: true,
				})
			})

			// t.Run("different", func(t *testing.T) {
			// 	runSchemaChangeCheck(t, schemaChangeTestCase{
			// 		Resource1: res1,
			// 		Resource2: res2,
			// 		Config1:   configVal("val1"),
			// 		Config2:   configVal("val2"),
			// 		DisablePlanResourceChange: !PRC,
			// 	})
			// })
		})
	}
}
