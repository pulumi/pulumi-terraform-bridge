// Copyright 2016-2025, Pulumi Corporation.
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

package valueshim

import (
	"encoding/json"
)

// Allows abstracting over cty.Value and tftypes.Value for purposes of computing raw state deltas. In particular for
// Plugin Framework based providers this helps avoid implicit set element reordering that cty.Value brings.
type Value interface {
	IsNull() bool
	GoString() string
	Type() Type
	AsValueSlice() []Value
	AsValueMap() map[string]Value

	// Marshals into the "raw state" JSON representation.
	Marshal() (json.RawMessage, error)

	// Removes a top-level property from an Object.
	Remove(key string) Value
}

type Type interface {
	IsNumberType() bool
	IsStringType() bool
	IsBooleanType() bool
	IsListType() bool
	IsMapType() bool
	IsSetType() bool
	IsObjectType() bool
	GoString() string
}
