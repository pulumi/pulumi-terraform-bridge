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

package info

import (
	"context"
	"testing"

	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/schema"
	"github.com/stretchr/testify/assert"
)

func TestValidateNameOverride(t *testing.T) {
	r := func() shim.Resource {
		return (&schema.Resource{
			Schema: schema.SchemaMap{
				"scalar1": (&schema.Schema{
					Type: shim.TypeString,
				}).Shim(),
				"list_of_scalar": (&schema.Schema{
					Type: shim.TypeList,
					Elem: (&schema.Schema{
						Type: shim.TypeInt,
					}).Shim(),
				}).Shim(),
				"object1": (&schema.Schema{
					Type: shim.TypeMap,
					Elem: (&schema.Resource{
						Schema: schema.SchemaMap{
							"nest1": (&schema.Schema{
								Type: shim.TypeString,
							}).Shim(),
							"nest2": (&schema.Schema{
								Type: shim.TypeList,
								Elem: (&schema.Schema{
									Type: shim.TypeInt,
								}).Shim(),
							}).Shim(),
						},
					}).Shim(),
				}).Shim(),
			},
		}).Shim()
	}

	tests := []struct {
		name        string
		info        *ResourceInfo
		expectedErr error
	}{
		{
			name:        "no override",
			info:        nil,
			expectedErr: nil,
		},
		{
			name: "valid field, name override",
			info: &ResourceInfo{
				Fields: map[string]*SchemaInfo{
					"scalar1": {Name: "foo"},
				},
			},
		},
		{
			name: "invalid field, name override",
			info: &ResourceInfo{
				Fields: map[string]*SchemaInfo{
					"scalar_missing": {Name: "foo"},
				},
			},
			expectedErr: errNoCorrespondingField,
		},
		{
			name: "invalid elem on scalar",
			info: &ResourceInfo{
				Fields: map[string]*SchemaInfo{
					"scalar1": {Elem: &SchemaInfo{}},
				},
			},
			expectedErr: errNoElemToOverride,
		},
		{
			name: "invalid name override on scalar list elem",
			info: &ResourceInfo{
				Fields: map[string]*SchemaInfo{
					"list_of_scalar": {
						Elem: &SchemaInfo{
							Name: "foo",
						},
					},
				},
			},
			expectedErr: errCannotSpecifyNameOnElem,
		},
		{
			name: "valid object field overrides",
			info: &ResourceInfo{
				Fields: map[string]*SchemaInfo{
					"object1": {
						Fields: map[string]*SchemaInfo{
							"nest1": {Name: "Foo"},
						},
					},
				},
			},
		},
		{
			name: "invalid object field overrides",
			info: &ResourceInfo{
				Fields: map[string]*SchemaInfo{
					"object1": {
						Elem: &SchemaInfo{
							Fields: map[string]*SchemaInfo{
								"nest1": {Name: "Foo"},
							},
						},
					},
				},
			},
			expectedErr: errElemForObject,
		},
		{
			name: "invalid fields on list",
			info: &ResourceInfo{
				Fields: map[string]*SchemaInfo{
					"list_of_scalar": {
						Fields: map[string]*SchemaInfo{
							"invalid": {},
						},
					},
				},
			},
			expectedErr: errCannotSpecifyFieldsOnListOrSet,
		},
		{
			name: "valid max items 1",
			info: &ResourceInfo{
				Fields: map[string]*SchemaInfo{
					"list_of_scalar": {MaxItemsOne: True()},
				},
			},
		},
		{
			name: "invalid max items 1 (nested)",
			info: &ResourceInfo{
				Fields: map[string]*SchemaInfo{
					"list_of_scalar": {
						Elem: &SchemaInfo{MaxItemsOne: True()},
					},
				},
			},
			expectedErr: errCannotSetMaxItemsOne,
		},
		{
			name: "invalid max items 1 (scalar)",
			info: &ResourceInfo{
				Fields: map[string]*SchemaInfo{
					"scalar1": {MaxItemsOne: True()},
				},
			},
			expectedErr: errCannotSetMaxItemsOne,
		},
		{
			name: "invalid max items 1 (object)",
			info: &ResourceInfo{
				Fields: map[string]*SchemaInfo{
					"object1": {MaxItemsOne: True()},
				},
			},
			expectedErr: errCannotSetMaxItemsOne,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := withResource(tt.info, r()).Validate(context.Background())
			if tt.expectedErr == nil {
				assert.NoError(t, err)
			} else {
				assert.ErrorIs(t, err, tt.expectedErr)
			}
		})
	}
}

func withResource(info *ResourceInfo, r shim.Resource) *ProviderInfo {
	token := "test_r"

	if info == nil {
		info = &ResourceInfo{}
	}

	p := &ProviderInfo{
		Name: "test",
		P: (&schema.Provider{
			ResourcesMap: schema.ResourceMap{
				token: r,
			},
		}).Shim(),
		Resources: map[string]*ResourceInfo{
			token: info,
		},
	}

	if p.Resources[token].Tok == "" {
		p.Resources[token].Tok = "test:index/r:R"
	}

	return p
}
