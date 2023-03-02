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
	"bytes"
	"context"
	"fmt"
	"hash/crc32"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

	testutils "github.com/pulumi/pulumi-terraform-bridge/testing/x"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	shimv2 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v2"
)

func TestRegressAws2352(t *testing.T) {
	ctx := context.Background()

	createDotStringHashcode := func(s string) int {
		v := int(crc32.ChecksumIEEE([]byte(s)))
		if v >= 0 {
			return v
		}
		if -v >= 0 {
			return -v
		}
		// v == MinInt
		return 0
	}

	endpointHashIPAddress := func(v interface{}) int {
		var buf bytes.Buffer
		m := v.(map[string]interface{})
		buf.WriteString(fmt.Sprintf("%s-%s-", m["subnet_id"].(string), m["ip"].(string)))
		return createDotStringHashcode(buf.String())
	}

	resource := &schema.Resource{
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"arn": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"direction": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"host_vpc_id": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"ip_address": {
				Type:     schema.TypeSet,
				Required: true,
				MinItems: 2,
				MaxItems: 10,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"ip": {
							Type:     schema.TypeString,
							Optional: true,
							Computed: true,
						},
						"ip_id": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"subnet_id": {
							Type:     schema.TypeString,
							Required: true,
						},
					},
				},
				Set: endpointHashIPAddress,
			},
			"name": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"security_group_ids": {
				Type:     schema.TypeSet,
				Required: true,
				ForceNew: true,
				MinItems: 1,
				MaxItems: 64,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},
		},
	}

	tfProvider := &schema.Provider{
		Schema: map[string]*schema.Schema{},
		ResourcesMap: map[string]*schema.Resource{
			"aws_route53_resolver_endpoint": resource,
		},
	}

	p := shimv2.NewProvider(tfProvider, shimv2.WithDiffStrategy(shimv2.PlanState))

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
			"aws_route53_resolver_endpoint": {Tok: "aws:route53/resolverEndpoint:ResolverEndpoint"},
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

	testCase := `
	{
	  "method": "/pulumirpc.ResourceProvider/Diff",
	  "request": {
	    "id": "rslvr-in-44b65ff0f2b3468ca",
	    "urn": "urn:pulumi:dev::pulumi-aws-2294::aws:route53/resolverEndpoint:ResolverEndpoint::t0yv0-us-east-1-resolver-inbound",
	    "olds": {
	      "__meta": "{\"e2bfb730-ecaa-11e6-8f88-34363bc7c4c0\":{\"create\":600000000000,\"delete\":600000000000,\"update\":600000000000}}",
	      "arn": "arn:aws:route53resolver:us-east-1:616138583583:resolver-endpoint/rslvr-in-44b65ff0f2b3468ca",
	      "direction": "INBOUND",
	      "hostVpcId": "vpc-0eabf180cc8009cfa",
	      "id": "rslvr-in-44b65ff0f2b3468ca",
	      "ipAddresses": [
		{
		  "ip": "10.42.0.247",
		  "ipId": "rni-84dfce61ad514e548",
		  "subnetId": "subnet-0f4c3afd2e3c0f146"
		},
		{
		  "ip": "10.42.1.219",
		  "ipId": "rni-3e6238a51f164e6eb",
		  "subnetId": "subnet-0618d94b7bb9b38b4"
		}
	      ],
	      "name": "t0yv0-us-east-1-resolver-inbound-ccbbe19",
	      "securityGroupIds": [
		"sg-001889228cb15f0d8"
	      ],
	      "tags": {},
	      "tagsAll": {}
	    },
	    "news": {
	      "__defaults": [
		"name"
	      ],
	      "direction": "INBOUND",
	      "ipAddresses": [
		{
		  "__defaults": [],
		  "subnetId": "subnet-0f4c3afd2e3c0f146"
		},
		{
		  "__defaults": [],
		  "subnetId": "subnet-0618d94b7bb9b38b4"
		}
	      ],
	      "name": "t0yv0-us-east-1-resolver-inbound-ccbbe19",
	      "securityGroupIds": [
		"sg-001889228cb15f0d8"
	      ]
	    }
	  },
	  "response": {
	    "stables": "*",
	    "changes": "DIFF_NONE",
	    "hasDetailedDiff": "*"
	  }
	}`
	testutils.Replay(t, server, testCase)
}
