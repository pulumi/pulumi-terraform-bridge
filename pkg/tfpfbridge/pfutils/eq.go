// Copyright 2016-2022, Pulumi Corporation.
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

package pfutils

import (
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

type Eq interface {
	Equal(path *tftypes.AttributePath, a, b tftypes.Value) (bool, error)
}

type defaultEq int

func (defaultEq) Equal(_ *tftypes.AttributePath, a, b tftypes.Value) (bool, error) {
	return a.Equal(b), nil
}

// Default equality for tftype.Value.
var DefaultEq Eq = defaultEq(0)

type nonComputedEq struct {
	Schema Schema
}

func (eq *nonComputedEq) Equal(p *tftypes.AttributePath, a, b tftypes.Value) (bool, error) {
	aNorm, err := replaceComputedAttributesWithNull(eq.Schema, p, a)
	if err != nil {
		return false, err
	}
	bNorm, err := replaceComputedAttributesWithNull(eq.Schema, p, b)
	if err != nil {
		return false, err
	}
	res := aNorm.Equal(bNorm)
	return res, nil
}

// Considers two tftype.Value values equal if all their non-computed attributes are equal.
func NonComputedEq(schema Schema) Eq {
	return &nonComputedEq{schema}
}

func replaceComputedAttributesWithNull(schema Schema,
	offset *tftypes.AttributePath, val tftypes.Value) (tftypes.Value, error) {
	return tftypes.Transform(val, func(p *tftypes.AttributePath, v tftypes.Value) (tftypes.Value, error) {
		realPath := joinPaths(offset, p)
		if attr, err := AttributeAtTerraformPath(schema, realPath); err == nil && attr.IsComputed() {
			return tftypes.NewValue(v.Type(), nil), nil
		}
		return v, nil
	})
}

func joinPaths(a, b *tftypes.AttributePath) *tftypes.AttributePath {
	return tftypes.NewAttributePathWithSteps(append(a.Steps(), b.Steps()...))
}
