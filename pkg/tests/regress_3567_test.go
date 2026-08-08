// Copyright 2016-2026, Pulumi Corporation.
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
	testutils "github.com/pulumi/providertest/replay"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	shimv2 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v2"
)

// Refresh must round-trip the __defaults marker instead of dropping it when it is empty.
//
// Dropping an empty __defaults list is not a no-op: an absent marker means "every old value may be
// reused as a default" and additionally marks the resource as autonamed, whereas an empty marker
// means "no old value is a default". Dropping it also makes the refreshed state disagree with the
// inputs the program produces on the next update, which defeats the engine's checkpoint-write
// elision and rewrites the whole state file once per unchanged resource.
//
// See https://github.com/pulumi/pulumi-terraform-bridge/issues/3567.
func TestRegress3567(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	res := &schema.Resource{
		Read: func(d *schema.ResourceData, meta interface{}) error {
			if err := d.Set("name", "n1"); err != nil {
				return err
			}
			return d.Set("settings", []interface{}{
				map[string]interface{}{"enabled": true},
			})
		},
		Schema: map[string]*schema.Schema{
			"name": {
				Type:     schema.TypeString,
				Required: true,
			},
			"settings": {
				Type:     schema.TypeList,
				Optional: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"enabled": {
							Type:     schema.TypeBool,
							Optional: true,
						},
					},
				},
			},
		},
	}

	tfProvider := &schema.Provider{
		Schema:       map[string]*schema.Schema{},
		ResourcesMap: map[string]*schema.Resource{"test_res": res},
	}

	p := shimv2.NewProvider(tfProvider)

	info := tfbridge.ProviderInfo{
		P:         p,
		Name:      "test",
		Version:   "0.0.1",
		Resources: map[string]*tfbridge.ResourceInfo{"test_res": {Tok: "test:index/res:Res"}},
	}

	server := tfbridge.NewProvider(ctx,
		nil,      /* hostClient */
		"test",   /* module */
		"",       /* version */
		p,        /* tf */
		info,     /* info */
		[]byte{}, /* pulumiSchema */
	)

	testCase := `
	{
	  "method": "/pulumirpc.ResourceProvider/Read",
	  "request": {
	    "id": "res1",
	    "urn": "urn:pulumi:dev::bridge-3567::test:index/res:Res::res",
	    "properties": {
	      "id": "res1",
	      "name": "n1",
	      "settings": {
		"enabled": true
	      }
	    },
	    "inputs": {
	      "__defaults": [],
	      "name": "n1",
	      "settings": {
		"__defaults": [],
		"enabled": true
	      }
	    }
	  },
	  "response": {
	    "id": "res1",
	    "inputs": {
	      "__defaults": [],
	      "name": "n1",
	      "settings": {
		"__defaults": [],
		"enabled": true
	      }
	    },
	    "properties": {
	      "id": "res1",
	      "name": "n1",
	      "settings": {
		"enabled": true
	      },
	      "*": "*"
	    }
	  }
	}
	`
	testutils.Replay(t, server, testCase)
}
