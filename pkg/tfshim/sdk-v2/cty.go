package sdkv2

import (
	"github.com/golang/glog"
	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func makeResourceRawConfig(config *terraform.ResourceConfig, resource *schema.Resource) cty.Value {
	original := schema.HCL2ValueFromConfigValue(config.Raw)
	coerced, err := resource.CoreConfigSchema().CoerceValue(original)
	if err != nil {
		glog.V(9).Infof("failed to coerce config: %w", err)
		return original
	}
	return coerced
}
