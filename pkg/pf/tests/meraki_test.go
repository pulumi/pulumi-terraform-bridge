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
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/go-cty/cty"
	fwresource "github.com/hashicorp/terraform-plugin-framework/resource"
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
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/require"

	pb "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/internal/providerbuilder"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/internal/schemashim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
)

type NetworksAlertsSettingsRs struct {
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

func merakiNetworkAlertSettingsSchema() map[string]schema.Attribute {
	return map[string]schema.Attribute{
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
	}
}

// NetworkAlertSettings has a complex schema in the Meraki provider making it interesting to end-to-end test on.
func Test_Meraki_NetworkAlertSettings(t *testing.T) {
	t.Parallel()

	t.Skip("TODO - this us currently failing")

	networkAlertSettingsSchema := schema.Schema{
		Attributes: merakiNetworkAlertSettingsSchema(),
	}

	r := pb.NewResource(pb.NewResourceArgs{
		Name: "r",
		CreateFunc: func(ctx context.Context, req fwresource.CreateRequest, resp *fwresource.CreateResponse) {
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

			diags := resp.State.Set(ctx, &data)

			resp.Diagnostics.Append(diags...)
		},
		ResourceSchema: networkAlertSettingsSchema,
		CustomID:       true,
	})

	provider := pb.NewProvider(pb.NewProviderArgs{
		AllResources: []pb.Resource{r},
	})

	sourceYAML, err := os.ReadFile(filepath.Join("testdata", "meraki", "Pulumi.yaml"))
	require.NoError(t, err)

	programYAML := string(sourceYAML)

	test := newPulumiTestWithOpts(t, provider, programYAML, pulumiTestOpts{
		resourceInfo: &info.Resource{
			ComputeID: tfbridge.DelegateIDField("networkId", "meraki", "https://example.org"),
		},
	})

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

// Regressing an issue discovered in Meraki provider tests as a unit test. This used to fail with the following:
//
//	source error: failed recovering value for turnaround check: rawStateRecoverNatural cannot process Null values
//	as they require a type in cty.Value
func Test_rawstate_meraki(t *testing.T) {
	alertDestinations := resource.NewObjectProperty(resource.PropertyMap{
		"allAdmins":     resource.NewBoolProperty(false),
		"emails":        resource.NewNullProperty(),
		"httpServerIds": resource.NewNullProperty(),
		"snmp":          resource.NewBoolProperty(false),
	})

	alert := func(type_ string, enabled bool, filters resource.PropertyValue) resource.PropertyValue {
		return resource.NewObjectProperty(resource.PropertyMap{
			"type":              resource.NewStringProperty(type_),
			"enabled":           resource.NewBoolProperty(enabled),
			"filters":           filters,
			"alertDestinations": alertDestinations,
		})
	}

	filtersWithTimeoutValue := func(timeoutValue resource.PropertyValue) resource.PropertyValue {
		return resource.NewObjectProperty(resource.PropertyMap{
			"period":    resource.NewNullProperty(),
			"threshold": resource.NewNullProperty(),
			"timeout":   timeoutValue,
		})
	}

	filtersWithTimeout := func(n int) resource.PropertyValue {
		if n == -1 {
			return filtersWithTimeoutValue(resource.NewNullProperty())
		}
		return filtersWithTimeoutValue(resource.NewNumberProperty(60))
	}

	filtersTimeout5 := filtersWithTimeout(5)
	filtersTimeout60 := filtersWithTimeout(60)

	filtersForUsageAlert := resource.NewObjectProperty(resource.PropertyMap{
		"period":    resource.NewNumberProperty(1200),
		"threshold": resource.NewNumberProperty(1.048576e+08),
		"timeout":   resource.NewNullProperty(),
	})

	noFilters := resource.NewNullProperty()

	alertResponse := func(type_ string, enabled bool, timeout int) resource.PropertyValue {
		return resource.NewObjectProperty(resource.PropertyMap{
			"type":              resource.NewStringProperty(type_),
			"alertDestinations": alertDestinations,
			"enabled":           resource.NewBoolProperty(enabled),
			"filters":           filtersWithTimeout(timeout),
		})
	}

	alertsResponses := resource.NewArrayProperty([]resource.PropertyValue{
		alertResponse("ampMalwareDetected", true, -1),
		alertResponse("ampMalwareBlocked", false, -1),
		alertResponse("applianceDown", true, 5),
		alertResponse("failoverEvent", true, -1),
		alertResponse("dhcpNoLeases", true, -1),
		alertResponse("rogueDhcp", true, -1),
		alertResponse("ipConflict", true, -1),
		alertResponse("ip6Conflict", false, -1),
		alertResponse("dhcp6naRenumber", false, -1),
		alertResponse("dhcp6pdRenumber", false, -1),
		alertResponse("cellularUpDown", true, -1),
		alertResponse("clientConnectivity", false, -1),
		alertResponse("vrrp", true, -1),
		alertResponse("vpnConnectivityChange", true, -1),
		alertResponse("settingsChanged", true, -1),
		alertResponse("usageAlert", false, -1),
		alertResponse("weeklyUmbrella", false, -1),
		alertResponse("prefixStarvation", false, -1),
		alertResponse("cellularGatewayDown", true, 5),
		alertResponse("portDown", true, 5),
		alertResponse("powerSupplyDown", true, -1),
		alertResponse("rpsBackup", true, -1),
		alertResponse("udldError", true, -1),
		alertResponse("portError", true, -1),
		alertResponse("portSpeed", true, -1),
		alertResponse("newDhcpServer", true, -1),
		alertResponse("switchDown", true, 5),
		alertResponse("gatewayDown", true, 5),
		alertResponse("gatewayToRepeater", true, -1),
		alertResponse("repeaterDown", false, 60),
		alertResponse("rogueAp", true, -1),
		alertResponse("snr", false, -1),
		alertResponse("highWirelessUsage", false, -1),
		alertResponse("onboarding", false, -1),
		alertResponse("weeklyPresence", false, -1),
		alertResponse("uplinkIp6Conflict", false, -1),
	})

	alerts := resource.NewArrayProperty([]resource.PropertyValue{
		alert("usageAlert", false, filtersForUsageAlert),
		alert("repeaterDown", false, filtersTimeout60),
		alert("ampMalwareBlocked", false, noFilters),
		alert("clientConnectivity", false, noFilters),
		alert("dhcp6naRenumber", false, noFilters),
		alert("dhcp6pdRenumber", false, noFilters),
		alert("highWirelessUsage", false, noFilters),
		alert("ip6Conflict", false, noFilters),
		alert("onboarding", false, noFilters),
		alert("prefixStarvation", false, noFilters),
		alert("snr", false, noFilters),
		alert("uplinkIp6Conflict", false, noFilters),
		alert("weeklyPresence", false, noFilters),
		alert("weeklyUmbrella", false, noFilters),
		alert("applianceDown", true, filtersTimeout5),
		alert("cellularGatewayDown", true, filtersTimeout5),
		alert("gatewayDown", true, filtersTimeout5),
		alert("portDown", true, filtersTimeout5),
		alert("switchDown", true, filtersTimeout5),
		alert("ampMalwareDetected", true, noFilters),
		alert("cellularUpDown", true, noFilters),
		alert("dhcpNoLeases", true, noFilters),
		alert("failoverEvent", true, noFilters),
		alert("gatewayToRepeater", true, noFilters),
		alert("ipConflict", true, noFilters),
		alert("newDhcpServer", true, noFilters),
		alert("portError", true, noFilters),
		alert("portSpeed", true, noFilters),
		alert("powerSupplyDown", true, noFilters),
		alert("rogueAp", true, noFilters),
		alert("rogueDhcp", true, noFilters),
		alert("rpsBackup", true, noFilters),
		alert("settingsChanged", true, noFilters),
		alert("switchCriticalTemperature", true, noFilters),
		alert("udldError", true, noFilters),
		alert("vpnConnectivityChange", true, noFilters),
		alert("vrrp", true, noFilters),
	})

	pm := resource.PropertyMap{
		"alerts":          alerts,
		"alertsResponses": alertsResponses,
		"defaultDestinations": resource.NewObjectProperty(resource.PropertyMap{
			"allAdmins":     resource.NewBoolProperty(true),
			"emails":        resource.NewNullProperty(),
			"httpServerIds": resource.NewNullProperty(),
			"snmp":          resource.NewBoolProperty(false),
		}),
		"muting": resource.NewObjectProperty(resource.PropertyMap{
			"byPortSchedules": resource.NewObjectProperty(resource.PropertyMap{
				"enabled": resource.NewBoolProperty(true),
			}),
		}),
		"networkId": resource.NewStringProperty("L_686235993220629487"),
	}

	rawState := cty.ObjectVal(map[string]cty.Value{
		"alerts": cty.SetVal([]cty.Value{
			cty.ObjectVal(map[string]cty.Value{
				"alert_destinations": cty.ObjectVal(map[string]cty.Value{
					"all_admins":      cty.False,
					"emails":          cty.NullVal(cty.Set(cty.String)),
					"http_server_ids": cty.NullVal(cty.Set(cty.String)),
					"snmp":            cty.False,
				}),
				"enabled": cty.False,
				"filters": cty.ObjectVal(map[string]cty.Value{
					"period":    cty.NumberIntVal(1200),
					"threshold": cty.NumberIntVal(1.048576e+08),
					"timeout":   cty.NullVal(cty.Number),
				}),
				"type": cty.StringVal("usageAlert"),
			}), cty.ObjectVal(map[string]cty.Value{
				"alert_destinations": cty.ObjectVal(map[string]cty.Value{
					"all_admins":      cty.False,
					"emails":          cty.NullVal(cty.Set(cty.String)),
					"http_server_ids": cty.NullVal(cty.Set(cty.String)),
					"snmp":            cty.False,
				}),
				"enabled": cty.False,
				"filters": cty.ObjectVal(map[string]cty.Value{
					"period":    cty.NullVal(cty.Number),
					"threshold": cty.NullVal(cty.Number),
					"timeout":   cty.NumberIntVal(60),
				}),
				"type": cty.StringVal("repeaterDown"),
			}), cty.ObjectVal(map[string]cty.Value{
				"alert_destinations": cty.ObjectVal(map[string]cty.Value{
					"all_admins":      cty.False,
					"emails":          cty.NullVal(cty.Set(cty.String)),
					"http_server_ids": cty.NullVal(cty.Set(cty.String)),
					"snmp":            cty.False,
				}),
				"enabled": cty.False,
				"filters": cty.NullVal(cty.Object(map[string]cty.Type{
					"period":    cty.Number,
					"threshold": cty.Number,
					"timeout":   cty.Number,
				})),
				"type": cty.StringVal("ampMalwareBlocked"),
			}), cty.ObjectVal(map[string]cty.Value{
				"alert_destinations": cty.ObjectVal(map[string]cty.Value{
					"all_admins":      cty.False,
					"emails":          cty.NullVal(cty.Set(cty.String)),
					"http_server_ids": cty.NullVal(cty.Set(cty.String)),
					"snmp":            cty.False,
				}),
				"enabled": cty.False,
				"filters": cty.NullVal(cty.Object(map[string]cty.Type{
					"period":    cty.Number,
					"threshold": cty.Number,
					"timeout":   cty.Number,
				})),
				"type": cty.StringVal("clientConnectivity"),
			}), cty.ObjectVal(map[string]cty.Value{
				"alert_destinations": cty.ObjectVal(map[string]cty.Value{
					"all_admins":      cty.False,
					"emails":          cty.NullVal(cty.Set(cty.String)),
					"http_server_ids": cty.NullVal(cty.Set(cty.String)),
					"snmp":            cty.False,
				}),
				"enabled": cty.False,
				"filters": cty.NullVal(cty.Object(map[string]cty.Type{
					"period":    cty.Number,
					"threshold": cty.Number,
					"timeout":   cty.Number,
				})),
				"type": cty.StringVal("dhcp6naRenumber"),
			}), cty.ObjectVal(map[string]cty.Value{
				"alert_destinations": cty.ObjectVal(map[string]cty.Value{
					"all_admins":      cty.False,
					"emails":          cty.NullVal(cty.Set(cty.String)),
					"http_server_ids": cty.NullVal(cty.Set(cty.String)),
					"snmp":            cty.False,
				}),
				"enabled": cty.False,
				"filters": cty.NullVal(cty.Object(map[string]cty.Type{
					"period":    cty.Number,
					"threshold": cty.Number,
					"timeout":   cty.Number,
				})),
				"type": cty.StringVal("dhcp6pdRenumber"),
			}), cty.ObjectVal(map[string]cty.Value{
				"alert_destinations": cty.ObjectVal(map[string]cty.Value{
					"all_admins":      cty.False,
					"emails":          cty.NullVal(cty.Set(cty.String)),
					"http_server_ids": cty.NullVal(cty.Set(cty.String)),
					"snmp":            cty.False,
				}),
				"enabled": cty.False,
				"filters": cty.NullVal(cty.Object(map[string]cty.Type{
					"period":    cty.Number,
					"threshold": cty.Number,
					"timeout":   cty.Number,
				})),
				"type": cty.StringVal("highWirelessUsage"),
			}), cty.ObjectVal(map[string]cty.Value{
				"alert_destinations": cty.ObjectVal(map[string]cty.Value{
					"all_admins":      cty.False,
					"emails":          cty.NullVal(cty.Set(cty.String)),
					"http_server_ids": cty.NullVal(cty.Set(cty.String)),
					"snmp":            cty.False,
				}),
				"enabled": cty.False,
				"filters": cty.NullVal(cty.Object(map[string]cty.Type{
					"period":    cty.Number,
					"threshold": cty.Number,
					"timeout":   cty.Number,
				})),
				"type": cty.StringVal("ip6Conflict"),
			}), cty.ObjectVal(map[string]cty.Value{
				"alert_destinations": cty.ObjectVal(map[string]cty.Value{
					"all_admins":      cty.False,
					"emails":          cty.NullVal(cty.Set(cty.String)),
					"http_server_ids": cty.NullVal(cty.Set(cty.String)),
					"snmp":            cty.False,
				}),
				"enabled": cty.False,
				"filters": cty.NullVal(cty.Object(map[string]cty.Type{
					"period":    cty.Number,
					"threshold": cty.Number,
					"timeout":   cty.Number,
				})),
				"type": cty.StringVal("onboarding"),
			}), cty.ObjectVal(map[string]cty.Value{
				"alert_destinations": cty.ObjectVal(map[string]cty.Value{
					"all_admins":      cty.False,
					"emails":          cty.NullVal(cty.Set(cty.String)),
					"http_server_ids": cty.NullVal(cty.Set(cty.String)),
					"snmp":            cty.False,
				}),
				"enabled": cty.False,
				"filters": cty.NullVal(cty.Object(map[string]cty.Type{
					"period":    cty.Number,
					"threshold": cty.Number,
					"timeout":   cty.Number,
				})),
				"type": cty.StringVal("prefixStarvation"),
			}), cty.ObjectVal(map[string]cty.Value{
				"alert_destinations": cty.ObjectVal(map[string]cty.Value{
					"all_admins":      cty.False,
					"emails":          cty.NullVal(cty.Set(cty.String)),
					"http_server_ids": cty.NullVal(cty.Set(cty.String)),
					"snmp":            cty.False,
				}),
				"enabled": cty.False,
				"filters": cty.NullVal(cty.Object(map[string]cty.Type{
					"period":    cty.Number,
					"threshold": cty.Number,
					"timeout":   cty.Number,
				})),
				"type": cty.StringVal("snr"),
			}), cty.ObjectVal(map[string]cty.Value{
				"alert_destinations": cty.ObjectVal(map[string]cty.Value{
					"all_admins":      cty.False,
					"emails":          cty.NullVal(cty.Set(cty.String)),
					"http_server_ids": cty.NullVal(cty.Set(cty.String)),
					"snmp":            cty.False,
				}),
				"enabled": cty.False,
				"filters": cty.NullVal(cty.Object(map[string]cty.Type{
					"period":    cty.Number,
					"threshold": cty.Number,
					"timeout":   cty.Number,
				})),
				"type": cty.StringVal("uplinkIp6Conflict"),
			}), cty.ObjectVal(map[string]cty.Value{
				"alert_destinations": cty.ObjectVal(map[string]cty.Value{
					"all_admins":      cty.False,
					"emails":          cty.NullVal(cty.Set(cty.String)),
					"http_server_ids": cty.NullVal(cty.Set(cty.String)),
					"snmp":            cty.False,
				}),
				"enabled": cty.False,
				"filters": cty.NullVal(cty.Object(map[string]cty.Type{
					"period":    cty.Number,
					"threshold": cty.Number,
					"timeout":   cty.Number,
				})),
				"type": cty.StringVal("weeklyPresence"),
			}), cty.ObjectVal(map[string]cty.Value{
				"alert_destinations": cty.ObjectVal(map[string]cty.Value{
					"all_admins":      cty.False,
					"emails":          cty.NullVal(cty.Set(cty.String)),
					"http_server_ids": cty.NullVal(cty.Set(cty.String)),
					"snmp":            cty.False,
				}),
				"enabled": cty.False,
				"filters": cty.NullVal(cty.Object(map[string]cty.Type{
					"period":    cty.Number,
					"threshold": cty.Number,
					"timeout":   cty.Number,
				})),
				"type": cty.StringVal("weeklyUmbrella"),
			}), cty.ObjectVal(map[string]cty.Value{
				"alert_destinations": cty.ObjectVal(map[string]cty.Value{
					"all_admins":      cty.False,
					"emails":          cty.NullVal(cty.Set(cty.String)),
					"http_server_ids": cty.NullVal(cty.Set(cty.String)),
					"snmp":            cty.False,
				}),
				"enabled": cty.True,
				"filters": cty.ObjectVal(map[string]cty.Value{
					"period":    cty.NullVal(cty.Number),
					"threshold": cty.NullVal(cty.Number),
					"timeout":   cty.NumberIntVal(5),
				}),
				"type": cty.StringVal("applianceDown"),
			}), cty.ObjectVal(map[string]cty.Value{
				"alert_destinations": cty.ObjectVal(map[string]cty.Value{
					"all_admins":      cty.False,
					"emails":          cty.NullVal(cty.Set(cty.String)),
					"http_server_ids": cty.NullVal(cty.Set(cty.String)),
					"snmp":            cty.False,
				}),
				"enabled": cty.True,
				"filters": cty.ObjectVal(map[string]cty.Value{
					"period":    cty.NullVal(cty.Number),
					"threshold": cty.NullVal(cty.Number),
					"timeout":   cty.NumberIntVal(5),
				}),
				"type": cty.StringVal("cellularGatewayDown"),
			}), cty.ObjectVal(map[string]cty.Value{
				"alert_destinations": cty.ObjectVal(map[string]cty.Value{
					"all_admins":      cty.False,
					"emails":          cty.NullVal(cty.Set(cty.String)),
					"http_server_ids": cty.NullVal(cty.Set(cty.String)),
					"snmp":            cty.False,
				}),
				"enabled": cty.True,
				"filters": cty.ObjectVal(map[string]cty.Value{
					"period":    cty.NullVal(cty.Number),
					"threshold": cty.NullVal(cty.Number),
					"timeout":   cty.NumberIntVal(5),
				}),
				"type": cty.StringVal("gatewayDown"),
			}), cty.ObjectVal(map[string]cty.Value{
				"alert_destinations": cty.ObjectVal(map[string]cty.Value{
					"all_admins":      cty.False,
					"emails":          cty.NullVal(cty.Set(cty.String)),
					"http_server_ids": cty.NullVal(cty.Set(cty.String)),
					"snmp":            cty.False,
				}),
				"enabled": cty.True,
				"filters": cty.ObjectVal(map[string]cty.Value{
					"period":    cty.NullVal(cty.Number),
					"threshold": cty.NullVal(cty.Number),
					"timeout":   cty.NumberIntVal(5),
				}),
				"type": cty.StringVal("portDown"),
			}), cty.ObjectVal(map[string]cty.Value{
				"alert_destinations": cty.ObjectVal(map[string]cty.Value{
					"all_admins":      cty.False,
					"emails":          cty.NullVal(cty.Set(cty.String)),
					"http_server_ids": cty.NullVal(cty.Set(cty.String)),
					"snmp":            cty.False,
				}),
				"enabled": cty.True,
				"filters": cty.ObjectVal(map[string]cty.Value{
					"period":    cty.NullVal(cty.Number),
					"threshold": cty.NullVal(cty.Number),
					"timeout":   cty.NumberIntVal(5),
				}),
				"type": cty.StringVal("switchDown"),
			}), cty.ObjectVal(map[string]cty.Value{
				"alert_destinations": cty.ObjectVal(map[string]cty.Value{
					"all_admins":      cty.False,
					"emails":          cty.NullVal(cty.Set(cty.String)),
					"http_server_ids": cty.NullVal(cty.Set(cty.String)),
					"snmp":            cty.False,
				}),
				"enabled": cty.True,
				"filters": cty.NullVal(cty.Object(map[string]cty.Type{
					"period":    cty.Number,
					"threshold": cty.Number,
					"timeout":   cty.Number,
				})),
				"type": cty.StringVal("ampMalwareDetected"),
			}), cty.ObjectVal(map[string]cty.Value{
				"alert_destinations": cty.ObjectVal(map[string]cty.Value{
					"all_admins":      cty.False,
					"emails":          cty.NullVal(cty.Set(cty.String)),
					"http_server_ids": cty.NullVal(cty.Set(cty.String)),
					"snmp":            cty.False,
				}),
				"enabled": cty.True,
				"filters": cty.NullVal(cty.Object(map[string]cty.Type{
					"period":    cty.Number,
					"threshold": cty.Number,
					"timeout":   cty.Number,
				})),
				"type": cty.StringVal("cellularUpDown"),
			}), cty.ObjectVal(map[string]cty.Value{
				"alert_destinations": cty.ObjectVal(map[string]cty.Value{
					"all_admins":      cty.False,
					"emails":          cty.NullVal(cty.Set(cty.String)),
					"http_server_ids": cty.NullVal(cty.Set(cty.String)),
					"snmp":            cty.False,
				}),
				"enabled": cty.True,
				"filters": cty.NullVal(cty.Object(map[string]cty.Type{
					"period":    cty.Number,
					"threshold": cty.Number,
					"timeout":   cty.Number,
				})),
				"type": cty.StringVal("dhcpNoLeases"),
			}), cty.ObjectVal(map[string]cty.Value{
				"alert_destinations": cty.ObjectVal(map[string]cty.Value{
					"all_admins":      cty.False,
					"emails":          cty.NullVal(cty.Set(cty.String)),
					"http_server_ids": cty.NullVal(cty.Set(cty.String)),
					"snmp":            cty.False,
				}),
				"enabled": cty.True,
				"filters": cty.NullVal(cty.Object(map[string]cty.Type{
					"period":    cty.Number,
					"threshold": cty.Number,
					"timeout":   cty.Number,
				})),
				"type": cty.StringVal("failoverEvent"),
			}), cty.ObjectVal(map[string]cty.Value{
				"alert_destinations": cty.ObjectVal(map[string]cty.Value{
					"all_admins":      cty.False,
					"emails":          cty.NullVal(cty.Set(cty.String)),
					"http_server_ids": cty.NullVal(cty.Set(cty.String)),
					"snmp":            cty.False,
				}),
				"enabled": cty.True,
				"filters": cty.NullVal(cty.Object(map[string]cty.Type{
					"period":    cty.Number,
					"threshold": cty.Number,
					"timeout":   cty.Number,
				})),
				"type": cty.StringVal("gatewayToRepeater"),
			}), cty.ObjectVal(map[string]cty.Value{
				"alert_destinations": cty.ObjectVal(map[string]cty.Value{
					"all_admins":      cty.False,
					"emails":          cty.NullVal(cty.Set(cty.String)),
					"http_server_ids": cty.NullVal(cty.Set(cty.String)),
					"snmp":            cty.False,
				}),
				"enabled": cty.True,
				"filters": cty.NullVal(cty.Object(map[string]cty.Type{
					"period":    cty.Number,
					"threshold": cty.Number,
					"timeout":   cty.Number,
				})),
				"type": cty.StringVal("ipConflict"),
			}), cty.ObjectVal(map[string]cty.Value{
				"alert_destinations": cty.ObjectVal(map[string]cty.Value{
					"all_admins":      cty.False,
					"emails":          cty.NullVal(cty.Set(cty.String)),
					"http_server_ids": cty.NullVal(cty.Set(cty.String)),
					"snmp":            cty.False,
				}),
				"enabled": cty.True,
				"filters": cty.NullVal(cty.Object(map[string]cty.Type{
					"period":    cty.Number,
					"threshold": cty.Number,
					"timeout":   cty.Number,
				})),
				"type": cty.StringVal("newDhcpServer"),
			}), cty.ObjectVal(map[string]cty.Value{
				"alert_destinations": cty.ObjectVal(map[string]cty.Value{
					"all_admins":      cty.False,
					"emails":          cty.NullVal(cty.Set(cty.String)),
					"http_server_ids": cty.NullVal(cty.Set(cty.String)),
					"snmp":            cty.False,
				}),
				"enabled": cty.True,
				"filters": cty.NullVal(cty.Object(map[string]cty.Type{
					"period":    cty.Number,
					"threshold": cty.Number,
					"timeout":   cty.Number,
				})),
				"type": cty.StringVal("portError"),
			}), cty.ObjectVal(map[string]cty.Value{
				"alert_destinations": cty.ObjectVal(map[string]cty.Value{
					"all_admins":      cty.False,
					"emails":          cty.NullVal(cty.Set(cty.String)),
					"http_server_ids": cty.NullVal(cty.Set(cty.String)),
					"snmp":            cty.False,
				}),
				"enabled": cty.True,
				"filters": cty.NullVal(cty.Object(map[string]cty.Type{
					"period":    cty.Number,
					"threshold": cty.Number,
					"timeout":   cty.Number,
				})),
				"type": cty.StringVal("portSpeed"),
			}), cty.ObjectVal(map[string]cty.Value{
				"alert_destinations": cty.ObjectVal(map[string]cty.Value{
					"all_admins":      cty.False,
					"emails":          cty.NullVal(cty.Set(cty.String)),
					"http_server_ids": cty.NullVal(cty.Set(cty.String)),
					"snmp":            cty.False,
				}),
				"enabled": cty.True,
				"filters": cty.NullVal(cty.Object(map[string]cty.Type{
					"period":    cty.Number,
					"threshold": cty.Number,
					"timeout":   cty.Number,
				})),
				"type": cty.StringVal("powerSupplyDown"),
			}), cty.ObjectVal(map[string]cty.Value{
				"alert_destinations": cty.ObjectVal(map[string]cty.Value{
					"all_admins":      cty.False,
					"emails":          cty.NullVal(cty.Set(cty.String)),
					"http_server_ids": cty.NullVal(cty.Set(cty.String)),
					"snmp":            cty.False,
				}),
				"enabled": cty.True,
				"filters": cty.NullVal(cty.Object(map[string]cty.Type{
					"period":    cty.Number,
					"threshold": cty.Number,
					"timeout":   cty.Number,
				})),
				"type": cty.StringVal("rogueAp"),
			}), cty.ObjectVal(map[string]cty.Value{
				"alert_destinations": cty.ObjectVal(map[string]cty.Value{
					"all_admins":      cty.False,
					"emails":          cty.NullVal(cty.Set(cty.String)),
					"http_server_ids": cty.NullVal(cty.Set(cty.String)),
					"snmp":            cty.False,
				}),
				"enabled": cty.True,
				"filters": cty.NullVal(cty.Object(map[string]cty.Type{
					"period":    cty.Number,
					"threshold": cty.Number,
					"timeout":   cty.Number,
				})),
				"type": cty.StringVal("rogueDhcp"),
			}), cty.ObjectVal(map[string]cty.Value{
				"alert_destinations": cty.ObjectVal(map[string]cty.Value{
					"all_admins":      cty.False,
					"emails":          cty.NullVal(cty.Set(cty.String)),
					"http_server_ids": cty.NullVal(cty.Set(cty.String)),
					"snmp":            cty.False,
				}),
				"enabled": cty.True,
				"filters": cty.NullVal(cty.Object(map[string]cty.Type{
					"period":    cty.Number,
					"threshold": cty.Number,
					"timeout":   cty.Number,
				})),
				"type": cty.StringVal("rpsBackup"),
			}), cty.ObjectVal(map[string]cty.Value{
				"alert_destinations": cty.ObjectVal(map[string]cty.Value{
					"all_admins":      cty.False,
					"emails":          cty.NullVal(cty.Set(cty.String)),
					"http_server_ids": cty.NullVal(cty.Set(cty.String)),
					"snmp":            cty.False,
				}),
				"enabled": cty.True,
				"filters": cty.NullVal(cty.Object(map[string]cty.Type{
					"period":    cty.Number,
					"threshold": cty.Number,
					"timeout":   cty.Number,
				})),
				"type": cty.StringVal("settingsChanged"),
			}), cty.ObjectVal(map[string]cty.Value{
				"alert_destinations": cty.ObjectVal(map[string]cty.Value{
					"all_admins":      cty.False,
					"emails":          cty.NullVal(cty.Set(cty.String)),
					"http_server_ids": cty.NullVal(cty.Set(cty.String)),
					"snmp":            cty.False,
				}),
				"enabled": cty.True,
				"filters": cty.NullVal(cty.Object(map[string]cty.Type{
					"period":    cty.Number,
					"threshold": cty.Number,
					"timeout":   cty.Number,
				})),
				"type": cty.StringVal("switchCriticalTemperature"),
			}), cty.ObjectVal(map[string]cty.Value{
				"alert_destinations": cty.ObjectVal(map[string]cty.Value{
					"all_admins":      cty.False,
					"emails":          cty.NullVal(cty.Set(cty.String)),
					"http_server_ids": cty.NullVal(cty.Set(cty.String)),
					"snmp":            cty.False,
				}),
				"enabled": cty.True,
				"filters": cty.NullVal(cty.Object(map[string]cty.Type{
					"period":    cty.Number,
					"threshold": cty.Number,
					"timeout":   cty.Number,
				})),
				"type": cty.StringVal("udldError"),
			}), cty.ObjectVal(map[string]cty.Value{
				"alert_destinations": cty.ObjectVal(map[string]cty.Value{
					"all_admins":      cty.False,
					"emails":          cty.NullVal(cty.Set(cty.String)),
					"http_server_ids": cty.NullVal(cty.Set(cty.String)),
					"snmp":            cty.False,
				}),
				"enabled": cty.True,
				"filters": cty.NullVal(cty.Object(map[string]cty.Type{
					"period":    cty.Number,
					"threshold": cty.Number,
					"timeout":   cty.Number,
				})),
				"type": cty.StringVal("vpnConnectivityChange"),
			}), cty.ObjectVal(map[string]cty.Value{
				"alert_destinations": cty.ObjectVal(map[string]cty.Value{
					"all_admins":      cty.False,
					"emails":          cty.NullVal(cty.Set(cty.String)),
					"http_server_ids": cty.NullVal(cty.Set(cty.String)),
					"snmp":            cty.False,
				}),
				"enabled": cty.True,
				"filters": cty.NullVal(cty.Object(map[string]cty.Type{
					"period":    cty.Number,
					"threshold": cty.Number,
					"timeout":   cty.Number,
				})),
				"type": cty.StringVal("vrrp"),
			}),
		}),
		"alerts_response": cty.SetVal([]cty.Value{
			cty.ObjectVal(map[string]cty.Value{
				"alert_destinations": cty.ObjectVal(map[string]cty.Value{
					"all_admins":      cty.False,
					"emails":          cty.NullVal(cty.Set(cty.String)),
					"http_server_ids": cty.NullVal(cty.Set(cty.String)),
					"snmp":            cty.False,
				}),
				"enabled": cty.False,
				"filters": cty.ObjectVal(map[string]cty.Value{
					"period":    cty.NullVal(cty.Number),
					"threshold": cty.NullVal(cty.Number),
					"timeout":   cty.NumberIntVal(60),
				}),
				"type": cty.StringVal("repeaterDown"),
			}), cty.ObjectVal(map[string]cty.Value{
				"alert_destinations": cty.ObjectVal(map[string]cty.Value{
					"all_admins":      cty.False,
					"emails":          cty.NullVal(cty.Set(cty.String)),
					"http_server_ids": cty.NullVal(cty.Set(cty.String)),
					"snmp":            cty.False,
				}),
				"enabled": cty.False,
				"filters": cty.ObjectVal(map[string]cty.Value{
					"period":    cty.NullVal(cty.Number),
					"threshold": cty.NullVal(cty.Number),
					"timeout":   cty.NullVal(cty.Number),
				}),
				"type": cty.StringVal("ampMalwareBlocked"),
			}), cty.ObjectVal(map[string]cty.Value{
				"alert_destinations": cty.ObjectVal(map[string]cty.Value{
					"all_admins":      cty.False,
					"emails":          cty.NullVal(cty.Set(cty.String)),
					"http_server_ids": cty.NullVal(cty.Set(cty.String)),
					"snmp":            cty.False,
				}),
				"enabled": cty.False,
				"filters": cty.ObjectVal(map[string]cty.Value{
					"period":    cty.NullVal(cty.Number),
					"threshold": cty.NullVal(cty.Number),
					"timeout":   cty.NullVal(cty.Number),
				}),
				"type": cty.StringVal("clientConnectivity"),
			}), cty.ObjectVal(map[string]cty.Value{
				"alert_destinations": cty.ObjectVal(map[string]cty.Value{
					"all_admins":      cty.False,
					"emails":          cty.NullVal(cty.Set(cty.String)),
					"http_server_ids": cty.NullVal(cty.Set(cty.String)),
					"snmp":            cty.False,
				}),
				"enabled": cty.False,
				"filters": cty.ObjectVal(map[string]cty.Value{
					"period":    cty.NullVal(cty.Number),
					"threshold": cty.NullVal(cty.Number),
					"timeout":   cty.NullVal(cty.Number),
				}),
				"type": cty.StringVal("dhcp6naRenumber"),
			}), cty.ObjectVal(map[string]cty.Value{
				"alert_destinations": cty.ObjectVal(map[string]cty.Value{
					"all_admins":      cty.False,
					"emails":          cty.NullVal(cty.Set(cty.String)),
					"http_server_ids": cty.NullVal(cty.Set(cty.String)),
					"snmp":            cty.False,
				}),
				"enabled": cty.False,
				"filters": cty.ObjectVal(map[string]cty.Value{
					"period":    cty.NullVal(cty.Number),
					"threshold": cty.NullVal(cty.Number),
					"timeout":   cty.NullVal(cty.Number),
				}),
				"type": cty.StringVal("dhcp6pdRenumber"),
			}), cty.ObjectVal(map[string]cty.Value{
				"alert_destinations": cty.ObjectVal(map[string]cty.Value{
					"all_admins":      cty.False,
					"emails":          cty.NullVal(cty.Set(cty.String)),
					"http_server_ids": cty.NullVal(cty.Set(cty.String)),
					"snmp":            cty.False,
				}),
				"enabled": cty.False,
				"filters": cty.ObjectVal(map[string]cty.Value{
					"period":    cty.NullVal(cty.Number),
					"threshold": cty.NullVal(cty.Number),
					"timeout":   cty.NullVal(cty.Number),
				}),
				"type": cty.StringVal("highWirelessUsage"),
			}), cty.ObjectVal(map[string]cty.Value{
				"alert_destinations": cty.ObjectVal(map[string]cty.Value{
					"all_admins":      cty.False,
					"emails":          cty.NullVal(cty.Set(cty.String)),
					"http_server_ids": cty.NullVal(cty.Set(cty.String)),
					"snmp":            cty.False,
				}),
				"enabled": cty.False,
				"filters": cty.ObjectVal(map[string]cty.Value{
					"period":    cty.NullVal(cty.Number),
					"threshold": cty.NullVal(cty.Number),
					"timeout":   cty.NullVal(cty.Number),
				}),
				"type": cty.StringVal("ip6Conflict"),
			}), cty.ObjectVal(map[string]cty.Value{
				"alert_destinations": cty.ObjectVal(map[string]cty.Value{
					"all_admins":      cty.False,
					"emails":          cty.NullVal(cty.Set(cty.String)),
					"http_server_ids": cty.NullVal(cty.Set(cty.String)),
					"snmp":            cty.False,
				}),
				"enabled": cty.False,
				"filters": cty.ObjectVal(map[string]cty.Value{
					"period":    cty.NullVal(cty.Number),
					"threshold": cty.NullVal(cty.Number),
					"timeout":   cty.NullVal(cty.Number),
				}),
				"type": cty.StringVal("onboarding"),
			}), cty.ObjectVal(map[string]cty.Value{
				"alert_destinations": cty.ObjectVal(map[string]cty.Value{
					"all_admins":      cty.False,
					"emails":          cty.NullVal(cty.Set(cty.String)),
					"http_server_ids": cty.NullVal(cty.Set(cty.String)),
					"snmp":            cty.False,
				}),
				"enabled": cty.False,
				"filters": cty.ObjectVal(map[string]cty.Value{
					"period":    cty.NullVal(cty.Number),
					"threshold": cty.NullVal(cty.Number),
					"timeout":   cty.NullVal(cty.Number),
				}),
				"type": cty.StringVal("prefixStarvation"),
			}), cty.ObjectVal(map[string]cty.Value{
				"alert_destinations": cty.ObjectVal(map[string]cty.Value{
					"all_admins":      cty.False,
					"emails":          cty.NullVal(cty.Set(cty.String)),
					"http_server_ids": cty.NullVal(cty.Set(cty.String)),
					"snmp":            cty.False,
				}),
				"enabled": cty.False,
				"filters": cty.ObjectVal(map[string]cty.Value{
					"period":    cty.NullVal(cty.Number),
					"threshold": cty.NullVal(cty.Number),
					"timeout":   cty.NullVal(cty.Number),
				}),
				"type": cty.StringVal("snr"),
			}), cty.ObjectVal(map[string]cty.Value{
				"alert_destinations": cty.ObjectVal(map[string]cty.Value{
					"all_admins":      cty.False,
					"emails":          cty.NullVal(cty.Set(cty.String)),
					"http_server_ids": cty.NullVal(cty.Set(cty.String)),
					"snmp":            cty.False,
				}),
				"enabled": cty.False,
				"filters": cty.ObjectVal(map[string]cty.Value{
					"period":    cty.NullVal(cty.Number),
					"threshold": cty.NullVal(cty.Number),
					"timeout":   cty.NullVal(cty.Number),
				}),
				"type": cty.StringVal("uplinkIp6Conflict"),
			}), cty.ObjectVal(map[string]cty.Value{
				"alert_destinations": cty.ObjectVal(map[string]cty.Value{
					"all_admins":      cty.False,
					"emails":          cty.NullVal(cty.Set(cty.String)),
					"http_server_ids": cty.NullVal(cty.Set(cty.String)),
					"snmp":            cty.False,
				}),
				"enabled": cty.False,
				"filters": cty.ObjectVal(map[string]cty.Value{
					"period":    cty.NullVal(cty.Number),
					"threshold": cty.NullVal(cty.Number),
					"timeout":   cty.NullVal(cty.Number),
				}),
				"type": cty.StringVal("usageAlert"),
			}), cty.ObjectVal(map[string]cty.Value{
				"alert_destinations": cty.ObjectVal(map[string]cty.Value{
					"all_admins":      cty.False,
					"emails":          cty.NullVal(cty.Set(cty.String)),
					"http_server_ids": cty.NullVal(cty.Set(cty.String)),
					"snmp":            cty.False,
				}),
				"enabled": cty.False,
				"filters": cty.ObjectVal(map[string]cty.Value{
					"period":    cty.NullVal(cty.Number),
					"threshold": cty.NullVal(cty.Number),
					"timeout":   cty.NullVal(cty.Number),
				}),
				"type": cty.StringVal("weeklyPresence"),
			}), cty.ObjectVal(map[string]cty.Value{
				"alert_destinations": cty.ObjectVal(map[string]cty.Value{
					"all_admins":      cty.False,
					"emails":          cty.NullVal(cty.Set(cty.String)),
					"http_server_ids": cty.NullVal(cty.Set(cty.String)),
					"snmp":            cty.False,
				}),
				"enabled": cty.False,
				"filters": cty.ObjectVal(map[string]cty.Value{
					"period":    cty.NullVal(cty.Number),
					"threshold": cty.NullVal(cty.Number),
					"timeout":   cty.NullVal(cty.Number),
				}),
				"type": cty.StringVal("weeklyUmbrella"),
			}), cty.ObjectVal(map[string]cty.Value{
				"alert_destinations": cty.ObjectVal(map[string]cty.Value{
					"all_admins":      cty.False,
					"emails":          cty.NullVal(cty.Set(cty.String)),
					"http_server_ids": cty.NullVal(cty.Set(cty.String)),
					"snmp":            cty.False,
				}),
				"enabled": cty.True,
				"filters": cty.ObjectVal(map[string]cty.Value{
					"period":    cty.NullVal(cty.Number),
					"threshold": cty.NullVal(cty.Number),
					"timeout":   cty.NumberIntVal(5),
				}),
				"type": cty.StringVal("applianceDown"),
			}), cty.ObjectVal(map[string]cty.Value{
				"alert_destinations": cty.ObjectVal(map[string]cty.Value{
					"all_admins":      cty.False,
					"emails":          cty.NullVal(cty.Set(cty.String)),
					"http_server_ids": cty.NullVal(cty.Set(cty.String)),
					"snmp":            cty.False,
				}),
				"enabled": cty.True,
				"filters": cty.ObjectVal(map[string]cty.Value{
					"period":    cty.NullVal(cty.Number),
					"threshold": cty.NullVal(cty.Number),
					"timeout":   cty.NumberIntVal(5),
				}),
				"type": cty.StringVal("cellularGatewayDown"),
			}), cty.ObjectVal(map[string]cty.Value{
				"alert_destinations": cty.ObjectVal(map[string]cty.Value{
					"all_admins":      cty.False,
					"emails":          cty.NullVal(cty.Set(cty.String)),
					"http_server_ids": cty.NullVal(cty.Set(cty.String)),
					"snmp":            cty.False,
				}),
				"enabled": cty.True,
				"filters": cty.ObjectVal(map[string]cty.Value{
					"period":    cty.NullVal(cty.Number),
					"threshold": cty.NullVal(cty.Number),
					"timeout":   cty.NumberIntVal(5),
				}),
				"type": cty.StringVal("gatewayDown"),
			}), cty.ObjectVal(map[string]cty.Value{
				"alert_destinations": cty.ObjectVal(map[string]cty.Value{
					"all_admins":      cty.False,
					"emails":          cty.NullVal(cty.Set(cty.String)),
					"http_server_ids": cty.NullVal(cty.Set(cty.String)),
					"snmp":            cty.False,
				}),
				"enabled": cty.True,
				"filters": cty.ObjectVal(map[string]cty.Value{
					"period":    cty.NullVal(cty.Number),
					"threshold": cty.NullVal(cty.Number),
					"timeout":   cty.NumberIntVal(5),
				}),
				"type": cty.StringVal("portDown"),
			}), cty.ObjectVal(map[string]cty.Value{
				"alert_destinations": cty.ObjectVal(map[string]cty.Value{
					"all_admins":      cty.False,
					"emails":          cty.NullVal(cty.Set(cty.String)),
					"http_server_ids": cty.NullVal(cty.Set(cty.String)),
					"snmp":            cty.False,
				}),
				"enabled": cty.True,
				"filters": cty.ObjectVal(map[string]cty.Value{
					"period":    cty.NullVal(cty.Number),
					"threshold": cty.NullVal(cty.Number),
					"timeout":   cty.NumberIntVal(5),
				}),
				"type": cty.StringVal("switchDown"),
			}), cty.ObjectVal(map[string]cty.Value{
				"alert_destinations": cty.ObjectVal(map[string]cty.Value{
					"all_admins":      cty.False,
					"emails":          cty.NullVal(cty.Set(cty.String)),
					"http_server_ids": cty.NullVal(cty.Set(cty.String)),
					"snmp":            cty.False,
				}),
				"enabled": cty.True,
				"filters": cty.ObjectVal(map[string]cty.Value{
					"period":    cty.NullVal(cty.Number),
					"threshold": cty.NullVal(cty.Number),
					"timeout":   cty.NullVal(cty.Number),
				}),
				"type": cty.StringVal("ampMalwareDetected"),
			}), cty.ObjectVal(map[string]cty.Value{
				"alert_destinations": cty.ObjectVal(map[string]cty.Value{
					"all_admins":      cty.False,
					"emails":          cty.NullVal(cty.Set(cty.String)),
					"http_server_ids": cty.NullVal(cty.Set(cty.String)),
					"snmp":            cty.False,
				}),
				"enabled": cty.True,
				"filters": cty.ObjectVal(map[string]cty.Value{
					"period":    cty.NullVal(cty.Number),
					"threshold": cty.NullVal(cty.Number),
					"timeout":   cty.NullVal(cty.Number),
				}),
				"type": cty.StringVal("cellularUpDown"),
			}), cty.ObjectVal(map[string]cty.Value{
				"alert_destinations": cty.ObjectVal(map[string]cty.Value{
					"all_admins":      cty.False,
					"emails":          cty.NullVal(cty.Set(cty.String)),
					"http_server_ids": cty.NullVal(cty.Set(cty.String)),
					"snmp":            cty.False,
				}),
				"enabled": cty.True,
				"filters": cty.ObjectVal(map[string]cty.Value{
					"period":    cty.NullVal(cty.Number),
					"threshold": cty.NullVal(cty.Number),
					"timeout":   cty.NullVal(cty.Number),
				}),
				"type": cty.StringVal("dhcpNoLeases"),
			}), cty.ObjectVal(map[string]cty.Value{
				"alert_destinations": cty.ObjectVal(map[string]cty.Value{
					"all_admins":      cty.False,
					"emails":          cty.NullVal(cty.Set(cty.String)),
					"http_server_ids": cty.NullVal(cty.Set(cty.String)),
					"snmp":            cty.False,
				}),
				"enabled": cty.True,
				"filters": cty.ObjectVal(map[string]cty.Value{
					"period":    cty.NullVal(cty.Number),
					"threshold": cty.NullVal(cty.Number),
					"timeout":   cty.NullVal(cty.Number),
				}),
				"type": cty.StringVal("failoverEvent"),
			}), cty.ObjectVal(map[string]cty.Value{
				"alert_destinations": cty.ObjectVal(map[string]cty.Value{
					"all_admins":      cty.False,
					"emails":          cty.NullVal(cty.Set(cty.String)),
					"http_server_ids": cty.NullVal(cty.Set(cty.String)),
					"snmp":            cty.False,
				}),
				"enabled": cty.True,
				"filters": cty.ObjectVal(map[string]cty.Value{
					"period":    cty.NullVal(cty.Number),
					"threshold": cty.NullVal(cty.Number),
					"timeout":   cty.NullVal(cty.Number),
				}),
				"type": cty.StringVal("gatewayToRepeater"),
			}), cty.ObjectVal(map[string]cty.Value{
				"alert_destinations": cty.ObjectVal(map[string]cty.Value{
					"all_admins":      cty.False,
					"emails":          cty.NullVal(cty.Set(cty.String)),
					"http_server_ids": cty.NullVal(cty.Set(cty.String)),
					"snmp":            cty.False,
				}),
				"enabled": cty.True,
				"filters": cty.ObjectVal(map[string]cty.Value{
					"period":    cty.NullVal(cty.Number),
					"threshold": cty.NullVal(cty.Number),
					"timeout":   cty.NullVal(cty.Number),
				}),
				"type": cty.StringVal("ipConflict"),
			}), cty.ObjectVal(map[string]cty.Value{
				"alert_destinations": cty.ObjectVal(map[string]cty.Value{
					"all_admins":      cty.False,
					"emails":          cty.NullVal(cty.Set(cty.String)),
					"http_server_ids": cty.NullVal(cty.Set(cty.String)),
					"snmp":            cty.False,
				}),
				"enabled": cty.True,
				"filters": cty.ObjectVal(map[string]cty.Value{
					"period":    cty.NullVal(cty.Number),
					"threshold": cty.NullVal(cty.Number),
					"timeout":   cty.NullVal(cty.Number),
				}),
				"type": cty.StringVal("newDhcpServer"),
			}), cty.ObjectVal(map[string]cty.Value{
				"alert_destinations": cty.ObjectVal(map[string]cty.Value{
					"all_admins":      cty.False,
					"emails":          cty.NullVal(cty.Set(cty.String)),
					"http_server_ids": cty.NullVal(cty.Set(cty.String)),
					"snmp":            cty.False,
				}),
				"enabled": cty.True,
				"filters": cty.ObjectVal(map[string]cty.Value{
					"period":    cty.NullVal(cty.Number),
					"threshold": cty.NullVal(cty.Number),
					"timeout":   cty.NullVal(cty.Number),
				}),
				"type": cty.StringVal("portError"),
			}), cty.ObjectVal(map[string]cty.Value{
				"alert_destinations": cty.ObjectVal(map[string]cty.Value{
					"all_admins":      cty.False,
					"emails":          cty.NullVal(cty.Set(cty.String)),
					"http_server_ids": cty.NullVal(cty.Set(cty.String)),
					"snmp":            cty.False,
				}),
				"enabled": cty.True,
				"filters": cty.ObjectVal(map[string]cty.Value{
					"period":    cty.NullVal(cty.Number),
					"threshold": cty.NullVal(cty.Number),
					"timeout":   cty.NullVal(cty.Number),
				}),
				"type": cty.StringVal("portSpeed"),
			}), cty.ObjectVal(map[string]cty.Value{
				"alert_destinations": cty.ObjectVal(map[string]cty.Value{
					"all_admins":      cty.False,
					"emails":          cty.NullVal(cty.Set(cty.String)),
					"http_server_ids": cty.NullVal(cty.Set(cty.String)),
					"snmp":            cty.False,
				}),
				"enabled": cty.True,
				"filters": cty.ObjectVal(map[string]cty.Value{
					"period":    cty.NullVal(cty.Number),
					"threshold": cty.NullVal(cty.Number),
					"timeout":   cty.NullVal(cty.Number),
				}),
				"type": cty.StringVal("powerSupplyDown"),
			}), cty.ObjectVal(map[string]cty.Value{
				"alert_destinations": cty.ObjectVal(map[string]cty.Value{
					"all_admins":      cty.False,
					"emails":          cty.NullVal(cty.Set(cty.String)),
					"http_server_ids": cty.NullVal(cty.Set(cty.String)),
					"snmp":            cty.False,
				}),
				"enabled": cty.True,
				"filters": cty.ObjectVal(map[string]cty.Value{
					"period":    cty.NullVal(cty.Number),
					"threshold": cty.NullVal(cty.Number),
					"timeout":   cty.NullVal(cty.Number),
				}),
				"type": cty.StringVal("rogueAp"),
			}), cty.ObjectVal(map[string]cty.Value{
				"alert_destinations": cty.ObjectVal(map[string]cty.Value{
					"all_admins":      cty.False,
					"emails":          cty.NullVal(cty.Set(cty.String)),
					"http_server_ids": cty.NullVal(cty.Set(cty.String)),
					"snmp":            cty.False,
				}),
				"enabled": cty.True,
				"filters": cty.ObjectVal(map[string]cty.Value{
					"period":    cty.NullVal(cty.Number),
					"threshold": cty.NullVal(cty.Number),
					"timeout":   cty.NullVal(cty.Number),
				}),
				"type": cty.StringVal("rogueDhcp"),
			}), cty.ObjectVal(map[string]cty.Value{
				"alert_destinations": cty.ObjectVal(map[string]cty.Value{
					"all_admins":      cty.False,
					"emails":          cty.NullVal(cty.Set(cty.String)),
					"http_server_ids": cty.NullVal(cty.Set(cty.String)),
					"snmp":            cty.False,
				}),
				"enabled": cty.True,
				"filters": cty.ObjectVal(map[string]cty.Value{
					"period":    cty.NullVal(cty.Number),
					"threshold": cty.NullVal(cty.Number),
					"timeout":   cty.NullVal(cty.Number),
				}),
				"type": cty.StringVal("rpsBackup"),
			}), cty.ObjectVal(map[string]cty.Value{
				"alert_destinations": cty.ObjectVal(map[string]cty.Value{
					"all_admins":      cty.False,
					"emails":          cty.NullVal(cty.Set(cty.String)),
					"http_server_ids": cty.NullVal(cty.Set(cty.String)),
					"snmp":            cty.False,
				}),
				"enabled": cty.True,
				"filters": cty.ObjectVal(map[string]cty.Value{
					"period":    cty.NullVal(cty.Number),
					"threshold": cty.NullVal(cty.Number),
					"timeout":   cty.NullVal(cty.Number),
				}),
				"type": cty.StringVal("settingsChanged"),
			}), cty.ObjectVal(map[string]cty.Value{
				"alert_destinations": cty.ObjectVal(map[string]cty.Value{
					"all_admins":      cty.False,
					"emails":          cty.NullVal(cty.Set(cty.String)),
					"http_server_ids": cty.NullVal(cty.Set(cty.String)),
					"snmp":            cty.False,
				}),
				"enabled": cty.True,
				"filters": cty.ObjectVal(map[string]cty.Value{
					"period":    cty.NullVal(cty.Number),
					"threshold": cty.NullVal(cty.Number),
					"timeout":   cty.NullVal(cty.Number),
				}),
				"type": cty.StringVal("udldError"),
			}), cty.ObjectVal(map[string]cty.Value{
				"alert_destinations": cty.ObjectVal(map[string]cty.Value{
					"all_admins":      cty.False,
					"emails":          cty.NullVal(cty.Set(cty.String)),
					"http_server_ids": cty.NullVal(cty.Set(cty.String)),
					"snmp":            cty.False,
				}),
				"enabled": cty.True,
				"filters": cty.ObjectVal(map[string]cty.Value{
					"period":    cty.NullVal(cty.Number),
					"threshold": cty.NullVal(cty.Number),
					"timeout":   cty.NullVal(cty.Number),
				}),
				"type": cty.StringVal("vpnConnectivityChange"),
			}), cty.ObjectVal(map[string]cty.Value{
				"alert_destinations": cty.ObjectVal(map[string]cty.Value{
					"all_admins":      cty.False,
					"emails":          cty.NullVal(cty.Set(cty.String)),
					"http_server_ids": cty.NullVal(cty.Set(cty.String)),
					"snmp":            cty.False,
				}),
				"enabled": cty.True,
				"filters": cty.ObjectVal(map[string]cty.Value{
					"period":    cty.NullVal(cty.Number),
					"threshold": cty.NullVal(cty.Number),
					"timeout":   cty.NullVal(cty.Number),
				}),
				"type": cty.StringVal("vrrp"),
			}),
		}),
		"default_destinations": cty.ObjectVal(map[string]cty.Value{
			"all_admins":      cty.True,
			"emails":          cty.NullVal(cty.Set(cty.String)),
			"http_server_ids": cty.NullVal(cty.Set(cty.String)),
			"snmp":            cty.False,
		}),
		"muting": cty.ObjectVal(map[string]cty.Value{
			"by_port_schedules": cty.ObjectVal(map[string]cty.Value{
				"enabled": cty.True,
			}),
		}),
		"network_id": cty.StringVal("L_686235993220629487"),
	})

	r := pb.NewResource(pb.NewResourceArgs{
		Name:           "r",
		ResourceSchema: schema.Schema{Attributes: merakiNetworkAlertSettingsSchema()},
		CustomID:       true,
	})

	provider := pb.NewProvider(pb.NewProviderArgs{
		AllResources: []pb.Resource{r},
	})

	ctx := context.Background()

	shimProvider := schemashim.ShimSchemaOnlyProvider(ctx, provider)

	schema := shimProvider.ResourcesMap().Get("testprovider_r").Schema()

	tfbridge.RawStateComputeDelta(ctx, schema, nil, pm, rawState)
}
