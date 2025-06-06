package sdkv2

import (
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/internal/internalinter"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

var _ = shim.InstanceDiff(v2InstanceDiff{})

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

type v2InstanceDiff struct {
	tf *terraform.InstanceDiff
	internalinter.Internal
}

func (d v2InstanceDiff) DiffEqualDecisionOverride() shim.DiffOverride {
	return shim.DiffNoOverride
}

func (d v2InstanceDiff) applyTimeoutOptions(opts shim.TimeoutOptions) {
	// This method is no longer used with PlanResourceChange; we handle timeouts more directly.
	if opts.ResourceTimeout != nil {
		err := d.encodeTimeouts(opts.ResourceTimeout)
		contract.AssertNoErrorf(err, "encodeTimeouts should never fail")
	}
	for timeoutKey, dur := range opts.TimeoutOverrides {
		d.setTimeout(dur, timeoutKey)
	}
}

func (d v2InstanceDiff) Attribute(key string) *shim.ResourceAttrDiff {
	return resourceAttrDiffToShim(d.tf.Attributes[key])
}

func (d v2InstanceDiff) HasNoChanges() bool {
	return len(d.Attributes()) == 0
}

func (d v2InstanceDiff) Attributes() map[string]shim.ResourceAttrDiff {
	m := map[string]shim.ResourceAttrDiff{}
	for k, v := range d.tf.Attributes {
		if v != nil {
			m[k] = *resourceAttrDiffToShim(v)
		}
	}
	return m
}

func (d v2InstanceDiff) ProposedState(res shim.Resource, priorState shim.InstanceState) (shim.InstanceState, error) {
	// In the current bridge versions, v2InstanceDiff is used by itself only when working with data source calls.
	// For data sources ProposedState is never called. Consider refactoring away from using shim.InstanceState to
	// represent those.
	//
	// When v2InstanceDiff is used as a struct embedded into v2InstanceDiff2, the outer struct re-implements
	// ProposedState() so again this method does not get called.
	contract.Failf("v2InstanceDiff().ProposedState() should not be called")
	return nil, nil
}

func (d v2InstanceDiff) PriorState() (shim.InstanceState, error) {
	return nil, fmt.Errorf("prior state is not available")
}

func (d v2InstanceDiff) Destroy() bool {
	return d.tf.Destroy
}

func (d v2InstanceDiff) RequiresNew() bool {
	return d.tf.RequiresNew()
}

func (d v2InstanceDiff) processIgnoreChanges(ignored shim.IgnoreChanges) {
	i := ignored()
	for k := range d.tf.Attributes {
		if _, ok := i[k]; ok {
			delete(d.tf.Attributes, k)
		} else {
			for attr := range i {
				if strings.HasPrefix(k, attr+".") {
					delete(d.tf.Attributes, k)
					break
				}
			}
		}
	}
}

func (d v2InstanceDiff) encodeTimeouts(timeouts *shim.ResourceTimeout) error {
	v2Timeouts := &schema.ResourceTimeout{}
	if timeouts != nil {
		v2Timeouts.Create = timeouts.Create
		v2Timeouts.Read = timeouts.Read
		v2Timeouts.Update = timeouts.Update
		v2Timeouts.Delete = timeouts.Delete
		v2Timeouts.Default = timeouts.Default
	}
	return v2Timeouts.DiffEncode(d.tf)
}

func (d v2InstanceDiff) setTimeout(timeout time.Duration, timeoutKey shim.TimeoutKey) {
	// this turns seconds to nanoseconds - TF wants it in this format
	timeoutValue := timeout.Nanoseconds()

	switch timeoutKey {
	case shim.TimeoutCreate:
		timeoutKey = schema.TimeoutCreate
	case shim.TimeoutRead:
		timeoutKey = schema.TimeoutRead
	case shim.TimeoutUpdate:
		timeoutKey = schema.TimeoutUpdate
	case shim.TimeoutDelete:
		timeoutKey = schema.TimeoutDelete
	case shim.TimeoutDefault:
		timeoutKey = schema.TimeoutDefault
	default:
		return
	}

	if d.tf.Meta == nil {
		d.tf.Meta = map[string]interface{}{}
	}

	timeouts, ok := d.tf.Meta[schema.TimeoutKey].(map[string]interface{})
	if !ok {
		d.tf.Meta[schema.TimeoutKey] = map[string]interface{}{
			string(timeoutKey): timeoutValue,
		}
	} else {
		timeouts[string(timeoutKey)] = timeoutValue
	}
}
