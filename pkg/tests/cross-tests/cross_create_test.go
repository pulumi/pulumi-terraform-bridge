package crosstests

import (
	"context"
	"testing"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/stretchr/testify/require"
)

func configModeAttrSchema() map[string]*schema.Schema {
	return map[string]*schema.Schema{
		"network_rulesets": {
			Type:       schema.TypeList,
			Optional:   true,
			MaxItems:   1,
			Computed:   true,
			ConfigMode: schema.SchemaConfigModeAttr,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"default_action": {
						Type:     schema.TypeString,
						Required: true,
					},
				},
			},
		},
	}
}

func TestConfigModeAttrNull(t *testing.T) {
	vals := make([]cty.Value, 0, 2)
	runTestCase(t, diffTestCase{
		Resource: &schema.Resource{
			Schema: configModeAttrSchema(),
			CreateContext: func(ctx context.Context, rd *schema.ResourceData, i interface{}) diag.Diagnostics {
				ruleset := rd.GetRawConfig().GetAttr("network_rulesets")
				vals = append(vals, ruleset)
				rd.SetId("newid")
				return nil
			},
		},
		Config1: map[string]any{
			"network_rulesets": nil,
		},
	})

	require.Equal(t, vals[0], vals[1])
}

func TestConfigModeAttrEmpty(t *testing.T) {
	vals := make([]cty.Value, 0, 2)
	runTestCase(t, diffTestCase{
		Resource: &schema.Resource{
			Schema: configModeAttrSchema(),
			CreateContext: func(ctx context.Context, rd *schema.ResourceData, i interface{}) diag.Diagnostics {
				ruleset := rd.GetRawConfig().GetAttr("network_rulesets")
				vals = append(vals, ruleset)
				rd.SetId("newid")
				return nil
			},
		},
		Config1: map[string]any{
			"network_rulesets": []any{},
		},
	})

	require.Equal(t, vals[0], vals[1])
}
