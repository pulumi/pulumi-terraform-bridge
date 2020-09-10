package sdkv2

import (
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"

	shim "github.com/pulumi/pulumi-terraform-bridge/v2/pkg/tfshim"
)

var _ = shim.ResourceConfig(v2ResourceConfig{})

type v2ResourceConfig struct {
	tf *terraform.ResourceConfig
}

func (c v2ResourceConfig) IsSet(key string) bool {
	if c.tf == nil {
		return false
	}

	if c.tf.IsComputed(key) {
		return true
	}

	if _, ok := c.tf.Get(key); ok {
		return true
	}

	return false
}
