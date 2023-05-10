// Copyright 2016-2022, Pulumi Corporation.
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

package testprovider

import (
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// Subset of pulumi-random provider.
func ProviderMiniCloudflare() *schema.Provider {
	resourceRuleset := func() *schema.Resource {
		return &schema.Resource{
			Description: "Deploy a ruleset for cloudflare",
			Schema:      resourceCloudflareRulesetSchema(),
		}
	}

	return &schema.Provider{
		Schema: map[string]*schema.Schema{},
		ResourcesMap: map[string]*schema.Resource{
			"cloudflare_ruleset": resourceRuleset(),
		},
	}
}

// An aggressively cut down version of cloudflare_ruleset.
func resourceCloudflareRulesetSchema() map[string]*schema.Schema {
	return map[string]*schema.Schema{
		"name": {
			Type:        schema.TypeString,
			Required:    true,
			ForceNew:    true,
			Description: "Name of the ruleset.",
		},
		"description": {
			Type:        schema.TypeString,
			Optional:    true,
			Description: "Brief summary of the ruleset and its intended use.",
		},
		"rules": {
			Type:        schema.TypeList,
			Optional:    true,
			Description: "List of rules to apply to the ruleset.",
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"id": {
						Type:        schema.TypeString,
						Computed:    true,
						Description: "Unique rule identifier.",
					},
					"version": {
						Type:        schema.TypeString,
						Computed:    true,
						Description: "Version of the ruleset to deploy.",
					},
					"action_parameters": {
						Type: schema.TypeList,
						// MaxItems:    1,
						Optional:    true,
						Description: "List of parameters that configure the behavior of the ruleset rule action.",
						Elem: &schema.Resource{
							Schema: map[string]*schema.Schema{
								"id": {
									Type:        schema.TypeString,
									Optional:    true,
									Description: "Identifier of the action parameter to modify.",
								},
								"phases": {
									Type:     schema.TypeList,
									Optional: true,
									Elem: &schema.Resource{
										Schema: map[string]*schema.Schema{
											"phase1": {Type: schema.TypeString},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}
