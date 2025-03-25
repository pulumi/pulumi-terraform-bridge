// Copyright 2016-2025, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tfbridgetests

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hexops/autogold/v2"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optpreview"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"

	pb "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/internal/providerbuilder"
)

type NetworksAlertsSettingsRs struct {
	// Workaround automatically added IDs in the test Pulumi bridge test framework.
	ID types.String `tfsdk:"id"`

	NetworkID           types.String                                                   `tfsdk:"network_id"`
	Alerts              *[]ResponseNetworksGetNetworkAlertsSettingsAlertsRs            `tfsdk:"alerts"`
	AlertsResponse      *[]ResponseNetworksGetNetworkAlertsSettingsAlertsRs            `tfsdk:"alerts_response"`
	DefaultDestinations *ResponseNetworksGetNetworkAlertsSettingsDefaultDestinationsRs `tfsdk:"default_destinations"`
	Muting              *RequestNetworksUpdateNetworkAlertsSettingsMutingRs            `tfsdk:"muting"`
}

type ResponseNetworksGetNetworkAlertsSettingsAlertsRs struct {
	AlertDestinations *ResponseNetworksGetNetworkAlertsSettingsAlertsAlertDestinationsRs `tfsdk:"alert_destinations"`
	Enabled           types.Bool                                                         `tfsdk:"enabled"`
	Filters           *ResponseNetworksGetNetworkAlertsSettingsAlertsFiltersRs           `tfsdk:"filters"`
	Type              types.String                                                       `tfsdk:"type"`
}

type ResponseNetworksGetNetworkAlertsSettingsAlertsFiltersRs struct {
	Timeout   types.Int64 `tfsdk:"timeout"`
	Threshold types.Int64 `tfsdk:"threshold"`
	Period    types.Int64 `tfsdk:"period"`
}

type ResponseNetworksGetNetworkAlertsSettingsAlertsAlertDestinationsRs struct {
	AllAdmins     types.Bool `tfsdk:"all_admins"`
	Emails        types.Set  `tfsdk:"emails"`
	HTTPServerIDs types.Set  `tfsdk:"http_server_ids"`
	SNMP          types.Bool `tfsdk:"snmp"`
}

type ResponseNetworksGetNetworkAlertsSettingsDefaultDestinationsRs struct {
	AllAdmins     types.Bool `tfsdk:"all_admins"`
	Emails        types.Set  `tfsdk:"emails"`
	HTTPServerIDs types.Set  `tfsdk:"http_server_ids"`
	SNMP          types.Bool `tfsdk:"snmp"`
}

type RequestNetworksUpdateNetworkAlertsSettingsMutingRs struct {
	ByPortSchedules *RequestNetworksUpdateNetworkAlertsSettingsMutingByPortSchedulesRs `tfsdk:"by_port_schedules"`
}

type RequestNetworksUpdateNetworkAlertsSettingsMutingByPortSchedulesRs struct {
	Enabled types.Bool `tfsdk:"enabled"`
}

// NetworkAlertSettings has a complex schema in the Meraki provider making it interesting to end-to-end test on.
func Test_Meraki_NetworkAlertSettings(t *testing.T) {
	t.Parallel()

	t.Skip("TODO - this us currently failing")

	networkAlertSettingsSchema := schema.Schema{
		Attributes: map[string]schema.Attribute{
			"alerts_response": schema.SetNestedAttribute{
				MarkdownDescription: `Alert-specific configuration for each type. Only alerts that pertain to the network can be updated.`,
				Computed:            true,
				PlanModifiers: []planmodifier.Set{
					setplanmodifier.UseStateForUnknown(),
				},
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{

						"alert_destinations": schema.SingleNestedAttribute{
							MarkdownDescription: `A hash of destinations for this specific alert`,
							Computed:            true,
							PlanModifiers: []planmodifier.Object{
								objectplanmodifier.UseStateForUnknown(),
							},
							Attributes: map[string]schema.Attribute{

								"all_admins": schema.BoolAttribute{
									MarkdownDescription: `If true, then all network admins will receive emails for this alert`,
									Computed:            true,
									PlanModifiers: []planmodifier.Bool{
										boolplanmodifier.UseStateForUnknown(),
									},
								},
								"emails": schema.SetAttribute{
									MarkdownDescription: `A list of emails that will receive information about the alert`,
									Computed:            true,
									PlanModifiers: []planmodifier.Set{
										setplanmodifier.UseStateForUnknown(),
									},

									ElementType: types.StringType,
								},
								"http_server_ids": schema.SetAttribute{
									MarkdownDescription: `A list of HTTP server IDs to send a Webhook to for this alert`,
									Computed:            true,
									PlanModifiers: []planmodifier.Set{
										setplanmodifier.UseStateForUnknown(),
									},
									Default:     setdefault.StaticValue(types.SetNull(types.StringType)),
									ElementType: types.StringType,
								},
								"snmp": schema.BoolAttribute{
									MarkdownDescription: `If true, then an SNMP trap will be sent for this alert if there is an SNMP trap server configured for this network`,
									Computed:            true,
									PlanModifiers: []planmodifier.Bool{
										boolplanmodifier.UseStateForUnknown(),
									},
								},
							},
						},
						"enabled": schema.BoolAttribute{
							MarkdownDescription: `A boolean depicting if the alert is turned on or off`,
							Computed:            true,
							PlanModifiers: []planmodifier.Bool{
								boolplanmodifier.UseStateForUnknown(),
							},
						},
						"filters": schema.SingleNestedAttribute{
							MarkdownDescription: `A hash of specific configuration data for the alert. Only filters specific to the alert will be updated.`,
							Computed:            true,
							PlanModifiers: []planmodifier.Object{
								objectplanmodifier.UseStateForUnknown(),
							},
							Attributes: map[string]schema.Attribute{

								"timeout": schema.Int64Attribute{
									Computed: true,
									PlanModifiers: []planmodifier.Int64{
										int64planmodifier.UseStateForUnknown(),
									},
								},
								"period": schema.Int64Attribute{
									Computed: true,
									PlanModifiers: []planmodifier.Int64{
										int64planmodifier.UseStateForUnknown(),
									},
								},
								"threshold": schema.Int64Attribute{
									Computed: true,
									PlanModifiers: []planmodifier.Int64{
										int64planmodifier.UseStateForUnknown(),
									},
								},
							},
						},
						"type": schema.StringAttribute{
							MarkdownDescription: `The type of alert`,
							Computed:            true,
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.UseStateForUnknown(),
							},
						},
					},
				},
			},
			"alerts": schema.SetNestedAttribute{
				MarkdownDescription: `Alert-specific configuration for each type. Only alerts that pertain to the network can be updated.`,
				Optional:            true,
				PlanModifiers: []planmodifier.Set{
					setplanmodifier.UseStateForUnknown(),
				},
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{

						"alert_destinations": schema.SingleNestedAttribute{
							MarkdownDescription: `A hash of destinations for this specific alert`,
							Optional:            true,
							PlanModifiers: []planmodifier.Object{
								objectplanmodifier.UseStateForUnknown(),
							},
							Attributes: map[string]schema.Attribute{

								"all_admins": schema.BoolAttribute{
									MarkdownDescription: `If true, then all network admins will receive emails for this alert`,
									Optional:            true,
									PlanModifiers: []planmodifier.Bool{
										boolplanmodifier.UseStateForUnknown(),
									},
								},
								"emails": schema.SetAttribute{
									MarkdownDescription: `A list of emails that will receive information about the alert`,
									Optional:            true,
									PlanModifiers: []planmodifier.Set{
										setplanmodifier.UseStateForUnknown(),
									},

									ElementType: types.StringType,
								},
								"http_server_ids": schema.SetAttribute{
									MarkdownDescription: `A list of HTTP server IDs to send a Webhook to for this alert`,
									Optional:            true,
									PlanModifiers: []planmodifier.Set{
										setplanmodifier.UseStateForUnknown(),
									},
									ElementType: types.StringType,
								},
								"snmp": schema.BoolAttribute{
									MarkdownDescription: `If true, then an SNMP trap will be sent for this alert if there is an SNMP trap server configured for this network`,
									Optional:            true,
									PlanModifiers: []planmodifier.Bool{
										boolplanmodifier.UseStateForUnknown(),
									},
								},
							},
						},
						"enabled": schema.BoolAttribute{
							MarkdownDescription: `A boolean depicting if the alert is turned on or off`,
							Optional:            true,
							PlanModifiers: []planmodifier.Bool{
								boolplanmodifier.UseStateForUnknown(),
							},
						},
						"filters": schema.SingleNestedAttribute{
							MarkdownDescription: `A hash of specific configuration data for the alert. Only filters specific to the alert will be updated.`,
							Optional:            true,
							PlanModifiers: []planmodifier.Object{
								objectplanmodifier.UseStateForUnknown(),
							},
							Attributes: map[string]schema.Attribute{

								"timeout": schema.Int64Attribute{
									Optional: true,
									PlanModifiers: []planmodifier.Int64{
										int64planmodifier.UseStateForUnknown(),
									},
								},
								"period": schema.Int64Attribute{
									Optional: true,
									PlanModifiers: []planmodifier.Int64{
										int64planmodifier.UseStateForUnknown(),
									},
								},
								"threshold": schema.Int64Attribute{
									Optional: true,
									PlanModifiers: []planmodifier.Int64{
										int64planmodifier.UseStateForUnknown(),
									},
								},
							},
						},
						"type": schema.StringAttribute{
							MarkdownDescription: `The type of alert`,
							Optional:            true,
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.UseStateForUnknown(),
							},
						},
					},
				},
			},
			"default_destinations": schema.SingleNestedAttribute{
				MarkdownDescription: `The network-wide destinations for all alerts on the network.`,
				Computed:            true,
				Optional:            true,
				PlanModifiers: []planmodifier.Object{
					objectplanmodifier.UseStateForUnknown(),
				},
				Attributes: map[string]schema.Attribute{

					"all_admins": schema.BoolAttribute{
						MarkdownDescription: `If true, then all network admins will receive emails.`,
						Computed:            true,
						Optional:            true,
						PlanModifiers: []planmodifier.Bool{
							boolplanmodifier.UseStateForUnknown(),
						},
					},
					"emails": schema.SetAttribute{
						MarkdownDescription: `A list of emails that will receive the alert(s).`,
						Computed:            true,
						Optional:            true,
						PlanModifiers: []planmodifier.Set{
							setplanmodifier.UseStateForUnknown(),
						},
						Default:     setdefault.StaticValue(types.SetNull(types.StringType)),
						ElementType: types.StringType,
					},
					"http_server_ids": schema.SetAttribute{
						MarkdownDescription: `A list of HTTP server IDs to send a Webhook to`,
						Computed:            true,
						Optional:            true,
						PlanModifiers: []planmodifier.Set{
							setplanmodifier.UseStateForUnknown(),
						},
						Default:     setdefault.StaticValue(types.SetNull(types.StringType)),
						ElementType: types.StringType,
					},
					"snmp": schema.BoolAttribute{
						MarkdownDescription: `If true, then an SNMP trap will be sent if there is an SNMP trap server configured for this network.`,
						Computed:            true,
						Optional:            true,
						PlanModifiers: []planmodifier.Bool{
							boolplanmodifier.UseStateForUnknown(),
						},
						Default: booldefault.StaticBool(false),
					},
				},
			},
			"muting": schema.SingleNestedAttribute{
				MarkdownDescription: `muting`,
				Computed:            true,
				Optional:            true,
				PlanModifiers: []planmodifier.Object{
					objectplanmodifier.UseStateForUnknown(),
				},
				Attributes: map[string]schema.Attribute{

					"by_port_schedules": schema.SingleNestedAttribute{
						MarkdownDescription: `by_port_schedules`,
						Computed:            true,
						Optional:            true,
						PlanModifiers: []planmodifier.Object{
							objectplanmodifier.UseStateForUnknown(),
						},
						Attributes: map[string]schema.Attribute{
							"enabled": schema.BoolAttribute{
								MarkdownDescription: `enabled`,
								Computed:            true,
								Optional:            true,
								PlanModifiers: []planmodifier.Bool{
									boolplanmodifier.UseStateForUnknown(),
								},
								Default: booldefault.StaticBool(false),
							},
						},
					},
				},
			},
			"network_id": schema.StringAttribute{
				MarkdownDescription: `networkId path parameter. Network ID`,
				Required:            true,
			},
		},
	}

	r := pb.NewResource(pb.NewResourceArgs{
		Name: "r",
		CreateFunc: func(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {

			// Retrieve values from plan
			var data NetworksAlertsSettingsRs

			var item types.Object
			resp.Diagnostics.Append(req.Plan.Get(ctx, &item)...)
			if resp.Diagnostics.HasError() {
				return
			}

			resp.Diagnostics.Append(item.As(ctx, &data, basetypes.ObjectAsOptions{
				UnhandledNullAsEmpty:    true,
				UnhandledUnknownAsEmpty: true,
			})...)

			if resp.Diagnostics.HasError() {
				return
			}

			data.ID = basetypes.NewStringValue("id-0")

			diags := resp.State.Set(ctx, &data)
			resp.Diagnostics.Append(diags...)

		},
		ResourceSchema: networkAlertSettingsSchema,
	})

	provider := pb.NewProvider(pb.NewProviderArgs{
		AllResources: []pb.Resource{r},
	})

	programYAML := `
        name: test-program
        runtime: yaml
        resources:
          my-res:
            type: testprovider:index:R
            properties:
              networkId: "my-network-id"
              defaultDestinations:
                snmp: false
                allAdmins: true
              muting:
                byPortSchedules:
                  enabled: true
              alerts:
                - type: applianceDown
                  alertDestinations:
                    allAdmins: false
                    snmp: false
                  enabled: true
                  filters:
                    timeout: 5
                - type: usageAlert
                  alertDestinations:
                    allAdmins: false
                    snmp: false
                  enabled: true
                  filters:
                    period: 1200
                    threshold: 104857600
                - type: dhcpNoLeases
                  alertDestinations:
                    allAdmins: false
                    snmp: false
                  enabled: true
                  filters: {}
        `

	test := newPulumiTest(t, provider, programYAML)

	previewAndUpdate := func(name, prog string, expectPreviewChanges, expectChanges autogold.Value) {
		test.WritePulumiYaml(t, prog)

		previewResult := test.Preview(t, optpreview.Diff())
		t.Logf("%s preview: %s", name, previewResult.StdOut+previewResult.StdErr)
		expectPreviewChanges.Equal(t, previewResult.ChangeSummary)

		upResult := test.Up(t)
		t.Logf("%s up: %s", name, upResult.StdOut+upResult.StdErr)
		expectChanges.Equal(t, upResult.Summary.ResourceChanges)

		t.Logf("STATE: %s", test.ExportStack(t).Deployment)
	}

	previewAndUpdate(
		"initial",
		programYAML,
		autogold.Expect(map[apitype.OpType]int{apitype.OpType("create"): 2}),
		autogold.Expect(&map[string]int{"create": 2}),
	)

	previewAndUpdate(
		"empty",
		programYAML,
		autogold.Expect(map[apitype.OpType]int{apitype.OpType("same"): 2}),
		autogold.Expect(&map[string]int{"same": 2}),
	)
}
