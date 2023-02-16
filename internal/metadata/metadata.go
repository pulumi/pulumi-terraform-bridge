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

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// The underlying value of a metadata blob.
type Data struct {
	m map[string]json.RawMessage
}

func New(data []byte) (*Data, error) {
	m := map[string]json.RawMessage{}
	if data != nil {
		err := json.Unmarshal(data, &m)
		if err != nil {
			return nil, err
		}
	}
	return &Data{m}, nil
}

func (d *Data) Marshal() []byte {
	if d == nil {
		d = &Data{m: make(map[string]json.RawMessage)}
	}
	bytes, err := json.MarshalIndent(d.m, "", "    ")
	contract.AssertNoErrorf(err, "internal: failed to marshal metadata")
	return bytes
}

func Set(d *Data, key string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	d.m[key] = data
	return nil
}

func Get[T any](d *Data, key string) (T, bool, error) {
	data, ok := d.m[key]
	var t T
	if !ok {
		return t, false, nil
	}
	err := json.Unmarshal(data, &t)
	return t, true, err
}

func Clone(data *Data) *Data {
	if data == nil {
		return nil
	}
	m := make(map[string]json.RawMessage, len(data.m))
	for k, v := range data.m {
		dst := make(json.RawMessage, len(v))
		n := copy(dst, v)
		contract.Assertf(n == len(v), "failed to perform full copy")
		m[k] = dst
	}
	return &Data{m}

}
