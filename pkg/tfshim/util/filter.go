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

package util

import (
	"context"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

// Filters provider ResourceMap and DataSourceMap by token. When reproducing issues on a large provider it can be
// helpful to wrap the provider in a FilteringProvider temporarily to only recompile and build schema for a handful
// problematic resources. For example:
//
//      p := &util.FilteringProvider{
//              Provider: shimv2.NewProvider(shim.NewProvider()),
//              ResourceFilter: func(tok string) bool {
//                      return tok == "azurerm_storage_management_policy"
//              },
//              DataSourceFilter: func(string) bool {
//                      return false
//              },
//      }
//
// Note that on a large provider Resources and DataSources mapping in ProviderInfo may now contain tokens that are
// removed by FilteringProvider, causing errors. These errors can be ignored while debugging:
//
//      PULUMI_SKIP_EXTRA_MAPPING_ERROR=1 make provider

type FilteringProvider struct {
	Provider         shim.Provider
	ResourceFilter   func(token string) bool
	DataSourceFilter func(token string) bool
}

var _ = (shim.Provider)((*FilteringProvider)(nil))

func (p *FilteringProvider) Schema() shim.SchemaMap {
	return p.Provider.Schema()
}

func (p *FilteringProvider) ResourcesMap() shim.ResourceMap {
	return &filteringMap{p.Provider.ResourcesMap(), p.ResourceFilter}
}

func (p *FilteringProvider) DataSourcesMap() shim.ResourceMap {
	return &filteringMap{p.Provider.DataSourcesMap(), p.DataSourceFilter}
}

func (p *FilteringProvider) Validate(ctx context.Context, c shim.ResourceConfig) ([]string, []error) {
	return p.Provider.Validate(ctx, c)
}

func (p *FilteringProvider) ValidateResource(
	ctx context.Context, t string, c shim.ResourceConfig,
) ([]string, []error) {
	return p.Provider.ValidateResource(ctx, t, c)
}

func (p *FilteringProvider) ValidateDataSource(
	ctx context.Context, t string, c shim.ResourceConfig,
) ([]string, []error) {
	return p.Provider.ValidateDataSource(ctx, t, c)
}

func (p *FilteringProvider) Configure(ctx context.Context, c shim.ResourceConfig) error {
	return p.Provider.Configure(ctx, c)
}

func (p *FilteringProvider) Diff(
	ctx context.Context, t string, s shim.InstanceState, c shim.ResourceConfig, opts shim.DiffOptions,
) (shim.InstanceDiff, error) {
	return p.Provider.Diff(ctx, t, s, c, opts)
}

func (p *FilteringProvider) Apply(
	ctx context.Context, t string, s shim.InstanceState, d shim.InstanceDiff,
) (shim.InstanceState, error) {
	return p.Provider.Apply(ctx, t, s, d)
}

func (p *FilteringProvider) Refresh(
	ctx context.Context, t string, s shim.InstanceState, c shim.ResourceConfig,
) (shim.InstanceState, error) {
	return p.Provider.Refresh(ctx, t, s, c)
}

func (p *FilteringProvider) ReadDataDiff(
	ctx context.Context, t string, c shim.ResourceConfig,
) (shim.InstanceDiff, error) {
	return p.Provider.ReadDataDiff(ctx, t, c)
}

func (p *FilteringProvider) ReadDataApply(
	ctx context.Context, t string, d shim.InstanceDiff,
) (shim.InstanceState, error) {
	return p.Provider.ReadDataApply(ctx, t, d)
}

func (p *FilteringProvider) Meta(ctx context.Context) interface{} {
	return p.Provider.Meta(ctx)
}

func (p *FilteringProvider) Stop(ctx context.Context) error {
	return p.Provider.Stop(ctx)
}

func (p *FilteringProvider) InitLogging(ctx context.Context) {
	p.Provider.InitLogging(ctx)
}

func (p *FilteringProvider) NewDestroyDiff(ctx context.Context, t string) shim.InstanceDiff {
	return p.Provider.NewDestroyDiff(ctx, t)
}

func (p *FilteringProvider) NewResourceConfig(
	ctx context.Context, object map[string]interface{},
) shim.ResourceConfig {
	return p.Provider.NewResourceConfig(ctx, object)
}

func (p *FilteringProvider) IsSet(ctx context.Context, v interface{}) ([]interface{}, bool) {
	return p.Provider.IsSet(ctx, v)
}

type filteringMap struct {
	inner       shim.ResourceMap
	tokenFilter func(string) bool
}

func (f *filteringMap) Range(each func(key string, value shim.Resource) bool) {
	f.inner.Range(func(key string, value shim.Resource) bool {
		if f.tokenFilter != nil && !f.tokenFilter(key) {
			return true
		}
		return each(key, value)
	})
}

func (f *filteringMap) Len() int {
	n := 0
	f.Range(func(key string, value shim.Resource) bool {
		n = n + 1
		return true
	})
	return n
}

func (f *filteringMap) Get(key string) shim.Resource {
	return f.inner.Get(key)
}

func (f *filteringMap) GetOk(key string) (shim.Resource, bool) {
	return f.inner.GetOk(key)
}

func (f *filteringMap) Set(key string, value shim.Resource) {
	f.inner.Set(key, value)
}

var _ shim.ResourceMap = (*filteringMap)(nil)
