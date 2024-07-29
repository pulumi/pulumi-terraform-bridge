package tfbridgetests

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
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optpreview"
	"github.com/stretchr/testify/assert"
)

func TestBasic(t *testing.T) {
	provBuilder := providerbuilder.Provider{
		TypeName:       "prov",
		Version:        "0.0.1",
		ProviderSchema: pschema.Schema{},
		AllResources: []providerbuilder.Resource{
			{
				Name: "test",
				ResourceSchema: rschema.Schema{
					Attributes: map[string]rschema.Attribute{
						"s": rschema.StringAttribute{Optional: true},
					},
				},
			},
		},
	}

	prov := bridgedProvider(&provBuilder)

	program := `
name: test
runtime: yaml
resources:
    mainRes:
        type: prov:index:Test
        properties:
            s: "hello"`

	pt := pulCheck(t, prov, program)

	pt.Up()
}

func TestReproAWS3323(t *testing.T) {
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
				Name: "test",
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
					state.Policy = basetypes.NewStringValue("{\"AWSOwnedKey\":true},\"Rules\":[{\"Resource\":[\"collection/member-search*\"],\"ResourceType\":\"collection\"}]")
					resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
					return
				},
			},
		},
	}

	prov := bridgedProvider(&provBuilder)

	program := `
name: test
runtime: yaml
resources:
    mainRes:
        type: prov:index:Test
        properties:
            policy: '{"Rules":[{"Resource":["collection/member-search*"],"ResourceType":"collection"}],"AWSOwnedKey":true}'
`

	pt := pulCheck(t, prov, program)

	pt.Up()

	pt.Preview(optpreview.Diff())
	dpl, err := pt.ExportStack().Deployment.MarshalJSON()
	assert.NoError(t, err)
	t.Logf("State: %v", string(dpl))
	diffs, err := pt.GrpcLog().Diffs()
	assert.NoError(t, err)
	for _, v := range diffs {
		autogold.Expect([]string{"policy", "policyVersion"}).Equal(t, v.Response.Diffs)

		olds := v.Request.Olds.Fields
		news := v.Request.News.Fields
		autogold.Expect(`{"AWSOwnedKey":true},"Rules":[{"Resource":["collection/member-search*"],"ResourceType":"collection"}]`).Equal(t, olds["policy"].GetStringValue())
		autogold.Expect(`{"Rules":[{"Resource":["collection/member-search*"],"ResourceType":"collection"}],"AWSOwnedKey":true}`).Equal(t, news["policy"].GetStringValue())
		autogold.Expect(nil).Equal(t, olds["policyVersion"])
		autogold.Expect(nil).Equal(t, news["policyVersion"])
	}
}
