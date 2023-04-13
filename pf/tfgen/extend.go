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

package tfgen

import (
	"fmt"
	"sort"
	"strings"

	"context"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/pulumi/pulumi-terraform-bridge/pf/internal/schemashim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	shimSchema "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/schema"
	"github.com/pulumi/pulumi-terraform-bridge/x/muxer"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

// Supports extending schema generation with resources and functions build against another framework.
type Extension interface {
	Extend(*tfbridge.ProviderInfo) (*tfbridge.ProviderInfo, error)
	NewDataSources() []tokens.ModuleMember
	NewResources() []tokens.Type
}

func SchemaOnlyPluginFrameworkProvider(ctx context.Context, provider provider.Provider) shim.Provider {
	return schemashim.ShimSchemaOnlyProvider(ctx, provider)
}

func Extend(info *tfbridge.ProviderInfo, extensions ...Extension) (*tfbridge.ProviderInfo, error) {
	copy := *info
	for _, e := range extensions {
		extended, err := e.Extend(&copy)
		if err != nil {
			return nil, err
		}
		copy = *extended
	}
	return &copy, nil
}

type ExtensionWithIndex struct {
	Extension     Extension
	ProviderIndex int
}

func ComputeExtendedDispatchTable(baselineProviderIndex int, extensions ...ExtensionWithIndex) *muxer.DispatchTable {
	dt := muxer.NewDispatchTable()
	dt.ConfigDefault = []int{baselineProviderIndex}
	dt.FunctionsDefault = &baselineProviderIndex
	dt.ResourcesDefault = &baselineProviderIndex
	for _, e := range extensions {
		for _, ds := range e.Extension.NewDataSources() {
			dt.Functions[string(ds)] = e.ProviderIndex
		}
		for _, res := range e.Extension.NewResources() {
			dt.Resources[string(res)] = e.ProviderIndex
		}
	}
	return dt
}

func NewResourceExtension(provider shim.Provider, resources map[string]*tfbridge.ResourceInfo) Extension {
	return &extension{
		provider:  provider,
		resources: resources,
	}
}

func NewDataSourceExtension(provider shim.Provider, dataSources map[string]*tfbridge.DataSourceInfo) Extension {
	return &extension{
		provider:    provider,
		dataSources: dataSources,
	}
}

type extension struct {
	provider    shim.Provider
	resources   map[string]*tfbridge.ResourceInfo
	dataSources map[string]*tfbridge.DataSourceInfo
}

var _ Extension = (*extension)(nil)

func (ext *extension) Extend(info *tfbridge.ProviderInfo) (*tfbridge.ProviderInfo, error) {
	copy := *info

	if ext.resources != nil {
		resources, err := disjointUnion(copy.P.ResourcesMap(), ext.provider.ResourcesMap())
		if err != nil {
			return nil, fmt.Errorf("failing to apply a ResourceExtension, ResourcesMap maps are not dijsoint: %v", err)
		}
		resourceInfos, err := disjointMapUnion(copy.Resources, ext.resources)
		if err != nil {
			return nil, fmt.Errorf("failing to apply a ResourceExtension, Resources maps are not dijsoint: %v", err)
		}
		copy.P = &simpleSchemaProvider{
			schema:      copy.P.Schema(),
			resources:   resources,
			dataSources: copy.P.DataSourcesMap(),
		}
		copy.Resources = resourceInfos
	}

	if ext.dataSources != nil {
		dsMap, err := disjointUnion(copy.P.DataSourcesMap(), ext.provider.DataSourcesMap())
		if err != nil {
			return nil, fmt.Errorf("failing to apply a DataSourceExtension, datasource maps are not dijsoint: %v", err)
		}
		dsInfos, err := disjointMapUnion(copy.DataSources, ext.dataSources)
		if err != nil {
			return nil, fmt.Errorf("failing to apply a DataSoruceExtension, datasource maps are not dijsoint: %v", err)
		}
		copy.P = &simpleSchemaProvider{
			schema:      copy.P.Schema(),
			resources:   copy.P.ResourcesMap(),
			dataSources: dsMap,
		}
		copy.DataSources = dsInfos
	}

	return &copy, nil
}

func (ext *extension) NewDataSources() (ds []tokens.ModuleMember) {
	for _, d := range ext.dataSources {
		ds = append(ds, d.Tok)
	}
	sort.SliceStable(ds, func(i, j int) bool {
		return string(ds[i]) < string(ds[j])
	})
	return
}

func (ext *extension) NewResources() (rs []tokens.Type) {
	for _, r := range ext.resources {
		rs = append(rs, r.Tok)
	}
	sort.SliceStable(rs, func(i, j int) bool {
		return string(rs[i]) < string(rs[j])
	})
	return
}

type simpleSchemaProvider struct {
	schemashim.SchemaOnlyProvider
	schema      shim.SchemaMap
	resources   shim.ResourceMap
	dataSources shim.ResourceMap
}

func (p *simpleSchemaProvider) Schema() shim.SchemaMap {
	return p.schema
}

func (p *simpleSchemaProvider) ResourcesMap() shim.ResourceMap {
	return p.resources
}

func (p *simpleSchemaProvider) DataSourcesMap() shim.ResourceMap {
	return p.dataSources
}

var _ shim.Provider = (*simpleSchemaProvider)(nil)

func disjointUnion(baseline, extension shim.ResourceMap) (shim.ResourceMap, error) {
	union, err := disjointMapUnion(toResourceMap(baseline), toResourceMap(extension))
	if err != nil {
		return nil, err
	}
	return shimSchema.ResourceMap(union), nil
}

func toResourceMap(rmap shim.ResourceMap) shimSchema.ResourceMap {
	m := map[string]shim.Resource{}
	rmap.Range(func(key string, value shim.Resource) bool {
		m[key] = value
		return true
	})
	return m
}

func disjointMapUnion[T any](baseline, extension map[string]T) (map[string]T, error) {
	u := make(map[string]T)

	conflictingKeys := map[string]bool{}

	for k, v := range baseline {
		if _, conflict := extension[k]; conflict {
			conflictingKeys[k] = true
		} else {
			u[k] = v
		}
	}

	for k, v := range extension {
		if _, conflict := baseline[k]; conflict {
			conflictingKeys[k] = true
		} else {
			u[k] = v
		}
	}

	if len(conflictingKeys) > 0 {
		var conflicts []string
		for k := range conflictingKeys {
			conflicts = append(conflicts, k)
		}
		sort.Strings(conflicts)
		return nil, fmt.Errorf("conflicting keys: %s", strings.Join(conflicts, ", "))
	}

	return u, nil
}
