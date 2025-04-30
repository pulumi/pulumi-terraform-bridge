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

func ProviderRequiredInputWithDefaultFunc() *schema.Provider {
	resourceRequiredInputWithDefaultFunc := func() *schema.Resource {
		return &schema.Resource{
			Schema: resourceRequiredInputWithDefaultSchema(),
		}
	}

	return &schema.Provider{
		Schema: map[string]*schema.Schema{},
		ResourcesMap: map[string]*schema.Resource{
			"testprovider_res": resourceRequiredInputWithDefaultFunc(),
		},
	}
}

func resourceRequiredInputWithDefaultSchema() map[string]*schema.Schema {
	return map[string]*schema.Schema{
		"name": {
			Type:     schema.TypeString,
			Required: true,
			DefaultFunc: func() (interface{}, error) {
				return "default", nil
			},
		},
		"other_name": {
			Type:     schema.TypeString,
			Required: true,
			Default:  "default",
		},
		"req": {
			Type:     schema.TypeString,
			Required: true,
		},
	}
}
