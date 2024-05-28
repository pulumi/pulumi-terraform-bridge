// Copyright 2016-2024, Pulumi Corporation.
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

package gather

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

type value struct {
	_type _type
	value tftypes.Value
}

var _ = attr.Value(value{})

func (v value) Type(context.Context) attr.Type { return v._type }

func (v value) ToTerraformValue(context.Context) (tftypes.Value, error) { return v.value, nil }

// Equal should return true if the Value is considered type and data
// value equivalent to the Value passed as an argument.
//
// Most types should verify the associated Type is exactly equal to prevent
// potential data consistency issues. For example:
//
//   - basetypes.Number is inequal to basetypes.Int64 or basetypes.Float64
//   - basetypes.String is inequal to a custom Go type that embeds it
//
// Additionally, most types should verify that known values are compared
// to comply with Terraform's data consistency rules. For example:
//
//   - In a list, element order is significant
//   - In a string, runes are compared byte-wise (e.g. whitespace is
//     significant in JSON-encoded strings)
func (v value) Equal(other attr.Value) bool {
	o, ok := other.(value)
	if !ok {
		return false
	}
	return v.value.Equal(o.value) && v._type.Equal(o._type)
}

// IsNull returns true if the Value is not set, or is explicitly set to null.
func (v value) IsNull() bool { return v.value.IsNull() }

// IsUnknown returns true if the value is not yet known.
func (v value) IsUnknown() bool { return !v.value.IsKnown() }

// String returns a summary representation of either the underlying Value,
// or UnknownValueString (`<unknown>`) when IsUnknown() returns true,
// or NullValueString (`<null>`) when IsNull() return true.
//
// This is an intentionally lossy representation, that are best suited for
// logging and error reporting, as they are not protected by
// compatibility guarantees within the framework.
func (v value) String() string { return v.value.String() }
