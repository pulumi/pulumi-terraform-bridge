// Copyright 2016-2022, Pulumi Corporation.
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
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tf2pulumi/il"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
)

var _ = (plugin.Provider)((*inmemoryProvider)(nil))

type inmemoryProvider struct {
	plugin.UnimplementedProvider

	name   tokens.Package
	schema []byte
	info   tfbridge.ProviderInfo
}

func newInMemoryProvider(name tokens.Package, schema []byte, info tfbridge.ProviderInfo) *inmemoryProvider {
	// Round-trip the info through a marshaler to normalize the types to the schema shim.
	return &inmemoryProvider{
		name:   name,
		schema: schema,
		info:   *tfbridge.MarshalProviderInfo(&info).Unmarshal(),
	}
}

func (p *inmemoryProvider) Pkg() tokens.Package {
	return p.name
}

func (p *inmemoryProvider) GetSchema(context.Context, plugin.GetSchemaRequest) (plugin.GetSchemaResponse, error) {
	return plugin.GetSchemaResponse{Schema: p.schema}, nil
}

func (p *inmemoryProvider) GetMapping(
	_ context.Context, req plugin.GetMappingRequest,
) (plugin.GetMappingResponse, error) {
	if req.Key == "tf" || req.Key == "terraform" {
		if req.Provider != "" && req.Provider != p.info.Name {
			return plugin.GetMappingResponse{}, fmt.Errorf("unknown provider %q", req.Provider)
		}
		info := tfbridge.MarshalProviderInfo(&p.info)
		mapping, err := json.Marshal(info)
		if err != nil {
			return plugin.GetMappingResponse{}, err
		}
		return plugin.GetMappingResponse{
			Data:     mapping,
			Provider: p.info.Name,
		}, nil
	}
	return plugin.GetMappingResponse{}, nil
}

func (p *inmemoryProvider) GetMappings(
	_ context.Context, req plugin.GetMappingsRequest,
) (plugin.GetMappingsResponse, error) {
	if req.Key == "tf" || req.Key == "terraform" {
		return plugin.GetMappingsResponse{
			Keys: []string{p.info.Name},
		}, nil
	}
	return plugin.GetMappingsResponse{}, nil
}

func (p *inmemoryProvider) GetPluginInfo(context.Context) (workspace.PluginInfo, error) {
	var version *semver.Version
	if p.info.Version != "" {
		v, err := semver.ParseTolerant(p.info.Version)
		if err != nil {
			return workspace.PluginInfo{}, fmt.Errorf("failed to parse pkg %q version: %w", p.name, err)
		}
		version = &v
	}
	return workspace.PluginInfo{
		Name:    p.info.Name,
		Kind:    apitype.ResourcePlugin,
		Version: version,
	}, nil
}

func (p *inmemoryProvider) Close() error {
	return nil
}

func (p *inmemoryProvider) SignalCancellation(context.Context) error {
	return nil
}

type inmemoryProviderHost struct {
	plugin.Host
	il.ProviderInfoSource

	provider *inmemoryProvider
}

func (host *inmemoryProviderHost) Provider(pkg workspace.PackageDescriptor) (plugin.Provider, error) {
	if tokens.Package(pkg.Name) == host.provider.Pkg() {
		return host.provider, nil
	}
	return host.Host.Provider(pkg)
}

// ResolvePlugin resolves a plugin kind, name, and optional semver to a candidate plugin
// to load. inmemoryProviderHost does this by checking if the generating provider is being
// loaded. If it is, it returns it's provider. Otherwise, we defer
// inmemoryProviderHost.Host.
func (host *inmemoryProviderHost) ResolvePlugin(spec workspace.PluginSpec) (*workspace.PluginInfo, error) {
	if spec.Name == host.provider.name.String() {
		info, err := host.provider.GetPluginInfo(context.TODO())
		if err != nil {
			return nil, err
		}
		return &info, nil
	}
	return host.Host.ResolvePlugin(spec)
}

func (host *inmemoryProviderHost) GetProviderInfo(
	registryName, namespace, name, version string,
) (*tfbridge.ProviderInfo, error) {
	if name == il.GetTerraformProviderName(host.provider.info) {
		return &host.provider.info, nil
	}
	return host.ProviderInfoSource.GetProviderInfo(registryName, namespace, name, version)
}

type cachingProviderHost struct {
	plugin.Host

	m     sync.RWMutex
	cache map[string]plugin.Provider
}

func newCachingProviderHost(host plugin.Host) plugin.Host {
	return &cachingProviderHost{
		Host:  host,
		cache: map[string]plugin.Provider{},
	}
}

func (host *cachingProviderHost) getProvider(key string) (plugin.Provider, bool) {
	host.m.RLock()
	defer host.m.RUnlock()

	provider, ok := host.cache[key]
	return provider, ok
}

func (host *cachingProviderHost) Provider(pkg workspace.PackageDescriptor) (plugin.Provider, error) {
	key := pkg.String()
	if provider, ok := host.getProvider(key); ok {
		return provider, nil
	}

	host.m.Lock()
	defer host.m.Unlock()

	provider, err := host.Host.Provider(pkg)
	if err != nil {
		return nil, err
	}
	host.cache[key] = provider
	return provider, nil
}
