// Copyright 2016-2026, Pulumi Corporation.
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

package schemashim

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	tflist "github.com/hashicorp/terraform-plugin-framework/list"
	lschema "github.com/hashicorp/terraform-plugin-framework/list/schema"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/stretchr/testify/require"

	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

func TestShimSchemaOnlyProviderDefersResourceAndDataSourceSchemas(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	prov := newCountingProvider("test", []string{"one", "two"}, []string{"lookup"})

	shimmed := ShimSchemaOnlyProvider(ctx, prov).(*SchemaOnlyProvider)

	require.Zero(t, prov.resourceSchemaCalls("one"))
	require.Zero(t, prov.resourceSchemaCalls("two"))
	require.Zero(t, prov.dataSourceSchemaCalls("lookup"))

	resources := shimmed.ResourcesMap()
	require.Equal(t, 2, resources.Len())
	require.NotNil(t, resources.Get("test_one"))
	_, ok := resources.GetOk("test_two")
	require.True(t, ok)
	resources.Range(func(_ string, r shim.Resource) bool {
		require.NotNil(t, r)
		return true
	})

	dataSources := shimmed.DataSourcesMap()
	require.Equal(t, 1, dataSources.Len())
	require.NotNil(t, dataSources.Get("test_lookup"))
	dataSources.Range(func(_ string, ds shim.Resource) bool {
		require.NotNil(t, ds)
		return true
	})

	require.Zero(t, prov.resourceSchemaCalls("one"))
	require.Zero(t, prov.resourceSchemaCalls("two"))
	require.Zero(t, prov.dataSourceSchemaCalls("lookup"))

	resources.Get("test_one").Schema()
	resources.Get("test_one").Schema()
	require.Equal(t, int32(1), prov.resourceSchemaCalls("one"))
	require.Zero(t, prov.resourceSchemaCalls("two"))

	dataSources.Get("test_lookup").Schema()
	dataSources.Get("test_lookup").Schema()
	require.Equal(t, int32(1), prov.dataSourceSchemaCalls("lookup"))
}

func TestShimSchemaOnlyProviderDefersListResourceSchemas(t *testing.T) {
	t.Parallel()

	prov := newCountingProvider("test", nil, nil)
	prov.addListResources("one", "two")

	shimmed := ShimSchemaOnlyProvider(t.Context(), prov).(*SchemaOnlyProvider)
	require.Zero(t, prov.listResourceSchemaCalls("one"))
	require.Zero(t, prov.listResourceSchemaCalls("two"))

	listResources := shimmed.ListResourcesMap()
	require.Equal(t, 2, listResources.Len())
	require.NotNil(t, listResources.Get("test_one"))
	_, ok := listResources.GetOk("test_two")
	require.True(t, ok)
	listResources.Range(func(_ string, r shim.Resource) bool {
		require.NotNil(t, r)
		return true
	})

	require.Zero(t, prov.listResourceSchemaCalls("one"))
	require.Zero(t, prov.listResourceSchemaCalls("two"))

	listResources.Get("test_one").Schema()
	listResources.Get("test_one").Schema()
	require.Equal(t, int32(1), prov.listResourceSchemaCalls("one"))
	require.Zero(t, prov.listResourceSchemaCalls("two"))
}

func TestLazySchemaLoadsOnceUnderConcurrentAccess(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	prov := newCountingProvider("test", []string{"thing"}, nil)
	shimmed := ShimSchemaOnlyProvider(ctx, prov).(*SchemaOnlyProvider)
	resource := shimmed.ResourcesMap().Get("test_thing")

	var wg sync.WaitGroup
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resource.Schema()
		}()
	}
	wg.Wait()

	require.Equal(t, int32(1), prov.resourceSchemaCalls("thing"))
}

func TestSchemaOnlyProviderServerDoesNotRequireFullProviderSchemaForResourceNames(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	prov := newCountingProvider("test", []string{"thing"}, nil)
	shimmed := ShimSchemaOnlyProvider(ctx, prov).(*SchemaOnlyProvider)

	server, err := shimmed.Server(ctx)
	require.NoError(t, err)
	require.Zero(t, prov.providerSchemaCalls.Load())
	require.Zero(t, prov.resourceSchemaCalls("thing"))

	configType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{}}
	configValue, err := tfprotov6.NewDynamicValue(configType, tftypes.NewValue(configType, map[string]tftypes.Value{}))
	require.NoError(t, err)

	resp, err := server.ValidateResourceConfig(ctx, &tfprotov6.ValidateResourceConfigRequest{
		TypeName: "test_thing",
		Config:   &configValue,
	})
	require.NoError(t, err)
	require.Empty(t, resp.Diagnostics)
	require.Zero(t, prov.providerSchemaCalls.Load())
	require.Equal(t, int32(1), prov.resourceSchemaCalls("thing"))
}

func TestLazySchemaDoesNotUseCanceledConstructionContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	prov := newCountingProvider("test", []string{"thing"}, nil)
	prov.resourceSchemaFailsOnCanceledContext["thing"] = true
	shimmed := ShimSchemaOnlyProvider(ctx, prov).(*SchemaOnlyProvider)
	cancel()

	require.NotPanics(t, func() {
		shimmed.ResourcesMap().Get("test_thing").Schema()
	})
	require.Equal(t, int32(1), prov.resourceSchemaCalls("thing"))
}

func TestLazySchemaPanicsWithStableContextualError(t *testing.T) {
	t.Parallel()

	prov := newCountingProvider("test", []string{"boom"}, nil)
	prov.resourceSchemaPanics["boom"] = true
	shimmed := ShimSchemaOnlyProvider(context.Background(), prov).(*SchemaOnlyProvider)
	resource := shimmed.ResourcesMap().Get("test_boom")

	first := panicMessage(func() { resource.Schema() })
	second := panicMessage(func() { resource.Schema() })

	require.Contains(t, first, "failed to load Terraform Plugin Framework resource schema test_boom")
	require.Contains(t, first, "panic: schema exploded")
	require.Equal(t, first, second)
	require.Equal(t, int32(1), prov.resourceSchemaCalls("boom"))
}

type countingProvider struct {
	typeName string

	resources     map[string]*atomic.Int32
	dataSources   map[string]*atomic.Int32
	listResources map[string]*atomic.Int32

	resourceSchemaFailsOnCanceledContext map[string]bool
	resourceSchemaPanics                 map[string]bool

	providerSchemaCalls atomic.Int32
}

func newCountingProvider(typeName string, resources, dataSources []string) *countingProvider {
	prov := &countingProvider{
		typeName:                             typeName,
		resources:                            map[string]*atomic.Int32{},
		dataSources:                          map[string]*atomic.Int32{},
		listResources:                        map[string]*atomic.Int32{},
		resourceSchemaFailsOnCanceledContext: map[string]bool{},
		resourceSchemaPanics:                 map[string]bool{},
	}
	for _, name := range resources {
		prov.resources[name] = &atomic.Int32{}
	}
	for _, name := range dataSources {
		prov.dataSources[name] = &atomic.Int32{}
	}
	return prov
}

func (p *countingProvider) addListResources(names ...string) {
	for _, name := range names {
		p.listResources[name] = &atomic.Int32{}
	}
}

func (p *countingProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = p.typeName
}

func (p *countingProvider) Schema(context.Context, provider.SchemaRequest, *provider.SchemaResponse) {
	p.providerSchemaCalls.Add(1)
}

func (*countingProvider) Configure(context.Context, provider.ConfigureRequest, *provider.ConfigureResponse) {
}

func (p *countingProvider) Resources(context.Context) []func() resource.Resource {
	resources := make([]func() resource.Resource, 0, len(p.resources))
	for name, calls := range p.resources {
		name := name
		calls := calls
		failOnCanceledContext := p.resourceSchemaFailsOnCanceledContext[name]
		panicOnSchema := p.resourceSchemaPanics[name]
		resources = append(resources, func() resource.Resource {
			return &countingResource{
				name:                  name,
				schemaCalls:           calls,
				failOnCanceledContext: failOnCanceledContext,
				panicOnSchema:         panicOnSchema,
			}
		})
	}
	return resources
}

func (p *countingProvider) DataSources(context.Context) []func() datasource.DataSource {
	dataSources := make([]func() datasource.DataSource, 0, len(p.dataSources))
	for name, calls := range p.dataSources {
		name := name
		calls := calls
		dataSources = append(dataSources, func() datasource.DataSource {
			return &countingDataSource{name: name, schemaCalls: calls}
		})
	}
	return dataSources
}

func (p *countingProvider) ListResources(context.Context) []func() tflist.ListResource {
	listResources := make([]func() tflist.ListResource, 0, len(p.listResources))
	for name, calls := range p.listResources {
		name := name
		calls := calls
		listResources = append(listResources, func() tflist.ListResource {
			return &countingListResource{name: name, schemaCalls: calls}
		})
	}
	return listResources
}

func (p *countingProvider) resourceSchemaCalls(name string) int32 {
	return p.resources[name].Load()
}

func (p *countingProvider) dataSourceSchemaCalls(name string) int32 {
	return p.dataSources[name].Load()
}

func (p *countingProvider) listResourceSchemaCalls(name string) int32 {
	return p.listResources[name].Load()
}

type countingResource struct {
	name                  string
	schemaCalls           *atomic.Int32
	failOnCanceledContext bool
	panicOnSchema         bool
}

func (r *countingResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_" + r.name
}

func (r *countingResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	r.schemaCalls.Add(1)
	if r.panicOnSchema {
		panic("schema exploded")
	}
	if r.failOnCanceledContext && ctx.Err() != nil {
		resp.Diagnostics.AddError("unexpected canceled context", ctx.Err().Error())
	}
}

func (*countingResource) Create(context.Context, resource.CreateRequest, *resource.CreateResponse) {}
func (*countingResource) Read(context.Context, resource.ReadRequest, *resource.ReadResponse)       {}
func (*countingResource) Update(context.Context, resource.UpdateRequest, *resource.UpdateResponse) {}
func (*countingResource) Delete(context.Context, resource.DeleteRequest, *resource.DeleteResponse) {}

type countingDataSource struct {
	name        string
	schemaCalls *atomic.Int32
}

func (ds *countingDataSource) Metadata(
	_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse,
) {
	resp.TypeName = req.ProviderTypeName + "_" + ds.name
}

func (ds *countingDataSource) Schema(context.Context, datasource.SchemaRequest, *datasource.SchemaResponse) {
	ds.schemaCalls.Add(1)
}

func (*countingDataSource) Read(context.Context, datasource.ReadRequest, *datasource.ReadResponse) {}

type countingListResource struct {
	name        string
	schemaCalls *atomic.Int32
}

func (r *countingListResource) Metadata(
	_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse,
) {
	resp.TypeName = req.ProviderTypeName + "_" + r.name
}

func (r *countingListResource) ListResourceConfigSchema(
	_ context.Context, _ tflist.ListResourceSchemaRequest, resp *tflist.ListResourceSchemaResponse,
) {
	r.schemaCalls.Add(1)
	resp.Schema = lschema.Schema{
		Attributes: map[string]lschema.Attribute{
			"filter": lschema.StringAttribute{Optional: true},
		},
	}
}

func (*countingListResource) List(context.Context, tflist.ListRequest, *tflist.ListResultsStream) {}

var (
	_ provider.Provider     = (*countingProvider)(nil)
	_ resource.Resource     = (*countingResource)(nil)
	_ datasource.DataSource = (*countingDataSource)(nil)
	_ tflist.ListResource   = (*countingListResource)(nil)
)

func panicMessage(f func()) (message string) {
	defer func() {
		if p := recover(); p != nil {
			message = fmt.Sprint(p)
		}
	}()
	f()
	return ""
}
