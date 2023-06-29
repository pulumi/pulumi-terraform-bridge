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

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

	testutils "github.com/pulumi/pulumi-terraform-bridge/testing/x"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	shimv2 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v2"
)

func TestRegressAws2490(t *testing.T) {
	ctx := context.Background()

	resourceIntegrationCreate := func(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
		// The real upstream code makes an API call and then calls resourceIntegrationRead. The API call does
		// not return passthrough_behavior and effectively this sets "passthrough_behavior" to nil.
		var v *string
		d.Set("passthrough_behavior", v)
		d.SetId("myid")
		return diag.Diagnostics{}
	}

	resource := &schema.Resource{
		CreateWithoutTimeout: resourceIntegrationCreate,
		Schema: map[string]*schema.Schema{
			"id": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"passthrough_behavior": {
				Type:     schema.TypeString,
				Optional: true,
				Default:  "WHEN_NO_MATCH",
			},
		},
	}

	tfProvider := &schema.Provider{
		Schema: map[string]*schema.Schema{},
		ResourcesMap: map[string]*schema.Resource{
			"aws_apigateway2_integration": resource,
		},
	}

	p := shimv2.NewProvider(tfProvider)

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
			"aws_apigateway2_integration": {Tok: "aws:apigatewayv2/integration:Integration"},
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

	t.Run("create", func(t *testing.T) {
		testCase := `
		{
		  "method": "/pulumirpc.ResourceProvider/Create",
		  "request": {
		    "urn": "urn:pulumi:dev::aws-2490::aws:apigatewayv2/integration:Integration::example",
		    "properties": {
		      "__defaults": [],
		      "passthroughBehavior": "NEVER"
		    }
		  },
		  "response": {
		    "id": "qkmc19h",
		    "properties": {
		      "passthroughBehavior": ""
		    }
		  }
		}`
		// Currently this is how it works.. Subsequently Pulumi stores passthroughBehavior output as "" in the
		// state input as "NEVER" in the state. Should output still be "" here?
		testutils.Replay(t, server, testCase)
	})

	t.Run("diff", func(t *testing.T) {
		testCase := `
		{
		  "method": "/pulumirpc.ResourceProvider/Diff",
		  "request": {
		    "id": "hacg44t",
		    "urn": "urn:pulumi:dev::aws-2490::aws:apigatewayv2/integration:Integration::example",
		    "olds": {
		      "passthroughBehavior": "",
		    },
		    "news": {
		      "__defaults": [],
		      "passthroughBehavior": "NEVER",
		    }
		  },
		  "response": {
		    "changes": "DIFF_SOME",
		    "diffs": [
		      "passthroughBehavior"
		    ],
		    "detailedDiff": {
		      "passthroughBehavior": {
			"kind": "UPDATE"
		      }
		    },
		    "hasDetailedDiff": true
		  }
		}
		`
		// Subsequently this is reported as a diff, which is undesirable. Apparently this is getting Pulumi
		// "output" in olds, but not getting old inputs. Should Pulumi be using old inputs as the basis for the
		// diff?
		testutils.Replay(t, server, testCase)
	})
}
