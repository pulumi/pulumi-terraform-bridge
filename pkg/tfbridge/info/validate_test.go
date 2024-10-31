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

	"github.com/stretchr/testify/assert"

	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/schema"
)

func ref[T any](t T) *T { return &t }

func TestValidateNameOverride(t *testing.T) {
	t.Parallel()
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
		info        *Resource
		expectedErr error
	}{
		{
			name:        "no override",
			info:        nil,
			expectedErr: nil,
		},
		{
			name: "valid field, name override",
			info: &Resource{
				Fields: map[string]*Schema{
					"scalar1": {Name: "foo"},
				},
			},
		},
		{
			name: "invalid field, name override",
			info: &Resource{
				Fields: map[string]*Schema{
					"scalar_missing": {Name: "foo"},
				},
			},
			expectedErr: errNoCorrespondingField,
		},
		{
			name: "invalid elem on scalar",
			info: &Resource{
				Fields: map[string]*Schema{
					"scalar1": {Elem: &Schema{}},
				},
			},
			expectedErr: errNoElemToOverride,
		},
		{
			name: "invalid name override on scalar list elem",
			info: &Resource{
				Fields: map[string]*Schema{
					"list_of_scalar": {
						Elem: &Schema{
							Name: "foo",
						},
					},
				},
			},
			expectedErr: errCannotSpecifyNameOnElem,
		},
		{
			name: "valid object field overrides",
			info: &Resource{
				Fields: map[string]*Schema{
					"object1": {
						Elem: &Schema{
							Fields: map[string]*Schema{
								"nest1": {Name: "Foo"},
							},
						},
					},
				},
			},
		},
		{
			name: "invalid object field overrides",
			info: &Resource{
				Fields: map[string]*Schema{
					"object1": {
						Fields: map[string]*Schema{
							"nest1": {Name: "Foo"},
						},
					},
				},
			},
			expectedErr: errFieldsShouldBeOnElem,
		},
		{
			name: "invalid fields on list",
			info: &Resource{
				Fields: map[string]*Schema{
					"list_of_scalar": {
						Fields: map[string]*Schema{
							"invalid": {},
						},
					},
				},
			},
			expectedErr: errCannotSpecifyFieldsOnCollection,
		},
		{
			name: "valid max items 1",
			info: &Resource{
				Fields: map[string]*Schema{
					"list_of_scalar": {MaxItemsOne: ref(true)},
				},
			},
		},
		{
			name: "invalid max items 1 (nested)",
			info: &Resource{
				Fields: map[string]*Schema{
					"list_of_scalar": {
						Elem: &Schema{MaxItemsOne: ref(true)},
					},
				},
			},
			expectedErr: errCannotSetMaxItemsOne,
		},
		{
			name: "invalid max items 1 (scalar)",
			info: &Resource{
				Fields: map[string]*Schema{
					"scalar1": {MaxItemsOne: ref(true)},
				},
			},
			expectedErr: errCannotSetMaxItemsOne,
		},
		{
			name: "invalid max items 1 (object)",
			info: &Resource{
				Fields: map[string]*Schema{
					"object1": {MaxItemsOne: ref(true)},
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

func withResource(info *Resource, r shim.Resource) *Provider {
	token := "test_r"

	if info == nil {
		info = &Resource{}
	}

	p := &Provider{
		Name: "test",
		P: (&schema.Provider{
			ResourcesMap: schema.ResourceMap{
				token: r,
			},
		}).Shim(),
		Resources: map[string]*Resource{
			token: info,
		},
	}

	if p.Resources[token].Tok == "" {
		p.Resources[token].Tok = "test:index/r:R"
	}

	return p
}
