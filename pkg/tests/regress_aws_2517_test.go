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

package tests

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	testutils "github.com/pulumi/pulumi-terraform-bridge/testing/x"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	shimv2 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v2"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

func TestRegressAws2517(t *testing.T) {
	ctx := context.Background()

	token := "aws:lb/targetGroup:TargetGroup"
	tfName := "aws_lb_target_group"

	resource := &schema.Resource{
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		ReadContext: func(_ context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
			m := make(map[string]any)
			if err := d.Set("target_failover", []any{m}); err != nil {
				panic(err)
			}
			return diag.Diagnostics{}
		},

		Schema: map[string]*schema.Schema{
			"target_failover": {
				Type:     schema.TypeList,
				Optional: true,
				Computed: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"on_deregistration": {
							Type:     schema.TypeString,
							Required: true,
						},
						"on_unhealthy": {
							Type:     schema.TypeString,
							Required: true,
						},
					},
				},
			},
		},
	}

	tfProvider := &schema.Provider{
		Schema: map[string]*schema.Schema{},
		ResourcesMap: map[string]*schema.Resource{
			tfName: resource,
		},
	}

	type testCase struct {
		name         string
		makeProvider func() shim.Provider
		interaction  string
	}

	testCases := []testCase{
		{
			name: "default",
			makeProvider: func() shim.Provider {
				return shimv2.NewProvider(tfProvider)
			},
			interaction: `
	                {
			  "method": "/pulumirpc.ResourceProvider/Read",
			  "request": {
			    "id": "arn:aws:elasticloadbalancing:us-east-1:616138583583:targetgroup/test-c530e0b/6aee61b47ab16785",
			    "urn": "urn:pulumi:dev::aws-2517::aws:lb/targetGroup:TargetGroup::foo",
			    "properties": {}
			  },
			  "response": {
			    "id": "*",
			    "inputs": "*",
			    "properties": {
			       "id": "*",
			       "targetFailovers": [null]
			    }
			  }
			}`,
		},
		{
			name: "with-flags",
			makeProvider: func() shim.Provider {
				return shimv2.NewProvider(tfProvider,
					shimv2.WithDiffStrategy(shimv2.PlanState),
					shimv2.WithPlanResourceChange(func(tfResourceType string) bool {
						return true
					}),
				)
			},
			interaction: `
	                {
			  "method": "/pulumirpc.ResourceProvider/Read",
			  "request": {
			    "id": "arn:aws:elasticloadbalancing:us-east-1:616138583583:targetgroup/test-c530e0b/6aee61b47ab16785",
			    "urn": "urn:pulumi:dev::aws-2517::aws:lb/targetGroup:TargetGroup::foo",
			    "properties": {}
			  },
			  "response": {
			    "id": "*",
			    "inputs": "*",
			    "properties": {
                               "__meta": "{\"schema_version\":\"0\"}",
			       "id": "*",
			       "targetFailovers": [{"onDeregistration":null, "onUnhealthy":null}]
			    }
			  }
			}`,
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			p := tc.makeProvider()

			info := tfbridge.ProviderInfo{
				P:           p,
				Name:        "aws",
				Description: "A Pulumi package for creating and managing Amazon Web Services (AWS) cloud resources.",
				Keywords:    []string{"pulumi", "aws"},
				License:     "Apache-2.0",
				Homepage:    "https://pulumi.io",
				Repository:  "https://github.com/phillipedwards/pulumi-aws",
				Version:     "0.0.2",
				Resources: map[string]*tfbridge.ResourceInfo{
					tfName: {Tok: tokens.Type(token)},
				},
			}

			server := tfbridge.NewProvider(ctx,
				nil,      /* hostClient */
				"aws",    /* module */
				"",       /* version */
				p,        /* tf */
				info,     /* info */
				[]byte{}, /* pulumiSchema */
			)

			testutils.Replay(t, server, tc.interaction)
		})
	}

}
