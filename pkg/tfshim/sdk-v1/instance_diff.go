package sdkv1

import (
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/terraform"

	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

var _ = shim.InstanceDiff(v1InstanceDiff{})

func resourceAttrDiffToShim(d *terraform.ResourceAttrDiff) *shim.ResourceAttrDiff {
	if d == nil {
		return nil
	}
	var t shim.DiffAttrType
	switch d.Type {
	case terraform.DiffAttrInput:
		t = shim.DiffAttrInput
	case terraform.DiffAttrOutput:
		t = shim.DiffAttrOutput
	default:
		t = shim.DiffAttrUnknown
	}

	return &shim.ResourceAttrDiff{
		Old:         d.Old,
		New:         d.New,
		NewComputed: d.NewComputed,
		NewRemoved:  d.NewRemoved,
		NewExtra:    d.NewExtra,
		RequiresNew: d.RequiresNew,
		Sensitive:   d.Sensitive,
		Type:        t,
	}
}

type v1InstanceDiff struct {
	tf *terraform.InstanceDiff
}

func (d v1InstanceDiff) applyTimeoutOptions(opts shim.TimeoutOptions) {
	if opts.ResourceTimeout != nil {
		err := d.encodeTimeouts(opts.ResourceTimeout)
		contract.AssertNoErrorf(err, "encodeTimeouts should never fail")
	}
	for timeoutKey, dur := range opts.TimeoutOverrides {
		d.setTimeout(dur, timeoutKey)
	}
}

func (d v1InstanceDiff) Attribute(key string) *shim.ResourceAttrDiff {
	return resourceAttrDiffToShim(d.tf.Attributes[key])
}

func (d v1InstanceDiff) Attributes() map[string]shim.ResourceAttrDiff {
	m := map[string]shim.ResourceAttrDiff{}
	for k, v := range d.tf.Attributes {
		if v != nil {
			m[k] = *resourceAttrDiffToShim(v)
		}
	}
	return m
}

func (d v1InstanceDiff) ProposedState(res shim.Resource, priorState shim.InstanceState) (shim.InstanceState, error) {
	var prior *terraform.InstanceState
	if priorState != nil {
		prior = priorState.(v1InstanceState).tf
	} else {
		prior = &terraform.InstanceState{
			Attributes: map[string]string{},
			Meta:       map[string]interface{}{},
		}
	}

	return v1InstanceState{tf: prior, diff: d.tf}, nil
}

func (d v1InstanceDiff) Destroy() bool {
	return d.tf.Destroy
}

func (d v1InstanceDiff) RequiresNew() bool {
	return d.tf.RequiresNew()
}

func (d v1InstanceDiff) processIgnoreChanges(ignored shim.IgnoreChanges) {
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

func (d v1InstanceDiff) encodeTimeouts(timeouts *shim.ResourceTimeout) error {
	v1Timeouts := &schema.ResourceTimeout{}
	if timeouts != nil {
		v1Timeouts.Create = timeouts.Create
		v1Timeouts.Read = timeouts.Read
		v1Timeouts.Update = timeouts.Update
		v1Timeouts.Delete = timeouts.Delete
		v1Timeouts.Default = timeouts.Default
	}
	return v1Timeouts.DiffEncode(d.tf)
}

func (d v1InstanceDiff) setTimeout(timeout time.Duration, timeoutKey shim.TimeoutKey) {
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
