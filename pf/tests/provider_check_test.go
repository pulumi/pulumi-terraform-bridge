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
	"fmt"
	"testing"

	pschema "github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/pulumi/pulumi-terraform-bridge/pf/tests/internal/providerbuilder"
	"github.com/pulumi/pulumi-terraform-bridge/pf/tfbridge"
	testutils "github.com/pulumi/pulumi-terraform-bridge/testing/x"
	tfbridge3 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
)

func TestCheck(t *testing.T) {

	type testCase struct {
		name   string
		schema schema.Schema
		replay string
	}

	testCases := []testCase{
		{
			"minimal",
			schema.Schema{
				Attributes: map[string]schema.Attribute{
					"id": schema.StringAttribute{Computed: true},
				},
			},
			`
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
			"prop",
			schema.Schema{
				Attributes: map[string]schema.Attribute{
					"id":   schema.StringAttribute{Computed: true},
					"prop": schema.StringAttribute{Optional: true},
				},
			},
			`
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
			"validators",
			schema.Schema{
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
			fmt.Sprintf(`
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
			"missing_required_prop",
			schema.Schema{
				Attributes: map[string]schema.Attribute{
					"id": schema.StringAttribute{Computed: true},
					"prop": schema.StringAttribute{
						Required: true,
					},
				},
			},
			`
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
                            "failures": [{"property": "prop", "reason": "Missing a required property"}]
			  }
			}`,
		},
		{
			// Unlike CheckConfig, unrecognized values are passed through without warning so that Pulumi
			// resources can extend the protocol without triggering warnings.
			"unrecognized_prop_passed_through",
			schema.Schema{
				Attributes: map[string]schema.Attribute{
					"id": schema.StringAttribute{Computed: true},
				},
			},
			`
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
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			testProvider := &providerbuilder.Provider{
				TypeName:       "testprovider",
				Version:        "0.0.1",
				ProviderSchema: pschema.Schema{},
				AllResources: []providerbuilder.Resource{{
					Name:           "res",
					ResourceSchema: tc.schema,
				}},
			}
			info := tfbridge3.ProviderInfo{
				Name:         "testprovider",
				Version:      "0.0.1",
				MetadataInfo: &tfbridge3.MetadataInfo{},
				Resources: map[string]*tfbridge3.ResourceInfo{
					"testprovider_res": {
						Tok: "testprovider:index/res:Res",
						Docs: &tfbridge3.DocInfo{
							Markdown: []byte("OK"),
						},
					},
				},
			}
			providerInfo := tfbridge.ProviderInfo{
				ProviderInfo: info,
				NewProvider: func() provider.Provider {
					return testProvider
				},
			}
			s := newProviderServer(t, providerInfo)

			if tc.replay != "" {
				testutils.Replay(t, s, tc.replay)
			}
		})
	}
}
