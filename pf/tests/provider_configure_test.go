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
	testutils "github.com/pulumi/providertest/replay"
	"github.com/pulumi/pulumi-terraform-bridge/pf/tests/internal/testprovider"
	tfpf "github.com/pulumi/pulumi-terraform-bridge/pf/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
)

func TestConfigure(t *testing.T) {
	t.Run("configiure communicates to create", func(t *testing.T) {
		// Test interaction of Configure and Create.
		//
		// TestConfigRes will read stringConfigProp information the provider receives via Configure.
		server := newProviderServer(t, testprovider.SyntheticTestBridgeProvider())
		testCase := `
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
		]`
		testutils.ReplaySequence(t, server, testCase)
	})

	t.Run("booleans", func(t *testing.T) {
		// Non-string properties caused trouble at some point, test booleans.
		server := newProviderServer(t, testprovider.SyntheticTestBridgeProvider())

		testCase := `
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
		}`
		testutils.Replay(t, server, testCase)
	})
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

		server := newProviderServer(t, providerInfo)

		testutils.Replay(t, server, `
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

		server := newProviderServer(t, providerInfo)

		testutils.Replay(t, server, `
			{
			  "method": "/pulumirpc.ResourceProvider/Configure",
			  "request": {"acceptResources": true},
			  "errors": ["problem with \"configProperty\" and \"CONFIG!\": some error with \"configProperty\" and \"CONFIG!\" but not config"]
			}`)
	})
}
