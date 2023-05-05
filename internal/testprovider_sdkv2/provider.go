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

package tpsdkv2

import (
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func Provider() *schema.Provider {
	objectSchema := &schema.Schema{
		Type:     schema.TypeList,
		Optional: true,
		Computed: true,
		MaxItems: 1,
		Elem: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"str_nested_prop": {
					Type:      schema.TypeString,
					Optional:  true,
					Sensitive: true,
				},
				"bool_nested_prop": {
					Type:     schema.TypeBool,
					Optional: true,
				},
			},
		},
	}

	secretObjectSchema := &schema.Schema{
		Type:     schema.TypeList,
		MaxItems: 1,
		Optional: true,
		Computed: true,
		Elem: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"str_nested_secret_prop": {
					Type:      schema.TypeString,
					Optional:  true,
					Sensitive: true,
				},
				"bool_nested_secret_prop": {
					Type:      schema.TypeBool,
					Optional:  true,
					Sensitive: true,
				},
			},
		},
	}

	return &schema.Provider{
		Schema: map[string]*schema.Schema{
			"str_config_prop": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},

			"secret_str_config_prop": {
				Type:      schema.TypeString,
				Sensitive: true,
				Optional:  true,
				Computed:  true,
			},

			"bool_config_prop": {
				Type:     schema.TypeBool,
				Optional: true,
				Computed: true,
			},

			"secret_bool_config_prop": {
				Type:      schema.TypeBool,
				Sensitive: true,
				Optional:  true,
				Computed:  true,
			},

			"object_prop": objectSchema,

			"secret_object_prop": secretObjectSchema,
		},
	}
}
