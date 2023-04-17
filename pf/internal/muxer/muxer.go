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

	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/pulumi/pulumi-terraform-bridge/pf/internal/schemashim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	shimSchema "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/schema"
	"github.com/pulumi/pulumi-terraform-bridge/x/muxer"
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
		p = newMergedProviderShim(shim)
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

// Check if a resource is from the PF.
func (m *ProviderShim) ResourceIsPF(resname string) bool {
	// In an augmented shim.Provider, underlying providers are PF providers iff they
	// are implemented as SchemaOnlyProviders.
	for _, p := range m.MuxedProviders {
		if _, ok := p.ResourcesMap().GetOk(resname); !ok {
			continue
		}
		_, ok := p.(*schemashim.SchemaOnlyProvider)
		return ok
	}
	return false
}

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

func newMergedProviderShim(provider shim.Provider) ProviderShim {
	return ProviderShim{
		simpleSchemaProvider: simpleSchemaProvider{
			schema:      provider.Schema(),
			resources:   provider.ResourcesMap(),
			dataSources: provider.DataSourcesMap(),
		},
		MuxedProviders: []shim.Provider{provider},
	}
}

func (m *ProviderShim) ResolveDispatch(info *tfbridge.ProviderInfo) muxer.DispatchTable {
	var dispatch muxer.DispatchTable
	dispatch.Resources = map[string]int{}
	dispatch.Functions = map[string]int{}

	for tfToken, res := range info.Resources {
		for i, p := range m.MuxedProviders {
			_, ok := p.ResourcesMap().GetOk(tfToken)
			if ok {
				dispatch.Resources[string(res.GetTok())] = i
				break
			}
		}
	}
	for tfToken, data := range info.DataSources {
		for i, p := range m.MuxedProviders {
			_, ok := p.DataSourcesMap().GetOk(tfToken)
			if ok {
				dispatch.Functions[string(data.GetTok())] = i
				break
			}
		}
	}

	return dispatch
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
