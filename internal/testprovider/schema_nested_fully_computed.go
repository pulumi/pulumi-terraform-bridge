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

func ProviderNestedFullyComputed() *schema.Provider {
	resourceNestedFullyComputedFunc := func() *schema.Resource {
		return &schema.Resource{
			Schema: resourceNestedFullyComputedSchema(),
		}
	}

	return &schema.Provider{
		Schema: map[string]*schema.Schema{},
		ResourcesMap: map[string]*schema.Resource{
			"testprovider_res": resourceNestedFullyComputedFunc(),
		},
	}
}

func resourceNestedFullyComputedSchema() map[string]*schema.Schema {
	return map[string]*schema.Schema{
		"block": {
			Type:     schema.TypeList,
			Required: true,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"a1": {
						Type:     schema.TypeString,
						Computed: true,
					},
					"a2": {
						Type:     schema.TypeString,
						Optional: true,
					},
					"a3": {
						Type:     schema.TypeString,
						Required: true,
					},
				},
			},
		},
		"other_block": {
			Type:     schema.TypeList,
			Optional: true,
			MaxItems: 1,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"b1": {
						Type:     schema.TypeString,
						Computed: true,
					},
					"b2": {
						Type:     schema.TypeString,
						Required: true,
					},
				},
			},
		},
	}
}
