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

package fixup_test

import (
	"testing"

	_ "github.com/hexops/autogold/v2" // autogold registers a flag for -update
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-terraform-bridge/v3/dynamic/internal/fixup"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/schema"
)

func TestFixMissingID(t *testing.T) {
	t.Parallel()

	p := info.Provider{
		Name: "test",
		P: (&schema.Provider{
			ResourcesMap: schema.ResourceMap{
				"test_res": (&schema.Resource{
					Schema: schema.SchemaMap{
						"some_property": (&schema.Schema{
							Type: shim.TypeString,
						}).Shim(),
					},
				}).Shim(),
			},
		}).Shim(),
	}

	err := fixup.Default(&p)
	require.NoError(t, err)
	assert.NotNil(t, p.Resources["test_res"].ComputeID)
}

func TestFixPropertyConflicts(t *testing.T) {
	t.Parallel()

	simpleID := (&schema.Schema{
		Type:     shim.TypeString,
		Computed: true,
	}).Shim()

	tests := []struct {
		name string

		schema schema.SchemaMap
		info   map[string]*info.Schema

		expected           map[string]*info.Schema
		expectComputeIDSet bool
	}{
		{
			name: "fix urn property name",
			schema: schema.SchemaMap{
				"urn": (&schema.Schema{
					Type: shim.TypeString,
				}).Shim(),
				"id": simpleID,
			},
			expected: map[string]*info.Schema{
				"urn": {Name: "testUrn"},
			},
		},
		{
			name: "ignore overridden urn property name",
			schema: schema.SchemaMap{
				"urn": (&schema.Schema{
					Type: shim.TypeString,
				}).Shim(),
				"id": simpleID,
			},
			info: map[string]*info.Schema{
				"urn": {Name: "overridden"},
			},
			expected: map[string]*info.Schema{
				"urn": {Name: "overridden"},
			},
		},
		{
			name: "fix ID property name (computed + optional)",
			schema: schema.SchemaMap{
				"id": (&schema.Schema{
					Type:     shim.TypeString,
					Optional: true,
					Computed: true,
				}).Shim(),
			},
			expected: map[string]*info.Schema{
				"id": {Name: "resId"},
			},
			expectComputeIDSet: true,
		},
		{
			name: "fix ID property name (required)",
			schema: schema.SchemaMap{
				"id": (&schema.Schema{
					Type:     shim.TypeString,
					Required: true,
				}).Shim(),
			},
			expected: map[string]*info.Schema{
				"id": {Name: "resId"},
			},
			expectComputeIDSet: true,
		},

		{
			name: "ignore output ID property name (computed)",
			schema: schema.SchemaMap{
				"id": (&schema.Schema{
					Type:     shim.TypeString,
					Computed: true,
				}).Shim(),
			},
			expected: nil,
		},
		{
			name: "ignore overridden ID property name",
			schema: schema.SchemaMap{
				"id": (&schema.Schema{
					Type:     shim.TypeString,
					Computed: true,
					Optional: true,
				}).Shim(),
			},
			info: map[string]*info.Schema{
				"id": {Name: "overridden"},
			},
			expected: map[string]*info.Schema{
				"id": {Name: "overridden"},
			},
		},

		// Test ID fallbacks

		{
			name: "fallback to resource and provider ID for property name",
			schema: schema.SchemaMap{
				"id": (&schema.Schema{
					Type:     shim.TypeString,
					Required: true,
				}).Shim(),
				"res_id": (&schema.Schema{
					Type: shim.TypeString,
				}).Shim(),
			},
			expected: map[string]*info.Schema{
				"id": {Name: "testResId"},
			},
			expectComputeIDSet: true,
		},
		{
			name: "fallback to literal ID for property name",
			schema: schema.SchemaMap{
				"id": (&schema.Schema{
					Type:     shim.TypeString,
					Required: true,
				}).Shim(),
				"res_id": (&schema.Schema{
					Type: shim.TypeString,
				}).Shim(),
				"test_res_id": (&schema.Schema{
					Type: shim.TypeString,
				}).Shim(),
			},
			expected: map[string]*info.Schema{
				"id": {Name: "resourceId"},
			},
			expectComputeIDSet: true,
		},
		{
			name: "fallback to provider ID for property name",
			schema: schema.SchemaMap{
				"id": (&schema.Schema{
					Type:     shim.TypeString,
					Required: true,
				}).Shim(),
				"res_id": (&schema.Schema{
					Type: shim.TypeString,
				}).Shim(),
				"test_res_id": (&schema.Schema{
					Type: shim.TypeString,
				}).Shim(),
				"resource_id": (&schema.Schema{
					Type: shim.TypeString,
				}).Shim(),
			},
			expected: map[string]*info.Schema{
				"id": {Name: "testId"},
			},
			expectComputeIDSet: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p := info.Provider{
				Name: "test",
				P: (&schema.Provider{
					ResourcesMap: schema.ResourceMap{
						"test_res": (&schema.Resource{
							Schema: tt.schema,
						}).Shim(),
					},
				}).Shim(),
				Resources: map[string]*info.Resource{
					"test_res": {
						Fields: tt.info,
					},
				},
			}
			err := fixup.Default(&p)
			require.NoError(t, err)

			r := p.Resources["test_res"]
			assert.Equal(t, tt.expected, r.Fields)

			if tt.expectComputeIDSet {
				assert.NotNil(t, r.ComputeID, "expected .ComputeID to be set")
			} else {
				assert.Nil(t, r.ComputeID, "expected .ComputeID to not be set")
			}
		})
	}
}

// TestFixIDKebabCaseProvider validates that providers with kebab-case names that have ID
// fields that need fixups still result in good (camelCase) property names for Pulumi
// schemas.
func TestFixIDKebabCaseProvider(t *testing.T) {
	t.Parallel()

	p := info.Provider{
		Name: "test-provider",
		P: (&schema.Provider{
			ResourcesMap: schema.ResourceMap{
				"test-provider_res": (&schema.Resource{
					Schema: schema.SchemaMap{
						"id": (&schema.Schema{
							Type:     shim.TypeString,
							Required: true,
						}).Shim(),

						// The provider name is the 3rd try for
						// naming this field. We first try
						// resource_id, then res_id
						// ("<resource_name>_id"), so these fields
						// must be present to test the provider
						// behavior.
						"res_id": (&schema.Schema{
							Type: shim.TypeString,
						}).Shim(),
					},
				}).Shim(),
			},
		}).Shim(),
	}
	err := fixup.Default(&p)
	require.NoError(t, err)

	r := p.Resources["test-provider_res"]
	assert.Equal(t, map[string]*info.Schema{
		"id": {Name: "testProviderResId"},
	}, r.Fields)
	assert.NotNil(t, r.ComputeID)
}

func TestFixProviderResourceName(t *testing.T) {
	t.Parallel()
	p := info.Provider{
		Name: "test",
		P: (&schema.Provider{
			ResourcesMap: schema.ResourceMap{
				"test_provider": (&schema.Resource{
					Schema: schema.SchemaMap{
						"id": (&schema.Schema{
							Type:     shim.TypeString,
							Computed: true,
						}).Shim(),
					},
				}).Shim(),
			},
		}).Shim(),
	}

	err := fixup.Default(&p)
	require.NoError(t, err)
	assert.Equal(t, tokens.Type("test:index/testProvider:TestProvider"), p.Resources["test_provider"].Tok)
}
