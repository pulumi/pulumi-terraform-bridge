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

package main

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-terraform-bridge/v3/dynamic/internal/shim/run"
	"github.com/pulumi/pulumi-terraform-bridge/v3/dynamic/parameterize"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
)

func TestInferResourcePrefix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		tfNames        []string
		expectedPrefix string
	}{
		{
			name: "divergent-prefix",
			tfNames: []string{
				"prefix_res1",
				"prefix_res2",
			},
			expectedPrefix: "prefix",
		},
		{
			name: "expected-prefix",
			tfNames: []string{
				"test_res1",
				"test_res2",
			},
			expectedPrefix: "test",
		},
		{
			name: "ambiguous-prefix",
			tfNames: []string{
				"test_res1",
				"test2_res",
			},
			// We expect "test" because that is the .Name of the provider.
			expectedPrefix: "test",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			resourceSchemas := make(map[string]*tfprotov6.Schema, len(tt.tfNames))
			for _, tf := range tt.tfNames {
				resourceSchemas[tf] = &tfprotov6.Schema{Block: &tfprotov6.SchemaBlock{}}
			}

			info, err := providerInfo(context.Background(), schemaOnlyProvider{
				name:    "test",
				version: "1.0.0",
				schema: &tfprotov6.GetProviderSchemaResponse{
					ResourceSchemas: resourceSchemas,
				},
			}, parameterize.Value{})
			require.NoError(t, err)

			assert.Equal(t, tt.expectedPrefix, info.GetResourcePrefix())
		})
	}
}

func TestFixTokenOverrides(t *testing.T) {
	t.Parallel()

	p, err := providerInfo(context.Background(), schemaOnlyProvider{
		name:    "test",
		version: "1.0.0",
		schema: &tfprotov6.GetProviderSchemaResponse{
			ResourceSchemas: map[string]*tfprotov6.Schema{
				"test_provider": {Block: &tfprotov6.SchemaBlock{
					Attributes: []*tfprotov6.SchemaAttribute{
						{Name: "id", Type: tftypes.String, Computed: true},
					},
				}},
				"test_index": {Block: &tfprotov6.SchemaBlock{
					Attributes: []*tfprotov6.SchemaAttribute{
						{Name: "id", Type: tftypes.String, Computed: true},
					},
				}},
			},
		},
	}, parameterize.Value{})
	require.NoError(t, err)

	assert.Equal(t, map[string]*info.Resource{
		"test_index":    {Tok: "test:index/index:Index"},
		"test_provider": {Tok: "test:index/testProvider:TestProvider"},
	}, p.Resources)
}

func TestFixHyphenToken(t *testing.T) {
	t.Parallel()

	p, err := providerInfo(context.Background(), schemaOnlyProvider{
		name:    "test",
		version: "1.0.0",
		schema: &tfprotov6.GetProviderSchemaResponse{
			ResourceSchemas: map[string]*tfprotov6.Schema{
				"test_my-token": {Block: &tfprotov6.SchemaBlock{
					Attributes: []*tfprotov6.SchemaAttribute{
						{Name: "id", Type: tftypes.String, Computed: true},
					},
				}},
				"test_many-hyphenated-blocks": {Block: &tfprotov6.SchemaBlock{
					Attributes: []*tfprotov6.SchemaAttribute{
						{Name: "id", Type: tftypes.String, Computed: true},
					},
				}},
			},
		},
	}, parameterize.Value{})
	require.NoError(t, err)

	assert.Equal(t, map[string]*info.Resource{
		"test_my-token":               {Tok: "test:index/myToken:MyToken"},
		"test_many-hyphenated-blocks": {Tok: "test:index/manyHyphenatedBlocks:ManyHyphenatedBlocks"},
	}, p.Resources)
}

func TestResourceFiltering(t *testing.T) {
	t.Parallel()

	provider := schemaOnlyProvider{
		name:    "test",
		version: "1.0.0",
		schema: &tfprotov6.GetProviderSchemaResponse{
			ResourceSchemas: map[string]*tfprotov6.Schema{
				"test_resource_a": {Block: &tfprotov6.SchemaBlock{}},
				"test_resource_b": {Block: &tfprotov6.SchemaBlock{}},
				"test_resource_c": {Block: &tfprotov6.SchemaBlock{}},
			},
			DataSourceSchemas: map[string]*tfprotov6.Schema{
				"test_data_a": {Block: &tfprotov6.SchemaBlock{}},
				"test_data_b": {Block: &tfprotov6.SchemaBlock{}},
			},
		},
	}

	t.Run("filter specific resources", func(t *testing.T) {
		// Test filtering to include only specific resources
		info, err := providerInfo(context.Background(), provider, parameterize.Value{
			Includes: []string{"test_resource_a", "test_data_a"},
		})
		require.NoError(t, err)

		// Should ignore all resources/datasources NOT in the Resources list
		expectedIgnored := []string{"test_resource_b", "test_resource_c", "test_data_b"}
		assert.ElementsMatch(t, expectedIgnored, info.IgnoreMappings)

		// Should only have the specified resources in the Resources map
		expectedResources := []string{"test_resource_a", "test_data_a"}
		actualResources := make([]string, 0, len(info.Resources)+len(info.DataSources))
		for tfName := range info.Resources {
			actualResources = append(actualResources, tfName)
		}
		for tfName := range info.DataSources {
			actualResources = append(actualResources, tfName)
		}
		assert.ElementsMatch(t, expectedResources, actualResources)
	})

	t.Run("empty resources includes all", func(t *testing.T) {
		// Test empty resources (should include all - existing behavior)
		info, err := providerInfo(context.Background(), provider, parameterize.Value{})
		require.NoError(t, err)
		assert.Empty(t, info.IgnoreMappings)

		// Should have all resources in the Resources and DataSources maps
		expectedResources := []string{"test_resource_a", "test_resource_b", "test_resource_c", "test_data_a", "test_data_b"}
		actualResources := make([]string, 0, len(info.Resources)+len(info.DataSources))
		for tfName := range info.Resources {
			actualResources = append(actualResources, tfName)
		}
		for tfName := range info.DataSources {
			actualResources = append(actualResources, tfName)
		}
		assert.ElementsMatch(t, expectedResources, actualResources)
	})

	t.Run("nil resources includes all", func(t *testing.T) {
		// Test nil resources (should include all - existing behavior)
		info, err := providerInfo(context.Background(), provider, parameterize.Value{
			Includes: nil,
		})
		require.NoError(t, err)
		assert.Empty(t, info.IgnoreMappings)

		// Should have all resources in the Resources and DataSources maps
		expectedResources := []string{"test_resource_a", "test_resource_b", "test_resource_c", "test_data_a", "test_data_b"}
		actualResources := make([]string, 0, len(info.Resources)+len(info.DataSources))
		for tfName := range info.Resources {
			actualResources = append(actualResources, tfName)
		}
		for tfName := range info.DataSources {
			actualResources = append(actualResources, tfName)
		}
		assert.ElementsMatch(t, expectedResources, actualResources)
	})

	t.Run("single resource filter has single resource", func(t *testing.T) {
		// Test filtering to a single resource
		info, err := providerInfo(context.Background(), provider, parameterize.Value{
			Includes: []string{"test_resource_b"},
		})
		require.NoError(t, err)

		expectedIgnored := []string{"test_resource_a", "test_resource_c", "test_data_a", "test_data_b"}
		assert.ElementsMatch(t, expectedIgnored, info.IgnoreMappings)

		// Should only have the single specified resource
		expectedResources := []string{"test_resource_b"}
		actualResources := make([]string, 0, len(info.Resources)+len(info.DataSources))
		for tfName := range info.Resources {
			actualResources = append(actualResources, tfName)
		}
		for tfName := range info.DataSources {
			actualResources = append(actualResources, tfName)
		}
		assert.ElementsMatch(t, expectedResources, actualResources)
	})

	t.Run("non-existent resource does not affect filter", func(t *testing.T) {
		// Test filtering with non-existent resource (should ignore all actual resources)
		info, err := providerInfo(context.Background(), provider, parameterize.Value{
			Includes: []string{"non_existent_resource"},
		})
		require.NoError(t, err)

		// All actual resources should be ignored
		expectedIgnored := []string{"test_resource_a", "test_resource_b", "test_resource_c", "test_data_a", "test_data_b"}
		assert.ElementsMatch(t, expectedIgnored, info.IgnoreMappings)

		// Should have no resources in the Resources and DataSources maps
		actualResources := make([]string, 0, len(info.Resources)+len(info.DataSources))
		for tfName := range info.Resources {
			actualResources = append(actualResources, tfName)
		}
		for tfName := range info.DataSources {
			actualResources = append(actualResources, tfName)
		}
		assert.Empty(t, actualResources)
	})
}

type schemaOnlyProvider struct {
	run.Provider
	name, url, version string

	schema *tfprotov6.GetProviderSchemaResponse
}

func (s schemaOnlyProvider) Name() string    { return s.name }
func (s schemaOnlyProvider) URL() string     { return s.url }
func (s schemaOnlyProvider) Version() string { return s.version }

func (s schemaOnlyProvider) GetProviderSchema(
	_ context.Context, req *tfprotov6.GetProviderSchemaRequest,
) (*tfprotov6.GetProviderSchemaResponse, error) {
	return s.schema, nil
}
