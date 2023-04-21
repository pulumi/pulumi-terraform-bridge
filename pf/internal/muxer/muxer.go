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
	"fmt"
	"sort"
	"strings"

	"context"

	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/pulumi/pulumi-terraform-bridge/pf/internal/schemashim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	shimSchema "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/schema"
	"github.com/pulumi/pulumi-terraform-bridge/x/muxer"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

func SchemaOnlyPluginFrameworkProvider(ctx context.Context, provider provider.Provider) shim.Provider {
	return schemashim.ShimSchemaOnlyProvider(ctx, provider)
}

// AugmentShimWithPF augments an existing shim with a PF provider.Provider.
func AugmentShimWithPF(ctx context.Context, shim shim.Provider, pf provider.Provider) *ProviderShim {

	var p ProviderShim
	if alreadyMerged, ok := shim.(*ProviderShim); ok {
		p = *alreadyMerged
	} else {
		p = newProviderShim(shim)
	}

	err := p.extend(schemashim.ShimSchemaOnlyProvider(ctx, pf))
	contract.AssertNoErrorf(err, "shim and pf provider are not disjoint")
	return &p
}

// A merged `shim.Provider` that remembers which `shim.Provider`s it is composed of.
type ProviderShim struct {
	simpleSchemaProvider

	MuxedProviders []shim.Provider
}

// Check if a Resource is served via in the Plugin Framework.
func (m *ProviderShim) ResourceIsPF(token string) bool {
	// In an augmented shim.Provider, underlying providers are PF providers iff they
	// are implemented as SchemaOnlyProviders.
	for _, p := range m.MuxedProviders {
		if _, ok := p.ResourcesMap().GetOk(token); !ok {
			continue
		}
		_, ok := p.(*schemashim.SchemaOnlyProvider)
		return ok

	}
	return false
}

// Check if a DataSource is served via the Plugin Framework.
func (m *ProviderShim) DataSourceIsPF(token string) bool {
	// In an augmented shim.Provider, underlying providers are PF providers iff they
	// are implemented as SchemaOnlyProviders.
	for _, p := range m.MuxedProviders {
		if _, ok := p.DataSourcesMap().GetOk(token); !ok {
			continue
		}
		_, ok := p.(*schemashim.SchemaOnlyProvider)
		return ok

	}
	return false
}

// Extend the `ProviderShim` with another `shim.Provider`.
//
// `provider` will be the `len(m.MuxedProviders)` when mappings are computed.
func (m *ProviderShim) extend(provider shim.Provider) error {
	res, err := disjointUnion(m.resources, provider.ResourcesMap())
	if err != nil {
		return fmt.Errorf("ResourcesMap is not disjoint: %w", err)
	}

	data, err := disjointUnion(m.dataSources, provider.DataSourcesMap())
	if err != nil {
		return fmt.Errorf("DataSourcesMap is not disjoint: %w", err)
	}

	m.resources = res
	m.dataSources = data
	m.MuxedProviders = append(m.MuxedProviders, provider)
	return nil
}

func newProviderShim(provider shim.Provider) ProviderShim {
	return ProviderShim{
		simpleSchemaProvider: simpleSchemaProvider{
			schema:      provider.Schema(),
			resources:   provider.ResourcesMap(),
			dataSources: provider.DataSourcesMap(),
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

	unbackedResources := resolveDispatchMap(m, dispatch.Resources, info.Resources,
		func(p shim.Provider) shim.ResourceMap { return p.ResourcesMap() })
	unbackedDatasources := resolveDispatchMap(m, dispatch.Functions, info.DataSources,
		func(p shim.Provider) shim.ResourceMap { return p.DataSourcesMap() })
	joinErr := func(label string, tks []string) error {
		return fmt.Errorf("%s without backing provider:\n- %s",
			label, strings.Join(tks, "\n- "))
	}

	switch {
	case len(unbackedResources) == 0 && len(unbackedDatasources) == 0:
		return dispatch, nil
	case len(unbackedResources) == 0:
		return dispatch, joinErr("DataSources", unbackedDatasources)
	case len(unbackedDatasources) == 0:
		return dispatch, joinErr("Resources", unbackedResources)
	default:
		return dispatch, multierror.Append(
			joinErr("Resources", unbackedResources),
			joinErr("DataSources", unbackedDatasources),
		)
	}
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
	u := copyMap(baseline)

	var conflictingKeys []string

	for k, v := range extension {
		if _, conflict := baseline[k]; conflict {
			conflictingKeys = append(conflictingKeys, k)
			continue
		}
		u[k] = v
	}

	if len(conflictingKeys) > 0 {
		sort.Strings(conflictingKeys)
		return nil, fmt.Errorf("conflicting keys: %s", strings.Join(conflictingKeys, ", "))
	}

	return u, nil
}

func copyMap[K comparable, V any](m map[K]V) map[K]V {
	if m == nil {
		return nil
	}
	out := make(map[K]V, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
