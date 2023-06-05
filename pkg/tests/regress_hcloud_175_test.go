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
	testutils "github.com/pulumi/pulumi-terraform-bridge/testing/x"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	shimv2 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v2"
)

func TestRegressHCloud175(t *testing.T) {
	ctx := context.Background()

	subnetResource := func() *schema.Resource {
		return &schema.Resource{
			// CreateContext: resourceNetworkSubnetCreate,
			// ReadContext:   resourceNetworkSubnetRead,
			// DeleteContext: resourceNetworkSubnetDelete,
			Importer: &schema.ResourceImporter{
				StateContext: schema.ImportStatePassthroughContext,
			},
			Schema: map[string]*schema.Schema{
				"network_id": {
					Type:     schema.TypeInt,
					Required: true,
					ForceNew: true,
				},
				"type": {
					Type:     schema.TypeString,
					Required: true,
					ForceNew: true,
					// ValidateFunc: validation.StringInSlice([]string{
					// 	"cloud",
					// 	"server",
					// 	"vswitch",
					// }, false),
				},
				"network_zone": {
					Type:     schema.TypeString,
					Required: true,
					ForceNew: true,
				},
				"ip_range": {
					Type:     schema.TypeString,
					Required: true,
					ForceNew: true,
				},
				"gateway": {
					Type:     schema.TypeString,
					Computed: true,
				},
				"vswitch_id": {
					Type:     schema.TypeInt,
					Optional: true,
					ForceNew: true,
				},
			},
		}
	}

	tfProvider := &schema.Provider{
		Schema: map[string]*schema.Schema{},
		ResourcesMap: map[string]*schema.Resource{
			"hcloud_subnet": subnetResource(),
		},
	}

	p := shimv2.NewProvider(tfProvider, shimv2.WithDiffStrategy(shimv2.PlanState))

	info := tfbridge.ProviderInfo{
		P:           p,
		Name:        "hcloud",
		Description: "etc",
		Keywords:    []string{"pulumi", "hcloud"},
		License:     "Apache-2.0",
		Version:     "0.0.1",
		Resources: map[string]*tfbridge.ResourceInfo{
			"hcloud_subnet": {Tok: "hcloud:index/networkSubnet:NetworkSubnet"},
		},
	}

	server := tfbridge.NewProvider(ctx,
		nil,      /* hostClient */
		"hcloud", /* module */
		"",       /* version */
		p,        /* tf */
		info,     /* info */
		[]byte{}, /* pulumiSchema */
	)

	testCase := `
	{
	  "method": "/pulumirpc.ResourceProvider/Create",
	  "request": {
	    "urn": "urn:pulumi:dev::repro-175::hcloud:index/networkSubnet:NetworkSubnet::subnet",
	    "properties": {
	      "__defaults": [],
	      "ipRange": "10.0.1.0/24",
	      "networkId": "04da6b54-80e4-46f7-96ec-b56ff0331ba9",
	      "networkZone": "eu-central",
	      "type": "cloud"
	    },
	    "preview": true
	  },
	  "errors": [
	    "rpc error: code = Unknown desc = diffing urn:pulumi:dev::repro-175::hcloud:index/networkSubnet:NetworkSubnet::subnet: value is not known"
	  ]
	}`
	testutils.Replay(t, server, testCase)

}
