// Package main is a tiny SDKv2 TF provider used by dynamic-bridge tests to
// reproduce the aws_cloudwatch_log_group `name` / `name_prefix`
// ConflictsWith pattern.
package main

import (
	"context"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/plugin"
)

func provider() *schema.Provider {
	return &schema.Provider{
		ResourcesMap: map[string]*schema.Resource{
			// log_group mirrors aws_cloudwatch_log_group's name / name_prefix
			// shape — both optional+computed and mutually exclusive via
			// ConflictsWith.
			"conflictsprovider_log_group": {
				Schema: map[string]*schema.Schema{
					"name": {
						Type:          schema.TypeString,
						Optional:      true,
						Computed:      true,
						ForceNew:      true,
						ConflictsWith: []string{"name_prefix"},
					},
					"name_prefix": {
						Type:          schema.TypeString,
						Optional:      true,
						Computed:      true,
						ForceNew:      true,
						ConflictsWith: []string{"name"},
					},
				},
				CreateContext: func(_ context.Context, d *schema.ResourceData, _ any) diag.Diagnostics {
					d.SetId("log-group-id")
					return nil
				},
				ReadContext:   func(_ context.Context, _ *schema.ResourceData, _ any) diag.Diagnostics { return nil },
				UpdateContext: func(_ context.Context, _ *schema.ResourceData, _ any) diag.Diagnostics { return nil },
				DeleteContext: func(_ context.Context, _ *schema.ResourceData, _ any) diag.Diagnostics { return nil },
			},
		},
	}
}

func main() {
	plugin.Serve(&plugin.ServeOpts{
		ProviderFunc: provider,
	})
}
