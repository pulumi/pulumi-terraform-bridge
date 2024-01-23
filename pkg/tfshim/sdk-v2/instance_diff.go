package sdkv2

import (
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
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
}

func (d v2InstanceDiff) Attribute(key string) *shim.ResourceAttrDiff {
	return resourceAttrDiffToShim(d.tf.Attributes[key])
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
	var prior *terraform.InstanceState
	if priorState != nil {
		prior = priorState.(v2InstanceState).tf
	} else {
		prior = &terraform.InstanceState{
			Attributes: map[string]string{},
			Meta:       map[string]interface{}{},
		}
	}

	return v2InstanceState{
		resource: res.(v2Resource).tf,
		tf:       prior,
		diff:     d.tf,
	}, nil
}

func (d v2InstanceDiff) Destroy() bool {
	return d.tf.Destroy
}

func (d v2InstanceDiff) RequiresNew() bool {
	return d.tf.RequiresNew()
}

func (d v2InstanceDiff) IgnoreChanges(ignored map[string]bool) {
	for k := range d.tf.Attributes {
		if ignored[k] {
			delete(d.tf.Attributes, k)
		} else {
			for attr := range ignored {
				if strings.HasPrefix(k, attr+".") {
					delete(d.tf.Attributes, k)
					break
				}
			}
		}
	}
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

func (d v2InstanceDiff) EncodeTimeouts(timeouts *shim.ResourceTimeout) error {
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

func (d v2InstanceDiff) SetTimeout(timeout float64, timeoutKey string) {
	timeoutValue := int64(timeout * 1000000000) //this turns seconds to nanoseconds - TF wants it in this format

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
			timeoutKey: timeoutValue,
		}
	} else {
		timeouts[timeoutKey] = timeoutValue
	}
}
