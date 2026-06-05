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

package tfbridgetests

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	tflist "github.com/hashicorp/terraform-plugin-framework/list"
	lschema "github.com/hashicorp/terraform-plugin-framework/list/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	sdkschema "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	presource "github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/require"
	grpcmetadata "google.golang.org/grpc/metadata"

	tfpf "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tfbridge"
	tfbridge0 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	bridgetokens "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/tokens"
	sdkv2 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v2"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/x/muxer"
	"github.com/pulumi/pulumi-terraform-bridge/v3/unstable/metadata"
)

func TestMuxedSDKv2OperationsDoNotLoadPFResourceSchemas(t *testing.T) {
	t.Parallel()

	metadataInfo := muxLazyProviderInfo(newMuxCountingProvider())
	metadata := genSDKSchema(t, metadataInfo)

	pfProvider := newMuxCountingProvider()
	info := muxLazyProviderInfo(pfProvider)
	server := newMuxedProviderServerWithSchema(t, info, metadata)
	pfProvider.requireZeroSchemaCalls(t)

	news, err := plugin.MarshalProperties(nil, plugin.MarshalOptions{})
	require.NoError(t, err)
	_, err = server.Check(t.Context(), &pulumirpc.CheckRequest{
		Urn:  "urn:pulumi:dev::proj::muxlazy:index/sdk:Sdk::sdk",
		News: news,
	})
	require.NoError(t, err)
	pfProvider.requireZeroSchemaCalls(t)

	_, err = server.Check(t.Context(), &pulumirpc.CheckRequest{
		Urn:  "urn:pulumi:dev::proj::muxlazy:index/one:One::one",
		News: news,
	})
	require.NoError(t, err)
	resourceOneCalls := pfProvider.resourceSchemaCalls("one")
	require.Positive(t, resourceOneCalls)
	require.Zero(t, pfProvider.resourceSchemaCalls("two"))
	require.Zero(t, pfProvider.resourceSchemaCalls("alias"))
	require.Zero(t, pfProvider.listResourceSchemaCalls("one"))
	require.Zero(t, pfProvider.listResourceSchemaCalls("two"))
	require.Zero(t, pfProvider.dataSourceSchemaCalls("lookup"))

	_, err = server.Invoke(t.Context(), &pulumirpc.InvokeRequest{
		Tok:  "muxlazy:index/getLookup:getLookup",
		Args: news,
	})
	require.NoError(t, err)
	require.Equal(t, resourceOneCalls, pfProvider.resourceSchemaCalls("one"))
	require.Zero(t, pfProvider.resourceSchemaCalls("two"))
	require.Zero(t, pfProvider.resourceSchemaCalls("alias"))
	require.Positive(t, pfProvider.dataSourceSchemaCalls("lookup"))
	dataSourceLookupCalls := pfProvider.dataSourceSchemaCalls("lookup")

	query, err := plugin.MarshalProperties(presource.PropertyMap{
		"filter": presource.NewStringProperty("selected"),
		"labels": presource.NewArrayProperty([]presource.PropertyValue{
			presource.NewStringProperty("east"),
			presource.NewStringProperty("west"),
		}),
	}, plugin.MarshalOptions{})
	require.NoError(t, err)

	stream := newMuxRecordingListStream(t.Context())
	err = server.List(&pulumirpc.ListRequest{
		Token: "muxlazy:index/one:One",
		Query: query,
	}, stream)
	require.NoError(t, err)
	require.Len(t, stream.sent, 1)
	require.Equal(t, "", stream.sent[0].GetContinuation().GetContinuationToken())
	require.Equal(t, resourceOneCalls, pfProvider.resourceSchemaCalls("one"))
	require.Zero(t, pfProvider.resourceSchemaCalls("two"))
	require.Zero(t, pfProvider.resourceSchemaCalls("alias"))
	require.Positive(t, pfProvider.listResourceSchemaCalls("one"))
	require.Zero(t, pfProvider.listResourceSchemaCalls("two"))
	require.Equal(t, "selected|east,west", pfProvider.listQuery("one"))
	require.Equal(t, dataSourceLookupCalls, pfProvider.dataSourceSchemaCalls("lookup"))

	_, err = server.Check(t.Context(), &pulumirpc.CheckRequest{
		Urn:  "urn:pulumi:dev::proj::muxlazy:index/aliasNew:AliasNew::alias-new",
		News: news,
	})
	require.NoError(t, err)
	aliasCalls := pfProvider.resourceSchemaCalls("alias")
	require.Positive(t, aliasCalls)
	require.Equal(t, resourceOneCalls, pfProvider.resourceSchemaCalls("one"))
	require.Zero(t, pfProvider.resourceSchemaCalls("two"))
	require.Positive(t, pfProvider.listResourceSchemaCalls("one"))
	require.Zero(t, pfProvider.listResourceSchemaCalls("two"))

	_, err = server.Check(t.Context(), &pulumirpc.CheckRequest{
		Urn:  "urn:pulumi:dev::proj::muxlazy:index/alias:Alias::alias-legacy",
		News: news,
	})
	require.NoError(t, err)
	require.Equal(t, aliasCalls, pfProvider.resourceSchemaCalls("alias"))
}

func TestMuxedRuntimeComputeTokensDoesNotLoadPFResourceSchemas(t *testing.T) {
	t.Parallel()

	buildInfo := muxLazyProviderInfo(newMuxCountingProvider())
	require.NoError(t, buildInfo.ComputeTokens(bridgetokens.SingleModule(
		buildInfo.GetResourcePrefix(), "index", bridgetokens.MakeStandard(buildInfo.Name),
	)))

	pfProvider := newMuxCountingProvider()
	runtimeInfo := muxLazyProviderInfo(pfProvider)
	runtimeMetadata := tfbridge0.ExtractRuntimeMetadata(buildInfo.MetadataInfo)
	runtimeInfo.MetadataInfo = tfbridge0.NewProviderMetadata(
		(*metadata.Data)(runtimeMetadata.Data).Marshal())
	require.NoError(t, runtimeInfo.ComputeTokens(bridgetokens.SingleModule(
		runtimeInfo.GetResourcePrefix(), "index", bridgetokens.MakeStandard(runtimeInfo.Name),
	)))
	pfProvider.requireZeroSchemaCalls(t)
}

func muxLazyProviderInfo(pfProvider *muxCountingProvider) tfbridge0.ProviderInfo {
	info := tfbridge0.ProviderInfo{
		Name:             "muxlazy",
		Version:          "0.0.1",
		UpstreamRepoPath: ".",
		MetadataInfo:     tfbridge0.NewProviderMetadata([]byte("{}")),
		P: tfpf.MuxShimWithPF(context.Background(),
			sdkv2.NewProvider(muxLazySDKProvider()),
			pfProvider),
		Resources: map[string]*tfbridge0.ResourceInfo{
			"test_sdk": {
				Tok:  tokens.Type("muxlazy:index/sdk:Sdk"),
				Docs: &tfbridge0.DocInfo{Markdown: []byte{' '}},
			},
			"test_one": {
				Tok:  tokens.Type("muxlazy:index/one:One"),
				Docs: &tfbridge0.DocInfo{Markdown: []byte{' '}},
			},
			"test_two": {
				Tok:  tokens.Type("muxlazy:index/two:Two"),
				Docs: &tfbridge0.DocInfo{Markdown: []byte{' '}},
			},
			"test_alias": {
				Tok:  tokens.Type("muxlazy:index/aliasNew:AliasNew"),
				Docs: &tfbridge0.DocInfo{Markdown: []byte{' '}},
			},
		},
		DataSources: map[string]*tfbridge0.DataSourceInfo{
			"test_lookup": {
				Tok:  tokens.ModuleMember("muxlazy:index/getLookup:getLookup"),
				Docs: &tfbridge0.DocInfo{Markdown: []byte{' '}},
			},
		},
	}
	info.RenameResourceWithAlias(
		"test_alias",
		tokens.Type("muxlazy:index/alias:Alias"),
		tokens.Type("muxlazy:index/aliasNew:AliasNew"),
		"index",
		"index",
		info.Resources["test_alias"],
	)
	return info
}

func muxLazySDKProvider() *sdkschema.Provider {
	return &sdkschema.Provider{
		ResourcesMap: map[string]*sdkschema.Resource{
			"test_sdk": {
				Schema: map[string]*sdkschema.Schema{
					"value": {Type: sdkschema.TypeString, Optional: true},
				},
			},
		},
	}
}

func newMuxedProviderServerWithSchema(
	t *testing.T, info tfbridge0.ProviderInfo, schemaBytes []byte,
) pulumirpc.ResourceProviderServer {
	resolver := info.P.(interface {
		ResolveDispatch(*tfbridge0.ProviderInfo) (muxer.DispatchTable, error)
	})
	dispatch, err := resolver.ResolveDispatch(&info)
	require.NoError(t, err)
	require.NoError(t, metadata.Set(info.GetMetadata(), "mux", dispatch))

	p, err := tfpf.MakeMuxedServer(t.Context(), info.Name, info, schemaBytes)(nil)
	require.NoError(t, err)
	return p
}

type muxCountingProvider struct {
	resources     map[string]*atomic.Int32
	dataSources   map[string]*atomic.Int32
	listResources map[string]*atomic.Int32
	listQueries   map[string]*atomic.Value
}

func newMuxCountingProvider() *muxCountingProvider {
	return &muxCountingProvider{
		resources: map[string]*atomic.Int32{
			"one":   {},
			"two":   {},
			"alias": {},
		},
		dataSources: map[string]*atomic.Int32{
			"lookup": {},
		},
		listResources: map[string]*atomic.Int32{
			"one": {},
			"two": {},
		},
		listQueries: map[string]*atomic.Value{
			"one": {},
			"two": {},
		},
	}
}

func (*muxCountingProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "test"
}

func (*muxCountingProvider) Schema(context.Context, provider.SchemaRequest, *provider.SchemaResponse) {
}

func (*muxCountingProvider) Configure(context.Context, provider.ConfigureRequest, *provider.ConfigureResponse) {
}

func (p *muxCountingProvider) Resources(context.Context) []func() resource.Resource {
	resources := make([]func() resource.Resource, 0, len(p.resources))
	for name, calls := range p.resources {
		name := name
		calls := calls
		resources = append(resources, func() resource.Resource {
			return &muxCountingResource{name: name, schemaCalls: calls}
		})
	}
	return resources
}

func (p *muxCountingProvider) DataSources(context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		func() datasource.DataSource {
			return &muxCountingDataSource{name: "lookup", schemaCalls: p.dataSources["lookup"]}
		},
	}
}

func (p *muxCountingProvider) ListResources(context.Context) []func() tflist.ListResource {
	listResources := make([]func() tflist.ListResource, 0, len(p.listResources))
	for name, calls := range p.listResources {
		name := name
		calls := calls
		query := p.listQueries[name]
		listResources = append(listResources, func() tflist.ListResource {
			return &muxCountingListResource{name: name, schemaCalls: calls, lastQuery: query}
		})
	}
	return listResources
}

func (p *muxCountingProvider) resourceSchemaCalls(name string) int32 {
	return p.resources[name].Load()
}

func (p *muxCountingProvider) dataSourceSchemaCalls(name string) int32 {
	return p.dataSources[name].Load()
}

func (p *muxCountingProvider) listResourceSchemaCalls(name string) int32 {
	return p.listResources[name].Load()
}

func (p *muxCountingProvider) listQuery(name string) string {
	value := p.listQueries[name].Load()
	if value == nil {
		return ""
	}
	return value.(string)
}

func (p *muxCountingProvider) requireZeroSchemaCalls(t *testing.T) {
	t.Helper()
	for name := range p.resources {
		require.Zero(t, p.resourceSchemaCalls(name))
	}
	for name := range p.dataSources {
		require.Zero(t, p.dataSourceSchemaCalls(name))
	}
	for name := range p.listResources {
		require.Zero(t, p.listResourceSchemaCalls(name))
	}
}

type muxCountingResource struct {
	name        string
	schemaCalls *atomic.Int32
}

func (r *muxCountingResource) Metadata(
	_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse,
) {
	resp.TypeName = req.ProviderTypeName + "_" + r.name
}

func (r *muxCountingResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	r.schemaCalls.Add(1)
	resp.Schema = muxCountingResourceSchema
}

func (*muxCountingResource) Create(context.Context, resource.CreateRequest, *resource.CreateResponse) {
}

func (*muxCountingResource) Read(context.Context, resource.ReadRequest, *resource.ReadResponse) {
}

func (*muxCountingResource) Update(context.Context, resource.UpdateRequest, *resource.UpdateResponse) {
}

func (*muxCountingResource) Delete(context.Context, resource.DeleteRequest, *resource.DeleteResponse) {
}

type muxCountingDataSource struct {
	name        string
	schemaCalls *atomic.Int32
}

func (ds *muxCountingDataSource) Metadata(
	_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse,
) {
	resp.TypeName = req.ProviderTypeName + "_" + ds.name
}

func (ds *muxCountingDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	ds.schemaCalls.Add(1)
	resp.Schema = muxCountingDataSourceSchema
}

func (*muxCountingDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("result"), "ok")...)
}

type muxCountingListResource struct {
	name        string
	schemaCalls *atomic.Int32
	lastQuery   *atomic.Value
}

func (r *muxCountingListResource) Metadata(
	_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse,
) {
	resp.TypeName = req.ProviderTypeName + "_" + r.name
}

func (r *muxCountingListResource) ListResourceConfigSchema(
	_ context.Context, _ tflist.ListResourceSchemaRequest, resp *tflist.ListResourceSchemaResponse,
) {
	r.schemaCalls.Add(1)
	resp.Schema = muxCountingListResourceSchema
}

func (r *muxCountingListResource) List(
	ctx context.Context, req tflist.ListRequest, stream *tflist.ListResultsStream,
) {
	var filter string
	var labels []string
	diags := req.Config.GetAttribute(ctx, path.Root("filter"), &filter)
	diags.Append(req.Config.GetAttribute(ctx, path.Root("labels"), &labels)...)
	if diags.HasError() {
		stream.Results = tflist.ListResultsStreamDiagnostics(diags)
		return
	}
	r.lastQuery.Store(filter + "|" + strings.Join(labels, ","))
	stream.Results = tflist.NoListResults
}

type muxRecordingListStream struct {
	ctx  context.Context
	sent []*pulumirpc.ListResponse
}

func newMuxRecordingListStream(ctx context.Context) *muxRecordingListStream {
	return &muxRecordingListStream{ctx: ctx}
}

func (s *muxRecordingListStream) Send(resp *pulumirpc.ListResponse) error {
	s.sent = append(s.sent, resp)
	return nil
}

func (s *muxRecordingListStream) SetHeader(grpcmetadata.MD) error { return nil }

func (s *muxRecordingListStream) SendHeader(grpcmetadata.MD) error { return nil }

func (s *muxRecordingListStream) SetTrailer(grpcmetadata.MD) {}

func (s *muxRecordingListStream) Context() context.Context { return s.ctx }

func (s *muxRecordingListStream) SendMsg(any) error { return nil }

func (s *muxRecordingListStream) RecvMsg(any) error { return nil }

var (
	_ provider.Provider                  = (*muxCountingProvider)(nil)
	_ provider.ProviderWithListResources = (*muxCountingProvider)(nil)
	_ resource.Resource                  = (*muxCountingResource)(nil)
	_ datasource.DataSource              = (*muxCountingDataSource)(nil)
	_ tflist.ListResource                = (*muxCountingListResource)(nil)
)

var (
	muxCountingResourceSchema = rschema.Schema{
		Attributes: map[string]rschema.Attribute{
			"id":    rschema.StringAttribute{Computed: true},
			"value": rschema.StringAttribute{Optional: true},
		},
	}
	muxCountingDataSourceSchema = dschema.Schema{
		Attributes: map[string]dschema.Attribute{
			"query":  dschema.StringAttribute{Optional: true},
			"result": dschema.StringAttribute{Computed: true},
		},
	}
	muxCountingListResourceSchema = lschema.Schema{
		Attributes: map[string]lschema.Attribute{
			"filter": lschema.StringAttribute{Optional: true},
			"labels": lschema.ListAttribute{ElementType: types.StringType, Optional: true},
		},
	}
)
