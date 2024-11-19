package tests

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tests/pulcheck"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	shimv2 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v2"
)

// TestPlanStateEdit tests that [shimv2.WithPlanStateEdit] can be used to effectively edit
// planed state.
//
// The test is set up to reproduce https://github.com/pulumi/pulumi-gcp/issues/2372.
func TestPlanStateEdit(t *testing.T) {
	t.Parallel()
	setLabelsDiff := func(_ context.Context, d *schema.ResourceDiff, _ interface{}) error {
		raw := d.Get("labels")
		if raw == nil {
			return nil
		}

		if d.Get("terraform_labels") == nil {
			return fmt.Errorf("`terraform_labels` field is not present in the resource schema")
		}

		// If "labels" field is computed, set "terraform_labels" and "effective_labels" to computed.
		// https://github.com/hashicorp/terraform-provider-google/issues/16217
		if !d.GetRawPlan().GetAttr("labels").IsWhollyKnown() {
			if err := d.SetNewComputed("terraform_labels"); err != nil {
				return fmt.Errorf("error setting terraform_labels to computed: %w", err)
			}

			return nil
		}

		// Merge provider default labels with the user defined labels in the resource to get terraform managed labels
		terraformLabels := make(map[string]string)

		labels := raw.(map[string]interface{})
		for k, v := range labels {
			terraformLabels[k] = v.(string)
		}

		if err := d.SetNew("terraform_labels", terraformLabels); err != nil {
			return fmt.Errorf("error setting new terraform_labels diff: %w", err)
		}

		return nil
	}

	const tfLabelsKey = "terraform_labels"

	fixEmptyLabels := func(ctx context.Context, req shimv2.PlanStateEditRequest) (cty.Value, error) {
		tfbridge.GetLogger(ctx).Debug("Invoked") // ctx is correctly passed and the logger is available

		assert.Equal(t, "prov_test", req.TfToken)
		assert.Equal(t, resource.PropertyMap{
			"__defaults": resource.NewProperty([]resource.PropertyValue{}),
			"labels": resource.NewProperty(resource.PropertyMap{
				"empty": resource.NewProperty(""),
				"key":   resource.NewProperty("val"),
			}),
		}, req.NewInputs)
		assert.Equal(t, resource.PropertyMap{
			"configValue": resource.NewProperty("configured"),
		}, req.ProviderConfig)

		m := req.PlanState.AsValueMap()
		effectiveLabels := m[tfLabelsKey].AsValueMap()
		effectiveLabels["empty"] = cty.StringVal("")
		m[tfLabelsKey] = cty.MapVal(effectiveLabels)
		return cty.ObjectVal(m), nil
	}

	res := &schema.Resource{
		Schema: map[string]*schema.Schema{
			"labels": {
				Type:     schema.TypeMap,
				Optional: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},
			tfLabelsKey: {
				Type:     schema.TypeMap,
				Computed: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},
		},
		CustomizeDiff: setLabelsDiff,
	}

	tfp := &schema.Provider{
		Schema: map[string]*schema.Schema{"config_value": {
			Type:     schema.TypeString,
			Optional: true,
		}},
		ResourcesMap: map[string]*schema.Resource{"prov_test": res},
	}
	bridgedProvider := pulcheck.BridgedProvider(t, "prov", tfp,
		pulcheck.WithStateEdit(fixEmptyLabels))
	program := `
name: test
runtime: yaml
resources:
  _:
    type: pulumi:providers:prov
    properties:
        configValue: "configured"
    defaultProvider: true
  mainRes:
    type: prov:index:Test
    properties:
      labels: { "key": "val", "empty": "" }
outputs:
  keyValue: ${mainRes.terraformLabels["key"]}
  emptyValue: ${mainRes.terraformLabels["empty"]}`
	pt := pulcheck.PulCheck(t, bridgedProvider, program)
	out := pt.Up(t)

	assert.Equal(t, "val", out.Outputs["keyValue"].Value)
	assert.Equal(t, "", out.Outputs["emptyValue"].Value)
}
