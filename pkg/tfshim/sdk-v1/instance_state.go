package sdkv1

import (
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/terraform"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	diff_reader "github.com/pulumi/terraform-diff-reader/sdk-v1"
)

var _ = shim.InstanceState(v1InstanceState{})

type v1InstanceState struct {
	tf   *terraform.InstanceState
	diff *terraform.InstanceDiff
}

func NewInstanceState(s *terraform.InstanceState) shim.InstanceState {
	return v1InstanceState{s, nil}
}

func IsInstanceState(s shim.InstanceState) (*terraform.InstanceState, bool) {
	if is, ok := s.(v1InstanceState); ok {
		return is.tf, true
	}
	return nil, false
}

func (s v1InstanceState) Type() string {
	return s.tf.Ephemeral.Type
}

func (s v1InstanceState) ID() string {
	return s.tf.ID
}

func (s v1InstanceState) Object(sch shim.SchemaMap) (map[string]interface{}, error) {
	obj := make(map[string]interface{})

	schemaMap := map[string]*schema.Schema(sch.(v1SchemaMap))

	attrs := s.tf.Attributes

	var reader schema.FieldReader = &schema.MapFieldReader{
		Schema: schemaMap,
		Map:    schema.BasicMapReader(attrs),
	}

	// If this is a state + a diff, use a diff reader rather than a map reader.
	if s.diff != nil {
		reader = &diff_reader.DiffFieldReader{
			Diff:   s.diff,
			Schema: schemaMap,
			Source: reader,
		}
	}

	// Read each top-level field out of the attributes.
	keys := make(map[string]bool)
	readAttributeField := func(key string) error {
		// Pull the top-level field out of this attribute key. If we've already read the top-level field, skip this
		// key.
		dot := strings.Index(key, ".")
		if dot != -1 {
			key = key[:dot]
		}
		if _, ok := keys[key]; ok {
			return nil
		}
		keys[key] = true

		// Read the top-level attribute for this key.
		res, err := reader.ReadField([]string{key})
		if err != nil {
			return err
		}
		if res.Value != nil && !res.Computed {
			obj[key] = res.Value
		}
		return nil
	}

	for key := range attrs {
		if err := readAttributeField(key); err != nil {
			return nil, err
		}
	}
	if s.diff != nil {
		for key := range s.diff.Attributes {
			if err := readAttributeField(key); err != nil {
				return nil, err
			}
		}
	}

	// Populate the "id" property if it is not set. Most schemas do not include this property, and leaving it out
	// can cause unnecessary diffs when refreshing/updating resources after a provider upgrade.
	if _, ok := obj["id"]; !ok {
		obj["id"] = attrs["id"]
	}

	return obj, nil
}

func (s v1InstanceState) Meta() map[string]interface{} {
	return s.tf.Meta
}
