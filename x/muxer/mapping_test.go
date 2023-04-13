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

package muxer

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
)

func TestMapping(t *testing.T) {
	s1 := schema.PackageSpec{
		Name: "pkg",
		Resources: map[string]schema.ResourceSpec{
			"pkg:mod:ResA": {},
		},
		Types: map[string]schema.ComplexTypeSpec{
			"pkg:index/type:Type": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type: "string",
				},
				Enum: []schema.EnumValueSpec{
					{Name: "Value1", Value: "v1"},
					{Name: "Value2", Value: "v2"},
				},
			},
			"pkg:index/type:Var": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type: "number",
				},
				Enum: []schema.EnumValueSpec{
					{Name: "Value1", Value: 1},
					{Name: "Value2", Value: 2},
				},
			},
		},
		Config: schema.ConfigSpec{
			Variables: map[string]schema.PropertySpec{
				"var1": {
					TypeSpec: schema.TypeSpec{
						Ref: "#/types/pkg:index/type:Var",
					},
				},
				"var2": {
					TypeSpec: schema.TypeSpec{
						Type: "number",
					},
				},
			},
			Required: []string{"var1", "var2"},
		},
		Provider: schema.ResourceSpec{
			InputProperties: map[string]schema.PropertySpec{
				"typ": {
					TypeSpec: schema.TypeSpec{
						Type: "string",
						Ref:  "#/types/pkg:index/type:Type",
					},
				},
			},
		},
	}
	s2 := schema.PackageSpec{
		Name: "pkg",
		Resources: map[string]schema.ResourceSpec{
			"pkg:mod:ResB": {},
		},
		Config: schema.ConfigSpec{
			Variables: map[string]schema.PropertySpec{
				"var2": {
					TypeSpec: schema.TypeSpec{
						Type: "number",
					},
				},
				"var3": {
					TypeSpec: schema.TypeSpec{
						Type: "int",
					},
				},
			},
			Required: []string{"var2"},
		},
	}

	computedMapping, sMuxed, err := Mapping([]schema.PackageSpec{s1, s2})
	require.NoError(t, err)

	assert.Equal(t, schema.PackageSpec{
		Name: "pkg",
		Types: map[string]schema.ComplexTypeSpec{
			"pkg:index/type:Type": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type: "string",
				},
				Enum: []schema.EnumValueSpec{
					{Name: "Value1", Value: "v1"},
					{Name: "Value2", Value: "v2"},
				},
			},
			"pkg:index/type:Var": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type: "number",
				},
				Enum: []schema.EnumValueSpec{
					{Name: "Value1", Value: 1},
					{Name: "Value2", Value: 2},
				},
			},
		},
		Config: schema.ConfigSpec{
			Variables: map[string]schema.PropertySpec{
				"var1": {
					TypeSpec: schema.TypeSpec{
						Ref: "#/types/pkg:index/type:Var",
					},
				},
				"var2": {
					TypeSpec: schema.TypeSpec{
						Type: "number",
					},
				},
				"var3": {
					TypeSpec: schema.TypeSpec{
						Type: "int",
					},
				},
			},
			Required: []string{"var1", "var2"},
		},
		Provider: schema.ResourceSpec{
			InputProperties: map[string]schema.PropertySpec{
				"typ": {
					TypeSpec: schema.TypeSpec{
						Type: "string",
						Ref:  "#/types/pkg:index/type:Type",
					},
				},
			},
		},
		Resources: map[string]schema.ResourceSpec{
			"pkg:mod:ResA": {},
			"pkg:mod:ResB": {},
		},
	}, sMuxed)

	assert.Equal(t, &DispatchTable{
		Resources: map[string]int{
			"pkg:mod:ResA": 0,
			"pkg:mod:ResB": 1,
		},
		Functions: map[string]int{},

		Config: map[string][]int{
			"var1": {0},
			"var2": {0, 1},
			"var3": {1},
		},
	}, computedMapping)
}

// In case of AWS there are extra types that should not be dropped by the muxer.
func TestMappingPreservesExtraTypes(t *testing.T) {
	s1 := schema.PackageSpec{
		Types: map[string]schema.ComplexTypeSpec{
			"aws:iam/ManagedPolicy:ManagedPolicy": {
				ObjectTypeSpec: schema.ObjectTypeSpec{Type: "string"},
				Enum: []schema.EnumValueSpec{
					{
						Name:  "APIGatewayServiceRolePolicy",
						Value: "arn:aws:iam::aws:policy/aws-service-role/APIGatewayServiceRolePolicy",
					},
				},
			},
		},
	}

	s2 := schema.PackageSpec{}

	_, spec, err := Mapping([]schema.PackageSpec{
		s1, s2,
	})
	require.NoError(t, err)
	assert.Equal(t, s1.Types, spec.Types)
}
