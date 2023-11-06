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

package metadata

import (
	"encoding/json"

	jsoniter "github.com/json-iterator/go"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// The underlying value of a metadata blob.
type Data struct{ M map[string]json.RawMessage }

func New(data []byte) (*Data, error) {
	m := map[string]json.RawMessage{}
	if len(data) > 0 {
		jsoni := jsoniter.ConfigCompatibleWithStandardLibrary
		err := jsoni.Unmarshal(data, &m)
		if err != nil {
			return nil, err
		}
	}
	return &Data{m}, nil
}

func (d *Data) Marshal() []byte {
	if d == nil {
		d = &Data{M: make(map[string]json.RawMessage)}
	}
	bytes, err := json.MarshalIndent(d.M, "", "    ")
	// `d.m` is a `map[string]json.RawMessage`. `json.MarshalIndent` errors only when
	// it is asked to serialize an unmarshalable type (complex, function or channel)
	// or a cyclic data structure. Because `string` and `json.RawMessage` are
	// trivially marshallable and cannot contain cycles, all values of `d.m` can be
	// marshaled without error.
	//
	// See https://pkg.go.dev/encoding/json#Marshal for details.
	contract.AssertNoErrorf(err, "internal: failed to marshal metadata")
	return bytes
}

// Set a piece of metadata to a value.
//
// Set errors only if value fails to serialize.
func Set(d *Data, key string, value any) error {
	if value == nil {
		delete(d.M, key)
		return nil
	}
	jsoni := jsoniter.ConfigCompatibleWithStandardLibrary
	data, err := jsoni.Marshal(value)
	if err != nil {
		return err
	}
	msg := json.RawMessage(data)
	d.M[key] = msg
	return nil
}

func Get[T any](d *Data, key string) (T, bool, error) {
	data, ok := d.M[key]
	var t T
	if !ok {
		return t, false, nil
	}
	jsoni := jsoniter.ConfigCompatibleWithStandardLibrary
	err := jsoni.Unmarshal(data, &t)
	return t, true, err
}

func Clone(data *Data) *Data {
	if data == nil {
		return nil
	}
	m := make(map[string]json.RawMessage, len(data.M))
	for k, v := range data.M {
		dst := make(json.RawMessage, len(v))
		n := copy(dst, v)
		// According to the documentation for `copy`:
		//
		//   Copy returns the number of elements copied, which will be the minimum
		//   of len(src) and len(dst).
		//
		// Since `len(src)` is `len(dst)`, and `copy` cannot copy more bytes the
		// its source, we know that `n == len(v)`.
		contract.Assertf(n == len(v), "failed to perform full copy")
		m[k] = dst
	}
	return &Data{m}
}
