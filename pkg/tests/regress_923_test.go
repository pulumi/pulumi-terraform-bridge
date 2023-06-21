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
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	testutils "github.com/pulumi/pulumi-terraform-bridge/testing/x"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	shimv2 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v2"
)

func TestRegress923(t *testing.T) {
	ctx := context.Background()

	resource := &schema.Resource{
		Read: func(d *schema.ResourceData, meta interface{}) error {
			if err := d.Set("name", "webhookname"); err != nil {
				return err
			}
			if strings.Contains(d.Id(), "webhooks") {
				return fmt.Errorf("ID should not contain 'webhooks'")
			}
			return nil
		},
		Schema: map[string]*schema.Schema{
			"name": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
		},
		SchemaVersion: 1,
		StateUpgraders: []schema.StateUpgrader{
			{
				Version: 0,
			},
		},
	}

	resource.StateUpgraders = []schema.StateUpgrader{
		{
			Version: 0,
			Type:    resource.CoreConfigSchema().ImpliedType(),
			Upgrade: func(
				ctx context.Context,
				rawState map[string]interface{},
				meta interface{},
			) (map[string]interface{}, error) {
				copy := map[string]interface{}{}
				for k, v := range rawState {
					if k == "id" {
						v = strings.ReplaceAll(v.(string), "webhooks", "webHooks")
					}
					copy[k] = v
				}
				return copy, nil
			},
		},
	}

	tfProvider := &schema.Provider{
		Schema: map[string]*schema.Schema{},
		ResourcesMap: map[string]*schema.Resource{
			"az_webhook": resource,
		},
	}

	p := shimv2.NewProvider(tfProvider)

	info := tfbridge.ProviderInfo{
		P:          p,
		Name:       "azure",
		Keywords:   []string{"pulumi", "azure"},
		License:    "Apache-2.0",
		Homepage:   "https://pulumi.io",
		Repository: "https://github.com/pulumi/pulumi-azure",
		Version:    "0.0.2",
		Resources: map[string]*tfbridge.ResourceInfo{
			"az_webhook": {Tok: "azure:containerservice/registryWebhook:RegistryWebhook"},
		},
	}

	server := tfbridge.NewProvider(ctx,
		nil,      /* hostClient */
		"azure",  /* module */
		"",       /* version */
		p,        /* tf */
		info,     /* info */
		[]byte{}, /* pulumiSchema */
	)

	testCase := `
	{
	  "method": "/pulumirpc.ResourceProvider/Read",
	  "request": {
	    "id": "/subscriptions/0282681f-7a9e-424b-80b2-96babd57a8a1/resourceGroups/example9e974ca4/providers/Microsoft.ContainerRegistry/registries/acr1963930c/webhooks/webhookca218fd",
	    "urn": "urn:pulumi:dev::bridge-923::azure:containerservice/registryWebhook:RegistryWebhook::webhook",
	    "properties": {
	      "__meta": "{\"e2bfb730-ecaa-11e6-8f88-34363bc7c4c0\":{\"create\":1800000000000,\"delete\":1800000000000,\"read\":300000000000,\"update\":1800000000000}}",
	      "actions": [
		"push"
	      ],
	      "customHeaders": {
		"Content-Type": "application/json"
	      },
	      "id": "/subscriptions/0282681f-7a9e-424b-80b2-96babd57a8a1/resourceGroups/example9e974ca4/providers/Microsoft.ContainerRegistry/registries/acr1963930c/webhooks/webhookca218fd",
	      "location": "eastus",
	      "name": "webhookca218fd",
	      "registryName": "acr1963930c",
	      "resourceGroupName": "example9e974ca4",
	      "scope": "mytag:*",
	      "serviceUri": "https://mywebhookreceiver.example/mytag",
	      "status": "enabled",
	      "tags": {}
	    },
	    "inputs": {
	      "__defaults": [
		"name"
	      ],
	      "actions": [
		"push"
	      ],
	      "customHeaders": {
		"Content-Type": "application/json",
		"__defaults": []
	      },
	      "location": "eastus",
	      "name": "webhookca218fd",
	      "registryName": "acr1963930c",
	      "resourceGroupName": "example9e974ca4",
	      "scope": "mytag:*",
	      "serviceUri": "https://mywebhookreceiver.example/mytag",
	      "status": "enabled"
	    }
	  },
	  "response": {
            "id": "*",
            "inputs": {
              "__defaults": [],
              "name": "webhookname"
            },
            "properties": {
              "id": "*",
              "__meta": "*",
              "name": "webhookname"
            }
          }
	}
        `
	testutils.Replay(t, server, testCase)
}
