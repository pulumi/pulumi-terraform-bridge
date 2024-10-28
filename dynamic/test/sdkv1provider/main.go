package main

import (
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/plugin"
	"github.com/hashicorp/terraform-plugin-sdk/terraform"
)

func Provider() *schema.Provider {
	return &schema.Provider{
		ConfigureFunc: func(*schema.ResourceData) (any, error) {
			return nil, nil
		},
		ResourcesMap: map[string]*schema.Resource{
			"test_res": {
				Schema: map[string]*schema.Schema{
					"f0": {Type: schema.TypeString, Required: true},
					"f1": {Type: schema.TypeInt, Computed: true},
				},
				Update: func(*schema.ResourceData, interface{}) error {
					return nil
				},
				Delete: func(*schema.ResourceData, any) error {
					return nil
				},
			},
		},
	}
}

func main() {
	plugin.Serve(&plugin.ServeOpts{
		ProviderFunc: func() terraform.ResourceProvider {
			return Provider()
		},
	})
}
