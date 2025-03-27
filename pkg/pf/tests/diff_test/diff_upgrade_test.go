package tfbridgetests

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/zclconf/go-cty/cty"

	pb "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/internal/providerbuilder"
	crosstests "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tests/internal/cross-tests"
)

func TestPFUpgradeRenamedProp(t *testing.T) {
	// TODO[pulumi/pulumi-terraform-bridge#2960]
	t.Skip("State upgrades are broken")
	t.Parallel()

	dataV0 := tftypes.Object{
		AttributeTypes: map[string]tftypes.Type{
			"id":   tftypes.String,
			"prop": tftypes.String,
		},
	}

	dataV1 := tftypes.Object{
		AttributeTypes: map[string]tftypes.Type{
			"id":    tftypes.String,
			"prop1": tftypes.String,
		},
	}

	res1 := pb.NewResource(pb.NewResourceArgs{
		ResourceSchema: rschema.Schema{
			Attributes: map[string]rschema.Attribute{"prop": rschema.StringAttribute{Optional: true}},
		},
	})

	res2 := pb.NewResource(pb.NewResourceArgs{
		ResourceSchema: rschema.Schema{
			Attributes: map[string]rschema.Attribute{"prop1": rschema.StringAttribute{Optional: true}},
			Version:    1,
		},
		UpgradeStateFunc: func(ctx context.Context) map[int64]resource.StateUpgrader {
			return map[int64]resource.StateUpgrader{
				0: {
					StateUpgrader: func(ctx context.Context, req resource.UpgradeStateRequest, resp *resource.UpgradeStateResponse) {
						val, err := req.RawState.Unmarshal(dataV0)
						if err != nil {
							resp.Diagnostics.AddError(fmt.Sprintf("failed to unmarshal state: %v", err), "")
							return
						}

						rawState := map[string]tftypes.Value{}
						err = val.As(&rawState)
						if err != nil {
							resp.Diagnostics.AddError(fmt.Sprintf("failed to unmarshal state: %v", err), "")
							return
						}

						dynamicValue, err := tfprotov6.NewDynamicValue(
							dataV1,
							tftypes.NewValue(dataV1, map[string]tftypes.Value{
								"id":    rawState["id"],
								"prop1": rawState["prop"],
							}),
						)
						if err != nil {
							resp.Diagnostics.AddError(fmt.Sprintf("failed to create dynamic value: %v", err), "")
							return
						}

						resp.DynamicValue = &dynamicValue
					},
				},
			}
		},
	})

	crosstests.Diff(t, res1,
		map[string]cty.Value{"prop": cty.StringVal("value")},
		map[string]cty.Value{"prop1": cty.StringVal("value")},
		crosstests.DiffProviderUpgradedSchema(res2),
	)
}

func TestPFUpgradeMovedProp(t *testing.T) {
	// TODO[pulumi/pulumi-terraform-bridge#2960]
	t.Skip("State upgrades are broken")
	t.Parallel()

	dataV0 := tftypes.Object{
		AttributeTypes: map[string]tftypes.Type{
			"id":   tftypes.String,
			"prop": tftypes.String,
		},
	}

	dataV1 := tftypes.Object{
		AttributeTypes: map[string]tftypes.Type{
			"id": tftypes.String,
			"obj": tftypes.Object{
				AttributeTypes: map[string]tftypes.Type{
					"prop": tftypes.String,
				},
			},
		},
	}

	res1 := pb.NewResource(pb.NewResourceArgs{
		ResourceSchema: rschema.Schema{
			Attributes: map[string]rschema.Attribute{"prop": rschema.StringAttribute{Optional: true}},
		},
	})

	res2 := pb.NewResource(pb.NewResourceArgs{
		ResourceSchema: rschema.Schema{
			Attributes: map[string]rschema.Attribute{"obj": rschema.ObjectAttribute{
				Optional: true,
				AttributeTypes: map[string]attr.Type{
					"prop": types.StringType,
				},
			}},
			Version: 1,
		},
		UpgradeStateFunc: func(ctx context.Context) map[int64]resource.StateUpgrader {
			return map[int64]resource.StateUpgrader{
				0: {
					StateUpgrader: func(ctx context.Context, req resource.UpgradeStateRequest, resp *resource.UpgradeStateResponse) {
						val, err := req.RawState.Unmarshal(dataV0)
						if err != nil {
							resp.Diagnostics.AddError(fmt.Sprintf("failed to unmarshal state: %v", err), "")
							return
						}

						rawState := map[string]tftypes.Value{}
						err = val.As(&rawState)
						if err != nil {
							resp.Diagnostics.AddError(fmt.Sprintf("failed to unmarshal state: %v", err), "")
							return
						}

						dynamicValue, err := tfprotov6.NewDynamicValue(
							dataV1,
							tftypes.NewValue(dataV1, map[string]tftypes.Value{
								"id": rawState["id"],
								"obj": tftypes.NewValue(dataV1.AttributeTypes["obj"], map[string]tftypes.Value{
									"prop": rawState["prop"],
								}),
							}),
						)
						if err != nil {
							resp.Diagnostics.AddError(fmt.Sprintf("failed to create dynamic value: %v", err), "")
							return
						}

						resp.DynamicValue = &dynamicValue
					},
				},
			}
		},
	})

	crosstests.Diff(t, res1,
		map[string]cty.Value{"prop": cty.StringVal("value")},
		map[string]cty.Value{"obj": cty.ObjectVal(map[string]cty.Value{"prop": cty.StringVal("value")})},
		crosstests.DiffProviderUpgradedSchema(res2),
	)
}
