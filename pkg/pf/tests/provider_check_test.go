// Copyright 2016-2023, Pulumi Corporation.
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
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	prschema "github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
	"github.com/hashicorp/terraform-plugin-mux/tf5to6server"
	sdkdiag "github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	sdkschema "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	testutils "github.com/pulumi/providertest/replay"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pb "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/internal/providerbuilder"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/proto"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tests/internal/testprovider"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tfbridge"
	tfbridge0 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
)

func TestPFCheck(t *testing.T) {
	t.Parallel()
	type testCase struct {
		name        string
		schema      schema.Schema
		replay      string
		replayMulti string

		callback tfbridge0.PreCheckCallback

		customizeResource func(*tfbridge0.ResourceInfo)
	}

	testCases := []testCase{
		{
			name: "minimal",
			schema: schema.Schema{
				Attributes: map[string]schema.Attribute{
					"id": schema.StringAttribute{Computed: true},
				},
			},
			replay: `
			{
			  "method": "/pulumirpc.ResourceProvider/Check",
			  "request": {
			    "urn": "urn:pulumi:st::pg::testprovider:index/res:Res::r",
			    "olds": {},
			    "news": {},
			    "randomSeed": "wqZZaHWVfsS1ozo3bdauTfZmjslvWcZpUjn7BzpS79c="
			  },
			  "response": {
			    "inputs": {}
			  }
			}`,
		},
		{
			name: "prop",
			schema: schema.Schema{
				Attributes: map[string]schema.Attribute{
					"id":   schema.StringAttribute{Computed: true},
					"prop": schema.StringAttribute{Optional: true},
				},
			},
			replay: `
			{
			  "method": "/pulumirpc.ResourceProvider/Check",
			  "request": {
			    "urn": "urn:pulumi:st::pg::testprovider:index/res:Res::r",
			    "olds": {},
			    "news": {"prop": "foo"},
			    "randomSeed": "wqZZaHWVfsS1ozo3bdauTfZmjslvWcZpUjn7BzpS79c="
			  },
			  "response": {
			    "inputs": {"prop": "foo"}
			  }
			}`,
		},
		{
			name: "validators",
			schema: schema.Schema{
				Attributes: map[string]schema.Attribute{
					"id": schema.StringAttribute{Computed: true},
					"prop": schema.StringAttribute{
						Optional: true,
						Validators: []validator.String{
							stringvalidator.LengthAtLeast(2),
						},
					},
				},
			},
			replay: fmt.Sprintf(`
			{
			  "method": "/pulumirpc.ResourceProvider/Check",
			  "request": {
			    "urn": "urn:pulumi:st::pg::testprovider:index/res:Res::r",
			    "olds": {},
			    "news": {"prop": "f"},
			    "randomSeed": "wqZZaHWVfsS1ozo3bdauTfZmjslvWcZpUjn7BzpS79c="
			  },
			  "response": {
                            "inputs": {"prop": "f"},
			    "failures": [{"reason": "%s"}]
			  }
			}`, "Invalid Attribute Value Length. Attribute prop string length must be "+
				"at least 2, got: 1. Examine values at 'r.prop'."),
		},
		{
			name: "missing_required_prop",
			schema: schema.Schema{
				Attributes: map[string]schema.Attribute{
					"id": schema.StringAttribute{Computed: true},
					"prop": schema.StringAttribute{
						Required: true,
					},
				},
			},
			replay: `
			{
			  "method": "/pulumirpc.ResourceProvider/Check",
			  "request": {
			    "urn": "urn:pulumi:st::pg::testprovider:index/res:Res::r",
			    "olds": {},
			    "news": {},
			    "randomSeed": "wqZZaHWVfsS1ozo3bdauTfZmjslvWcZpUjn7BzpS79c="
			  },
			  "response": {
                            "inputs": {},
                            "failures": [{"reason": "Missing required property 'prop'"}]
			  }
			}`,
		},
		{
			// Unlike CheckConfig, unrecognized values are passed through without warning so that Pulumi
			// resources can extend the protocol without triggering warnings.
			name: "unrecognized_prop_passed_through",
			schema: schema.Schema{
				Attributes: map[string]schema.Attribute{
					"id": schema.StringAttribute{Computed: true},
				},
			},
			replay: `
			{
			  "method": "/pulumirpc.ResourceProvider/Check",
			  "request": {
			    "urn": "urn:pulumi:st::pg::testprovider:index/res:Res::r",
			    "olds": {},
			    "news": {"prop": "foo"},
			    "randomSeed": "wqZZaHWVfsS1ozo3bdauTfZmjslvWcZpUjn7BzpS79c="
			  },
			  "response": {
                            "inputs": {"prop": "foo"}
			  }
			}`,
		},
		{
			name: "enforce_schema_secrets",
			schema: schema.Schema{
				Attributes: map[string]schema.Attribute{
					"id":   schema.StringAttribute{Computed: true},
					"prop": schema.StringAttribute{Optional: true, Sensitive: true},
				},
			},
			replayMulti: `
			[
			  {
			    "method": "/pulumirpc.ResourceProvider/Configure",
			    "request": {
			      "args": {
				"version": "4.8.0"
			      },
			      "acceptSecrets": true,
			      "acceptResources": true
			    },
			    "response": {
			      "supportsPreview": true,
			      "acceptResources": true,
			      "supportsAutonamingConfiguration": true
			    }
			  },
			  {
			    "method": "/pulumirpc.ResourceProvider/Check",
			    "request": {
			      "urn": "urn:pulumi:st::pg::testprovider:index/res:Res::r",
			      "olds": {},
			      "news": {
				"prop": "foo"
			      },
			      "randomSeed": "wqZZaHWVfsS1ozo3bdauTfZmjslvWcZpUjn7BzpS79c="
			    },
			    "response": {
			      "inputs": {
				"prop": {
                                  "4dabf18193072939515e22adb298388d": "1b47061264138c4ac30d75fd1eb44270",
                                  "value": "foo"
                                }
			      }
			    }
			  }
			]`,
		},
		{
			name: "callback",
			schema: schema.Schema{
				Attributes: map[string]schema.Attribute{
					"id":   schema.StringAttribute{Computed: true},
					"prop": schema.StringAttribute{Required: true},
				},
			},
			replayMulti: `
			[
			  {
			    "method": "/pulumirpc.ResourceProvider/Configure",
			    "request": {
			      "args": {
				"prop": "global"
			      },
			      "variables": {
				"prop": "global"
			      },
			      "acceptSecrets": true,
			      "acceptResources": true
			    },
			    "response": {
			      "supportsPreview": true,
			      "acceptResources": true,
			      "supportsAutonamingConfiguration": true
			    }
			  },
			  {
			    "method": "/pulumirpc.ResourceProvider/Check",
			    "request": {
			      "urn": "urn:pulumi:st::pg::testprovider:index/res:Res::r",
			      "olds": {},
			      "news": {},
			      "randomSeed": "wqZZaHWVfsS1ozo3bdauTfZmjslvWcZpUjn7BzpS79c="
			    },
			    "response": {
			      "inputs": {
				"prop": "global"
			      }
			    }
			  }
			]`,
			callback: func(
				ctx context.Context, config, meta resource.PropertyMap,
			) (resource.PropertyMap, error) {
				t.Logf("Meta: %#v", meta)
				result := config.Copy()
				result["prop"] = meta["prop"]
				urn := tfbridge0.GetUrn(ctx)
				assert.Equal(t, urn, resource.URN("urn:pulumi:st::pg::testprovider:index/res:Res::r"))
				return result, nil
			},
		},
		{
			name: "default application can consult prior state",
			schema: schema.Schema{
				Attributes: map[string]schema.Attribute{
					"id": schema.StringAttribute{Computed: true},
					"s":  schema.StringAttribute{Optional: true},
				},
			},
			customizeResource: func(info *tfbridge0.ResourceInfo) {
				info.Fields["s"] = &tfbridge0.SchemaInfo{
					Default: &tfbridge0.DefaultInfo{
						ComputeDefault: func(
							_ context.Context,
							opts tfbridge0.ComputeDefaultOptions,
						) (any, error) {
							return opts.PriorState["s"].StringValue(), nil
						},
					},
				}
			},
			replay: `
			{
			  "method": "/pulumirpc.ResourceProvider/Check",
			  "request": {
			    "urn": "urn:pulumi:st::pg::testprovider:index/res:Res::r",
			    "olds": {
                               "s": "oldString"
                            },
			    "news": {},
			    "randomSeed": "wqZZaHWVfsS1ozo3bdauTfZmjslvWcZpUjn7BzpS79c="
			  },
			  "response": {
			    "inputs": {
                              "s": "oldString"
                            }
			  }
			}`,
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			testProvider := pb.NewProvider(pb.NewProviderArgs{
				ProviderSchema: prschema.Schema{
					Attributes: map[string]prschema.Attribute{
						"prop": prschema.StringAttribute{
							Optional: true,
						},
					},
				},
				AllResources: []pb.Resource{
					pb.NewResource(pb.NewResourceArgs{
						Name:           "res",
						ResourceSchema: tc.schema,
					}),
				},
			})
			res := tfbridge0.ResourceInfo{
				Tok: "testprovider:index/res:Res",
				Docs: &tfbridge0.DocInfo{
					Markdown: []byte("OK"),
				},
				PreCheckCallback: tc.callback,
				Fields:           map[string]*tfbridge0.SchemaInfo{},
			}
			if tc.customizeResource != nil {
				tc.customizeResource(&res)
			}
			info := tfbridge0.ProviderInfo{
				Name:         "testprovider",
				P:            tfbridge.ShimProvider(testProvider),
				Version:      "0.0.1",
				MetadataInfo: &tfbridge0.MetadataInfo{},
				Resources: map[string]*tfbridge0.ResourceInfo{
					"testprovider_res": &res,
				},
			}
			s, err := newProviderServer(t, info)
			require.NoError(t, err)
			if tc.replay != "" {
				testutils.Replay(t, s, tc.replay)
			}
			if tc.replayMulti != "" {
				testutils.ReplaySequence(t, s, tc.replayMulti)
			}
		})
	}
}

func TestSDKv2PreCheckCallbackValueNotDroppedByConflictStripping(t *testing.T) {
	t.Parallel()

	sdkProv := &sdkschema.Provider{
		ResourcesMap: map[string]*sdkschema.Resource{
			"testprovider_log_group": {
				Schema: map[string]*sdkschema.Schema{
					"name": {
						Type:          sdkschema.TypeString,
						Optional:      true,
						Computed:      true,
						ForceNew:      true,
						ConflictsWith: []string{"name_prefix"},
					},
					"name_prefix": {
						Type:          sdkschema.TypeString,
						Optional:      true,
						Computed:      true,
						ForceNew:      true,
						ConflictsWith: []string{"name"},
					},
				},
				CreateContext: func(
					_ context.Context, d *sdkschema.ResourceData, _ any,
				) sdkdiag.Diagnostics {
					d.SetId("log-group-id")
					return nil
				},
				ReadContext: func(
					_ context.Context, _ *sdkschema.ResourceData, _ any,
				) sdkdiag.Diagnostics {
					return nil
				},
				DeleteContext: func(
					_ context.Context, _ *sdkschema.ResourceData, _ any,
				) sdkdiag.Diagnostics {
					return nil
				},
			},
		},
	}

	v6server, err := tf5to6server.UpgradeServer(t.Context(), func() tfprotov5.ProviderServer {
		return sdkProv.GRPCProvider()
	})
	require.NoError(t, err)

	const callbackName = "name-from-precheck-callback"

	info := tfbridge0.ProviderInfo{
		Name:         "testprovider",
		Version:      "0.0.1",
		P:            proto.New(t.Context(), v6server),
		MetadataInfo: &tfbridge0.MetadataInfo{},
		Resources: map[string]*tfbridge0.ResourceInfo{
			"testprovider_log_group": {
				Tok:  "testprovider:index/logGroup:LogGroup",
				Docs: &tfbridge0.DocInfo{Markdown: []byte("OK")},
				ComputeID: func(_ context.Context, state resource.PropertyMap) (resource.ID, error) {
					if v, ok := state["id"]; ok && v.IsString() {
						return resource.ID(v.StringValue()), nil
					}
					return "log-group-id", nil
				},
				Fields: map[string]*tfbridge0.SchemaInfo{
					// Rename the SDKv2 "id" so it doesn't collide with Pulumi's reserved id.
					"id":   {Name: "logGroupId"},
					"name": tfbridge0.AutoName("name", 50, "-"),
				},
				PreCheckCallback: func(
					_ context.Context,
					config, _ resource.PropertyMap,
				) (resource.PropertyMap, error) {
					out := config.Copy()
					out["name"] = resource.NewProperty(callbackName)
					return out, nil
				},
			},
		},
	}

	server, err := newProviderServer(t, info)
	require.NoError(t, err)

	news, err := plugin.MarshalProperties(resource.PropertyMap{
		"namePrefix": resource.NewProperty("example-"),
	}, plugin.MarshalOptions{})
	require.NoError(t, err)

	resp, err := server.Check(t.Context(), &pulumirpc.CheckRequest{
		Urn:        "urn:pulumi:st::pg::testprovider:index/logGroup:LogGroup::lg",
		News:       news,
		RandomSeed: []byte("wqZZaHWVfsS1ozo3bdauTfZmjslvWcZpUjn7BzpS79c="),
	})
	require.NoError(t, err)

	checked, err := plugin.UnmarshalProperties(resp.Inputs, plugin.MarshalOptions{KeepSecrets: true})
	require.NoError(t, err)

	name := checked["name"]
	assert.Equal(t, callbackName, name.StringValue())
}

func TestCheckWithIntID(t *testing.T) {
	t.Parallel()
	server, err := newProviderServer(t, testprovider.SyntheticTestBridgeProvider())
	require.NoError(t, err)
	testCase := `
        {
          "method": "/pulumirpc.ResourceProvider/Check",
          "request": {
            "urn": "urn:pulumi:test-stack::basicprogram::testbridge:index/intID:IntID::r1",
            "news": {
              "name": "name"
            },
            "olds": {},
            "randomSeed": "wqZZaHWVfsS1ozo3bdauTfZmjslvWcZpUjn7BzpS79c="
          },
          "response": {
            "inputs": {
              "name": "name"
            }
          }
        }
        `
	testutils.Replay(t, server, testCase)
}
