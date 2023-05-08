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

package defaults

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/schema"
)

func TestApplyDefaultInfoValues(t *testing.T) {

	var schemaMap shim.SchemaMap = schema.SchemaMap{
		"string_prop": (&schema.Schema{Type: shim.TypeString, Optional: true}).Shim(),

		"object_prop": (&schema.Schema{
			Type:     shim.TypeMap,
			Optional: true,
			Elem: (&schema.Resource{
				Schema: schema.SchemaMap{
					"x_prop": (&schema.Schema{Type: shim.TypeString, Optional: true}).Shim(),
					"y_prop": (&schema.Schema{Type: shim.TypeString, Optional: true}).Shim(),
				},
			}).Shim(),
		}).Shim(),
	}

	type testCase struct {
		name             string
		env              map[string]string
		resourceInstance *tfbridge.PulumiResource
		props            resource.PropertyMap
		expected         resource.PropertyMap
		fieldInfos       map[string]*tfbridge.SchemaInfo
	}

	testCases := []testCase{
		{
			name: "simple top-level string",
			fieldInfos: map[string]*tfbridge.SchemaInfo{
				"string_prop": {
					Default: &tfbridge.DefaultInfo{
						Value: "defaultValue",
					},
				},
			},
			expected: resource.PropertyMap{
				"stringProp": resource.NewStringProperty("defaultValue"),
			},
		},
		{
			name: "nested string",
			fieldInfos: map[string]*tfbridge.SchemaInfo{
				"object_prop": {
					Fields: map[string]*tfbridge.SchemaInfo{
						"y_prop": {
							Default: &tfbridge.DefaultInfo{
								Value: "Y",
							},
						},
					},
				},
			},
			props: resource.PropertyMap{
				"objectProp": resource.NewObjectProperty(resource.PropertyMap{
					"xProp": resource.NewStringProperty("X"),
				}),
			},
			expected: resource.PropertyMap{
				"objectProp": resource.NewObjectProperty(resource.PropertyMap{
					"xProp": resource.NewStringProperty("X"),
					"yProp": resource.NewStringProperty("Y"),
				}),
			},
		},
		{
			name: "nested string does not create object",
			fieldInfos: map[string]*tfbridge.SchemaInfo{
				"object_prop": {
					Fields: map[string]*tfbridge.SchemaInfo{
						"y_prop": {
							Default: &tfbridge.DefaultInfo{
								Value: "Y",
							},
						},
					},
				},
			},
			props:    resource.PropertyMap{},
			expected: resource.PropertyMap{},
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			actual := ApplyDefaultInfoValues(ctx, schemaMap, tc.fieldInfos, tc.resourceInstance, tc.props)
			assert.Equal(t, tc.expected, actual)
		})
	}

}
