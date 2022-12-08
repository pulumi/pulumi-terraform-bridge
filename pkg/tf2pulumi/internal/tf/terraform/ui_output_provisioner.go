package terraform

import (
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tf2pulumi/internal/tf/addrs"
)

// ProvisionerUIOutput is an implementation of UIOutput that calls a hook
// for the output so that the hooks can handle it.
type ProvisionerUIOutput struct {
	InstanceAddr    addrs.AbsResourceInstance
	ProvisionerType string
	Hooks           []Hook
}

func (o *ProvisionerUIOutput) Output(msg string) {
	for _, h := range o.Hooks {
		h.ProvisionOutput(o.InstanceAddr, o.ProvisionerType, msg)
	}
}
