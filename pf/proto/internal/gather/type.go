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

type _type struct{ s tftypes.Type }

var _ = attr.Type(_type{})

// TerraformType returns the tftypes.Type that should be used to
// represent this type. This constrains what user input will be
// accepted and what kind of data can be set in state. The framework
// will use this to translate the Type to something Terraform can
// understand.
func (t _type) TerraformType(context.Context) tftypes.Type { return t.s }

// ValueFromTerraform returns a Value given a tftypes.Value. This is
// meant to convert the tftypes.Value into a more convenient Go type
// for the provider to consume the data with.
func (t _type) ValueFromTerraform(_ context.Context, v tftypes.Value) (attr.Value, error) {
	return value{_type: t, value: v}, nil
}

// ValueType should return the attr.Value type returned by
// ValueFromTerraform. The returned attr.Value can be any null, unknown,
// or known value for the type, as this is intended for type detection
// and improving error diagnostics.
func (t _type) ValueType(context.Context) attr.Value {
	return value{_type: t}
}

// Equal should return true if the Type is considered equivalent to the
// Type passed as an argument.
//
// Most types should verify the associated Type is exactly equal to prevent
// potential data consistency issues. For example:
//
//   - basetypes.Number is inequal to basetypes.Int64 or basetypes.Float64
//   - basetypes.String is inequal to a custom Go type that embeds it
func (t _type) Equal(other attr.Type) bool {
	o, ok := other.(_type)
	if !ok {
		return false
	}
	return t.s.Equal(o.s)
}

// String should return a human-friendly version of the Type.
func (t _type) String() string { return t.s.String() }

func (t _type) ApplyTerraform5AttributePathStep(step tftypes.AttributePathStep) (interface{}, error) {
	return t.s.ApplyTerraform5AttributePathStep(step)
}
