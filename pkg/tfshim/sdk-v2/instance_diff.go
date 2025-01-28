package sdkv2

import (
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"

	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

func resourceAttrDiffToShim(d *terraform.ResourceAttrDiff) *shim.ResourceAttrDiff {
	if d == nil {
		return nil
	}
	return &shim.ResourceAttrDiff{
		Old:         d.Old,
		New:         d.New,
		NewComputed: d.NewComputed,
		NewRemoved:  d.NewRemoved,
		NewExtra:    d.NewExtra,
		RequiresNew: d.RequiresNew,
		Sensitive:   d.Sensitive,
		Type:        shim.DiffAttrUnknown,
	}
}
