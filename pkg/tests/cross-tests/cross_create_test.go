package crosstests

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/stretchr/testify/require"
)

func configModeAttrSchema(configMode schema.SchemaConfigMode) map[string]*schema.Schema {
	return map[string]*schema.Schema{
		"network_rulesets": {
			Type:       schema.TypeList,
			Optional:   true,
			MaxItems:   1,
			Computed:   true,
			ConfigMode: configMode,
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

func TestConfigModeAttr(t *testing.T) {
	// nonNilRuleset := map[string]map[string]string{
	// 	"network_rulesets": {"default_action": "Deny"},
	// }
	params := []struct {
		configMode schema.SchemaConfigMode
		value      any
	}{
		{schema.SchemaConfigModeAttr, nil},
		{schema.SchemaConfigModeBlock, nil},
		{schema.SchemaConfigModeAttr, []any{}},
		{schema.SchemaConfigModeBlock, []any{}},
		// {schema.SchemaConfigModeAttr, nonNilRuleset},
		// {schema.SchemaConfigModeBlock, nonNilRuleset},
	}

	for _, param := range params {
		t.Run(fmt.Sprintf("%q/%s", param.configMode, param.value), func(t *testing.T) {
			vals := make([]cty.Value, 0, 2)
			runTestCase(t, diffTestCase{
				Resource: &schema.Resource{
					Schema: configModeAttrSchema(param.configMode),
					CreateContext: func(ctx context.Context, rd *schema.ResourceData, i interface{}) diag.Diagnostics {
						ruleset := rd.GetRawConfig().GetAttr("network_rulesets")
						vals = append(vals, ruleset)
						rd.SetId("newid")
						return nil
					},
				},
				Config2: map[string]any{
					"network_rulesets": param.value,
				},
			})

			require.Equal(t, vals[0], vals[1])
		})
	}
}
