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
	"bytes"
	"encoding/json"
	"strings"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	diff_reader "github.com/pulumi/terraform-diff-reader/sdk-v2"

	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

var _ = shim.InstanceState(v2InstanceState{})

type v2InstanceState struct {
	resource *schema.Resource
	tf       *terraform.InstanceState
	diff     *terraform.InstanceDiff
}

func NewInstanceState(s *terraform.InstanceState) shim.InstanceState {
	return v2InstanceState{tf: s}
}

func NewInstanceStateForResource(s *terraform.InstanceState, resource *schema.Resource) shim.InstanceState {
	return v2InstanceState{
		resource: resource,
		tf:       s,
		diff:     nil,
	}
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
	return s.objectV1(sch)
}

// This is needed because json.Unmarshal uses float64 for numbers by default which truncates int64 numbers.
func unmarshalJSON(data []byte, v interface{}) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	return dec.Decode(v)
}

// objectFromCtyValue takes a cty.Value and converts it to JSON object.
// We do not care about type checking the values, we just want to do our best to recursively convert
// the cty.Value to the underlying value
//
// NOTE: one of the transforms this needs to handle is converting unknown values.
// cty.Value that are also unknown cannot be converted to their underlying value. To get
// around this we just convert to a sentinel, which so far does not seem to cause any issues downstream
func objectFromCtyValue(v cty.Value) map[string]interface{} {
	var path cty.Path
	buf := &bytes.Buffer{}
	// The round trip here to JSON is redundant, we could instead convert from cty to map[string]interface{} directly
	err := marshal(v, v.Type(), path, buf)
	contract.AssertNoErrorf(err, "Failed to marshal cty.Value to a JSON string value")

	var m map[string]interface{}
	err = unmarshalJSON(buf.Bytes(), &m)
	contract.AssertNoErrorf(err, "failed to unmarshal: %s", buf.String())

	return m
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
