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

	"github.com/hashicorp/go-cty/cty"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

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
