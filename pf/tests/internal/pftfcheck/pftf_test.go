package pftfcheck

import (
	"context"
	"testing"

	pschema "github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hexops/autogold/v2"
	"github.com/pulumi/pulumi-terraform-bridge/pf/tests/internal/providerbuilder"
)

func TestBasic123(t *testing.T) {
	type resourceSecurityPolicyData struct {
		ID            types.String `tfsdk:"id"`
		Policy        types.String `tfsdk:"policy"`
		PolicyVersion types.String `tfsdk:"policy_version"`
	}
	provBuilder := providerbuilder.Provider{
		TypeName:       "prov",
		Version:        "0.0.1",
		ProviderSchema: pschema.Schema{},
		AllResources: []providerbuilder.Resource{
			{
				Name: "res",
				ResourceSchema: rschema.Schema{
					Attributes: map[string]rschema.Attribute{
						"policy": rschema.StringAttribute{
							Required: true,
						},
						"policy_version": rschema.StringAttribute{
							Computed: true,
						},
					},
				},
				CreateFunc: func(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
					var plan resourceSecurityPolicyData
					req.Plan.Get(ctx, &plan)
					state := plan
					state.ID = basetypes.NewStringValue("id")
					state.PolicyVersion = basetypes.NewStringValue("MTcyMjI2MzQyMzk0N18x")
					state.Policy = basetypes.NewStringValue("{\"AWSOwnedKey\":true,\"Rules\":[{\"Resource\":[\"collection/member-search*\"],\"ResourceType\":\"collection\"}]}")
					resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
				},
			},
		},
	}

	driver := NewTfDriverPF(t, t.TempDir(), &provBuilder)

	driver.Write(t, `
resource "prov_res" "test" {
    policy = jsonencode({"Rules":[{"Resource":["collection/member-search*"],"ResourceType":"collection"}],"AWSOwnedKey":true})
}
`)

	plan := driver.Plan(t)
	driver.Apply(t, plan)
	newPlan := driver.Plan(t)
	if rawPlan, ok := newPlan.RawPlan.(map[string]interface{}); ok {
		changes := rawPlan["resource_changes"]
		autogold.Expect([]interface{}{map[string]interface{}{
			"address": "prov_res.test", "change": map[string]interface{}{
				"actions": []interface{}{"no-op"},
				"after": map[string]interface{}{
					"id":             "id",
					"policy":         `{"AWSOwnedKey":true,"Rules":[{"Resource":["collection/member-search*"],"ResourceType":"collection"}]}`,
					"policy_version": "MTcyMjI2MzQyMzk0N18x",
				},
				"after_sensitive": map[string]interface{}{},
				"after_unknown":   map[string]interface{}{},
				"before": map[string]interface{}{
					"id":             "id",
					"policy":         `{"AWSOwnedKey":true,"Rules":[{"Resource":["collection/member-search*"],"ResourceType":"collection"}]}`,
					"policy_version": "MTcyMjI2MzQyMzk0N18x",
				},
				"before_sensitive": map[string]interface{}{},
			},
			"mode":          "managed",
			"name":          "test",
			"provider_name": "registry.terraform.io/hashicorp/prov",
			"type":          "prov_res",
		}}).Equal(t, changes)
	}
}
