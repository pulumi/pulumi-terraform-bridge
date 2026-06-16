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

package tokens

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	ptokens "github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/internal/metadatakeys"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/schema"
	md "github.com/pulumi/pulumi-terraform-bridge/v3/unstable/metadata"
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

	err := applyDefaultFixups(&p)
	require.NoError(t, err)
	assert.NotNil(t, p.Resources["test_res"].ComputeID)
}

func TestFixMissingIDPreservesExistingComputeID(t *testing.T) {
	t.Parallel()

	manualComputeID := func(ctx context.Context, state resource.PropertyMap) (resource.ID, error) {
		return resource.ID("manual"), nil
	}

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
		Resources: map[string]*info.Resource{
			"test_res": {
				ComputeID: manualComputeID,
			},
		},
	}

	err := applyDefaultFixups(&p)
	require.NoError(t, err)

	got, err := p.Resources["test_res"].ComputeID(context.Background(), resource.PropertyMap{})
	require.NoError(t, err)
	assert.Equal(t, resource.ID("manual"), got)
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
				"urn": (&schema.Schema{Type: shim.TypeString}).Shim(),
				"id":  simpleID,
			},
			expected: map[string]*info.Schema{
				"urn": {Name: "testUrn"},
			},
		},
		{
			name: "ignore overridden urn property name",
			schema: schema.SchemaMap{
				"urn": (&schema.Schema{Type: shim.TypeString}).Shim(),
				"id":  simpleID,
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
		{
			name: "fallback to resource and provider ID for property name",
			schema: schema.SchemaMap{
				"id": (&schema.Schema{
					Type:     shim.TypeString,
					Required: true,
				}).Shim(),
				"res_id": (&schema.Schema{Type: shim.TypeString}).Shim(),
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
				"res_id":      (&schema.Schema{Type: shim.TypeString}).Shim(),
				"test_res_id": (&schema.Schema{Type: shim.TypeString}).Shim(),
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
				"res_id":      (&schema.Schema{Type: shim.TypeString}).Shim(),
				"test_res_id": (&schema.Schema{Type: shim.TypeString}).Shim(),
				"resource_id": (&schema.Schema{Type: shim.TypeString}).Shim(),
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
			err := applyDefaultFixups(&p)
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
						"res_id": (&schema.Schema{Type: shim.TypeString}).Shim(),
					},
				}).Shim(),
			},
		}).Shim(),
	}
	err := applyDefaultFixups(&p)
	require.NoError(t, err)

	r := p.Resources["test-provider_res"]
	assert.Equal(t, map[string]*info.Schema{
		"id": {Name: "testProviderResId"},
	}, r.Fields)
	assert.NotNil(t, r.ComputeID)
}

func TestFixIDPreservesExistingComputeID(t *testing.T) {
	t.Parallel()

	manualComputeID := func(ctx context.Context, state resource.PropertyMap) (resource.ID, error) {
		return resource.ID("manual"), nil
	}

	p := info.Provider{
		Name: "test",
		P: (&schema.Provider{
			ResourcesMap: schema.ResourceMap{
				"test_res": (&schema.Resource{
					Schema: schema.SchemaMap{
						"id": (&schema.Schema{
							Type:     shim.TypeString,
							Required: true,
						}).Shim(),
					},
				}).Shim(),
			},
		}).Shim(),
		Resources: map[string]*info.Resource{
			"test_res": {
				ComputeID: manualComputeID,
			},
		},
	}

	err := applyDefaultFixups(&p)
	require.NoError(t, err)

	r := p.Resources["test_res"]
	assert.Equal(t, "resId", r.Fields["id"].Name)
	got, err := r.ComputeID(context.Background(), resource.PropertyMap{})
	require.NoError(t, err)
	assert.Equal(t, resource.ID("manual"), got)
}

func TestFixProviderResourceRenames(t *testing.T) {
	t.Parallel()

	p := info.Provider{
		Name: "test",
		P: (&schema.Provider{
			ResourcesMap: schema.ResourceMap{
				"test_provider": (&schema.Resource{
					Schema: schema.SchemaMap{},
				}).Shim(),
			},
		}).Shim(),
	}

	p.Resources = map[string]*info.Resource{
		"test_provider": {Tok: "test:index/provider:Provider"},
	}

	err := fixProviderResource(&p, false)
	require.NoError(t, err)

	require.Contains(t, p.Resources, "test_provider")
	assert.Equal(t, ptokens.Type("test:index/testProvider:TestProvider"), p.Resources["test_provider"].Tok)
}

func TestFixProviderResourceSkipsExistingTokens(t *testing.T) {
	t.Parallel()

	p := info.Provider{
		Name: "test",
		P: (&schema.Provider{
			ResourcesMap: schema.ResourceMap{
				"test_provider": (&schema.Resource{
					Schema: schema.SchemaMap{},
				}).Shim(),
			},
		}).Shim(),
	}

	p.Resources = map[string]*info.Resource{
		"test_provider": {Tok: "test:index/provider:Provider"},
	}

	err := fixProviderResource(&p, true)
	require.NoError(t, err)

	assert.Equal(t, ptokens.Type("test:index/provider:Provider"), p.Resources["test_provider"].Tok)
}

func TestFixMissingIDStoresPresentNilResourceEntry(t *testing.T) {
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
		Resources: map[string]*info.Resource{
			"test_res": nil,
		},
	}

	err := applyDefaultFixups(&p)
	require.NoError(t, err)
	require.NotNil(t, p.Resources["test_res"])
	assert.NotNil(t, p.Resources["test_res"].ComputeID)
}

func TestFixPropertyNamedPulumiRenamedPulumiInfo(t *testing.T) {
	t.Parallel()

	p := info.Provider{
		Name: "test",
		P: (&schema.Provider{
			ResourcesMap: schema.ResourceMap{
				"test_res": (&schema.Resource{
					Schema: schema.SchemaMap{
						"pulumi": (&schema.Schema{Type: shim.TypeString}).Shim(),
					},
				}).Shim(),
			},
		}).Shim(),
	}

	err := applyDefaultFixups(&p)
	require.NoError(t, err)
	assert.NotNil(t, p.Resources["test_res"])
	assert.Equal(t, "pulumiInfo", p.Resources["test_res"].Fields["pulumi"].Name)
}

func TestFixPropertyConflictWithIgnoredMappings(t *testing.T) {
	t.Parallel()

	p := info.Provider{
		Name: "test",
		P: (&schema.Provider{
			ResourcesMap: schema.ResourceMap{
				"test_ignored": (&schema.Resource{
					Schema: schema.SchemaMap{
						"urn": (&schema.Schema{Type: shim.TypeString}).Shim(),
						"id": (&schema.Schema{
							Type:     shim.TypeString,
							Computed: true,
						}).Shim(),
					},
				}).Shim(),
				"test_processed": (&schema.Resource{
					Schema: schema.SchemaMap{
						"urn": (&schema.Schema{Type: shim.TypeString}).Shim(),
						"id": (&schema.Schema{
							Type:     shim.TypeString,
							Computed: true,
						}).Shim(),
					},
				}).Shim(),
			},
		}).Shim(),
		IgnoreMappings: []string{"test_ignored"},
	}

	err := applyDefaultFixups(&p)
	require.NoError(t, err)

	assert.NotContains(t, p.Resources, "test_ignored")
	assert.Contains(t, p.Resources, "test_processed")
	assert.Equal(t, "testUrn", p.Resources["test_processed"].Fields["urn"].Name)
}

func TestFixMissingIDsWithIgnoredMappings(t *testing.T) {
	t.Parallel()

	p := info.Provider{
		Name: "test",
		P: (&schema.Provider{
			ResourcesMap: schema.ResourceMap{
				"test_ignored": (&schema.Resource{
					Schema: schema.SchemaMap{
						"some_property": (&schema.Schema{Type: shim.TypeString}).Shim(),
					},
				}).Shim(),
				"test_processed": (&schema.Resource{
					Schema: schema.SchemaMap{
						"some_property": (&schema.Schema{Type: shim.TypeString}).Shim(),
					},
				}).Shim(),
			},
		}).Shim(),
		IgnoreMappings: []string{"test_ignored"},
	}

	err := applyDefaultFixups(&p)
	require.NoError(t, err)

	assert.NotContains(t, p.Resources, "test_ignored")
	assert.Contains(t, p.Resources, "test_processed")
	assert.NotNil(t, p.Resources["test_processed"].ComputeID)
}

func TestDefaultFixupsTolerateMetadataInfoWithoutData(t *testing.T) {
	t.Parallel()

	var schemaCalls atomic.Int32
	p := info.Provider{
		Name: "test",
		P: countingResourceProvider(schema.SchemaMap{
			"some_property": (&schema.Schema{Type: shim.TypeString}).Shim(),
		}, &schemaCalls),
		MetadataInfo: &info.Metadata{Path: "must be non-empty"},
	}
	require.NoError(t, applyDefaultFixups(&p))
	require.Positive(t, schemaCalls.Load())
	require.NotNil(t, p.Resources["test_res"].ComputeID)
}

func TestPrecomputedDefaultFixupsAvoidResourceSchema(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		schema schema.SchemaMap
		verify func(*testing.T, *info.Resource)
	}{
		{
			name: "missing ID",
			schema: schema.SchemaMap{
				"some_property": (&schema.Schema{Type: shim.TypeString}).Shim(),
			},
			verify: func(t *testing.T, res *info.Resource) {
				t.Helper()
				require.NotNil(t, res.ComputeID)
				got, err := res.ComputeID(context.Background(), resource.PropertyMap{})
				require.NoError(t, err)
				assert.Equal(t, resource.ID("missing ID"), got)
				assert.Empty(t, res.Fields)
			},
		},
		{
			name: "required ID delegates to renamed field",
			schema: schema.SchemaMap{
				"id": (&schema.Schema{
					Type:     shim.TypeString,
					Required: true,
				}).Shim(),
			},
			verify: func(t *testing.T, res *info.Resource) {
				t.Helper()
				require.Equal(t, "resId", res.Fields["id"].Name)
				require.NotNil(t, res.ComputeID)
				got, err := res.ComputeID(context.Background(), resource.PropertyMap{
					"resId": resource.NewStringProperty("abc"),
				})
				require.NoError(t, err)
				assert.Equal(t, resource.ID("abc"), got)
			},
		},
		{
			name: "computed non-string ID uses missing ID",
			schema: schema.SchemaMap{
				"id": (&schema.Schema{
					Type:     shim.TypeInt,
					Computed: true,
				}).Shim(),
			},
			verify: func(t *testing.T, res *info.Resource) {
				t.Helper()
				require.Equal(t, "resId", res.Fields["id"].Name)
				require.NotNil(t, res.ComputeID)
				got, err := res.ComputeID(context.Background(), resource.PropertyMap{})
				require.NoError(t, err)
				assert.Equal(t, resource.ID("missing ID"), got)
			},
		},
		{
			name: "reserved property names",
			schema: schema.SchemaMap{
				"id": (&schema.Schema{
					Type:     shim.TypeString,
					Computed: true,
				}).Shim(),
				"urn":    (&schema.Schema{Type: shim.TypeString}).Shim(),
				"pulumi": (&schema.Schema{Type: shim.TypeString}).Shim(),
			},
			verify: func(t *testing.T, res *info.Resource) {
				t.Helper()
				assert.Equal(t, "testUrn", res.Fields["urn"].Name)
				assert.Equal(t, "pulumiInfo", res.Fields["pulumi"].Name)
				assert.Nil(t, res.ComputeID)
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			metadataData, err := md.New(nil)
			require.NoError(t, err)
			var buildSchemaCalls atomic.Int32
			build := info.Provider{
				Name:         "test",
				P:            precomputedFixupProvider{Provider: countingResourceProvider(tt.schema, &buildSchemaCalls)},
				MetadataInfo: &info.Metadata{Data: metadataData, Path: "bridge-metadata.json"},
			}
			require.NoError(t, applyDefaultFixups(&build))
			require.Positive(t, buildSchemaCalls.Load(), "build-time fixups should inspect the schema")

			var runtimeSchemaCalls atomic.Int32
			runtime := info.Provider{
				Name: "test",
				P: precomputedFixupProvider{
					Provider: countingResourceProvider(tt.schema, &runtimeSchemaCalls),
				},
				MetadataInfo: info.NewProviderMetadata(
					runtimeMetadataBytes(t, build.MetadataInfo.Data)),
			}
			require.NoError(t, applyDefaultFixups(&runtime))
			require.Zero(t, runtimeSchemaCalls.Load(), "runtime fixups should use metadata instead of Schema()")
			require.NotNil(t, runtime.Resources["test_res"])
			tt.verify(t, runtime.Resources["test_res"])

			buildMapping := info.MarshalProvider(&build)
			runtimeMapping := info.MarshalProvider(&runtime)
			require.Contains(t, buildMapping.Resources, "test_res")
			require.Contains(t, runtimeMapping.Resources, "test_res")
			assert.Equal(t,
				buildMapping.Resources["test_res"].Fields,
				runtimeMapping.Resources["test_res"].Fields,
				"runtime metadata replay should preserve mapping field metadata")
		})
	}
}

func TestDefaultFixupMetadataPreservesComputeIDAcrossRepeatedBuildPasses(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		schema schema.SchemaMap
		verify func(*testing.T, *info.Resource)
	}{
		{
			name: "missing ID",
			schema: schema.SchemaMap{
				"some_property": (&schema.Schema{Type: shim.TypeString}).Shim(),
			},
			verify: func(t *testing.T, res *info.Resource) {
				t.Helper()
				require.NotNil(t, res.ComputeID)
				got, err := res.ComputeID(context.Background(), resource.PropertyMap{})
				require.NoError(t, err)
				assert.Equal(t, resource.ID("missing ID"), got)
			},
		},
		{
			name: "delegate ID",
			schema: schema.SchemaMap{
				"id": (&schema.Schema{
					Type:     shim.TypeString,
					Required: true,
				}).Shim(),
			},
			verify: func(t *testing.T, res *info.Resource) {
				t.Helper()
				require.Equal(t, "resId", res.Fields["id"].Name)
				require.NotNil(t, res.ComputeID)
				got, err := res.ComputeID(context.Background(), resource.PropertyMap{
					"resId": resource.NewStringProperty("abc"),
				})
				require.NoError(t, err)
				assert.Equal(t, resource.ID("abc"), got)
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			metadataData, err := md.New(nil)
			require.NoError(t, err)
			var buildSchemaCalls atomic.Int32
			build := info.Provider{
				Name:         "test",
				P:            precomputedFixupProvider{Provider: countingResourceProvider(tt.schema, &buildSchemaCalls)},
				MetadataInfo: &info.Metadata{Data: metadataData, Path: "bridge-metadata.json"},
			}

			require.NoError(t, applyDefaultFixups(&build))
			first, found, err := md.Get[defaultResourceSchemaFixups](
				metadataData, metadatakeys.DefaultResourceSchemaFixups)
			require.NoError(t, err)
			require.True(t, found)
			require.NotNil(t, first.Resources["test_res"].ComputeID)

			require.NoError(t, applyDefaultFixups(&build))
			second, found, err := md.Get[defaultResourceSchemaFixups](
				metadataData, metadatakeys.DefaultResourceSchemaFixups)
			require.NoError(t, err)
			require.True(t, found)
			require.NotNil(t, second.Resources["test_res"].ComputeID)
			assert.Equal(t, *first.Resources["test_res"].ComputeID, *second.Resources["test_res"].ComputeID)

			var runtimeSchemaCalls atomic.Int32
			runtime := info.Provider{
				Name: "test",
				P: precomputedFixupProvider{
					Provider: countingResourceProvider(tt.schema, &runtimeSchemaCalls),
				},
				MetadataInfo: info.NewProviderMetadata(
					runtimeMetadataBytes(t, metadataData)),
			}
			require.NoError(t, applyDefaultFixups(&runtime))
			require.Zero(t, runtimeSchemaCalls.Load(), "runtime fixups should use metadata instead of Schema()")
			tt.verify(t, runtime.Resources["test_res"])
		})
	}
}

func TestDefaultFixupMetadataRewritesCurrentResources(t *testing.T) {
	t.Parallel()

	metadataData, err := md.New(nil)
	require.NoError(t, err)
	require.NoError(t, md.Set(metadataData, metadatakeys.DefaultResourceSchemaFixups, defaultResourceSchemaFixups{
		Resources: map[string]defaultResourceSchemaFixup{
			"test_stale": {
				ComputeID: &defaultComputeIDFixup{Kind: defaultComputeIDMissing},
			},
		},
	}))

	var schemaCalls atomic.Int32
	build := info.Provider{
		Name: "test",
		P: precomputedFixupProvider{Provider: countingResourceProvider(schema.SchemaMap{
			"some_property": (&schema.Schema{Type: shim.TypeString}).Shim(),
		}, &schemaCalls)},
		MetadataInfo: &info.Metadata{Data: metadataData, Path: "bridge-metadata.json"},
	}
	require.NoError(t, applyDefaultFixups(&build))
	require.Positive(t, schemaCalls.Load(), "build-time fixups should inspect the schema")

	fixups, found, err := md.Get[defaultResourceSchemaFixups](
		metadataData, metadatakeys.DefaultResourceSchemaFixups)
	require.NoError(t, err)
	require.True(t, found)
	assert.Contains(t, fixups.Resources, "test_res")
	assert.NotContains(t, fixups.Resources, "test_stale")
}

func TestDefaultFixupMetadataClearsWhenNoCurrentResources(t *testing.T) {
	t.Parallel()

	metadataData, err := md.New(nil)
	require.NoError(t, err)
	require.NoError(t, md.Set(metadataData, metadatakeys.DefaultResourceSchemaFixups, defaultResourceSchemaFixups{
		Resources: map[string]defaultResourceSchemaFixup{
			"test_stale": {
				ComputeID: &defaultComputeIDFixup{Kind: defaultComputeIDMissing},
			},
		},
	}))

	build := info.Provider{
		Name: "test",
		P: precomputedFixupProvider{
			Provider: (&schema.Provider{ResourcesMap: schema.ResourceMap{}}).Shim(),
		},
		MetadataInfo: &info.Metadata{Data: metadataData, Path: "bridge-metadata.json"},
	}
	require.NoError(t, applyDefaultFixups(&build))

	_, found, err := md.Get[defaultResourceSchemaFixups](
		metadataData, metadatakeys.DefaultResourceSchemaFixups)
	require.NoError(t, err)
	assert.False(t, found)
}

func runtimeMetadataBytes(t *testing.T, data info.ProviderMetadata) []byte {
	t.Helper()
	runtimeMetadata := md.Clone((*md.Data)(data))
	require.NoError(t, md.Set(runtimeMetadata, metadatakeys.RuntimeMetadata, true))
	return runtimeMetadata.Marshal()
}

func countingResourceProvider(resourceSchema schema.SchemaMap, calls *atomic.Int32) shim.Provider {
	return (&schema.Provider{
		ResourcesMap: schema.ResourceMap{
			"test_res": countingResource{
				Resource: (&schema.Resource{Schema: resourceSchema}).Shim(),
				calls:    calls,
			},
		},
	}).Shim()
}

type precomputedFixupProvider struct {
	shim.Provider
}

func (p precomputedFixupProvider) ResourceSchemaFixupsMayBePrecomputed(tfToken string) bool {
	_, ok := p.ResourcesMap().GetOk(tfToken)
	return ok
}

type countingResource struct {
	shim.Resource
	calls *atomic.Int32
}

func (r countingResource) Schema() shim.SchemaMap {
	r.calls.Add(1)
	return r.Resource.Schema()
}
