package sdkv2

import (
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"

	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

func configFromShim(c shim.ResourceConfig) *terraform.ResourceConfig {
	if c == nil {
		return nil
	}
	return c.(v2ResourceConfig).tf
}

func stateToShim(r *schema.Resource, s *terraform.InstanceState) shim.InstanceState {
	if s == nil {
		return nil
	}
	return NewInstanceStateForResource(s, r)
}

func diffFromShim(d shim.InstanceDiff) *terraform.InstanceDiff {
	if d == nil {
		return nil
	}
	return d.(*v2InstanceDiff2).tf
}

func diffToShim(d *terraform.InstanceDiff) shim.InstanceDiff {
	if d == nil {
		return nil
	}
	return &v2InstanceDiff2{tf: d}
}
