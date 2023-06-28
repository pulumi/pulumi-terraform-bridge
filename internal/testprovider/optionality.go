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

func ProviderOptionality() *schema.Provider {
	res := func() *schema.Resource {
		return &schema.Resource{
			Description: "This is a provider to test how we decide what properties required vs optional.",

			Schema: map[string]*schema.Schema{
				"o1": {
					Description: "A computed integer. This should be optional.",
					Type:        schema.TypeInt,
					Computed:    true,
				},
				"o2": {
					Description: "A computed or maybe user provided integer. " +
						"This should be optional, since both the user and " +
						"the provider can choose to not fill it.",
					Type:     schema.TypeInt,
					Computed: true,
					Optional: true,
				},
				"o3": {
					Description: "A maybe user provided integer. This should be optional.",
					Type:        schema.TypeInt,
					Optional:    true,
				},
				"r1": {
					Description: "A user provided integer. This should be required " +
						"as an input and output.",
					Type:     schema.TypeInt,
					Required: true,
				},
			},
		}
	}

	return &schema.Provider{
		Schema: map[string]*schema.Schema{},
		ResourcesMap: map[string]*schema.Resource{
			"res": res(),
		},
	}
}
