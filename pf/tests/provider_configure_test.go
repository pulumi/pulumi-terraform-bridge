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
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/pulumi/providertest/replay"
	"github.com/pulumi/pulumi-terraform-bridge/pf/tests/internal/cross-tests"
	"github.com/pulumi/pulumi-terraform-bridge/pf/tests/internal/testprovider"
	tfpf "github.com/pulumi/pulumi-terraform-bridge/pf/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
)

func TestConfigure(t *testing.T) {
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
