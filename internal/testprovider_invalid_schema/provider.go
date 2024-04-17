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

package tpinvschema

import (
	"context"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// An assortment of invalid resources according to
// https://github.com/pulumi/terraform-plugin-sdk/blob/b374785cb6462f8d89eb31d4874a9e3228d74633/helper/schema/resource.go#L1132
//
//nolint:lll
func Provider() *schema.Provider {
	return &schema.Provider{
		Schema: map[string]*schema.Schema{},
		ResourcesMap: map[string]*schema.Resource{
			"invalid_res": {
				Schema: map[string]*schema.Schema{
					"opt_and_req_prop": {
						Type:     schema.TypeString,
						Optional: true,
						Required: true,
					},
				},
				Create: func(*schema.ResourceData, interface{}) error { return nil },
				Update: func(*schema.ResourceData, interface{}) error { return nil },
				Read:   func(*schema.ResourceData, interface{}) error { return nil },
				Delete: func(*schema.ResourceData, interface{}) error { return nil },
			},
			"no_read_res": {
				Schema: map[string]*schema.Schema{},
				Create: func(*schema.ResourceData, interface{}) error { return nil },
				Delete: func(*schema.ResourceData, interface{}) error { return nil },
			},
			"block_string_res": {
				Schema: map[string]*schema.Schema{
					"block_string_prop": {
						Type:       schema.TypeString,
						Required:   true,
						ConfigMode: schema.SchemaConfigModeBlock,
						Elem: &schema.Schema{
							Type: schema.TypeString,
						},
					},
				},
				Create: func(*schema.ResourceData, interface{}) error { return nil },
				Update: func(*schema.ResourceData, interface{}) error { return nil },
				Read:   func(*schema.ResourceData, interface{}) error { return nil },
				Delete: func(*schema.ResourceData, interface{}) error { return nil },
			},
			"computed_default_res": {
				Schema: map[string]*schema.Schema{
					"computed_default_prop": {
						Type:     schema.TypeString,
						Computed: true,
						Default:  "default",
					},
				},
				Create: func(*schema.ResourceData, interface{}) error { return nil },
				Read:   func(*schema.ResourceData, interface{}) error { return nil },
				Delete: func(*schema.ResourceData, interface{}) error { return nil },
			},
			"max_string_res": {
				Schema: map[string]*schema.Schema{
					"max_string_prop": {
						Type:     schema.TypeString,
						MaxItems: 1,
						Optional: true,
					},
				},
				Create: func(*schema.ResourceData, interface{}) error { return nil },
				Update: func(*schema.ResourceData, interface{}) error { return nil },
				Read:   func(*schema.ResourceData, interface{}) error { return nil },
				Delete: func(*schema.ResourceData, interface{}) error { return nil },
			},
			"create_and_create_context_res": {
				Schema: map[string]*schema.Schema{},
				Create: func(*schema.ResourceData, interface{}) error { return nil },
				Read:   func(*schema.ResourceData, interface{}) error { return nil },
				Delete: func(*schema.ResourceData, interface{}) error { return nil },
				CreateContext: func(context.Context, *schema.ResourceData, interface{}) diag.Diagnostics {
					return nil
				},
			},
		},
	}
}
