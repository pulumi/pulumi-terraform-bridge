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
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

	testutils "github.com/pulumi/pulumi-terraform-bridge/testing/x"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	shimv2 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v2"
	"net"
)

// See https://github.com/pulumi/pulumi-terraform-bridge/issues/1020
func TestRegress1020(t *testing.T) {
	ctx := context.Background()

	CIDRBlocksEqual := func(cidr1, cidr2 string) bool {
		ip1, ipnet1, err := net.ParseCIDR(cidr1)
		if err != nil {
			return false
		}
		ip2, ipnet2, err := net.ParseCIDR(cidr2)
		if err != nil {
			return false
		}

		return ip2.String() == ip1.String() && ipnet2.String() == ipnet1.String()
	}

	emptySchema := func() *schema.Schema {
		return &schema.Schema{
			Type:     schema.TypeList,
			Optional: true,
			MaxItems: 1,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{},
			},
		}
	}

	resource := &schema.Resource{
		Schema: map[string]*schema.Schema{
			"addresses": {
				Type:     schema.TypeSet,
				Optional: true,
				MaxItems: 10000,
				Elem:     &schema.Schema{Type: schema.TypeString},
				DiffSuppressFunc: func(k, old, new string, d *schema.ResourceData) bool {
					if d.GetRawPlan().IsNull() {
						panic(fmt.Sprintf("%s", "NULL GetRawPlan"))
					}
					if d.GetRawPlan().GetAttr("addresses").IsWhollyKnown() {
						o, n := d.GetChange("addresses")
						oldAddresses := o.(*schema.Set).List()
						newAddresses := n.(*schema.Set).List()
						if len(oldAddresses) == len(newAddresses) {
							for _, ov := range oldAddresses {
								hasAddress := false
								for _, nv := range newAddresses {
									if CIDRBlocksEqual(ov.(string), nv.(string)) {
										hasAddress = true
										break
									}
								}
								if !hasAddress {
									return false
								}
							}
							return true
						}
					}
					return false
				},
			},
			"name": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"scope": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"ip_address_version": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},

			"rule": {
				Type:     schema.TypeSet,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"name": {
							Type:     schema.TypeString,
							Required: true,
						},
						"override": {
							Type:     schema.TypeList,
							Optional: true,
							MaxItems: 1,
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"count": emptySchema(),
									"none":  emptySchema(),
								},
							},
						},
						"priority": {
							Type:     schema.TypeInt,
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
			"aws_wafv2_ip_set": resource,
		},
	}

	p := shimv2.NewProvider(tfProvider) // , shimv2.WithDiffStrategy(shimv2.PlanState))

	info := tfbridge.ProviderInfo{
		P:           p,
		Name:        "aws",
		Description: "A Pulumi package for creating and managing Amazon Web Services (AWS) cloud resources.",
		Keywords:    []string{"pulumi", "aws"},
		License:     "Apache-2.0",
		Homepage:    "https://pulumi.io",
		Repository:  "https://github.com/pulumi/pulumi-aws",
		Version:     "0.0.2",
		Resources: map[string]*tfbridge.ResourceInfo{
			"aws_wafv2_ip_set": {Tok: "aws:wafv2/ipSet:IpSet"},
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
  "method": "/pulumirpc.ResourceProvider/Create",
  "request": {
    "urn": "urn:pulumi:dev::repro-1020::aws:wafv2/ipSet:IpSet::ip6_sample",
    "properties": {
      "__defaults": [
        "name"
      ],
      "addresses": [
        "2001:0db8:85a3:0000:0000:8a2e:0370:7334/32"
      ],
      "ipAddressVersion": "IPV6",
      "name": "ip6_sample-e8442ad",
      "scope": "CLOUDFRONT"
    },
    "preview": true
  },
  "response": {}
}`
	testutils.Replay(t, server, testCase)
}
