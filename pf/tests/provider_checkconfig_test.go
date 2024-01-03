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

	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/structpb"

	hostclient "github.com/pulumi/pulumi/pkg/v3/resource/provider"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/pulumi/pulumi-terraform-bridge/pf/tests/internal/providerbuilder"
	"github.com/pulumi/pulumi-terraform-bridge/pf/tfbridge"
	testutils "github.com/pulumi/pulumi-terraform-bridge/testing/x"
	tfbridge0 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

func TestCheckConfig(t *testing.T) {
	t.Run("minimal", func(t *testing.T) {
		schema := schema.Schema{}
		testutils.Replay(t, makeProviderServer(t, schema), `
		{
		  "method": "/pulumirpc.ResourceProvider/CheckConfig",
		  "request": {
		    "urn": "urn:pulumi:dev::teststack::pulumi:providers:testprovider::test",
		    "olds": {},
		    "news": {
		      "version": "6.54.0"
		    }
		  },
		  "response": {
		    "inputs": {
		      "version": "6.54.0"
		    }
		  }
		}`)
	})

	t.Run("config_value", func(t *testing.T) {
		schema := schema.Schema{
			Attributes: map[string]schema.Attribute{
				"config_value": schema.StringAttribute{
					Optional: true,
				},
			},
		}

		// Ensure Pulumi can configure config_value in the testprovider.
		testutils.Replay(t, makeProviderServer(t, schema), `
		{
		  "method": "/pulumirpc.ResourceProvider/CheckConfig",
		  "request": {
		    "urn": "urn:pulumi:dev::teststack::pulumi:providers:testprovider::test",
		    "olds": {},
		    "news": {
	              "configValue": "foo",
		      "version": "6.54.0"
		    }
		  },
		  "response": {
		    "inputs": {
	              "configValue": "foo",
		      "version": "6.54.0"
		    }
		  }
		}`)
	})

	t.Run("unknown_config_value", func(t *testing.T) {
		// Currently if a top-level config property is a Computed value, or it's a composite value with any
		// Computed values inside, the engine sends a sentinel string. Ensure that CheckConfig propagates the
		// same sentinel string back to the engine.

		schema := schema.Schema{
			Attributes: map[string]schema.Attribute{
				"config_value": schema.StringAttribute{
					Optional: true,
				},
				"scopes": schema.ListAttribute{
					Optional:    true,
					ElementType: types.StringType,
				},
			},
		}

		assert.Equal(t, "dd056dcd-154b-4c76-9bd3-c8f88648b5ff", plugin.UnknownObjectValue)
		assert.Equal(t, "6a19a0b0-7e62-4c92-b797-7f8e31da9cc2", plugin.UnknownArrayValue)
		testutils.Replay(t, makeProviderServer(t, schema), `
		{
		  "method": "/pulumirpc.ResourceProvider/CheckConfig",
		  "request": {
		    "urn": "urn:pulumi:dev::teststack::pulumi:providers:testprovider::test",
		    "olds": {},
		    "news": {
	              "configValue": "04da6b54-80e4-46f7-96ec-b56ff0331ba9",
	              "scopes": "04da6b54-80e4-46f7-96ec-b56ff0331ba9",
		      "version": "6.54.0"
		    }
		  },
		  "response": {
		    "inputs": {
	              "configValue": "dd056dcd-154b-4c76-9bd3-c8f88648b5ff",
	              "scopes": "6a19a0b0-7e62-4c92-b797-7f8e31da9cc2",
		      "version": "6.54.0"
		    }
		  }
		}`)
	})

	t.Run("config_changed", func(t *testing.T) {
		// In this scenario Pulumi plans an update plan when a config has changed on an existing stack.
		schema := schema.Schema{
			Attributes: map[string]schema.Attribute{
				"config_value": schema.StringAttribute{
					Optional: true,
				},
			},
		}

		testutils.Replay(t, makeProviderServer(t, schema), `
		{
		  "method": "/pulumirpc.ResourceProvider/CheckConfig",
		  "request": {
		    "urn": "urn:pulumi:dev::teststack::pulumi:providers:testprovider::test",
		    "olds": {
	              "config_value": "foo",
		      "version": "6.54.0"
	            },
		    "news": {
	              "config_value": "bar",
		      "version": "6.54.0"
		    }
		  },
		  "response": {
		    "inputs": {
	              "config_value": "bar",
		      "version": "6.54.0"
		    }
		  }
		}`)
	})

	t.Run("invalid_config_value", func(t *testing.T) {
		// Test error reporting when an unrecognized property is sent.
		schema := schema.Schema{}
		provider := makeProviderServer(t, schema)
		ctx := context.Background()
		args, err := structpb.NewStruct(map[string]interface{}{
			"requiredprop": "baz",
		})
		require.NoError(t, err)
		// Default provider.
		resp, err := provider.CheckConfig(ctx, &pulumirpc.CheckRequest{
			Urn:  "urn:pulumi:r::cloudflare-record-ts::pulumi:providers:cloudflare::default_5_2_1",
			News: args,
		})
		require.NoError(t, err)
		require.Equal(t, 1, len(resp.Failures))
		assert.Equal(t, "could not validate provider configuration: "+
			"Invalid or unknown key. Check `pulumi config get testprovider:requiredprop`.",
			resp.Failures[0].Reason)
		// Explicit provider.
		resp, err = provider.CheckConfig(ctx, &pulumirpc.CheckRequest{
			Urn:  "urn:pulumi:r::cloudflare-record-ts::pulumi:providers:cloudflare::explicitprovider",
			News: args,
		})
		require.NoError(t, err)
		require.Equal(t, 1, len(resp.Failures))
		assert.Equal(t, "could not validate provider configuration: "+
			"Invalid or unknown key. Examine values at 'explicitprovider.requiredprop'.",
			resp.Failures[0].Reason)
	})

	t.Run("levenshtein_correction", func(t *testing.T) {
		schema := schema.Schema{
			Attributes: map[string]schema.Attribute{
				"config_value": schema.StringAttribute{
					Optional: true,
				},
			},
		}
		provider := makeProviderServer(t, schema)
		ctx := context.Background()
		args, err := structpb.NewStruct(map[string]interface{}{
			"cofnigValue": "baz",
		})
		require.NoError(t, err)
		resp, err := provider.CheckConfig(ctx, &pulumirpc.CheckRequest{
			Urn:  "urn:pulumi:r::cloudflare-record-ts::pulumi:providers:cloudflare::default_5_2_1",
			News: args,
		})
		require.NoError(t, err)
		require.NoError(t, err)
		require.Equal(t, 1, len(resp.Failures))
		require.Equal(t, "could not validate provider configuration: "+
			"Invalid or unknown key. Check `pulumi config get testprovider:cofnigValue`. "+
			"Did you mean `testprovider:configValue`?",
			resp.Failures[0].Reason)

	})

	t.Run("validators", func(t *testing.T) {
		schema := schema.Schema{
			Attributes: map[string]schema.Attribute{
				"my_prop": schema.StringAttribute{
					Optional: true,
					Validators: []validator.String{
						stringvalidator.LengthAtLeast(2),
					},
				},
			},
		}
		ctx := context.Background()
		s := makeProviderServer(t, schema)
		args, err := structpb.NewStruct(map[string]any{"myProp": "s"})
		require.NoError(t, err)
		res, err := s.CheckConfig(ctx, &pulumirpc.CheckRequest{
			Urn:  "urn:pulumi:r::prov::pulumi:providers:prov::explicitprovider",
			News: args,
		})
		assert.NoError(t, err)
		require.Equal(t, 1, len(res.Failures))
		require.Equal(t, "could not validate provider configuration: Invalid Attribute Value Length. "+
			"Attribute my_prop string length must be at least 2, got: 1. "+
			"Examine values at 'explicitprovider.myProp'.", res.Failures[0].Reason)

		// default provider
		args, err = structpb.NewStruct(map[string]any{"myProp": "s"})
		require.NoError(t, err)
		res, err = s.CheckConfig(ctx, &pulumirpc.CheckRequest{
			Urn:  "urn:pulumi:r::prov::pulumi:providers:prov::default_5_2_1",
			News: args,
		})
		assert.NoError(t, err)
		require.Equal(t, 1, len(res.Failures))
		require.Equal(t, "could not validate provider configuration: Invalid Attribute Value Length. "+
			"Attribute my_prop string length must be at least 2, got: 1. "+
			"Check `pulumi config get testprovider:myProp`.", res.Failures[0].Reason)
	})

	t.Run("missing_required_config_value_default_provider", func(t *testing.T) {
		desc := "A very important required attribute"
		schema := schema.Schema{
			Attributes: map[string]schema.Attribute{
				"req_prop": schema.StringAttribute{
					Required:    true,
					Description: desc,
				},
			},
		}
		testutils.Replay(t, makeProviderServer(t, schema), fmt.Sprintf(`
		{
		  "method": "/pulumirpc.ResourceProvider/CheckConfig",
		  "request": {
		    "urn": "urn:pulumi:test1::example::pulumi:providers:prov::default_1_1_42",
		    "olds": {},
		    "news": {
		      "version": "6.54.0"
		    }
		  },
		  "response": {
	            "inputs": {
		      "version": "6.54.0"
	            },
                    "failures": [{
                       "reason": "Provider is missing a required configuration key, try %s: A very important required attribute"
                    }]
	          }
		}`, "`pulumi config set testprovider:reqProp`"))
	})

	t.Run("missing_required_config_value_explicit_provider", func(t *testing.T) {
		desc := "A very important required attribute"
		schema := schema.Schema{
			Attributes: map[string]schema.Attribute{
				"req_prop": schema.StringAttribute{
					Required:    true,
					Description: desc,
				},
			},
		}
		testutils.Replay(t, makeProviderServer(t, schema), `
		{
		  "method": "/pulumirpc.ResourceProvider/CheckConfig",
		  "request": {
		    "urn": "urn:pulumi:test1::example::pulumi:providers:prov::explicitprovider",
		    "olds": {},
		    "news": {
		      "version": "6.54.0"
		    }
		  },
		  "response": {
	            "inputs": {
		      "version": "6.54.0"
	            },
	            "failures": [{
	               "reason": "Missing required property 'reqProp': A very important required attribute"
	            }]
	          }
		}`)
	})

	t.Run("flattened_compound_values", func(t *testing.T) {
		// Providers may have nested objects or arrays in their configuration space. As of Pulumi v3.63.0 these
		// may be coming over the wire under a flattened JSON-in-protobuf encoding. This test makes sure they
		// are recognized correctly.

		// Examples here are taken from pulumi-gcp, scopes is a list and batching is a nested object.
		schema := schema.Schema{
			Attributes: map[string]schema.Attribute{
				"scopes": schema.ListAttribute{
					Optional:    true,
					ElementType: types.StringType,
				},
			},
			Blocks: map[string]schema.Block{
				"batching": schema.SingleNestedBlock{
					Attributes: map[string]schema.Attribute{
						"send_after":      schema.StringAttribute{Optional: true},
						"enable_batching": schema.BoolAttribute{Optional: true},
					},
				},
			},
		}

		testutils.Replay(t, makeProviderServer(t, schema), `
		{
		  "method": "/pulumirpc.ResourceProvider/CheckConfig",
		  "request": {
		    "urn": "urn:pulumi:dev::testcfg::pulumi:providers:gcp::test",
		    "olds": {},
		    "news": {
		      "batching": "{\"enableBatching\":true,\"sendAfter\":\"1s\"}",
		      "scopes": "[\"a\",\"b\"]",
		      "version": "6.54.0"
		    }
		  },
		  "response": {
	            "inputs": {
		      "batching": {"enableBatching":true,"sendAfter":"1s"},
		      "scopes": ["a","b"],
		      "version": "6.54.0"
	            }
	          }
		}`)
	})

	t.Run("enforce_schema_secrets", func(t *testing.T) {
		// If the schema marks a config property as sensitive, enforce the secret bit on that property.
		schema := schema.Schema{
			Attributes: map[string]schema.Attribute{
				"mysecret": schema.StringAttribute{
					Optional:  true,
					Sensitive: true,
				},
			},
		}
		testutils.Replay(t, makeProviderServer(t, schema), `
		{
		  "method": "/pulumirpc.ResourceProvider/CheckConfig",
		  "request": {
		    "urn": "urn:pulumi:dev::teststack::pulumi:providers:testprovider::test",
		    "olds": {},
		    "news": {
	              "mysecret": "foo",
		      "version": "6.54.0"
		    }
		  },
		  "response": {
		    "inputs": {
	              "mysecret": {
			"4dabf18193072939515e22adb298388d": "1b47061264138c4ac30d75fd1eb44270",
	                "value": "foo"
	              },
		      "version": "6.54.0"
		    }
		  }
		}`)
	})

	t.Run("enforce_schema_nested_secrets", func(t *testing.T) {
		// Flattened compound values may encode that some nested properties are sensitive. There is currently no
		// way to preserve the secret-ness accurately in the JSON-in-proto encoding. Instead of this, bridged
		// providers approximate and mark the entire property as secret when any of the components are
		// sensitive.
		schema := schema.Schema{
			Attributes: map[string]schema.Attribute{
				"scopes": schema.ListAttribute{
					Optional:    true,
					ElementType: types.StringType,
				},
			},
			Blocks: map[string]schema.Block{
				"batching": schema.SingleNestedBlock{
					Attributes: map[string]schema.Attribute{
						"send_after": schema.StringAttribute{
							Optional:  true,
							Sensitive: true,
						},
						"enable_batching": schema.BoolAttribute{Optional: true},
					},
				},
			},
		}
		testutils.Replay(t, makeProviderServer(t, schema), `
	        {
	          "method": "/pulumirpc.ResourceProvider/CheckConfig",
	          "request": {
	            "urn": "urn:pulumi:dev::testcfg::pulumi:providers:gcp::test",
	            "olds": {},
	            "news": {
	              "batching": "{\"enableBatching\":true,\"sendAfter\":\"1s\"}",
	              "scopes": "[\"a\",\"b\"]",
	              "version": "6.54.0"
	            }
	          },
	          "response": {
	            "inputs": {
                  "batching": {
                    "enableBatching": true,
                    "sendAfter": {
                  	"4dabf18193072939515e22adb298388d": "1b47061264138c4ac30d75fd1eb44270",
                  	"value": "1s"
                    }
                  },
                  "scopes": [
                    "a",
                    "b"
                  ],
                  "version": "6.54.0"
                }
	          }
	        }`)
	})

	t.Run("tolerate pluginDownloadURL", func(t *testing.T) {
		schema := schema.Schema{}
		testutils.Replay(t, makeProviderServer(t, schema), `
		{
		  "method": "/pulumirpc.ResourceProvider/CheckConfig",
		  "request": {
		    "urn": "urn:pulumi:test1::typescript-example::pulumi:providers:authress::default_1_1_42_dirty_github_/api.github.com/Authress/pulumi-authress",
		    "olds": {},
		    "news": {
		      "pluginDownloadURL": "github://api.github.com/Authress/pulumi-authress",
		      "version": "1.1.42+dirty"
		    }
		  },
		  "response": {"inputs": {
		      "pluginDownloadURL": "github://api.github.com/Authress/pulumi-authress",
		      "version": "1.1.42+dirty"
		  }}
		}`)
	})
}

func TestPreConfigureCallback(t *testing.T) {

	t.Run("PreConfigureCallback called by CheckConfig", func(t *testing.T) {
		schema := schema.Schema{
			Attributes: map[string]schema.Attribute{
				"config_value": schema.StringAttribute{
					Optional: true,
				},
			},
		}
		callCounter := 0
		s := makeProviderServer(t, schema, func(info *tfbridge0.ProviderInfo) {
			info.PreConfigureCallback = func(vars resource.PropertyMap, config shim.ResourceConfig) error {
				require.Equal(t, "bar", vars["configValue"].StringValue())
				require.Truef(t, config.IsSet("configValue"), "configValue should be set")
				require.Falsef(t, config.IsSet("unknownProp"), "unknownProp should not be set")
				callCounter++
				return nil
			}
		})
		testutils.Replay(t, s, `
		{
		  "method": "/pulumirpc.ResourceProvider/CheckConfig",
		  "request": {
		    "urn": "urn:pulumi:dev::teststack::pulumi:providers:testprovider::test",
		    "olds": {},
		    "news": {
                      "configValue": "bar",
		      "version": "6.54.0"
		    }
		  },
		  "response": {
		    "inputs": {
                      "configValue": "bar",
		      "version": "6.54.0"
		    }
		  }
		}`)
		require.Equalf(t, 1, callCounter, "PreConfigureCallback should be called once")
	})

	t.Run("PreConfigureCallbackWithLoggger called by CheckConfig", func(t *testing.T) {
		schema := schema.Schema{
			Attributes: map[string]schema.Attribute{
				"config_value": schema.StringAttribute{
					Optional: true,
				},
			},
		}
		callCounter := 0
		s := makeProviderServer(t, schema, func(info *tfbridge0.ProviderInfo) {
			info.PreConfigureCallbackWithLogger = func(
				ctx context.Context,
				host *hostclient.HostClient,
				vars resource.PropertyMap,
				config shim.ResourceConfig,
			) error {
				require.Equal(t, "bar", vars["configValue"].StringValue())
				require.Truef(t, config.IsSet("configValue"), "configValue should be set")
				require.Falsef(t, config.IsSet("unknownProp"), "unknownProp should not be set")
				callCounter++
				return nil
			}
		})
		testutils.Replay(t, s, `
		{
		  "method": "/pulumirpc.ResourceProvider/CheckConfig",
		  "request": {
		    "urn": "urn:pulumi:dev::teststack::pulumi:providers:testprovider::test",
		    "olds": {},
		    "news": {
	              "configValue": "bar",
		      "version": "6.54.0"
		    }
		  },
		  "response": {
		    "inputs": {
	              "configValue": "bar",
		      "version": "6.54.0"
		    }
		  }
		}`)
		require.Equalf(t, 1, callCounter, "PreConfigureCallbackWithLogger should be called once")
	})

	t.Run("PreConfigureCallback can modify config values", func(t *testing.T) {
		schema := schema.Schema{
			Attributes: map[string]schema.Attribute{
				"config_value": schema.StringAttribute{
					Optional: true,
				},
			},
		}
		s := makeProviderServer(t, schema, func(info *tfbridge0.ProviderInfo) {
			info.PreConfigureCallback = func(vars resource.PropertyMap, config shim.ResourceConfig) error {
				vars["configValue"] = resource.NewStringProperty("updated")
				return nil
			}
		})
		testutils.Replay(t, s, `
		{
		  "method": "/pulumirpc.ResourceProvider/CheckConfig",
		  "request": {
		    "urn": "urn:pulumi:dev::teststack::pulumi:providers:testprovider::test",
		    "olds": {},
		    "news": {
		      "version": "6.54.0"
		    }
		  },
		  "response": {
		    "inputs": {
	              "configValue": "updated",
		      "version": "6.54.0"
		    }
		  }
		}`)
	})

	t.Run("PreConfigureCallbackWithLogger can modify config values", func(t *testing.T) {
		m := func(
			ctx context.Context,
			host *hostclient.HostClient,
			vars resource.PropertyMap,
			config shim.ResourceConfig,
		) error {
			vars["configValue"] = resource.NewStringProperty("updated")
			return nil
		}
		schema := schema.Schema{
			Attributes: map[string]schema.Attribute{
				"config_value": schema.StringAttribute{
					Optional: true,
				},
			},
		}
		s := makeProviderServer(t, schema, func(info *tfbridge0.ProviderInfo) {
			info.PreConfigureCallbackWithLogger = m
		})
		testutils.Replay(t, s, `
		{
		  "method": "/pulumirpc.ResourceProvider/CheckConfig",
		  "request": {
		    "urn": "urn:pulumi:dev::teststack::pulumi:providers:testprovider::test",
		    "olds": {},
		    "news": {
		      "version": "6.54.0"
		    }
		  },
		  "response": {
		    "inputs": {
	              "configValue": "updated",
		      "version": "6.54.0"
		    }
		  }
		}`)
	})

	t.Run("PreConfigureCallback not called at preview with unknown values", func(t *testing.T) {
		m := func(
			ctx context.Context,
			host *hostclient.HostClient,
			vars resource.PropertyMap,
			config shim.ResourceConfig,
		) error {
			if cv, ok := vars["configValue"]; ok {
				// This used to panic when cv is a resource.Computed.
				cv.StringValue()
			}
			// PreConfigureCallback should not even be called.
			t.FailNow()
			return nil
		}
		schema := schema.Schema{
			Attributes: map[string]schema.Attribute{
				"config_value": schema.StringAttribute{
					Optional: true,
				},
			},
		}
		s := makeProviderServer(t, schema, func(info *tfbridge0.ProviderInfo) {
			info.PreConfigureCallbackWithLogger = m
		})
		testutils.Replay(t, s, `
		{
		  "method": "/pulumirpc.ResourceProvider/CheckConfig",
		  "request": {
		    "urn": "urn:pulumi:dev::teststack::pulumi:providers:testprovider::test",
		    "olds": {},
		    "news": {
		      "version": "6.54.0",
	              "configValue": "04da6b54-80e4-46f7-96ec-b56ff0331ba9"
		    }
		  },
		  "response": {
		    "inputs": {
		      "version": "6.54.0",
	              "configValue": "dd056dcd-154b-4c76-9bd3-c8f88648b5ff"
		    }
		  }
		}`)
	})
}

func makeProviderServer(
	t *testing.T,
	schema schema.Schema,
	customize ...func(*tfbridge0.ProviderInfo),
) pulumirpc.ResourceProviderServer {
	testProvider := &providerbuilder.Provider{
		TypeName:       "testprovider",
		Version:        "0.0.1",
		ProviderSchema: schema,
	}
	info := tfbridge0.ProviderInfo{
		Name:         "testprovider",
		Version:      "0.0.1",
		MetadataInfo: &tfbridge0.MetadataInfo{},
		P:            tfbridge.ShimProvider(testProvider),
	}
	for _, c := range customize {
		c(&info)
	}
	return newProviderServer(t, info)
}
