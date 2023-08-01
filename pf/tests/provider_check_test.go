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
	pschema "github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"

	"github.com/pulumi/pulumi-terraform-bridge/pf/tests/internal/providerbuilder"
	"github.com/pulumi/pulumi-terraform-bridge/pf/tfbridge"
	testutils "github.com/pulumi/pulumi-terraform-bridge/testing/x"
	tfbridge0 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
)

func TestCheck(t *testing.T) {

	type testCase struct {
		name        string
		schema      schema.Schema
		replay      string
		replayMulti string
		callback    tfbridge0.PreCheckCallback
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
			      "acceptResources": true
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
			      "acceptResources": true
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
				_ context.Context, config, meta resource.PropertyMap,
			) (resource.PropertyMap, error) {
				t.Logf("Meta: %#v", meta)
				config["prop"] = meta["prop"]
				return config, nil
			},
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			testProvider := &providerbuilder.Provider{
				TypeName: "testprovider",
				Version:  "0.0.1",
				ProviderSchema: pschema.Schema{
					Attributes: map[string]pschema.Attribute{
						"prop": pschema.StringAttribute{
							Optional: true,
						},
					},
				},
				AllResources: []providerbuilder.Resource{{
					Name:           "res",
					ResourceSchema: tc.schema,
				}},
			}
			info := tfbridge0.ProviderInfo{
				Name:         "testprovider",
				P:            tfbridge.ShimProvider(testProvider),
				Version:      "0.0.1",
				MetadataInfo: &tfbridge0.MetadataInfo{},
				Resources: map[string]*tfbridge0.ResourceInfo{
					"testprovider_res": {
						Tok: "testprovider:index/res:Res",
						Docs: &tfbridge0.DocInfo{
							Markdown: []byte("OK"),
						},
						PreCheckCallback: tc.callback,
					},
				},
			}
			s := newProviderServer(t, info)

			if tc.replay != "" {
				testutils.Replay(t, s, tc.replay)
			}
			if tc.replayMulti != "" {
				testutils.ReplaySequence(t, s, tc.replayMulti)
			}
		})
	}
}
