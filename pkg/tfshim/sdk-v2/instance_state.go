package sdkv2

import (
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	shim "github.com/pulumi/pulumi-terraform-bridge/v2/pkg/tfshim"
)

var _ = shim.InstanceState(v2InstanceState{})

type v2InstanceState struct {
	tf *terraform.InstanceState
}

func NewInstanceState(s *terraform.InstanceState) shim.InstanceState {
	return v2InstanceState{s}
}

func IsInstanceState(s shim.InstanceState) (*terraform.InstanceState, bool) {
	if is, ok := s.(v2InstanceState); ok {
		return is.tf, true
	}
	return nil, false
}

func (s v2InstanceState) Type() string {
	return s.tf.Ephemeral.Type
}

func (s v2InstanceState) ID() string {
	return s.tf.ID
}

func (s v2InstanceState) Object(sch shim.SchemaMap) (map[string]interface{}, error) {
	obj := make(map[string]interface{})
	attrs := s.tf.Attributes

	reader := &schema.MapFieldReader{
		Schema: map[string]*schema.Schema(sch.(v2SchemaMap)),
		Map:    schema.BasicMapReader(attrs),
	}

	// Read each top-level field out of the attributes.
	keys := make(map[string]bool)
	for key := range attrs {
		// Pull the top-level field out of this attribute key. If we've already read the top-level field, skip this
		// key.
		dot := strings.Index(key, ".")
		if dot != -1 {
			key = key[:dot]
		}
		if _, ok := keys[key]; ok {
			continue
		}
		keys[key] = true

		// Read the top-level attribute for this key.
		res, err := reader.ReadField([]string{key})
		if err != nil {
			return nil, err
		}
		if res.Value != nil {
			obj[key] = res.Value
		}
	}

	// Populate the "id" property if it is not set. Most schemas do not include this property, and leaving it out
	// can cause unnecessary diffs when refreshing/updating resources after a provider upgrade.
	if _, ok := obj["id"]; !ok {
		obj["id"] = attrs["id"]
	}

	return obj, nil
}

func (s v2InstanceState) Meta() map[string]interface{} {
	return s.tf.Meta
}
