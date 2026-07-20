package main

import (
	"fmt"

	goversion "github.com/hashicorp/go-version"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/plugin"
	"github.com/hashicorp/terraform-plugin-sdk/terraform"
)

func Provider() *schema.Provider {
	p := &schema.Provider{
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
	p.ConfigureFunc = func(*schema.ResourceData) (any, error) {
		terraformVersion, err := goversion.NewVersion(p.TerraformVersion)
		if err != nil {
			return nil, fmt.Errorf("invalid Terraform version %q: %w", p.TerraformVersion, err)
		}
		minimumVersion := goversion.Must(goversion.NewVersion("1.0.0"))
		if terraformVersion.LessThan(minimumVersion) {
			return nil, fmt.Errorf("terraform version %s is below minimum version %s",
				terraformVersion, minimumVersion)
		}
		return nil, nil
	}
	return p
}

func main() {
	plugin.Serve(&plugin.ServeOpts{
		ProviderFunc: func() terraform.ResourceProvider {
			return Provider()
		},
	})
}
