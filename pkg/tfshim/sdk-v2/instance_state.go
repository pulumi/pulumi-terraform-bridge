// Copyright 2016-2023, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sdkv2

import (
	"strings"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"

	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	diff_reader "github.com/pulumi/terraform-diff-reader/sdk-v2"
)

var _ = shim.InstanceState(v2InstanceState{})

type v2InstanceState struct {
	resource *schema.Resource
	tf       *terraform.InstanceState
	diff     *terraform.InstanceDiff
}

func NewInstanceState(resource *schema.Resource, s *terraform.InstanceState) shim.InstanceState {
	st := v2InstanceState{
		resource: resource,
		tf:       s,
		diff:     nil,
	}
	contract.Assertf(st.resource != nil, "v2InstanceState.resource != nil")
	return st
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
	strat := GetInstanceStateStrategy(v2Resource{s.resource})
	if strat == CtyInstanceState {
		return s.objectViaCty(sch)
	}
	return s.objectV1(sch)
}

// This version of Object uses TF built-ins AttrsAsObjectValue and schema.ApplyDiff with cty.Value intermediate form.
func (s v2InstanceState) objectViaCty(sch shim.SchemaMap) (map[string]interface{}, error) {
	rSchema := s.resource.CoreConfigSchema()
	ty := rSchema.ImpliedType()

	// Read InstanceState as a cty.Value.
	v, err := s.tf.AttrsAsObjectValue(ty)
	contract.AssertNoErrorf(err, "AttrsAsObjectValue failed")

	// If there is a diff, apply it.
	if s.diff != nil {
		v, err = schema.ApplyDiff(v, s.diff, rSchema)
		contract.AssertNoErrorf(err, "schema.ApplyDiff failed")
	}

	// Now we need to translate cty.Value to a JSON-like form. This could have been avoided if surrounding Pulumi
	// code accepted a cty.Value and translated that to resource.PropertyValue, but that is currently not the case.
	//
	// An additional complication is that unkown values cannot serialize, so first replace them with sentinels.
	v, err = cty.Transform(v, func(_ cty.Path, v cty.Value) (cty.Value, error) {
		if !v.IsKnown() {
			return cty.StringVal(UnknownVariableValue), nil
		}
		return v, nil
	})
	contract.AssertNoErrorf(err, "Failed to encode unknowns with UnknownVariableValue")

	obj, err := schema.StateValueToJSONMap(v, v.Type())
	contract.AssertNoErrorf(err, "schema.StateValueToJSONMap failed")

	return obj, nil
}

// The legacy version of Object used custom Pulumi code forked from TF sources.
func (s v2InstanceState) objectV1(sch shim.SchemaMap) (map[string]interface{}, error) {
	obj := make(map[string]interface{})

	schemaMap := map[string]*schema.Schema(sch.(v2SchemaMap))

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
		// Pull the top-level field out of this attribute key. If we've already read the top-level field, skip
		// this key.
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

func (s v2InstanceState) Meta() map[string]interface{} {
	return s.tf.Meta
}
