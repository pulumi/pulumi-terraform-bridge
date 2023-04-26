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

package tfbridge

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
)

func TestTerraformResourceName(t *testing.T) {
	urn := resource.URN("urn:pulumi:dev::stack1::random:index/randomInteger:RandomInteger::priority")
	p := &provider{
		info: ProviderInfo{
			ProviderInfo: tfbridge.ProviderInfo{
				Resources: map[string]*tfbridge.ResourceInfo{
					"random_integer": {Tok: "random:index/randomInteger:RandomInteger"},
				},
			},
		},
	}
	name, err := p.terraformResourceName(urn.Type())
	assert.NoError(t, err)
	assert.Equal(t, name, "random_integer")
}

func TestApplySecrets(t *testing.T) {
	t.Parallel()

	input1 := func() resource.PropertyMap {
		return resource.PropertyMap{
			"field1": resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewNullProperty(),
				resource.MakeSecret(resource.NewStringProperty("f1")),
			}),
			"field2": resource.MakeSecret(resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewNullProperty(),
				resource.NewStringProperty("f2"),
			})),
		}
	}

	tests := []struct {
		input    resource.PropertyMap
		output   resource.PropertyMap
		expected resource.PropertyMap
	}{
		{ // Empty outputs
			input: input1(),
		},
		{ // No secrets on output but output is the same shape
			input: input1(),
			output: resource.PropertyMap{
				"field1": resource.NewArrayProperty([]resource.PropertyValue{
					resource.NewNullProperty(),
					resource.NewStringProperty("f1"),
				}),
				"field2": resource.NewArrayProperty([]resource.PropertyValue{
					resource.NewNullProperty(),
					resource.NewStringProperty("f2"),
				}),
			},
			expected: input1(),
		},
		{ // Output has changed shape above where a secret was
			input: input1(),
			output: resource.PropertyMap{
				"field1": resource.NewStringProperty("combined"),
			},
			expected: resource.PropertyMap{
				"field1": resource.MakeSecret(resource.NewStringProperty("combined")),
			},
		},
		{ // Output has changes shape where the secret was
			input: input1(),
			output: resource.PropertyMap{
				"field2": resource.NewObjectProperty(resource.PropertyMap{
					"new": resource.NewStringProperty("shape"),
				}),
			},
			expected: resource.PropertyMap{
				"field2": resource.MakeSecret(resource.NewObjectProperty(resource.PropertyMap{
					"new": resource.NewStringProperty("shape"),
				})),
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run("", func(t *testing.T) {
			t.Parallel()
			secrets := findSecretPaths(tt.input)

			actual := applySecretPaths(tt.output, secrets)

			assert.Equal(t, tt.expected, actual)
		})
	}
}
