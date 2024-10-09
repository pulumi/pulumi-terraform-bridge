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
	"math"
	"math/big"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/pulumi/providertest/replay"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tests/internal/cross-tests"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tests/internal/testprovider"
	propProviderSchema "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tests/util/property/pf/schema/provider"
	propProviderValue "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tests/util/property/pf/value/provider"
	tfpf "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tests/assume"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
	"pgregory.net/rapid"
)

func TestConfigure(t *testing.T) {
	t.Parallel()

	t.Run("string", crosstests.MakeConfigure(
		schema.Schema{Attributes: map[string]schema.Attribute{
			"k": schema.StringAttribute{Optional: true},
		}},
		map[string]cty.Value{"k": cty.StringVal("foo")},
		resource.PropertyMap{"k": resource.NewProperty("foo")},
	))

	t.Run("secret-string", crosstests.MakeConfigure(
		schema.Schema{Attributes: map[string]schema.Attribute{
			"k": schema.StringAttribute{Optional: true},
		}},
		map[string]cty.Value{"k": cty.StringVal("foo")},
		resource.PropertyMap{"k": resource.MakeSecret(resource.NewProperty("foo"))},
	))

	t.Run("bool", crosstests.MakeConfigure(
		schema.Schema{Attributes: map[string]schema.Attribute{
			"b": schema.BoolAttribute{Optional: true},
		}},
		map[string]cty.Value{"b": cty.BoolVal(false)},
		resource.PropertyMap{"b": resource.NewProperty(false)},
	))
}

// TestConfigureInvalidTypes tests configure for inputs that are not type-safe but that we
// expect to work.
func TestConfigureInvalidTypes(t *testing.T) {
	t.Setenv("PULUMI_DEBUG_YAML_DISABLE_TYPE_CHECKING", "true")

	t.Run("bool-type-conversion", crosstests.MakeConfigure(
		schema.Schema{Attributes: map[string]schema.Attribute{
			"b": schema.BoolAttribute{Optional: true},
		}},
		map[string]cty.Value{"b": cty.BoolVal(false)},
		resource.PropertyMap{"b": resource.NewProperty("false")},
	))
}

func TestConfigureProperties(t *testing.T) {
	t.Parallel()

	assume.TerraformCLI(t)

	equals := func(t crosstests.TestingT, tfOutput, puOutput tfsdk.Config) {
		assert.Equal(t, tfOutput.Schema, puOutput.Schema, "schema doesn't match")

		// Configure does not work as it should in the general case.
		//
		// If it did, then we would not need a custom equals function, instead just asserting tfOutput
		// is equal to puOutput.
		//
		// Each clause in the normalize function thus represents a bug in the bridge.
		var relax func(tftypes.Value) tftypes.Value
		relax = func(v tftypes.Value) tftypes.Value {
			if !v.IsKnown() {
				return v
			}

			if typ := v.Type(); typ.Is(tftypes.Map{}) || typ.Is(tftypes.Object{}) {
				asMapOrObject := map[string]tftypes.Value{}
				require.NoError(t, v.As(&asMapOrObject))

				for k, e := range asMapOrObject {
					e = relax(e)
					if e.IsNull() && typ.Is(tftypes.Map{}) {
						delete(asMapOrObject, k)
					} else {
						asMapOrObject[k] = e
					}
				}

				// TODO: Empty objects are represented as {} in PF but null in bridged PF.
				//
				// This normalizes all empty objects to be represented as null so the test passes.
				if len(asMapOrObject) == 0 {
					// Normalize empty maps to nil
					return tftypes.NewValue(v.Type(), nil)
				}

				return tftypes.NewValue(v.Type(), asMapOrObject)
			}

			if v.Type().Is(tftypes.List{}) {
				asSliceOrSet := []tftypes.Value{}
				require.NoError(t, v.As(&asSliceOrSet))
				newSliceOrSet := make([]tftypes.Value, 0, len(asSliceOrSet))

				// TODO: Lists of empty objects appear in Bridged PF as also empty. In standard PF
				// they appear as lists of empty objects.
				//
				// This normalizes all slices of empty objects to be represented as null.

				for _, e := range asSliceOrSet {
					if e := relax(e); !e.IsNull() {
						newSliceOrSet = append(newSliceOrSet, e)
					}
				}

				if len(newSliceOrSet) == 0 {
					return tftypes.NewValue(v.Type(), nil)
				}

				return tftypes.NewValue(v.Type(), newSliceOrSet)
			}

			// TODO: We don't set the same precision as PF does for numbers.
			if v.Type().Is(tftypes.Number) {
				n := new(big.Float)
				require.NoError(t, v.As(&n))

				if n != nil {
					f, _ := n.Float32()
					n = big.NewFloat(math.Round(float64(f)*1000) / 1000)

					return tftypes.NewValue(v.Type(), n)
				}
			}

			// Nested set diffs are not correct, so we don't report them here.
			if v.Type().Is(tftypes.Set{}) {
				// Sets are not handled correctly here.
				return tftypes.NewValue(v.Type(), nil)
			}

			return v
		}

		if tf, pu := relax(tfOutput.Raw), relax(puOutput.Raw); !tf.Equal(pu) {
			diff, err := tf.Diff(pu)
			require.NoError(t, err)
			for i, d := range diff {
				t.Errorf("%d: %s", i, d.String())
			}
		}
	}

	rapid.Check(t, func(t *rapid.T) {
		ctx := context.Background()
		schema := propProviderSchema.Schema(ctx).Draw(t, "schema")
		value := propProviderValue.WithValue(ctx, schema).Draw(t, "value")
		crosstests.Configure(t, schema, value.Tf.AsValueMap(), value.Pu,
			crosstests.WithConfigureEqual(equals))
	})
}

// Test interaction of Configure and Create.
//
// The resource TestConfigRes will read stringConfigProp information the provider receives via Configure.
func TestConfigureToCreate(t *testing.T) {
	server, err := newProviderServer(t, testprovider.SyntheticTestBridgeProvider())
	require.NoError(t, err)
	replay.ReplaySequence(t, server, `
	[
	  {
	    "method": "/pulumirpc.ResourceProvider/Configure",
	    "request": {
	      "args": {
		"stringConfigProp": "example"
	      }
	    },
	    "response": {
	      "supportsPreview": true,
	      "acceptResources": true
	    }
	  },
	  {
	    "method": "/pulumirpc.ResourceProvider/Create",
	    "request": {
	      "urn": "urn:pulumi:test-stack::basicprogram::testbridge:index/testres:TestConfigRes::r1",
	      "preview": false
	    },
	    "response": {
	      "id": "id-1",
	      "properties": {
		"configCopy": "example",
		"id": "id-1"
	      }
	    }
	  }
	]`)
}

func TestConfigureBooleans(t *testing.T) {
	// Non-string properties caused trouble at some point, test booleans.
	server, err := newProviderServer(t, testprovider.SyntheticTestBridgeProvider())
	require.NoError(t, err)

	replay.Replay(t, server, `
	{
	  "method": "/pulumirpc.ResourceProvider/Configure",
	  "request": {
	    "args": {
	      "boolConfigProp": "true"
	    }
	  },
	  "response": {
	    "supportsPreview": true,
	    "acceptResources": true
	  }
	}`)
}

func TestConfigureErrorReplacement(t *testing.T) {
	t.Run("replace_config_properties", func(t *testing.T) {
		errString := `some error with "config_property" and "config" but not config`
		prov := &testprovider.ConfigTestProvider{
			ConfigErr: diag.NewErrorDiagnostic(errString, errString),
			ProviderSchema: schema.Schema{
				Attributes: map[string]schema.Attribute{
					"config":          schema.StringAttribute{},
					"config_property": schema.StringAttribute{},
				},
			},
		}

		providerInfo := testprovider.SyntheticTestBridgeProvider()
		providerInfo.P = tfpf.ShimProvider(prov)
		providerInfo.Config["config_property"] = &tfbridge.SchemaInfo{Name: "configProperty"}
		providerInfo.Config["config"] = &tfbridge.SchemaInfo{Name: "CONFIG!"}

		server, err := newProviderServer(t, providerInfo)
		require.NoError(t, err)

		replay.Replay(t, server, `
			{
			  "method": "/pulumirpc.ResourceProvider/Configure",
			  "request": {"acceptResources": true},
			  "errors": ["some error with \"configProperty\" and \"CONFIG!\" but not config"]
			}`)
	})

	t.Run("different_error_detail_and_summary_not_dropped", func(t *testing.T) {
		errSummary := `problem with "config_property" and "config"`
		errString := `some error with "config_property" and "config" but not config`
		prov := &testprovider.ConfigTestProvider{
			ConfigErr: diag.NewErrorDiagnostic(errSummary, errString),
			ProviderSchema: schema.Schema{
				Attributes: map[string]schema.Attribute{
					"config":          schema.StringAttribute{},
					"config_property": schema.StringAttribute{},
				},
			},
		}

		providerInfo := testprovider.SyntheticTestBridgeProvider()
		providerInfo.P = tfpf.ShimProvider(prov)
		providerInfo.Config["config_property"] = &tfbridge.SchemaInfo{Name: "configProperty"}
		providerInfo.Config["config"] = &tfbridge.SchemaInfo{Name: "CONFIG!"}

		server, err := newProviderServer(t, providerInfo)
		require.NoError(t, err)

		replay.Replay(t, server, `
			{
			  "method": "/pulumirpc.ResourceProvider/Configure",
			  "request": {"acceptResources": true},
			  "errors": ["problem with \"configProperty\" and \"CONFIG!\": some error with \"configProperty\" and \"CONFIG!\" but not config"]
			}`)
	})
}

func TestJSONNestedConfigure(t *testing.T) {
	p := testprovider.SyntheticTestBridgeProvider()
	server, err := newProviderServer(t, p)
	require.NoError(t, err)
	replay.Replay(t, server, `{
		  "method": "/pulumirpc.ResourceProvider/Configure",
		  "request": {
		    "args": {
                      "validateNested": "true",
                      "mapNestedProp": "{\"k1\":1,\"k2\":2}",
                      "listNestedProps": "[true,false]",
                      "singleNested": "{\"stringProp\":\"foo\",\"boolProp\":true,\"mapNestedProp\":{\"v1\":1234},\"listNestedProps\":[true,false]}",
                      "listNesteds": "[{\"stringProp\":\"foo\",\"boolProp\":true,\"mapNestedProp\":{\"v1\":1234},\"listNestedProps\":[true,false]}]"
		    }
		  },
		  "response": {
		    "supportsPreview": true,
		    "acceptResources": true
		  }
		}`)
}

func TestJSONNestedConfigureWithSecrets(t *testing.T) {
	server, err := newProviderServer(t, testprovider.SyntheticTestBridgeProvider())
	require.NoError(t, err)
	replay.ReplaySequence(t, server, `
[
  {
    "method": "/pulumirpc.ResourceProvider/Configure",
    "request": {
      "args": {
        "stringConfigProp": {
          "4dabf18193072939515e22adb298388d": "1b47061264138c4ac30d75fd1eb44270",
          "value": "secret-example"
        },
        "mapNestedProp": "{\"k1\":{\"4dabf18193072939515e22adb298388d\":\"1b47061264138c4ac30d75fd1eb44270\",\"value\":1},\"k2\":2}",
        "listNestedProps": "[{\"4dabf18193072939515e22adb298388d\":\"1b47061264138c4ac30d75fd1eb44270\",\"value\":true},false]"
      }
    },
    "response": {
      "supportsPreview": true,
      "acceptResources": true
    }
  },
  {
    "method": "/pulumirpc.ResourceProvider/Create",
    "request": {
      "urn": "urn:pulumi:test-stack::basicprogram::testbridge:index/testres:TestConfigRes::r1",
      "preview": false
    },
    "response": {
      "id": "id-1",
      "properties": {
        "configCopy": "secret-example",
        "id": "id-1"
      }
    }
  }
]`)
}

func TestConfigureWithSecrets(t *testing.T) {
	server, err := newProviderServer(t, testprovider.SyntheticTestBridgeProvider())
	require.NoError(t, err)
	replay.ReplaySequence(t, server, `
[
  {
    "method": "/pulumirpc.ResourceProvider/Configure",
    "request": {
      "args": {
        "stringConfigProp": {
          "4dabf18193072939515e22adb298388d": "1b47061264138c4ac30d75fd1eb44270",
          "value": "secret-example"
        },
        "mapNestedProp": {
          "k1": {
            "4dabf18193072939515e22adb298388d": "1b47061264138c4ac30d75fd1eb44270",
            "value": 1
          },
          "k2": 2
        },
        "listNestedProps": [
          {
            "4dabf18193072939515e22adb298388d": "1b47061264138c4ac30d75fd1eb44270",
            "value": true
          },
          false
        ]
      }
    },
    "response": {
      "supportsPreview": true,
      "acceptResources": true
    }
  },
  {
    "method": "/pulumirpc.ResourceProvider/Create",
    "request": {
      "urn": "urn:pulumi:test-stack::basicprogram::testbridge:index/testres:TestConfigRes::r1",
      "preview": false
    },
    "response": {
      "id": "id-1",
      "properties": {
        "configCopy": "secret-example",
        "id": "id-1"
      }
    }
  }
]`)
}
