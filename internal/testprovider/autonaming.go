// Copyright 2016-2024, Pulumi Corporation.
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

func AutonamingProvider() *schema.Provider {
	r := func(r func() map[string]*schema.Schema) *schema.Resource {
		return &schema.Resource{
			Schema: r(),
		}
	}

	return &schema.Provider{
		Schema: map[string]*schema.Schema{},
		ResourcesMap: map[string]*schema.Resource{
			"auto_v1": r(resourceAutoNamedV1),
			"auto_v2": r(resourceAutoNamedV2),
		},
	}
}

func resourceAutoNamedV1() map[string]*schema.Schema {
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
	}
}

func resourceAutoNamedV2() map[string]*schema.Schema {
	return map[string]*schema.Schema{
		"name": {
			Type:        schema.TypeList,
			Elem:        schema.TypeString,
			Required:    true,
			ForceNew:    true,
			Description: "Name of the ruleset.",
		},
		"description": {
			Type:        schema.TypeString,
			Optional:    true,
			Description: "Brief summary of the ruleset and its intended use.",
		},
	}
}
