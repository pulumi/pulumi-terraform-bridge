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
func ProviderMiniRandom() *schema.Provider {
	resourceInteger := func() *schema.Resource {
		return &schema.Resource{
			Description: "The resource `random_integer` generates random values from a given range, described " +
				"by the `min` and `max` attributes of a given resource.\n" +
				"\n" +
				"This resource can be used in conjunction with resources that have the `create_before_destroy` " +
				"lifecycle flag set, to avoid conflicts with unique names during the brief period where both the " +
				"old and new resources exist concurrently.",

			Schema: map[string]*schema.Schema{
				"keepers": {
					Description: "Arbitrary map of values that, when changed, will trigger recreation of " +
						"resource. See [the main provider documentation](../index.html) for more information.",
					Type:     schema.TypeMap,
					Optional: true,
					ForceNew: true,
				},

				"min": {
					Description: "The minimum inclusive value of the range.",
					Type:        schema.TypeInt,
					Required:    true,
					ForceNew:    true,
				},

				"max": {
					Description: "The maximum inclusive value of the range.",
					Type:        schema.TypeInt,
					Required:    true,
					ForceNew:    true,
				},

				"seed": {
					Description: "A custom seed to always produce the same value.",
					Type:        schema.TypeString,
					Optional:    true,
					ForceNew:    true,
				},

				"result": {
					Description: "The random integer result.",
					Type:        schema.TypeInt,
					Computed:    true,
				},

				"id": {
					Type:     schema.TypeString,
					Computed: true,
				},
			},
		}
	}

	return &schema.Provider{
		Schema: map[string]*schema.Schema{},
		ResourcesMap: map[string]*schema.Resource{
			"random_integer": resourceInteger(),
		},
	}
}
