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
	"math/big"
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
	//
	// This is the representation expected on the TF protocol UpgradeResourceState method.
	//
	// For correctly encoding DynamicPseudoType values to {"type": "...", "value": "..."} structures, the
	// schemaType is needed. This encoding will be used when the schema a type is a DynamicPseudoType but
	// the value type is a concrete type.
	//
	// In situations where the DynamicPseudoType encoding is not needed, you can also call Marshal with
	// value.Type() to assume the intrinsic type of the value.
	Marshal(schemaType Type) (json.RawMessage, error)

	// Removes a top-level property from an Object.
	Remove(key string) Value

	StringValue() string
	NumberValue() float64
	BigFloatValue() *big.Float
	BoolValue() bool
}

type Type interface {
	IsNumberType() bool
	IsStringType() bool
	IsBooleanType() bool
	IsListType() bool
	IsMapType() bool
	IsSetType() bool
	IsObjectType() bool
	IsDynamicType() bool
	AttributeType(name string) (Type, bool)
	ElementType() (Type, bool)
	GoString() string
}
