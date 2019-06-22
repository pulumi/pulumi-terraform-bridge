// Copyright 2016-2018, Pulumi Corporation.
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

package tfbridge

import (
	"encoding/json"

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/resource"
)

// TransformJSONDocument permits either a string, which is presumed to represent an already-stringified JSON document,
// or a map/array, which will be transformed into its JSON representation.
func TransformJSONDocument(v resource.PropertyValue) (resource.PropertyValue, error) {
	// We can't marshal properties that contain unknowns. Turn these into an unknown value instead.
	if v.ContainsUnknowns() {
		return resource.MakeComputed(resource.NewStringProperty("")), nil
	}

	if v.IsString() {
		return v, nil
	} else if v.IsObject() || v.IsArray() {
		m := v.Mappable()
		b, err := json.Marshal(m)
		if err != nil {
			return resource.PropertyValue{}, err
		}
		return resource.NewStringProperty(string(b)), nil
	}
	return resource.PropertyValue{},
		errors.Errorf("expected string or JSON map; got %T", v.V)
}
