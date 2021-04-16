package sdkv1

import (
	"github.com/hashicorp/terraform-plugin-sdk/terraform"

	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

var _ = shim.ResourceConfig(v1ResourceConfig{})

type v1ResourceConfig struct {
	tf *terraform.ResourceConfig
}

func (c v1ResourceConfig) IsSet(key string) bool {
	return c.tf.IsSet(key)
}
