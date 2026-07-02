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

package muxer

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/internal/schemashim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	shimschema "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/schema"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/x/muxer"
)

func SchemaOnlyPluginFrameworkProvider(ctx context.Context, provider provider.Provider) shim.Provider {
	return schemashim.ShimSchemaOnlyProvider(ctx, provider)
}

// AugmentShimWithPF augments an existing shim with a PF provider.Provider.
//
// If there is overlap between shim and pf, shim will dominate.
func AugmentShimWithPF(ctx context.Context, shim shim.Provider, pf provider.Provider) *ProviderShim {
	p, _, _, functions := augmentShimWithPF(ctx, shim, pf)
	// Unlike resources and data sources, conflicting provider function names cannot be
	// resolved by letting one provider dominate: Terraform function names are global to
	// a provider, so a collision is always a configuration error.
	if len(functions) > 0 {
		contract.Failf("Provider functions are not disjoint: conflicting names: %s", strings.Join(functions, ", "))
	}
	return p
}

func augmentShimWithPF(
	ctx context.Context, shim shim.Provider, pf provider.Provider,
) (*ProviderShim, []string, []string, []string) {
	var p ProviderShim
	if alreadyMerged, ok := shim.(*ProviderShim); ok {
		p = *alreadyMerged
	} else {
		p = newProviderShim(shim)
	}

	r, d, f := p.extend(schemashim.ShimSchemaOnlyProvider(ctx, pf))
	return &p, r, d, f
}

// AugmentShimWithDisjointPF augments an existing shim with a PF provider.Provider.
//
// This function asserts that there is no overlap between providers.
func AugmentShimWithDisjointPF(ctx context.Context, shim shim.Provider, pf provider.Provider) *ProviderShim {
	p, resources, datasources, functions := augmentShimWithPF(ctx, shim, pf)

	var errs []string
	if len(resources) > 0 {
		errs = append(errs,
			fmt.Sprintf("ResourcesMap is not disjoint: conflicting keys: %s", strings.Join(resources, ", ")))
	}
	if len(datasources) > 0 {
		errs = append(errs,
			fmt.Sprintf("DataSourceMap is not disjoint: conflicting keys: %s", strings.Join(datasources, ", ")))
	}
	if len(functions) > 0 {
		errs = append(errs, fmt.Sprintf("Functions are not disjoint: conflicting names: %s", strings.Join(functions, ", ")))
	}
	if len(errs) > 0 {
		contract.Failf("Providers are not disjoint:\n- %s", strings.Join(errs, "\n- "))
	}

	return p
}

// A merged `shim.Provider` that remembers which `shim.Provider`s it is composed of.
type ProviderShim struct {
	simpleSchemaProvider

	MuxedProviders []shim.Provider
}

type listResourceMapProvider interface {
	ListResourcesMap() shim.ResourceMap
}

// ListResourcesMap is not part of shim.Provider: plain SDKv2 providers do not
// expose Terraform protocol list resources, while PF schema-only providers may.
// Treat providers without this optional method as having no list resources.
func listResourcesMap(provider shim.Provider) shim.ResourceMap {
	if provider, ok := provider.(listResourceMapProvider); ok {
		return provider.ListResourcesMap()
	}
	return shimschema.ResourceMap{}
}

// Check if a Resource is served via in the Plugin Framework.
func (m *ProviderShim) ResourceIsPF(token string) bool {
	// In an augmented shim.Provider, underlying providers are PF providers iff they
	// are implemented as SchemaOnlyProviders.
	for _, p := range m.MuxedProviders {
		if _, ok := p.ResourcesMap().GetOk(token); !ok {
			continue
		}
		_, ok := p.(pf.ShimProvider)
		return ok
	}
	return false
}

// ResourceSchemaFixupsMayBePrecomputed reports whether token is PF-owned and
// therefore eligible to consume build-time default-fixup metadata at runtime.
// The caller still requires runtime metadata with an entry for the resource
// before it skips live schema inspection.
func (m *ProviderShim) ResourceSchemaFixupsMayBePrecomputed(token string) bool {
	return m.ResourceIsPF(token)
}

// Check if a DataSource is served via the Plugin Framework.
func (m *ProviderShim) DataSourceIsPF(token string) bool {
	// In an augmented shim.Provider, underlying providers are PF providers iff they
	// are implemented as SchemaOnlyProviders.
	for _, p := range m.MuxedProviders {
		if _, ok := p.DataSourcesMap().GetOk(token); !ok {
			continue
		}
		_, ok := p.(pf.ShimProvider)
		return ok
	}
	return false
}

// Extend the `ProviderShim` with another `shim.Provider`.
//
// `provider` will be the `len(m.MuxedProviders)` when mappings are computed.
func (m *ProviderShim) extend(provider shim.Provider) ([]string, []string, []string) {
	res := newUnionMap(m.resources, provider.ResourcesMap())
	conflictingResources := res.ConflictingKeys()
	data := newUnionMap(m.dataSources, provider.DataSourcesMap())
	conflictingDataSources := data.ConflictingKeys()
	listResources := newUnionMap(m.listResources, listResourcesMap(provider))
	functions, conflictingFunctions := unionFunctions(m.functions, provider.Functions())
	m.resources = res
	m.dataSources = data
	m.listResources = listResources
	m.functions = functions
	m.MuxedProviders = append(m.MuxedProviders, provider)
	return conflictingResources, conflictingDataSources, conflictingFunctions
}

// Union two function maps, preferring baseline on conflict.
func unionFunctions(baseline, extension map[string]shim.Function) (map[string]shim.Function, []string) {
	if len(extension) == 0 {
		return baseline, nil
	}
	union := make(map[string]shim.Function, len(baseline)+len(extension))
	for name, fn := range baseline {
		union[name] = fn
	}
	var conflicts []string
	for name, fn := range extension {
		if _, ok := union[name]; ok {
			conflicts = append(conflicts, name)
			continue
		}
		union[name] = fn
	}
	sort.Strings(conflicts)
	return union, conflicts
}

func newProviderShim(provider shim.Provider) ProviderShim {
	return ProviderShim{
		simpleSchemaProvider: simpleSchemaProvider{
			schema:        provider.Schema(),
			resources:     provider.ResourcesMap(),
			dataSources:   provider.DataSourcesMap(),
			listResources: listResourcesMap(provider),
			functions:     provider.Functions(),
		},
		MuxedProviders: []shim.Provider{provider},
	}
}

// Assign each Resource and DataSource mapped in `info` whichever runtime provider defines
// it.
//
// For alias based mappings to work correctly at runtime, it is necessary to call
// `ResolveDispatch` before running the provider.
func (m *ProviderShim) ResolveDispatch(info *tfbridge.ProviderInfo) (muxer.DispatchTable, error) {
	var dispatch muxer.DispatchTable
	dispatch.Resources = map[string]int{}
	dispatch.Functions = map[string]int{}
	dispatch.ListResources = map[string]int{}

	unbackedResources := resolveDispatchMap(m, dispatch.Resources, info.Resources,
		func(p shim.Provider) shim.ResourceMap { return p.ResourcesMap() })
	unbackedDatasources := resolveDispatchMap(m, dispatch.Functions, info.DataSources,
		func(p shim.Provider) shim.ResourceMap { return p.DataSourcesMap() })
	// Provider-defined functions share the Pulumi schema's functions section (and the
	// Invoke RPC) with data sources, so they dispatch through the same table.
	unbackedFunctions := resolveFunctionDispatch(m, dispatch.Functions, info.Functions)
	// List ownership is intentionally separate from CRUD ownership. Muxed providers
	// can expose a Terraform list resource from a PF sidecar for an SDKv2 CRUD resource.
	unbackedListResources := resolveDispatchMap(m, dispatch.ListResources, info.Resources, listResourcesMap)
	joinErr := func(label string, tks []string) error {
		return fmt.Errorf("%s without backing provider:\n- %s",
			label, strings.Join(tks, "\n- "))
	}

	var errs multierror.Error
	if len(unbackedResources) > 0 {
		errs.Errors = append(errs.Errors, joinErr("Resources", unbackedResources))
	}
	if len(unbackedDatasources) > 0 {
		errs.Errors = append(errs.Errors, joinErr("DataSources", unbackedDatasources))
	}
	if len(unbackedFunctions) > 0 {
		errs.Errors = append(errs.Errors, joinErr("Functions", unbackedFunctions))
	}
	if len(unbackedListResources) > 0 {
		errs.Errors = append(errs.Errors, joinErr("ListResources", unbackedListResources))
	}
	if len(errs.Errors) > 0 {
		return dispatch, &errs
	}
	return dispatch, nil
}

// Resolve provider-defined functions into their originating providers.
func resolveFunctionDispatch(
	m *ProviderShim, dispatch map[string]int, info map[string]*tfbridge.FunctionInfo,
) (unbacked []string) {
	for tfName, fn := range info {
		if _, ok := m.Functions()[tfName]; !ok {
			// This function is not in any sub-provider. The bridge will error
			// later, so we can safely ignore now.
			continue
		}
		var found bool
		for i, p := range m.MuxedProviders {
			if _, ok := p.Functions()[tfName]; ok {
				dispatch[string(fn.Tok)] = i
				found = true
				break
			}
		}
		if !found {
			unbacked = append(unbacked, tfName)
		}
	}
	return unbacked
}

// Resolve either resources or datasoruces into their originating providers.
//
// A resource/datasource is considered "from" a provider if the provider serves a
// resource/datasource for the token or if the provider serves a resource that matches the
// resource inserted into the global map of m. The latter logic is used to handle aliases
// which are inserted via:
//
//	p.P.ResourcesMap().Set(legacyResourceName, p.P.ResourcesMap().Get(resourceName))
func resolveDispatchMap[T interface{ GetTok() tokens.Token }](
	m *ProviderShim, dispatch map[string]int, info map[string]T,
	mapKind func(shim.Provider) shim.ResourceMap,
) (unbacked []string) {
	var reverseLookupMap map[shim.Resource]int
	for tfToken, res := range info {
		r, ok := mapKind(m).GetOk(tfToken)
		if !ok {
			// This resource/datasource is not in any sub-provider nor was it
			// set after the fact. The bridge will error later, so we can
			// safely ignore now.
			continue
		}

		var found bool
		for i, p := range m.MuxedProviders {
			_, ok := mapKind(p).GetOk(tfToken)
			if ok {
				dispatch[string(res.GetTok())] = i
				found = true
				break
			}
		}
		if !found {
			if reverseLookupMap == nil {
				reverseLookupMap = map[shim.Resource]int{}
				for i, p := range m.MuxedProviders {
					mapKind(p).Range(func(_ string, r shim.Resource) bool {
						reverseLookupMap[r] = i
						return true
					})
				}
			}
			i, ok := reverseLookupMap[r]
			if ok {
				dispatch[string(res.GetTok())] = i
				mapKind(m.MuxedProviders[i]).Set(tfToken, r)
				found = true
			}
		}

		if !found {
			unbacked = append(unbacked, tfToken)
		}
	}
	return unbacked
}

type simpleSchemaProvider struct {
	schemashim.SchemaOnlyProvider
	schema        shim.SchemaMap
	resources     shim.ResourceMap
	dataSources   shim.ResourceMap
	listResources shim.ResourceMap
	functions     map[string]shim.Function
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

func (p *simpleSchemaProvider) ListResourcesMap() shim.ResourceMap {
	return p.listResources
}

func (p *simpleSchemaProvider) Functions() map[string]shim.Function {
	return p.functions
}

var _ shim.Provider = (*simpleSchemaProvider)(nil)
